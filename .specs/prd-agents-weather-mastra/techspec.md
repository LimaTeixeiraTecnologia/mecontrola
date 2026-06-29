<!-- spec-hash-prd: 031b016adff4b7973adbe240a2448e5998e92ca99a8118c6d35effaf3bf92cf0 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — `internal/agents` (weather, paridade Mastra) sobre `internal/platform`

## Resumo Executivo

Construir o módulo de domínio `internal/agents` como **port fiel do exemplo weather do Mastra** em Go, **consumindo** o substrato `internal/platform` (agent, memory, scorer, tool, workflow/kernel, llm) sem reimplementar mecanismo, **persistido em Postgres** (tabelas `platform_*` da migration `000003`) e **wired ao WhatsApp** como caminho primário de entrada. O módulo é montado por DI manual (sem `init()`, sem estado global) num `module.go` que instancia: provider OpenRouter, tool `get-weather`, scorers (code-based + LLM-judged), memória (thread/message/working/semantic recall) e `AgentRuntime`, registrando o agente `weather-agent` e o workflow `weather-workflow`. A entrada WhatsApp (texto livre) é mapeada para `AgentRuntime.Execute(InboundRequest)`; a resposta é enviada pelo gateway WhatsApp existente. O `weather-workflow` (city→activities, com agent-como-step via streaming) é registrado no kernel e exercitado pela suite de conformidade; pode ser acionado sob demanda.

Duas decisões materiais sustentam a entrega: (1) **conectar a indexação assíncrona de embeddings** (gap B3) via um **decorator publicador de `MessageStore`** em `internal/platform/memory` que emite evento de outbox após `Append`, consumido por um worker idempotente (por `event_id`) que gera embedding (OpenRouter) e grava `platform_embeddings` — fora do caminho crítico; (2) **eliminar fisicamente `internal/agent`** e **desligar o onboarding conversacional do WhatsApp**, simplificando o dispatcher para uma rota única → `internal/agents`, removendo bindings e e2e dependentes. O endurecimento de governança (go-implementation R0–R7, zero comentários, testify/suite whitebox, DTOs `Validate()`, DMMF state-as-type, cardinalidade de métricas, gofmt e gates de import) é gate de pronto, com `grep internal/agent` (≠ `internal/platform/agent`) retornando vazio.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Layering (setas = "depende de"; nunca o inverso):

```
WhatsApp inbound (internal/platform/whatsapp: handler → signature → dispatcher)
        │  rota única (agentRoute)
        ▼
internal/agents (CONSUMIDOR de domínio weather)
   module.go ─ monta e injeta tudo
   ├─ agent.go      weather-agent (instructions, model, tools, memory) ── platform/agent
   ├─ tool.go       get-weather (open-meteo) ───────────────────────────  platform/tool
   ├─ workflow.go   weather-workflow (fetch → plan, agent-como-step) ────  platform/workflow (kernel)
   ├─ scorers.go    tool-call-accuracy, completeness, translation ───────  platform/scorer
   ├─ application/usecases  HandleInbound (Thread→Run via AgentRuntime) ─  platform/agent + memory
   ├─ infrastructure/weather  open-meteo client (IO externo)
   └─ infrastructure/messaging/database/consumers  WhatsApp inbound consumer (adapter fino)
        │
        ▼ (consome, nunca o contrário)
internal/platform/{agent, memory, scorer, tool, llm, workflow}  ── Postgres (platform_*, workflow_*)
```

Componentes novos (em `internal/agents/`):

- **`module.go`** — construtor `NewModule(deps) (Module, error)`; DI de provider/tool/agent/workflow/scorers/memory/runtime; expõe a rota WhatsApp, o consumer e os jobs para `cmd/*`.
- **`agent.go`** — `buildWeatherAgent(provider, tool, o11y) agent.Agent` com instructions equivalentes ao Mastra; registrado no `AgentRegistry`.
- **`tool.go`** — `buildWeatherTool(weatherClient) tool.ToolHandle` via `tool.NewTool[WeatherInput, WeatherOutput]`.
- **`workflow.go`** — `buildWeatherWorkflow(agentRef) workflow.Definition[WeatherState]` (steps `fetch-weather` → `plan-activities`).
- **`scorers.go`** — `buildWeatherScorers(provider) []scorer.ScorerEntry`.
- **`domain/`** — value objects `Forecast`, `WeatherConditions`, mapeamento `weatherCode→condição` (closed type), e quaisquer `Decide*` puros de formatação determinística.
- **`application/dtos/input/`** — DTOs com `Validate()`.
- **`application/usecases/handle_inbound.go`** — orquestra `AgentRuntime.Execute`.
- **`infrastructure/weather/`** — cliente open-meteo (geocoding + forecast) via `httpclient`.
- **`infrastructure/messaging/database/consumers/`** — consumer do evento inbound WhatsApp (adapter fino → usecase).

Componentes alterados em `internal/platform/memory` (genérico, resolve B3):

- **`publishingMessageStore`** (decorator de `MessageStore`) — após `Append` bem-sucedido, publica evento de indexação no outbox.
- Registro do **`EmbeddingIndexHandler`** como consumer/worker (idempotente por `event_id`).

Removidos: **`internal/agent/**` inteiro**; wiring em `cmd/server` e `cmd/worker`; rota de onboarding conversacional/ativação no dispatcher; e2e de onboarding dependentes de `internal/agent`.

### Fluxo de dados (WhatsApp → resposta)

1. Inbound HTTP → verificação de assinatura → `dispatcher.Route` (dedup, principal, rate limit) → callback `agentRoute(ctx, msg)`.
2. `agentRoute` publica evento outbox inbound (`agents.whatsapp.inbound.v1`, payload `{user_id, peer, text, message_id}`).
3. Worker: consumer de `internal/agents` decodifica e chama `HandleInbound` → `AgentRuntime.Execute(InboundRequest{ResourceID: peer/user, ThreadID: derivado, AgentID: "weather-agent", Message: text, MessageID})`.
4. `AgentRuntime`: resolve Thread (`platform_threads`), abre Run (`platform_runs`, status `running`), injeta RuntimeContext, executa o `weather-agent` (que chama a tool `get-weather`), persiste mensagens (`platform_messages`) via `MessageStore` decorado → publica evento de indexação.
5. Resposta enviada via `whatsAppGateway.SendTextMessage(toE164, reply)`.
6. Run fechado (`succeeded|failed`, duração). `ScorerRunner` amostra e persiste `platform_scorer_results` fora do caminho crítico.
7. Worker de indexação consome o evento, gera embedding (OpenRouter `Embed`), grava `platform_embeddings` (idempotente).

## Design de Implementação

### Interfaces Chave

Tool (input/output tipados; paridade get-weather):

```go
type WeatherInput struct {
    Location string `json:"location"`
}
type WeatherOutput struct {
    Temperature float64 `json:"temperature"`
    FeelsLike   float64 `json:"feelsLike"`
    Humidity    float64 `json:"humidity"`
    WindSpeed   float64 `json:"windSpeed"`
    WindGust    float64 `json:"windGust"`
    Conditions  string  `json:"conditions"`
    Location    string  `json:"location"`
}
```

Estado do workflow (S tipado; preservado entre steps):

```go
type WeatherState struct {
    City      string
    Forecast  Forecast
    Activities string
}
```

Decorator publicador de mensagens (resolve B3, em `internal/platform/memory`):

```go
type MessageIndexPublisher interface {
    PublishIndex(ctx context.Context, p IndexMessagePayload) error
}

func NewPublishingMessageStore(next MessageStore, pub MessageIndexPublisher, model string) MessageStore
```

Cliente open-meteo (IO externo do consumidor):

```go
type WeatherClient interface {
    Geocode(ctx context.Context, name string) (lat, lon float64, resolved string, err error)
    Forecast(ctx context.Context, lat, lon float64) (Forecast, error)
}
```

Módulo (wiring para `cmd/*`):

```go
type Module struct {
    WhatsAppAgentRoute func(ctx context.Context, msg payload.Message) dispatcher.RouteOutcome
    EventHandlers      []events.Registration
    Jobs               []worker.Job
}

func NewModule(deps Deps) (Module, error)
```

### Modelos de Dados

Reuso integral de `platform_*` (migration `000003`, sem nova migration salvo justificativa em ADR):

- `platform_threads` / `platform_messages` / `platform_resources` — thread, turns, working memory (chaves opacas `resource_id`/`thread_id`).
- `platform_runs` — Run auditável (status fechado, `outcome`, duração).
- `platform_embeddings` — vetor 1536 (HNSW, UNIQUE parcial `(source_message_pk, model)` + `ON CONFLICT DO NOTHING` já presentes).
- `platform_scorer_results` — resultados de scorer vinculados ao Run.
- `workflow_runs` / `workflow_steps` — snapshots do kernel (durável p/ o weather-workflow).

Tipos fechados (DMMF state-as-type) no domínio: mapeamento `weatherCode` como enum/constante; `Forecast` como value object com smart constructor; reuso de `agent.RunStatus`/`ToolOutcome`, `scorer.ScorerKind`/`SamplingType`, `workflow.RunStatus/StepStatus`.

### Endpoints de API

Não há endpoint público novo: a entrada é o webhook WhatsApp já existente (`internal/platform/whatsapp/handlers`). O módulo expõe apenas a callback `WhatsAppAgentRoute` (adapter fino que publica o inbound) e o consumer no worker. O `weather-workflow` é biblioteca interna acionada via `Engine[S].Start`.

## Pontos de Integração

- **OpenRouter** (`internal/platform/llm`): `Complete`/`Stream` para o agente e o judge scorer; `Embed` (`openai/text-embedding-3-small`, 1536) para indexação. Auth `Bearer`; timeouts/circuit breaker/fallback já na plataforma.
- **open-meteo** (consumidor): geocoding `https://geocoding-api.open-meteo.com/v1/search?name=...&count=1` + forecast `https://api.open-meteo.com/v1/forecast?...`; via `httpclient` com timeout; erro explícito quando a cidade não é encontrada (`ErrLocationNotFound`) ou upstream falha.
- **WhatsApp (Meta)**: inbound pelo handler/dispatcher existentes; outbound via gateway existente (`SendTextMessage(toE164, text)`).
- **Outbox/Worker** (`internal/platform/outbox` + `internal/platform/worker`): evento de indexação de embeddings; idempotência por `event_id`; shutdown cooperativo.

## Abordagem de Testes

### Testes Unitários

- `tool.go`/`infrastructure/weather`: tabela de cenários; open-meteo via `httptest` (sucesso, cidade inexistente, upstream 5xx, mapeamento de `weather_code`).
- `domain`: `Forecast` smart constructor, mapeamento de condições, `IsValid()`/parse dos tipos fechados (sem mocks).
- `application/usecases` (HandleInbound): testify/suite whitebox (R-TESTING-001), `fake.NewProvider()`, `dependencies` struct + IIFE por mock; mock só de portas (AgentRuntime/registry/gateway).
- `scorers.go`: code-based determinístico; translation judge com provider fake (structured output válido/ inválido → falha explícita).
- `publishingMessageStore` (platform/memory): verifica publicação de evento após `Append` e propagação de erro.

### Testes de Integração

Adotados (≥2 critérios: IO crítico Postgres+pgvector; risco real de divergência SQL/recall; custo proporcional). Build tag `//go:build integration`, `testcontainers-go` `pgvector/pgvector:pg16`, migrations `000001..000003` aplicadas.

- Memória: Thread/Message/WorkingMemory round-trip; **semantic recall real** (Index→Recall ANN) provando que a indexação assíncrona popula `platform_embeddings` e o recall retorna itens escopados por `resource_id`.
- Run auditável: `platform_runs` com status/duração; `platform_scorer_results` vinculados.
- Indexação assíncrona: consumer idempotente (replay de `event_id` não duplica).

### Testes E2E

- **Conformidade weather** promovida de `test/conformance/weather` para exercitar `internal/agents` real: agent sync+stream, tool I/O, weather-workflow (agent-como-step, fim-de-stream/structured output), memória, scorers, runtime context, suspend/resume.
- **WhatsApp E2E**: simular inbound (texto "clima em <cidade>?") → asserir resposta e persistência (thread/message/run/embedding/scorer_results). Variante real atrás de `RUN_REAL_LLM=1` (OpenRouter + Postgres reais), fora do gate de merge; CI padrão determinístico (provider fake + testcontainers).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Indexação assíncrona (platform/memory)** — `publishingMessageStore` + evento + handler/worker idempotente (resolve B3). Base para a memória do agente funcionar de verdade.
2. **`internal/agents` domínio + tool + cliente open-meteo** — value objects, `get-weather`, cliente HTTP.
3. **agent + scorers + workflow** — `weather-agent`, scorers, `weather-workflow` (agent-como-step), promovendo `test/conformance/weather`.
4. **module.go + usecase HandleInbound** — DI completo sobre a plataforma; `AgentRuntime` com `MessageStore` decorado.
5. **Canal WhatsApp** — rota inbound (publica outbox) + consumer no worker + resposta via gateway.
6. **Cutover/eliminação** — apagar `internal/agent/**`; religar `cmd/server`/`cmd/worker`; desligar onboarding conversacional do dispatcher; ajustar/remover e2e; migrar config `AGENT_*`→config do módulo.
7. **Conformidade + gates** — suite determinística + variante `RUN_REAL_LLM`; gates de governança e gofmt; `grep internal/agent` vazio.

### Dependências Técnicas

- Postgres com pgvector (prod + testcontainers); `OPENROUTER_API_KEY` para variante real.
- `internal/platform` funcional (entregue em `prd-platform-mastra`), exceto o gap B3 (passo 1).
- Gateway WhatsApp existente reutilizável.

## Monitoramento e Observabilidade

Reutiliza a stack existente (devkit-go `observability` → OTLP → otel-lgtm), sem trilha paralela. Cardinalidade controlada (labels enums fechados): `agent_id`, `channel`, `workflow`, `status`, `tool`, `outcome`, `scorer_id`, `kind`. Proibido `resource_id`/`thread_id`/`correlation_key` como label. Spans reutilizados da plataforma (`agent.runtime.execute`, `agent.execute`, `agent.stream`, `llm.*`, `memory.recall`, `scorer.score`, `workflow.*`). Métricas novas do módulo limitam-se a labels enums (ex.: contador de inbound por `channel`/`outcome`).

## Considerações Técnicas

### Decisões Chave

- **ADR-001** (`adr-001-layering-agents-consumidor.md`) — `internal/agents` é consumidor de domínio sobre `internal/platform`; proibido importar `internal/agent`; weather como consumidor de referência vivo.
- **ADR-002** (`adr-002-indexacao-assincrona-embeddings.md`) — indexação assíncrona via decorator publicador de `MessageStore` + outbox→worker idempotente (resolve B3).
- **ADR-003** (`adr-003-canal-whatsapp-agentruntime.md`) — mapeamento WhatsApp→`AgentRuntime` (texto livre = `Message`; `peer/user`→`resourceId/threadId`; agente direto é o caminho primário; workflow city→activities exercitado por conformidade/sob demanda).
- **ADR-004** (`adr-004-cutover-eliminacao-internal-agent.md`) — eliminação física de `internal/agent` + desligamento do onboarding conversacional do WhatsApp; rota única no dispatcher; gate `grep` vazio.
- **ADR-005** (`adr-005-estrategia-testes-conformidade.md`) — testes determinísticos + variante `RUN_REAL_LLM`; promoção de `test/conformance/weather` a consumidor de produção.

### Riscos Conhecidos

- **Remoção de módulo vivo (irreversível)**: mitigação — apagar `internal/agent` apenas após `internal/agents` wired e build/CI verdes; gate `grep internal/agent` (≠ platform) vazio como critério de pronto; commit isolado para rollback fácil.
- **agent-como-step + streaming (RF-13)**: validação fim-de-stream/structured output; usar o contrato `Result(ctx)` que drena `Deltas()` (fix B5 da review) — teste de >64 deltas sem drenar.
- **Indexação assíncrona (RF-18)**: leak/duplicação — idempotência por `event_id` + `ON CONFLICT (source_message_pk, model)`; shutdown cooperativo; teste de replay.
- **Onboarding desligado**: o comando "ATIVAR <token>" deixa de ser atendido no WhatsApp; risco de regressão de ativação de conta — decisão de produto explícita (RF-24); documentar no runbook.
- **Drift de config**: migração `AGENT_*`→nova config; risco de variável órfã — checklist no cutover.

### Conformidade com Padrões

- `go-implementation` R0–R7 (sem `init()`, sem abstração de tempo, sem `panic` produção, `context.Context` em IO, `errors.Join`/wrapping, recursos modernos).
- `R-ADAPTER-001` — adapters finos (consumer/handler → usecase), zero comentários em Go de produção.
- `R-DTO-VALIDATE-001` — `Validate()` em todo input DTO.
- `R-TESTING-001` — testify/suite whitebox para use cases.
- `R-WF-KERNEL-001` / `R-AGENT-WF-001` — preservadas; `internal/agents` consome o kernel e o primitivo de agent; não introduz domínio no kernel; tipos fechados na fronteira; LLM só no provider/parse; Run auditável.
- `governance.md` — precedência DMMF state-as-type.

Gates de verificação (devem retornar vazio antes de merge):

```bash
grep -rn --include="*.go" "internal/agent\"" internal/ cmd/ test/ | grep -v "internal/platform/agent" \
  && echo "FAIL: referencia a internal/agent" && exit 1 || true

grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/agents/ | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentarios em producao" && exit 1 || true

test -d internal/agent && echo "FAIL: internal/agent ainda existe" && exit 1 || true
```

### Mapeamento Requisito → Decisão → Teste

| RF | Decisão/Componente | Teste |
|----|--------------------|-------|
| RF-01,02,03,04 | módulo consumidor, DI, R0-R7/DMMF (ADR-001) | unit module wiring; gates grep |
| RF-05,06,07,08 | weather-agent + provider OpenRouter + tool/memory binding | unit; conformidade sync+stream |
| RF-09,10 | tool get-weather + cliente open-meteo | unit httptest; conformidade |
| RF-11,12,13 | weather-workflow (fetch→plan, agent-como-step) (ADR-003) | conformidade workflow; fim-de-stream |
| RF-14,15,16 | scorers code-based + LLM-judged + runner | unit; integração results |
| RF-17,18,19 | memória + indexação assíncrona (ADR-002) | integração recall ANN; replay idempotente |
| RF-20,21,22 | canal WhatsApp → AgentRuntime; Run auditável (ADR-003) | integração platform_runs; E2E inbound |
| RF-23,24,25,26,27 | cutover/eliminação (ADR-004) | gate grep vazio; build/CI verdes |
| RF-28,29,30 | O11Y reutilizada; testes determinísticos + RUN_REAL_LLM (ADR-005) | gates; conformidade |

### Arquivos Relevantes e Dependentes

Novos: `internal/agents/**`, `internal/platform/memory/publishing_message_store.go` (+ consumer/worker de indexação).
Alterados: `cmd/server/server.go`, `cmd/server/whatsapp_wiring.go`, `cmd/worker/worker.go`, `internal/platform/whatsapp/dispatcher/dispatcher.go` (rota única), `configs/config.go`, `taskfiles/gates.yml` (gate `internal/agent` ausente).
Removidos: `internal/agent/**`; `internal/onboarding/e2e/*` que importam `internal/agent` (ou reescritos); wiring de onboarding conversacional no dispatcher.
Base aproveitada: `test/conformance/weather/*`, `migrations/000003_*`, `internal/platform/*`.

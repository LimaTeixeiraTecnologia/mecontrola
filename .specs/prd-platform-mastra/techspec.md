<!-- spec-hash-prd: 6b96b1c597b7039ee9388e726e1b5a64d69bb0772db6509ad4c26d759a8dfaba -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Plataforma Mastra-paridade em `internal/platform`

## Resumo Executivo

Esta especificação detalha como construir, em `internal/platform`, um substrato genérico e production-ready de **Agentes, Memória, Workflows e Scorers** com paridade comportamental ao Mastra, consumindo OpenRouter para LLM/embeddings e Postgres como persistência única. O kernel existente `internal/platform/workflow` (`Engine[S any]`, `Step[S]`, `Store`, `Codec.MergePatch`, combinadores, housekeeping) é **aproveitado e evoluído como base** — não reescrito (ADR-001). Sobre ele, novos pacotes irmãos de camada superior implementam os primitivos semânticos genéricos que antes eram exclusivos do `internal/agent` (descontinuado): `internal/platform/agent` (lifecycle, registry, AgentRuntime Thread→Run, structured output, streaming, hooks), `internal/platform/memory` (Thread, Message, WorkingMemory, recuperação semântica via pgvector), `internal/platform/tool` (contrato de tool tipado), `internal/platform/scorer` (evals code-based + LLM-judged) e `internal/platform/llm` (provider OpenRouter genérico com complete/stream/embed). O kernel permanece puro (sem LLM, sem domínio); o layering é unidirecional: `consumidor → agent/memory/scorer/tool → llm + workflow(kernel)`, e o kernel nunca importa camada superior (ADR-002).

A entrega é validada por dois mecanismos de prova de production-ready, não por claims: (1) um **consumidor de referência** — o exemplo weather do `limateixeira-agents` portado para Go — exercitando todas as capacidades end-to-end; (2) uma **suite de conformidade** com variante E2E contra OpenRouter real e Postgres real atrás da flag `RUN_REAL_LLM`, e camada determinística (unit + integração com testcontainers Postgres+pgvector) no CI padrão. O endurecimento anti-desvio é codificado em tipos fechados (state-as-type), zero comentários em Go de produção, gates de verificação reemitidos das regras `R-WF-KERNEL-001`/`R-AGENT-WF-001` apontando para os caminhos finais, e uma migration golang-migrate `000003` com `down` simétrico que remove as 7 tabelas `agent_*` e cria o storage genérico inspirado no Mastra.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Layering (setas = "depende de"; nunca o inverso):

```
consumidor (ex.: test/conformance/weather)
        │
        ▼
internal/platform/agent ──► internal/platform/tool
        │      │      └────► internal/platform/memory ──► internal/platform/llm (Embed)
        │      └───────────► internal/platform/scorer ──► internal/platform/llm (Complete)
        │
        ├──► internal/platform/llm        (Complete | Stream | Embed via OpenRouter)
        └──► internal/platform/workflow   (KERNEL — Engine[S], Step[S], Store, Codec)   ◄── base aproveitada
                       │
                       └──► internal/platform/workflow/infrastructure/postgres (Store adapter)
```

Componentes novos:

- **`internal/platform/llm`** — provider OpenRouter genérico (relocado e generalizado a partir de `internal/agent/.../openrouter`, que será apagado). Expõe `Complete`, `Stream` (SSE) e `Embed`. Sem semântica de domínio (sem `intentJSONSchema`); o schema de structured output é injetado pelo chamador.
- **`internal/platform/tool`** — contrato `ToolHandle` (adapter fino de I/O tipado) + helper genérico `NewTool[I,O]` que valida contra schema; `Registry` por id. Paridade com `@mastra/core/tools`.
- **`internal/platform/memory`** — `ThreadGateway` (`resourceId`/`threadId` opacos), `MessageStore` (turns), `WorkingMemory` (escopo resource), `SemanticRecall` (pgvector) + `Summarizer`. Paridade com a camada memory do Mastra.
- **`internal/platform/agent`** — `Agent` (Execute síncrono + Stream), `AgentRegistry`/`WorkflowRegistry`, `AgentRuntime` (Thread→Run auditável), `Hooks` de ciclo de vida, contrato de structured output, `RuntimeContext` (DI não persistido). Paridade com `@mastra/core/agent`.
- **`internal/platform/scorer`** — `Scorer` (code-based + LLM-judged com structured output), `Sampling`, `ScorerRunner` assíncrono, persistência de resultados. Paridade com `@mastra/core/evals`.

Componentes evoluídos:

- **`internal/platform/workflow`** (kernel) — preservado; única evolução é tornar explícito o repasse do `RuntimeContext` por `context.Context` aos steps (sem persistir), mantendo `S` como única fonte durável (ADR-001, ADR-007).
- **Schema Postgres** — migration `000003` (ADR-005).

### Fluxo de dados (execução de agente)

1. Consumidor chama `AgentRuntime.Execute(ctx, InboundRequest{ResourceID, ThreadID, AgentID, Message, MessageID})`.
2. `ThreadGateway.GetOrCreate(resourceID, threadID)` resolve o Thread (chaves opacas).
3. Abre `Run` semântico (status `running`) referenciando `thread_pk`; injeta `RuntimeContext` no `ctx`.
4. `WorkflowRegistry.Resolve(agentID|kind)` retorna a `Definition[S]` do kernel; `Engine[S].Start/Resume` executa os steps. Steps de parse usam `llm.Complete/Stream`; steps de domínio usam tools/bindings do consumidor.
5. Structured output validado na fronteira (síncrono) ou na conclusão do stream (ADR-003).
6. Mensagem persistida em `MessageStore`; WorkingMemory atualizável; conteúdo indexado em `SemanticRecall` (pgvector).
7. `Run` fechado (`succeeded`/`failed`) com `duration_ms`; `ScorerRunner` amostra e avalia o run assincronamente, persistindo `platform_scorer_results`.
8. Observabilidade emitida na stack existente (devkit-go `observability.Observability`), labels de cardinalidade controlada.

## Design de Implementação

### Interfaces Chave

LLM provider genérico (`internal/platform/llm`):

```go
type Provider interface {
    Slug() string
    Complete(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request) (TokenStream, error)
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type TokenStream interface {
    Deltas() <-chan string
    Close() error
    Err() error
}
```

Structured output (validação na fronteira; ADR-003):

```go
type Schema struct {
    Name   string
    Strict bool
    Schema map[string]any
}

type StructuredContract[T any] interface {
    Schema() Schema
    Decode(raw []byte) (T, error)
}
```

Tool (paridade `@mastra/core/tools`):

```go
type ToolHandle interface {
    ID() string
    Description() string
    Parameters() map[string]any
    Invoke(ctx context.Context, argsJSON []byte) ([]byte, error)
}

func NewTool[I, O any](id, desc string, in, out Schema,
    exec func(context.Context, I) (O, error)) ToolHandle
```

Agent (paridade `@mastra/core/agent`):

```go
type Agent interface {
    ID() string
    Execute(ctx context.Context, in Request) (Result, error)
    Stream(ctx context.Context, in Request) (ResultStream, error)
}

type ResultStream interface {
    Deltas() <-chan string
    Result(ctx context.Context) (Result, error)
}
```

`Result(ctx)` só retorna após o canal `Deltas()` fechar; quando há `StructuredContract`, valida o conteúdo acumulado e retorna erro explícito se não conformar (ADR-003).

AgentRuntime (Thread→Run auditável; R-AGENT-WF-001.6 re-escopado):

```go
type AgentRuntime interface {
    Execute(ctx context.Context, in InboundRequest) (Outcome, error)
}

type WorkflowRegistry[S any] interface {
    Resolve(agentID string) (workflow.Definition[S], bool)
}
```

Memory (paridade camada memory do Mastra):

```go
type ThreadGateway interface {
    GetOrCreate(ctx context.Context, resourceID, threadID string) (Thread, error)
}
type MessageStore interface {
    Append(ctx context.Context, threadPK uuid.UUID, m Message) error
    Recent(ctx context.Context, threadPK uuid.UUID, limit int) ([]Message, error)
}
type WorkingMemory interface {
    Get(ctx context.Context, resourceID string) (string, error)
    Upsert(ctx context.Context, resourceID, content string) error
}
type SemanticRecall interface {
    Index(ctx context.Context, resourceID, threadID, content string) error
    Recall(ctx context.Context, resourceID, query string, k int) ([]RecallHit, error)
}
```

Scorer (paridade `@mastra/core/evals`):

```go
type Scorer interface {
    ID() string
    Kind() ScorerKind
    Score(ctx context.Context, s RunSample) (ScoreResult, error)
}
type ScorerRunner interface {
    Observe(ctx context.Context, runID uuid.UUID, s RunSample)
}
```

Runtime context (DI não persistido; ADR-007):

```go
func WithRuntime(ctx context.Context, rc Runtime) context.Context
func RuntimeFrom(ctx context.Context) (Runtime, bool)
```

### Tipos fechados (state-as-type) — endurecimento anti-desvio

Reutilizados do kernel (sem duplicar): `workflow.RunStatus`, `workflow.StepStatus`, `workflow.SuspendReason` (já com `String()`/`IsValid()`/`Parse*`).

Novos tipos fechados (com `String()`/`IsValid()`/`Parse*`, nunca `string` solta na fronteira):

- `agent.RunStatus`: `running | succeeded | failed`.
- `agent.ToolOutcome`: `routed | clarify | usecaseError | missingResolver | replay`.
- `agent.AwaitingKind`: `none | confirm` (extensível só por nova constante tipada).
- `agent.ExecutionMode`: `sync | stream`.
- `scorer.ScorerKind`: `codeBased | llmJudged`.
- `scorer.SamplingType`: `ratio | always | never`.

### Modelos de Dados

Tabelas KEPT (kernel, inalteradas): `mecontrola.workflow_runs`, `mecontrola.workflow_steps`.

Tabelas DROP (migration `000003` up) — domínio do `internal/agent` descontinuado: `agent_sessions`, `agent_decisions`, `agent_threads`, `agent_runs`, `agent_working_memory`, `agent_observations`, `agent_processed_events`.

Tabelas NEW (genéricas, chaves opacas, inspiradas no storage Mastra — DDL final na migration, ADR-005):

- `platform_threads` — `id uuid pk`, `resource_id text`, `thread_id text`, `title text`, `metadata jsonb`, `created_at`, `updated_at`; `UNIQUE(resource_id, thread_id)`.
- `platform_messages` — `id uuid pk`, `thread_pk uuid fk→platform_threads`, `resource_id text`, `role text`, `content text`, `parts jsonb`, `created_at`; índice `(thread_pk, created_at)`.
- `platform_resources` — `resource_id text pk`, `working_memory text`, `metadata jsonb`, `updated_at` (working memory escopo resource).
- `platform_runs` — `id uuid pk`, `thread_pk uuid fk`, `resource_id text`, `thread_id text`, `agent_id text`, `workflow text`, `correlation_key text`, `status text`, `outcome text`, `error text`, `started_at`, `ended_at`, `duration_ms bigint`; índice `(thread_pk, started_at)`. Vincula ao snapshot do kernel por `(workflow, correlation_key)`.
- `platform_embeddings` — `id uuid pk`, `resource_id text`, `thread_id text`, `source_message_pk uuid`, `content text`, `embedding vector(1536)`, `model text`, `created_at`; índice **HNSW** sobre `embedding`. Requer `CREATE EXTENSION IF NOT EXISTS vector`. Dimensionalidade 1536 = `openai/text-embedding-3-small` via OpenRouter (configurável por env).
- `platform_scorer_results` — `id uuid pk`, `run_id uuid fk→platform_runs`, `scorer_id text`, `kind text`, `score double precision`, `reason text`, `metadata jsonb`, `sampled boolean`, `created_at`.

Invariantes de dados (R-WF-KERNEL-001.1 / RF-37): nenhuma coluna com semântica de domínio (`intent_kind`, `category_id`, etc.); idempotência/dedup por chave opaca (`message_id`/`correlation_key`), nunca por tipo de domínio.

### Endpoints de API

Não aplicável diretamente: a plataforma é uma biblioteca interna consumida in-process por módulos. Os adapters de entrada (HTTP/consumer/job) que disparam `AgentRuntime.Execute` pertencem ao consumidor e seguem `R-ADAPTER-001` (adapter fino → usecase). O consumidor de referência weather expõe seu fluxo via teste/E2E, não via endpoint público.

## Pontos de Integração

- **OpenRouter** (`internal/platform/llm`): `POST {base}/api/v1/chat/completions` (complete e stream `stream:true` SSE) e endpoint de embeddings. Auth `Authorization: Bearer <APIKey>` + headers `HTTP-Referer`/`X-Title`. Tratamento de erro: `ErrProviderUpstream` em status não-2xx, classificação de status (`unauthorized|no_credit|rate_limited|timeout|upstream_5xx|client_4xx`), timeout via `httpclient.WithTimeout(cfg.RequestTimeout)`, circuit breaker e fallback chain reaproveitados como steps/decorators (não no kernel).
- **open-meteo** (apenas no consumidor de referência weather): geocoding + forecast via `weatherTool`; é IO externo do consumidor, fora da plataforma.
- **Postgres + pgvector**: persistência única; extensão `vector` habilitada na migration e provisionada em produção estendendo `deployment/docker/Dockerfile.postgres` (base `postgres:16`); integração usa imagem `pgvector/pgvector:pg16` (PG16 prod×teste alinhados). Embeddings gerados via OpenRouter (`openai/text-embedding-3-small`, 1536) e gravados em `platform_embeddings`.
- **Indexação assíncrona de embeddings**: a mensagem é persistida e um evento é emitido via `internal/platform/outbox`; um consumer/worker (`internal/platform/worker`) gera o embedding e grava `platform_embeddings` **fora do caminho crítico** do agente (recall eventual; sem latência LLM na execução). Idempotência por `event_id`.

## Abordagem de Testes

### Testes Unitários

- Use cases (`internal/platform/*/application/usecases/*_test.go`): padrão canônico **testify/suite whitebox** com `dependencies` struct + IIFE por mock e `fake.NewProvider()` (R-TESTING-001). Mocks gerados por mockery para portas (`Store`, `Provider`, `ThreadGateway`, `MessageStore`, `SemanticRecall`, `Scorer`).
- Domínio puro (tipos fechados, `Decode` de structured output, sampling): tabela de cenários sem mocks; validação de `IsValid()`/`Parse*` para cada estado.
- Kernel: cobertura existente preservada (`engine_test.go`, `codec_test.go`, `combinators_test.go`); adicionar casos de repasse de `RuntimeContext` sem persistência.
- Mock apenas de serviços externos (OpenRouter); nunca mockar Postgres em unit — vai para integração.

### Testes de Integração

Critérios atendidos (≥2): fronteiras de IO críticas (Postgres, pgvector) onde mock não garante correção; risco real de divergência entre `MergePatch`/snapshot e SQL; custo de testcontainers proporcional. **Adotados.**

- Build tag `//go:build integration`; `testcontainers-go` com imagem Postgres habilitada para pgvector (ex.: `pgvector/pgvector:pg16`), migrations `000001..000003` aplicadas no setup.
- Cobertura: `Store` (insert/load/save CAS/append/list/delete), suspend/resume real com `MergePatch` sobre snapshot, `SemanticRecall.Index/Recall` (ANN real), `WorkingMemory`, `platform_runs`/`platform_scorer_results`.
- Dados de teste: fixtures por suite; SQL direto permitido em `_test.go` (exceção R-ADAPTER-001.2).

### Testes E2E

- **Suite de conformidade** (`test/conformance/...`) com o **consumidor de referência weather** portado para Go (`weatherAgent` + `weatherTool` + `weatherWorkflow` + scorers), exercitando: execução síncrona, streaming (`agent.Stream` dentro de step de workflow), structured output (síncrono e fim-de-stream), tool com I/O tipado, workflow multi-step com agent-como-step, suspend/resume idempotente, memória thread/working/longo-prazo (recall pgvector), runtime context e scorers (code-based + LLM-judged).
- Variante **real** atrás da flag de ambiente `RUN_REAL_LLM=1`: usa **OpenRouter real** e **Postgres real**; executada sob demanda/nightly, fora do gate de merge. Sem a flag, a suite roda em modo determinístico (provider fake + Postgres testcontainers) e o E2E que exige LLM real é `t.Skip`.
- Prova de production-ready (não claim): cada RF tem cenário E2E mapeado (ver "Mapeamento Requisito → Decisão → Teste").

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **`internal/platform/llm`** — provider OpenRouter genérico (complete/stream/embed) + structured output injetável. Base de tudo que usa LLM; primeiro porque agent/scorer/memory dependem dele.
2. **`internal/platform/memory`** — Thread/Message/WorkingMemory + portas; semantic recall depende de `llm.Embed` e da migration.
3. **Migration `000003`** — DROP `agent_*`, CREATE storage genérico + `vector` (ADR-005). Necessária antes da integração de memory/runs/scorers.
4. **`internal/platform/tool`** — contrato de tool tipado (independente; pode ir em paralelo a 1–2).
5. **`internal/platform/agent`** — registry, AgentRuntime (Thread→Run), Execute/Stream, hooks, structured output; consome 1,2,4 e o kernel.
6. **`internal/platform/scorer`** — code-based + LLM-judged + runner + persistência; consome 1 e 5.
7. **Evolução do kernel** — repasse de RuntimeContext (mínima, sem quebrar invariantes).
8. **Consumidor de referência weather + suite de conformidade** — integração e E2E; reemissão dos gates de governança.

### Dependências Técnicas

- Postgres com extensão `vector` disponível no ambiente (prod + testcontainers).
- Credencial OpenRouter (`OPENROUTER_API_KEY`) para a variante E2E real.
- devkit-go `observability` (já em uso pelo kernel).
- `internal/agent` removido antes do passo 8 (gates de governança reemitidos passam a apontar para os caminhos finais).

## Monitoramento e Observabilidade

Reutiliza a stack existente (devkit-go `observability.Observability` → OTLP → otel-lgtm), sem trilha paralela (RF-29). Cardinalidade controlada (RF-30):

- Métricas do kernel preservadas: `workflow_runs_total`, `workflow_run_duration_seconds`, `workflow_steps_total`, `workflow_step_duration_seconds`, `workflow_suspend_total`, `workflow_resume_total`, `workflow_version_conflict_total`. Labels: `workflow`, `step`, `status`, `outcome`.
- Novas métricas do agent: `agent_runs_total`, `agent_run_duration_seconds`, `agent_tool_invocations_total`, `agent_stream_total`. Labels permitidos (enums fechados): `agent_id`, `channel`, `workflow`, `status`, `tool`, `outcome`. **Proibido** `resource_id`/`thread_id`/`correlation_key`/`category_id` como label.
- LLM: `agent_llm_provider_call_total`, `agent_llm_provider_errors_total`, `agent_llm_tokens_total`, `agent_llm_provider_latency_seconds` (relocados; labels `model`, `status`, `reason`, `type`).
- Scorers: `scorer_runs_total`, `scorer_duration_seconds` (labels `scorer_id`, `kind`, `outcome`).
- Tracing: spans `agent.runtime.execute`, `agent.execute`, `agent.stream`, `llm.complete`, `llm.stream`, `llm.embed`, `memory.recall`, `scorer.score`, mantendo os spans do kernel (`workflow.engine.*`, `workflow.step.execute`).

## Considerações Técnicas

### Decisões Chave

Cada decisão material tem ADR rastreável nesta pasta:

- **ADR-001** (`adr-001-evoluir-kernel-workflow.md`) — Aproveitar e evoluir o kernel `internal/platform/workflow`; não reescrever.
- **ADR-002** (`adr-002-layering-internal-platform.md`) — Fronteiras de pacote e layering unidirecional em `internal/platform`.
- **ADR-003** (`adr-003-streaming-structured-output.md`) — Conciliação streaming × structured output: validação na conclusão do stream, falha explícita.
- **ADR-004** (`adr-004-memoria-longo-prazo-pgvector.md`) — Memória de longo prazo com pgvector + embeddings via OpenRouter, Postgres único.
- **ADR-005** (`adr-005-migracao-storage-mastra.md`) — Migration `000003`: DROP `agent_*` + storage genérico inspirado no Mastra + `vector`, `down` simétrico.
- **ADR-006** (`adr-006-scorers-evals.md`) — Scorers/Evals (code-based + LLM-judged) com sampling e persistência.
- **ADR-007** (`adr-007-runtime-context-di.md`) — Runtime context tipado (DI) via `context.Context`, não persistido.
- **ADR-008** (`adr-008-estrategia-testes-conformidade.md`) — Estratégia de testes/conformidade (E2E real atrás de flag + integração testcontainers + gates de governança).

### Riscos Conhecidos

- **Tensão streaming × structured output**: structured output é validável só com a resposta completa. Mitigação: contrato validado na conclusão do stream com falha explícita (ADR-003); deltas entregues como texto, resultado estruturado disponível em `Result(ctx)` pós-stream.
- **pgvector indisponível** em algum ambiente: bloquearia recall semântico e a migration. Mitigação: `CREATE EXTENSION IF NOT EXISTS vector` na migration; imagem testcontainers com pgvector; gate de pré-flight documentado; recall semântico isolado atrás de porta (`SemanticRecall`) para degradação controlada.
- **Custo/flakiness de OpenRouter real** no CI: mitigado por `RUN_REAL_LLM` (E2E real fora do gate de merge); CI padrão determinístico (ADR-008).
- **Type erasure no registry de tools** (Go sem generics heterogêneos em mapa): mitigado por `NewTool[I,O]` que encapsula marshaling/validação e expõe `ToolHandle` não-genérico.
- **Migration down pesado** (recriar 7 tabelas `agent_*`): mitigado copiando o DDL exato de `000001` no `down` de `000003` (reversibilidade real, ADR-005).
- **Vazamento de domínio para a plataforma** sob pressão de prazo: mitigado por gates de verificação reemitidos (grep) em CI (ADR-002/ADR-008).

### Conformidade com Padrões

Regras aplicáveis de `.claude/rules/`:

- `R-WF-KERNEL-001` (alterada 2026-06-29) — kernel puro, sem LLM/domínio, estados fechados, cardinalidade, merge-patch; gate de import estendido para proibir o kernel de importar `internal/platform/{agent,memory,llm,scorer,tool}`.
- `R-AGENT-WF-001` (alterada 2026-06-29, re-escopada para `internal/platform/agent`) — roteamento `Workflow→Tool→binding→usecase`, tool fina, tipos fechados, LLM só no step de parse, Run auditável, Thread-first (`resourceId`/`threadId`), pending step/gate HITL.
- `R-ADAPTER-001` — adapters finos, zero comentários em Go de produção.
- `R-DTO-VALIDATE-001` — `Validate()` em todo input DTO da plataforma.
- `R-TESTING-001` — testify/suite whitebox para use cases.
- `governance.md` — precedência DMMF state-as-type.

Gates de verificação reemitidos (devem retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/transactions\|internal/billing\|internal/identity\|internal/platform/agent\|internal/platform/memory\|internal/platform/llm\|internal/platform/scorer\|internal/platform/tool" \
  internal/platform/workflow/ \
  && echo "FAIL: kernel importa dominio ou camada superior" && exit 1 || true

grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "openai\|anthropic\|openrouter\|gemini\|mistral\|llm\." \
  internal/platform/workflow/ \
  && echo "FAIL: LLM no kernel" && exit 1 || true

grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*//" internal/platform/agent/ internal/platform/memory/ \
  internal/platform/llm/ internal/platform/scorer/ internal/platform/tool/ 2>/dev/null \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentarios em Go de producao" && exit 1 || true

grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  '"resource_id"\|"thread_id"\|"correlation_key"\|"category_id"' \
  internal/platform/agent/ internal/platform/scorer/ 2>/dev/null \
  && echo "FAIL: label de alta cardinalidade" && exit 1 || true
```

### Mapeamento Requisito → Decisão → Teste

| RF | Decisão/Componente | Teste |
|----|--------------------|-------|
| RF-01,02,03 | `agent.Agent` Execute/Stream, AgentRegistry (ADR-002) | unit registry; E2E weather sync+stream |
| RF-02 (hooks) | `agent.Hooks` ciclo de vida | unit hooks pré/pós/tool |
| RF-04 | `llm.Provider` OpenRouter | integração llm real (flag); unit fake |
| RF-05,06,07,08 | StructuredContract + validação fronteira/fim-de-stream (ADR-003) | unit Decode; E2E translationScorer |
| RF-09,10 | `tool.ToolHandle`/`NewTool` | unit NewTool; E2E weatherTool |
| RF-11,12,13,14 | kernel `Engine[S]`+combinadores; agent-como-step (ADR-001) | kernel tests; E2E weatherWorkflow |
| RF-15,16,17,18,19 | Store Postgres; suspend/resume MergePatch; housekeeping | integração suspend/resume; idempotência replay |
| RF-20,21,22 | memory Thread/Message/WorkingMemory | integração Postgres |
| RF-23,24,25 | SemanticRecall pgvector + Summarizer (ADR-004) | integração recall ANN; E2E recall |
| RF-26,27 | RuntimeContext via context (ADR-007) | unit não-persistência; kernel test |
| RF-28,29,30 | Run auditável + métricas cardinalidade controlada | integração platform_runs; gate grep |
| RF-31,32,33,34 | layering + tipos fechados (ADR-002) | gates grep; unit IsValid/Parse |
| RF-35..39 | migration 000003 (ADR-005) | integração up/down; pgvector |
| RF-40,41,42,43 | scorer code-based + LLM-judged + sampling (ADR-006) | unit sampling; integração results; E2E scorers |
| RF-44,45,46 | weather consumer + conformidade (ADR-008) | E2E flag RUN_REAL_LLM; integração testcontainers |

### Arquivos Relevantes e Dependentes

Existentes (base/aproveitados):

- `internal/platform/workflow/{engine,step,store,codec,combinators,housekeeping}.go` — kernel preservado/evoluído.
- `internal/platform/workflow/infrastructure/postgres/store.go` — adapter Store.
- `internal/agent/infrastructure/providers/openrouter/client.go` — fonte a relocar/generalizar (módulo será apagado).
- `migrations/000001_initial_schema.{up,down}.sql` — DDL das `agent_*` a copiar para o `down` de `000003`.
- `.claude/rules/workflow-kernel.md`, `.claude/rules/agent-workflows-tools.md` — regras alteradas; gates reemitidos.

Novos (a criar — caminhos finais; sem listar conteúdo de implementação):

- `internal/platform/llm/`, `internal/platform/tool/`, `internal/platform/memory/`, `internal/platform/agent/`, `internal/platform/scorer/` (cada um com `domain`/`application`/`infrastructure` conforme arquitetura).
- `migrations/000003_platform_mastra.{up,down}.sql`.
- `test/conformance/weather/` — consumidor de referência + E2E.
- `.specs/prd-platform-mastra/adr-001..008-*.md`.

## Decisões Fechadas (nível techspec)

Todas as questões de techspec foram fechadas (não há item em aberto). Valores são defaults overridáveis por env/config, sem mudança de contrato:

- **Embedding**: `openai/text-embedding-3-small` via OpenRouter, `vector(1536)`; índice **HNSW**. Configurável por env; validado na variante E2E real.
- **pgvector**: produção via extensão adicionada ao `deployment/docker/Dockerfile.postgres` (base `postgres:16`); integração via `pgvector/pgvector:pg16` (PG16 alinhado).
- **Indexação de embeddings**: assíncrona via `internal/platform/outbox` + `internal/platform/worker`, idempotente por `event_id`, fora do caminho crítico.
- **Limites operacionais (defaults derivados do código atual)**: TTL de suspensão `10m`; retry `MaxAttempts=3`, backoff `200ms → 10s`; `RequestTimeout=30s`; circuit breaker reusando os knobs atuais; concorrência de worker `8`; housekeeping/retention diário. Todos overridáveis.
- **Sampling de scorers**: `ratio=1.0` em dev e na suite de conformidade; `ratio=0.1` default em produção. Overridável por agente/scorer.
- **Tabelas do kernel**: `workflow_runs`/`workflow_steps` **mantidas** intactas (ADR-001); storage novo só adiciona `platform_*`.
- **Consumidor de referência**: `test/conformance/weather` (fora de `internal/platform`; é consumidor, não plataforma).

Nenhum item bloqueia a implementação; todos têm default fixado e ponto de validação em teste.

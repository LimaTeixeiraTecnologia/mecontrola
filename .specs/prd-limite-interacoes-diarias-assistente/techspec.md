<!-- spec-hash-prd: cd02d0df73fe39ccd93c198ca927f951ef6ec55338490bb7375268344e975d62 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica

## Resumo Executivo

Gate de limite diário de 30 interações no fluxo inbound do assistente WhatsApp, implementado como novo use case resolvedor em `internal/agents/application/usecases`, consultado pelo consumer `whatsapp_inbound` exatamente nos dois pontos de dispatch ao agente (texto e áudio transcrito), no mesmo padrão estrutural do resolvedor de onboarding já existente. A contagem reuso `mecontrola.platform_runs` via novo repositório em `internal/agents/infrastructure/repositories/postgres`, com contrato privado declarado no pacote consumidor. Nenhuma migration, nenhuma alteração no substrato `internal/platform/{agent,memory,workflow}`, nenhuma mudança de comportamento para usuários dentro da cota.

Decisões arquiteturais chave: gate como resolvedor via `ConsumerOption` (ADR-001), contagem via `platform_runs` com porta no consumidor (ADR-002), desativação operacional com valor zero, janela do dia calculada a partir do fuso America/Sao_Paulo já injetado no módulo.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes novos:

- `internal/agents/application/usecases/daily_interaction_limit.go`: use case `ResolveDailyInteractionLimit` com método `Execute(ctx, userID string) (DailyLimitResult, error)`. Responsável por capturar o instante corrente, calcular o início do dia em America/Sao_Paulo, consultar a contagem e decidir. Espelha o contrato do resolvedor de onboarding (`resolve_onboarding_or_agent.go:17-21`).
- `internal/agents/application/usecases/decide_daily_interaction_limit.go`: função pura `DecideDailyInteractionLimit(consumed int, limit int) bool`, sem IO e sem `context.Context`, seguindo o padrão de `DecideAudioTranscription` (`decide_audio_transcription.go:98`).
- `internal/agents/infrastructure/repositories/postgres/daily_interaction_counter.go`: repositório `NewDailyInteractionCounter(db database.DBTX, o11y)` retornando a interface privada do pacote `usecases`, no padrão de `audit_reconciliation.go:19-21` (SQL com `database/sql` puro, placeholders `$N`, schema `mecontrola.`).

Componentes modificados:

- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`: nova `ConsumerOption` `WithDailyLimitResolver` e método `tryDailyLimit`, chamado após `tryResume` e antes de `handleAgentInbound` nos dois pontos de dispatch (`Handle` linha 226-230 e o closure de dispatch em `handleAudioInbound` linha 281-286). O consumer permanece adapter fino: consulta o resolvedor e envia `result.Message` via `sendReply` existente.
- `internal/agents/module.go`: wiring do repositório e do use case, reutilizando `deps.DB` (`module.go:100`), `deps.O11y` e o `brazilLoc` já existente (`module.go:276-279`), e nova option no consumer (bloco `module.go:415-439`).
- `configs/config.go`: dois campos novos em `AgentConfig` (`configs/config.go:180-199`), duas entradas em `envKeys()` (bloco `:551-567`), defaults em `setAgentDefaults()` (`:1431-1445`) e validação em `validateAgentAudio()` ou validator novo chamado em `Validate()` (`:824`).
- `cmd/server/server.go` e `cmd/worker/worker.go`: repasse dos dois novos valores de config para `agents.Deps`, no padrão do repasse de `AudioUncertainReply` (`cmd/server/server.go:256-257`).
- `internal/agents/module.go` struct `Deps` (`module.go:99-114`): dois campos novos `DailyInteractionLimit int` e `DailyLimitReply string`, no padrão dos campos diretos `InboundTimeout` e `AgentMaxTokens`.

Fluxo de dados: payload inbound → dedup → `tryResume` (workflows suspensos e onboarding, fora da cota) → `tryDailyLimit` (consulta contagem em `platform_runs`; se bloqueado, resposta estática e encerra) → `handleAgentInbound` (fluxo atual inalterado).

## Design de Implementação

### Interfaces Chave

Contrato do resolvedor, no pacote `usecases`, espelhando `OnboardingResult`:

```go
type DailyLimitResult struct {
    Blocked bool
    Message string
}

type dailyInteractionCounter interface {
    CountStartedSince(ctx context.Context, resourceID string, since time.Time) (int, error)
}

type ResolveDailyInteractionLimit struct {
    counter dailyInteractionCounter
    loc     *time.Location
    limit   int
    reply   string
    o11y    observability.Observability
    total   observability.Counter
}

func (uc *ResolveDailyInteractionLimit) Execute(ctx context.Context, userID string) (DailyLimitResult, error)
```

Interface do consumer (adapter fino), espelhando `onboardingResolver` (`whatsapp_inbound_consumer.go:34-36`):

```go
type dailyLimitResolver interface {
    Execute(ctx context.Context, userID string) (usecases.DailyLimitResult, error)
}
```

Função de decisão pura:

```go
func DecideDailyInteractionLimit(consumed int, limit int) bool
```

Retorna `true` quando `consumed >= limit` (bloquear). Chamada somente quando `limit > 0`.

### Modelos de Dados

- Leitura: `SELECT COUNT(*) FROM mecontrola.platform_runs WHERE resource_id = $1 AND started_at >= $2`. Coberta pelo índice existente `platform_runs_resource_started_idx (resource_id, started_at DESC)` (`migrations/000001_initial_schema.up.sql:2390-2391`). Nenhuma tabela, coluna ou índice novo.
- Janela do dia: `now := time.Now().UTC()` capturado no `Execute`; início do dia calculado como `time.Date(y, m, d, 0, 0, 0, 0, loc)` a partir de `now.In(uc.loc)` e convertido de volta para UTC antes da query (coluna `started_at` é `TIMESTAMPTZ`).
- Config novas: `AGENT_DAILY_INTERACTION_LIMIT` (int, default 30, faixa `[0..10000]`, zero desativa) e `WA_MSG_DAILY_LIMIT_REACHED` (string, default em PT-BR informando limite e renovação à meia noite, obrigatória não vazia quando limite maior que zero).
- Sem DTO novo de entrada: o resolvedor recebe apenas `userID string`, como o resolvedor de onboarding.

### Endpoints de API

Nenhum. Funcionalidade exclusiva do pipeline de mensagens inbound.

## Pontos de Integração

Nenhuma integração externa nova. A especificação depende apenas de Postgres já operante e do fuso já carregado no módulo. A documentação oficial do OpenRouter ([API Limits](https://openrouter.ai/docs/api-reference/limits)) confirma que não há limite por usuário final no provedor, portanto nenhuma integração com headers `X-RateLimit-*` ou `GET /api/v1/key` é usada nesta fatia.

## Abordagem de Testes

### Testes Unitários

- `decide_daily_interaction_limit_test.go`: table-driven cobrindo fronteira inclusiva (29 de 30 permite, 30 de 30 bloqueia, 31 de 30 bloqueia, zero consumido permite).
- `daily_interaction_limit_test.go`: suite testify no padrão de `handle_inbound_test.go:20-35`, com fake manual do `dailyInteractionCounter` (interfaces privadas do módulo usam mocks manuais, padrão de `whatsapp_inbound_consumer_test.go:33-103`). Cenários: permitido abaixo do limite; bloqueado no limite com mensagem configurada; limite zero desativa sem consultar o contador (asserção de que o fake não foi chamado); erro do contador propaga com wrap `%w`; virada de dia calculada corretamente com relógio fixo via injeção do instante, sem `sleep` (determinismo exigido por R-TEST-001).
- `whatsapp_inbound_consumer_test.go`: cenários novos com `mockDailyLimitResolver` manual: bloqueio envia a mensagem estática e não chama `handleInbound`; permitido segue para `handleInbound`; gate não é consultado quando `tryResume` trata a mensagem (onboarding ou workflow suspenso); gate consultado também no dispatch de áudio transcrito; ordem da cadeia dedup → resume → limite → agente coberta por teste de ordenação no padrão de `TestResumeDispatcherPrecedesOnboarding` (`whatsapp_inbound_consumer_test.go:953-988`).
- `configs/config_test.go`: default 30, valor de ambiente respeitado, faixa `[0..10000]` rejeitada fora, mensagem vazia rejeitada quando limite ativo, no padrão de `config_test.go:953-1060`.

### Testes de Integração

Critérios do template atendidos: fronteira de IO crítica (query de contagem contra Postgres real) e infraestrutura de testcontainers já existente, custo proporcional.

- `daily_interaction_counter_integration_test.go` com build tag `//go:build integration`, pacote externo, suite testify, banco via `testcontainer.Postgres` (`internal/platform/testcontainer/postgres.go:13-16`) com migrations reais. Cenários: conta apenas Runs do `resource_id` informado; conta apenas Runs com `started_at` dentro da janela; retorna zero sem erro quando não há Runs; fronteira exata do `since` inclusiva.

### Testes E2E

Não adotados nesta fatia. O comportamento de ponta a ponta do consumer já é coberto pela suite unitária do consumer com fakes e o risco residual fica na query, coberta pelo teste de integração. Golden tests (`internal/agents/application/golden`) não exercem consumer nem `HandleInbound` (`harness.go:13-26`), portanto não são impactados e não precisam de novos casos.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. Config (`configs/config.go` + testes de config): base de tudo, sem dependência de código novo.
2. Repositório `daily_interaction_counter.go` + teste de integração: fecha a fronteira de IO antes de depender dela.
3. `DecideDailyInteractionLimit` + `ResolveDailyInteractionLimit` + testes unitários: regra de negócio sobre a porta já testada.
4. Consumer (`WithDailyLimitResolver`, `tryDailyLimit`, dois pontos de dispatch) + testes de ordenação e bloqueio.
5. Wiring (`Deps`, `module.go`, `cmd/server/server.go`, `cmd/worker/worker.go`) + validação completa dos gates.

### Dependências Técnicas

- Nenhuma infraestrutura nova, nenhuma migration, nenhuma biblioteca nova.
- Depende apenas de `database.DBTX` (`internal/platform/database/db.go:8-13`), do índice existente e do `brazilLoc` já em escopo no módulo.

## Monitoramento e Observabilidade

- Métrica nova: counter `agents_daily_limit_total` (unidade `"1"`, snake_case com sufixo `_total`, convenção de `observability/STANDARD.md:272-284`) com label `outcome` fechado (`allowed` ou `blocked`), no padrão de `resolve_onboarding_or_agent.go:43-46`. Proibido label `user_id`/`resource_id` (`STANDARD.md:127-131`).
- Logs: bloqueio registrado em nível `Info` com `slog` estruturado; erro de contagem registrado em nível `Error` com wrap `%w`, seguindo o padrão de falha do consumer (`whatsapp_inbound_consumer.go:339`).
- Traces: span `agents.usecase.resolve_daily_interaction_limit` no `Execute`, no padrão dos demais use cases.
- Sinal de produto: taxa de `outcome=blocked` alimenta a decisão futura de calibragem do limite, objetivo declarado no PRD.

## Considerações Técnicas

### Decisões Chave

- ADR-001 (`adr-001-gate-como-resolvedor-no-consumer.md`): gate implementado como resolvedor de aplicação consultado pelo consumer nos dois pontos de dispatch, espelhando o padrão do onboarding. Alternativas rejeitadas: gate dentro de `HandleInbound` (polui o contrato `agent.Outcome` com estado de bloqueio) e gate no `AgentRuntime` do platform (viola a regra de substrato genérico e mistura regra de produto no kernel).
- ADR-002 (`adr-002-contagem-via-platform-runs-com-porta-no-consumidor.md`): contagem via `platform_runs` com interface privada no pacote `usecases` e implementação em `repositories/postgres`. Alternativas rejeitadas: tabela de quota nova com contador incremental (estado paralelo a sincronizar e migration nova, rejeitada pelo PRD RF-02); estender `agent.RunStore` do platform (necessidade de um único consumidor vazando para o substrato, e mocks de `platform/agent` fora do `.mockery.yml`, drift confirmado).
- Desativação por valor zero: `AGENT_DAILY_INTERACTION_LIMIT=0` faz o resolvedor retornar permitido sem consultar o banco. Um único knob operacional, decidido na rodada de clarificação.
- Fail closed com erro tipado: falha na contagem propaga erro com wrap `%w` e segue o fluxo de erro existente do consumer (RF-12), sem liberação silenciosa e sem resposta inventada; a deduplicação é compensada pelo mecanismo existente `compensateDedup` (`whatsapp_inbound_consumer.go:307-315`), permitindo reprocessamento.

### Riscos Conhecidos

- Áudio que cruza o limite consome transcrição STT antes do gate: perda aceita e registrada no PRD (RF-13); mitigação futura seria gate prévio ao download, fora de escopo.
- Contagem acoplada a `platform_runs`: se um segundo agente for registrado para o mesmo `resource_id`, a cota passa a ser compartilhada; hoje só existe `mecontrola-agent` (`whatsapp_inbound_consumer.go:23`). Mitigação: revisão da ADR-002 ao introduzir novo agente.
- Custo da query por mensagem: um `COUNT` indexado por mensagem inbound; com o índice `(resource_id, started_at DESC)` o custo é O(linhas do dia do usuário), desprezível frente a uma chamada de LLM. Se volumetria futura exigir, projeção de contagem pode virar coluna agregada, sem mudança de contrato.
- Drift de layout do módulo (`infrastructure/persistence` majoritário vs `repositories/postgres` canônico): o repositório novo segue o layout canônico com precedente interno (`audit_reconciliation.go`), registrado para futura consolidação, sem migração retroativa nesta fatia.
- Mocks de `internal/platform/agent` fora do `.mockery.yml`: drift existente, não agravado por esta fatia, pois o contrato novo é privado do módulo e usa fake manual.

### Conformidade com Padrões

- R-ADAPTER-001 (adapters finos): consumer apenas consulta resolvedor e envia resposta; zero SQL, zero branching de domínio; validado por `task ci:agent-boundary` (`scripts/ci/agent-data-boundary.sh`).
- R6.3 (interface no consumidor) e R6.2 (tipos concretos por padrão): contrato privado em `usecases`, struct concreta no repositório.
- R6.7 (sem `clock.Clock`): `time.Now().UTC()` inline no use case, fuso recebido por construtor como `*time.Location`.
- DMMF: decisão em função `Decide*` pura; resultado como struct tipada `DailyLimitResult`; divergência de fluxo por valor de retorno e `errors.Is`/`%w`, sem `switch` em string.
- R5.10: erros com wrap `%w` e tratamento único; R5.12: sem `panic`; R5.26: identificadores sem prefixo `_`; R7.1 `any`; R7.2 `log/slog` via `o11y`.
- Zero comentários em Go de produção [HARD], validado por `task ci:zero-comments`.
- Cardinalidade de métricas: somente label `outcome`, validado por `task ci:platform-gates` e `observability/STANDARD.md`.
- Skills obrigatórias da implementação: `go-implementation` (task types `usecase-write`, `repository`, `consumer`, `cross-cutting` para config), `mastra` (gate antes de `AgentRuntime.Execute`, substrato intocado), `domain-modeling-production` (princípios DMMF, sem bundle pois não há agregado, comando ou evento novo), `design-patterns-mandatory` (decisão `não aplicar padrão`: resolvedor espelha estrutura existente, sem abstração nova).
- Matriz de validação (AGENTS.md, application + infrastructure + adapter): `go build ./...`, `go vet ./...`, `go test -race -count=1 ./internal/agents/... ./configs/...`, `golangci-lint run` no escopo, `task ci:agent-boundary`, `task ci:zero-comments`, teste de integração com tag `integration` para o repositório.

### Mapeamento Requisito, Decisão e Teste

| Requisito | Decisão | Teste |
| --- | --- | --- |
| RF-01 | Gate antes do dispatch nos dois pontos (ADR-001) | Teste de ordenação no consumer |
| RF-02 | Query em `platform_runs` com índice existente (ADR-002) | Integração do repositório |
| RF-03 | Início do dia via `brazilLoc` injetado | Unitário do use case com instante fixo |
| RF-04 | `DecideDailyInteractionLimit` com `consumed >= limit` | Unitário table-driven da decisão |
| RF-05 | `WA_MSG_DAILY_LIMIT_REACHED` em `AgentConfig` | Config test + cenário de bloqueio no consumer |
| RF-06 | Resolvedor em `usecases`, consumer fino | Gate `ci:agent-boundary` + testes do consumer |
| RF-07 | Gate posicionado após `tryResume` | Cenário de não consulta quando resume trata |
| RF-08 | `AGENT_DAILY_INTERACTION_LIMIT`, zero desativa | Config test + unitário sem chamada ao contador |
| RF-09 | Counter `agents_daily_limit_total` com label `outcome` | Asserção de métrica no fake de observabilidade |
| RF-10 | Bloqueio retorna antes de `handleAgentInbound` | Cenário de bloqueio sem chamada a `handleInbound` |
| RF-11 | Run inserido só após o gate; falha posterior não devolve cota | Coberto pela ordenação; sem teste dedicado novo |
| RF-12 | Erro do contador propaga com `%w` e `compensateDedup` | Cenário de erro no consumer e no use case |
| RF-13 | Gate também no dispatch de áudio transcrito | Cenário de áudio bloqueado no consumer |
| RF-14 | Gate posicionado após dedup | Coberto pela ordenação existente |

### Arquivos Relevantes e Dependentes

- Novos: `internal/agents/application/usecases/daily_interaction_limit.go`, `internal/agents/application/usecases/decide_daily_interaction_limit.go`, `internal/agents/infrastructure/repositories/postgres/daily_interaction_counter.go`, e os testes correspondentes.
- Modificados: `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`, `internal/agents/module.go`, `configs/config.go`, `configs/config_test.go`, `cmd/server/server.go`, `cmd/worker/worker.go`, `.env.example` e `deployment/config/prod.env` (documentação das novas variáveis).
- Somente leitura (evidência, sem alteração): `internal/platform/agent/runtime.go`, `internal/platform/agent/ports.go`, `migrations/000001_initial_schema.up.sql`, `internal/agents/application/usecases/resolve_onboarding_or_agent.go`.

<!-- spec-hash-prd: 5cfa6f4370ee4c7bcfdc18d739e95fcdf72a8c87aa5a4769b75ca9a10f5fb2f3 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Ordenação e Idempotência do Fluxo WhatsApp do Agente

PRD: `.specs/prd-whatsapp-ordenacao-idempotencia/prd.md` (v2, RF-01..19)
ADRs: [ADR-001](adr-001-claim-particionado-por-usuario.md), [ADR-002](adr-002-confirmacao-honesta-tooloutcome.md),
[ADR-003](adr-003-onboarding-start-resume-idempotente.md), [ADR-004](adr-004-observabilidade-e-deploy-seguro.md)

## Resumo Executivo

O diagnóstico provou que o fora-de-ordem e as respostas incoerentes vêm de **ausência de
serialização por usuário** (2 workers despachando o mesmo `outbox_events` com
`ORDER BY next_attempt_at ... FOR UPDATE SKIP LOCKED`, sem partição por `aggregate_user_id`), de
**escrita não-idempotente** (advisory lock de sessão desligado e incompatível com pgbouncer
`pool_mode=transaction`), de um **TOCTOU** no início do onboarding (`resolve_onboarding_or_agent.go`
faz `Load`→check→`Start` sem atomicidade; a 2ª `Start` viola o índice único parcial e vira
`onboarding_error` genérico) e de um **caminho de sucesso alucinado + resposta vazia** (`agent.go`
engole erro de tool em `content=""`; `runtime.go` hardcoda `RunStatusSucceeded`/`ToolOutcomeRouted`;
`sendReply` com `content==""` retorna `nil` sem enviar nem errar).

A estratégia é: **(1)** transformar o claim do outbox em **claim particionado** — no máximo 1 evento
em voo por `aggregate_user_id`, ordenado pelo **timestamp da Meta** — garantindo FIFO por usuário sem
segurar conexão durante o LLM (ADR-001); **(2)** propagar o `agent.ToolOutcome` (tipo fechado já
existente) até a decisão de resposta, deixar de engolir erro de tool, derivar o `RunStatus` do
resultado real e **proibir envio vazio** (ADR-002); **(3)** tornar o `Start` do onboarding
**idempotente-resume** (capturar violação do índice único → retomar) sob a serialização por usuário,
e **persistir os turnos de onboarding** em `platform_messages` (ADR-003); **(4)** ajustar a
**amostragem de traces** (os spans já existem) para tornar o caminho inbound observável, expor o
conflito de Start e corrigir `OTEL_SERVICE_VERSION` e o `stop_grace_period` do deploy (ADR-004). Nada
de broker/sharding: usa o outbox Postgres existente, respeitando R-WF-KERNEL-001, R-AGENT-WF-001,
R-ADAPTER-001 e a skill `go-implementation`.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes **modificados** (nenhum novo componente de infraestrutura):

- `internal/platform/outbox/` — **modificado**: `ClaimBatch` passa a claim particionado por usuário;
  novo índice; `Insert` mantém contrato. (RF-01/02/03/18)
- `internal/agents/module.go` (`buildWhatsAppAgentRoute`) e
  `internal/platform/whatsapp/payload/` — **modificado**: propagar `msg.Timestamp` (Meta) até o
  `OccurredAt` do evento outbox. (RF-18)
- `internal/platform/agent/agent.go` (`invokeToolCall`, `completeWithTools`) — **modificado**:
  não engolir erro de tool; sinalizar falha tipada ao loop. (RF-06/07)
- `internal/platform/agent/runtime.go` (`Execute`) — **modificado**: derivar `RunStatus`/`ToolOutcome`
  do resultado real; propagar outcome no `Outcome`. (RF-06)
- `internal/agents/.../consumers/whatsapp_inbound_consumer.go` (`sendReply`) — **modificado**: guarda
  contra envio vazio → fallback honesto; nunca `return nil` silencioso. (RF-08)
- `internal/agents/application/tools/*.go` (write tools) — **modificado**: output tipado carrega o
  `ToolOutcome`, não apenas `IsReplay`. (RF-07)
- `cmd/worker/worker.go` — **modificado**: idempotência ligada por padrão (remover gate
  `WriteAdvisoryLock`); o lock por-usuário passa a ser propriedade do claim particionado, não do
  advisory de sessão. (RF-04)
- `internal/agents/infrastructure/persistence/advisory_key_locker.go` — **REMOVIDO**: o caminho de
  `pg_advisory_lock` de sessão é redundante (claim particionado garante 1 evento por usuário em voo) e
  inseguro sob pgbouncer transaction-pool. Serialização = claim particionado + UNIQUE do
  `agents_write_ledger`. (decisão travada; ver ADR-001/ADR-002)
- `internal/agents/application/usecases/resolve_onboarding_or_agent.go` +
  `internal/platform/workflow/engine.go` (`Start`) — **modificado**: Start idempotente-resume;
  persistir turnos de onboarding. (RF-09/10/11/12)
- `internal/platform/workflow/engine.go` (métricas) e provider OTel — **modificado**: outcome/contador
  do conflito de Start; amostragem de trace parent-based. (RF-13/14/15)
- `deployment/compose/compose.swarm.yml` — **modificado**: `stop_grace_period`,
  `OTEL_SERVICE_VERSION=${IMAGE_TAG}`. (RF-16)

### Fluxo de Dados (alvo)

```
WhatsApp → Caddy → server-{1,2}: handler síncrono
  (assinatura → dedup wamid → principal → ratelimit → publica outbox com OccurredAt = Meta timestamp)
  → 200 OK
outbox_events (Postgres via pgbouncer transaction-pool)
  ▲  ClaimBatch PARTICIONADO: 1 evento em voo por aggregate_user_id, ORDER BY occurred_at
worker-{1,2} dispatcher (tick 500ms)
  → WhatsAppInboundConsumer.Handle
     → resolveOnboarding (Start idempotente-resume, atômico) | handleInbound (agent runtime)
        → OpenRouter loop (erro de tool = outcome tipado, não content vazio)
        → persiste turnos (agente E onboarding) em platform_messages
     → sendReply: content vazio = fallback honesto (NUNCA envio em branco)
```

## Design de Implementação

### Interfaces Chave

Claim particionado (adapter Postgres; sem mudança de assinatura pública do repositório — a lógica
muda no SQL). Assinatura atual preservada:

```go
type OutboxRepository interface {
    ClaimBatch(ctx context.Context, lockedBy string, batchSize int) ([]Row, error)
    Insert(ctx context.Context, evt Event, maxAttempts int) error
    MarkPublished(ctx context.Context, id string) error
    MarkPendingRetry(ctx context.Context, id string, nextAttemptAt time.Time) error
    MarkFailed(ctx context.Context, id string, lastErr string) error
    ResetStuck(ctx context.Context, stuckAfter time.Duration) (int, error)
}
```

Outcome do agente propagado (DMMF state-as-type — reutiliza `agent.ToolOutcome` fechado já existente
em `internal/platform/agent/types.go:48`):

```go
type Outcome struct {
    RunID   uuid.UUID
    Content string
    Status  RunStatus     // derivado do resultado real, não hardcoded
    Outcome ToolOutcome   // NOVO no Outcome: routed|clarify|usecaseError|missingResolver|replay|reconciled
    Mode    ExecutionMode
}
```

Tool result tipado no loop do agente (não engolir erro):

```go
// invokeToolCall passa a devolver a falha de forma tipada para o loop,
// para o LLM receber um tool message com erro estruturado (não content="").
func (a *agentImpl) invokeToolCall(ctx, toolMap, tc) (llm.Message, toolExecStatus, bool)
// toolExecStatus fechado: toolExecOK | toolExecError (nunca silencioso)
```

### Modelos de Dados

`outbox_events` — **sem colunas novas**; reaproveita `aggregate_user_id` (UUID, já existe) e
`occurred_at` (TIMESTAMPTZ, já existe). Migration `000002_*` adiciona:

```sql
-- Índice de suporte ao claim particionado (varredura por usuário pendente, ordenado por chegada)
CREATE INDEX IF NOT EXISTS outbox_events_user_pending_occurred_idx
    ON mecontrola.outbox_events (aggregate_user_id, occurred_at)
    WHERE status = 1 AND aggregate_user_id IS NOT NULL;

-- Backstop de "1 em voo por usuário": impede 2 linhas Processing do mesmo usuário
CREATE UNIQUE INDEX IF NOT EXISTS outbox_events_user_inflight_uidx
    ON mecontrola.outbox_events (aggregate_user_id)
    WHERE status = 2 AND aggregate_user_id IS NOT NULL;
```

`ClaimBatch` (SQL alvo) — reivindica apenas eventos de usuários **sem** evento em voo (status=2),
ordenando por `occurred_at`; o índice único acima é o backstop se dois dispatchers colidirem:

```sql
WITH claimable AS (
  SELECT id
    FROM mecontrola.outbox_events o
   WHERE o.status = 1
     AND o.next_attempt_at <= now()
     AND (
          o.aggregate_user_id IS NULL
       OR NOT EXISTS (
            SELECT 1 FROM mecontrola.outbox_events p
             WHERE p.aggregate_user_id = o.aggregate_user_id
               AND p.status = 2)
     )
     AND NOT EXISTS (                       -- no máximo 1 pendente por usuário neste lote
            SELECT 1 FROM mecontrola.outbox_events e2
             WHERE e2.aggregate_user_id = o.aggregate_user_id
               AND e2.status = 1
               AND e2.occurred_at < o.occurred_at)
   ORDER BY o.occurred_at
   LIMIT $2
   FOR UPDATE SKIP LOCKED
)
UPDATE mecontrola.outbox_events t
   SET status = 2, locked_at = now(), locked_by = $1, updated_at = now()
  FROM claimable c
 WHERE t.id = c.id
RETURNING t.id, t.event_type, t.aggregate_type, t.aggregate_id, t.aggregate_user_id,
          t.payload, t.metadata, t.attempts, t.max_attempts, t.occurred_at;
```

Nota de robustez: a violação do `outbox_events_user_inflight_uidx` (corrida entre worker-1/2) é
tratada como "usuário já em voo" → o evento permanece pendente e é reivindicado no próximo tick; não
é erro fatal. Eventos com `aggregate_user_id IS NULL` (sistêmicos) não são serializados.

`whatsapp_inbound_payload` (evento) — o `OccurredAt` do `outbox.NewEvent` passa a receber o
`msg.Timestamp` da Meta convertido para `time.Time` (hoje usa `time.Now().UTC()` em
`module.go` `buildWhatsAppAgentRoute`). `payload/parser.go` já expõe `msg.Timestamp` (string epoch).

`platform_messages` — **sem mudança de schema**; onboarding passa a usar o mesmo contrato
`memory.MessageStore.Append` que o agent runtime já usa (`runtime.go:138-153`), gravando os turnos na
**mesma thread do agente** — `(resourceId=userID, threadId=peer)` resolvida via
`ThreadGateway.GetOrCreate` — para um histórico único e contínuo (decisão travada; ADR-003).

### Endpoints de API

Nenhum endpoint novo. O webhook `POST /api/v1/whatsapp/inbound` mantém contrato; muda apenas o
`OccurredAt` propagado ao outbox e o processamento de todas as mensagens do lote (RF-17).

## Pontos de Integração

- **OpenRouter (LLM):** inalterado como provider; muda o tratamento de erro de tool no loop (o LLM
  passa a receber tool message de erro estruturado, permitindo resposta honesta).
- **pgbouncer `pool_mode=transaction`:** a serialização por usuário NÃO usa lock de sessão; o claim
  particionado opera em transações curtas (claim + mark), liberando a conexão durante o LLM. O caminho
  `pg_advisory_lock` de sessão (`advisory_key_locker.go`) é **removido** (decisão travada) — não há
  lock auxiliar; a garantia é o claim particionado + UNIQUE do ledger.
- **Trace no hop assíncrono:** o `traceparent` (W3C) é **propagado no `metadata` do `outbox_events`**
  na publicação e restaurado no consumer, costurando `webhook → worker → LLM → envio` num único trace
  (decisão travada; ADR-004). Producer e consumer permanecem adapters finos.
- **Docker Swarm:** `compose.swarm.yml` recebe `stop_grace_period: 30s` e
  `OTEL_SERVICE_VERSION=${IMAGE_TAG}`; deploy mantém `order: stop-first` + **gate de CI anti-storm**
  (serializa/consolida releases) — decisão travada (ADR-004).

## Abordagem de Testes

### Testes Unitários

Padrão canônico testify/suite (R-TESTING-001; whitebox, `fake.NewProvider()`, dependencies+IIFE):

- `outbox` claim particionado: suite sobre repositório com fixtures — cenários: 3 eventos do mesmo
  usuário só liberam 1 por vez; usuários distintos processam em paralelo; evento sistêmico
  (`user_id NULL`) não bloqueia; ordenação por `occurred_at`.
- `agent.invokeToolCall`: erro de tool → `toolExecError` e tool message com erro (não `content==""`).
- `agent.runtime.Execute`: `RunStatus`/`ToolOutcome` derivados do resultado; `Outcome.Outcome`
  preenchido; conteúdo vazio nunca vira sucesso silencioso.
- `whatsapp_inbound_consumer.sendReply`: `content==""` → fallback honesto e métrica `no_reply`, nunca
  envio em branco (o gateway não é chamado com vazio).
- `resolve_onboarding_or_agent`: violação do índice único no `Start` → resume (não `onboarding_error`).
- write tools: output carrega `ToolOutcome`; `usecaseError`/`missingResolver` nunca produzem
  confirmação de sucesso.

### Testes de Integração

**Necessários — sim.** Critérios atendidos: (a) fronteiras de IO críticas (Postgres/outbox) onde
mocks não garantem correção de concorrência; (b) incidente real já ocorreu (fora-de-ordem);
(c) custo de testcontainers proporcional ao risco (dado financeiro). Usar `testcontainers-go` com
build tag `//go:build integration`:

- **Concorrência por usuário (CA-01):** subir Postgres real, N eventos do mesmo usuário, 2 workers
  concorrentes; asserir zero execução concorrente por usuário (via timeline de `platform_runs`) e
  ordem FIFO das respostas.
- **Idempotência sob redelivery (CA-02):** reprocessar o mesmo `message_id` → 1 linha em
  `agents_write_ledger`, 0 duplicatas.
- **Start idempotente-resume (CA-04):** duas `Start` concorrentes → 1 run ativo, a 2ª retoma, 0
  `onboarding_error`.
- **Confirmação honesta (CA-03):** forçar erro de persistência → nenhuma confirmação de sucesso e
  nenhum envio vazio.

### Testes E2E

Ensaio de rolling deploy sob carga sintética de conversas (CA-05): validar ausência de respostas
duplicadas e lag de publicação p95 < 5s durante `docker service update` com `order: start-first`.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Migration `000002`** (índices do claim particionado) — base para tudo. Sem downtime (índices
   parciais, `IF NOT EXISTS`).
2. **Claim particionado no outbox** (ADR-001) — corrige a raiz do fora-de-ordem; maior valor.
3. **Propagação do timestamp da Meta** (RF-18) + **processar todo o lote do webhook** (RF-17).
4. **Confirmação honesta** (ADR-002): `invokeToolCall` + `runtime` + `sendReply` + tools tipadas +
   idempotência default. Depende de nada acima; pode paralelizar com (2) por subagente distinto.
5. **Onboarding Start idempotente-resume + persistir turnos** (ADR-003). Depende de (2) para a
   atomicidade por usuário.
6. **Observabilidade + deploy** (ADR-004): amostragem parent-based, outcome de conflito de Start,
   `OTEL_SERVICE_VERSION`, `stop_grace_period`.
7. **Testes de integração + ensaio de deploy** (CA-01..08).

Orquestração recomendada (memória `feedback_subagents_orchestration`): paralelizar por área —
outbox/claim, agent-runtime/confirmação, onboarding, observabilidade/deploy.

### Dependências Técnicas

- Postgres com pgbouncer `pool_mode=transaction` (já em produção).
- `testcontainers-go` para os testes de integração.
- Sem infra nova.

## Monitoramento e Observabilidade

- **Traces (RF-13):** spans já existem (`whatsapp.handler.inbound`, `whatsapp.dispatcher.route`,
  `agent.runtime.execute`, `llm.complete`, `workflow.engine.*`). Adotar **sampler parent-based com
  raiz AlwaysOn no caminho inbound**, pois hoje `0.1` derruba 90% (explica a ausência no Tempo). Não
  criar spans novos onde já existem. **Propagar `traceparent` no `metadata` do `outbox_events`**
  (decisão travada) para costurar o hop assíncrono server→worker num único trace fim-a-fim.
- **Métrica de conflito (RF-15):** `workflow_version_conflict_total` já existe (label `workflow`),
  mas cobre só CAS no Save. Adicionar contador/label para o **conflito de Start resolvido como resume**
  (ex.: `outcome="resumed_on_conflict"` em `workflow_runs_total` ou contador dedicado), cardinalidade
  controlada (sem `user_id`/`correlation_key`).
- **Métricas de ordenação/idempotência:** expor lag `occurred_at → published_at` (p95, alerta > 30s),
  contagem de duplicatas de escrita (deve ser 0), `no_reply`/`send_error` do consumer, e reivindicações
  bloqueadas por "usuário em voo".
- **Versão (RF-16):** `OTEL_SERVICE_VERSION=${IMAGE_TAG}` para telemetria refletir o binário.
- **Dashboards Grafana existentes** (stack otel-lgtm) recebem os painéis de lag e taxa `onboarding_error`.

## Considerações Técnicas

### Decisões Chave

- **ADR-001 — Claim particionado por usuário** (vs. advisory de sessão / xact-lock abrangendo o LLM).
- **ADR-002 — Confirmação honesta via `ToolOutcome` + guarda de envio vazio** (fim do sucesso alucinado).
- **ADR-003 — Onboarding Start idempotente-resume + persistência de turnos**.
- **ADR-004 — Observabilidade (amostragem/conflito de Start) + deploy seguro**.

### Riscos Conhecidos

- **Contenção no claim particionado sob alto fan-out:** o `NOT EXISTS` por usuário adiciona custo; o
  índice `outbox_events_user_pending_occurred_idx` mitiga. Se p95 de lag subir, ADR-001 prevê evolução
  para partição física por hash de usuário na fase 2.000–10.000 (sem reescrita estrutural).
- **Backstop de índice único em voo:** colisão worker-1/2 gera erro de constraint tratado como
  "adiar", não fatal — precisa de tratamento explícito de `unique_violation` (não vazar como falha).
- **Persistir turnos de onboarding** aumenta escrita em `platform_messages`; volume baixo (turnos
  humanos), aceitável.
- **Amostragem parent-based** aumenta volume de traces do inbound; custo de storage no otel-lgtm
  (retenção 30d) — dimensionar taxa.

### Conformidade com Padrões

- **R-WF-KERNEL-001:** o kernel `internal/platform/workflow` permanece genérico; a mudança de Start
  idempotente-resume é no mecanismo (captura de `unique_violation` → resume via `Load`), sem domínio.
- **R-AGENT-WF-001:** `ToolOutcome`/`RunStatus` continuam tipos fechados (state-as-type); tools finas
  (adapter → usecase); LLM só nas call-sites sancionadas; Run auditável com outcome real.
- **R-ADAPTER-001:** zero comentários em Go de produção; adapters (consumer/producer/handler) finos;
  SQL só no adapter Postgres.
- **R-DTO-VALIDATE-001, R-TESTING-001:** DTOs de input com `Validate()`; testes em testify/suite.
- **R-TXN-004 / R-WF-KERNEL-001.4:** métricas sem `user_id`/`correlation_key`/`category_id`.
- **go-implementation + DMMF:** `Decide*` puro onde aplicável; smart constructors; discriminated
  unions; `errors.Join`/`%w`; sem `Result[T,E]`/currying/DSL; sem abstrair tempo (`time.Now().UTC()`
  inline); sem `init()`; goroutines canceláveis.

### Arquivos Relevantes e Dependentes

- `internal/platform/outbox/storage_postgres.go` (`ClaimBatch`), `outbox.go`, `dispatcher.go`,
  `status.go`, `configs/config.go` (OutboxConfig).
- `migrations/000002_*.up.sql` / `.down.sql` (novos índices).
- `internal/agents/module.go` (`buildWhatsAppAgentRoute`), `internal/platform/whatsapp/payload/parser.go`,
  `.../payload/types.go`.
- `internal/platform/agent/agent.go` (`invokeToolCall`, `completeWithTools`), `runtime.go`, `types.go`.
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (`sendReply`).
- `internal/agents/application/tools/{register_expense,register_income,register_card_purchase}.go`.
- `internal/agents/application/usecases/{idempotent_write,resolve_onboarding_or_agent}.go`,
  `cmd/worker/worker.go` (gate `WriteAdvisoryLock`).
- `internal/platform/workflow/engine.go` (`Start`, métricas), `.../infrastructure/postgres/store.go`.
- `internal/agents/application/workflows/onboarding_workflow.go` (persistência de turnos).
- `internal/platform/agent/runtime.go` (contrato `messages.Append`), `internal/platform/memory/...`.
- `deployment/compose/compose.swarm.yml` (server-1/2, worker-1/2: `stop_grace_period`,
  `OTEL_SERVICE_VERSION`, amostragem).
- `cmd/server/server.go`, `cmd/worker/worker.go` (config do provider OTel/sampler).

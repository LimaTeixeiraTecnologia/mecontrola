# Arquitetura Completa — MeControla

## 1. Visão Geral

MeControla é um **monolito modular em Go 1.26.4** com 9 bounded contexts independentes e uma camada de plataforma transversal. A aplicação é distribuída em **3 binários** via Cobra CLI, todos compilados a partir do mesmo módulo Go.

```
cmd/
├── server   → HTTP server (porta 8080): handlers, webhook WhatsApp, outbox dispatcher
├── worker   → Background jobs (porta 8081 health): cron, consumers de eventos
└── migrate  → Migrations com advisory lock (golang-migrate)
```

**Stack principal:**

| Camada | Tecnologia |
|--------|-----------|
| Linguagem | Go 1.26.4 |
| HTTP Router | go-chi/chi v5 |
| Banco de Dados | PostgreSQL 16 + pgBouncer (pool transaction) |
| Driver DB | jackc/pgx v5 + jmoiron/sqlx |
| Migrations | golang-migrate v4 (embedded SQL) |
| Observabilidade | OpenTelemetry SDK → OtelCollector → Grafana LGTM |
| LLM | OpenRouter API (Gemini Flash Lite / Mistral Small) |
| Config | spf13/viper + .env |
| Mocks | vektra/mockery v2 |
| Testes | stretchr/testify + testcontainers + cucumber/godog (BDD) |
| Imagem | gcr.io/distroless/static-debian12:nonroot (UID 65532) |
| Deploy | Docker Swarm + Caddy (TLS) + pgBackrest (backup S3) |
| Tarefas | Task 3.51.1 (Taskfile.yml) |

---

## 2. Estrutura de Diretórios

```
mecontrola/
├── cmd/
│   ├── main.go                       # Cobra root: registra server, worker, migrate, migrate-down
│   ├── server/
│   │   ├── server.go                 # Bootstrap HTTP, Chi, OTel, healthchecks, routers, outbox
│   │   └── whatsapp_wiring.go        # Wiring específico WhatsApp webhook
│   ├── worker/
│   │   ├── worker.go                 # Bootstrap jobs (cron), consumers, OTel
│   │   └── health.go                 # Health check HTTP :8081
│   └── migrate/
│       ├── migrate.go                # Apply migrations (advisory lock, iofs embed)
│       └── {migrate_test,advisory_lock_test}.go
│
├── internal/
│   ├── agent/                        # LLM integration, Workflow/Tool pattern
│   ├── billing/                      # Kiwify, subscriptions, reconciliation
│   ├── budgets/                      # Orçamentos, allocations, threshold alerts
│   ├── card/                         # Cartões de crédito (PCI RF-16)
│   ├── categories/                   # Catálogo de categorias, busca textual
│   ├── identity/                     # Usuários, auth HMAC-SHA256, principal
│   ├── onboarding/                   # Magic token, ativação WhatsApp, LLM steps
│   ├── transactions/                 # Ledger financeiro, DMMF Decide*, recorrência
│   └── platform/                     # Capacidades transversais (ver seção 5)
│
├── configs/
│   └── config.go                     # Viper: ~55KB, structs por módulo, 60+ variáveis
│
├── migrations/
│   ├── embed.go                      # //go:embed *.sql
│   ├── 000001_initial_schema.{up,down}.sql
│   └── 000002_seed_reference_data.{up,down}.sql
│
├── deployment/
│   ├── docker/Dockerfile             # Multi-stage: builder → distroless
│   ├── compose/{compose,local,prod,swarm}.yml
│   ├── caddy/Caddyfile
│   ├── pgbouncer/                    # pool mode: transaction
│   ├── pgbackrest/                   # backup S3 + encryption
│   ├── telemetry/grafana/            # otelcol-config, loki-config, tempo-config, prometheus
│   ├── dashboards/                   # agent-runtime-overview, mecontrola-api, infra, ops
│   ├── monitoring/                   # alertmanager, prometheus-rules
│   └── terraform/                   # IaC (AWS, rede)
│
├── docs/
│   ├── diagrams/                     # Este arquivo + diagramas por módulo
│   ├── plans/
│   ├── postman/
│   ├── runs/
│   └── reviews/
│
├── taskfiles/                        # build, test, lint, security, mocks, migrate, deploy
├── .specs/                           # PRD, ADRs, execution reports
├── .github/workflows/                # ci-cd.yml, e2e.yml, auto-merge.yml
└── .claude/                          # Skills, rules, hooks Claude Code
```

---

## 3. Fluxo Completo: WhatsApp → Banco de Dados

### 3.1 Recepção Síncrona (HTTP Server)

```
[Meta WhatsApp Cloud API]
  │
  │  POST /api/v1/whatsapp/messages
  │  Header: X-Hub-Signature-256
  │  Body: JSON payload
  ▼
internal/platform/whatsapp/handlers/inbound_handler.go
  InboundHandler.Handle()
  │
  ├─ Preserva raw body via signature.RawBodyBuffer
  ├─ HMAC-SHA256 verify: signature/hmac.go
  │    X-Hub-Signature-256 == HMAC(WHATSAPP_APP_SECRET, rawBody)
  │    → 401 se falhar
  │
  ▼
internal/platform/whatsapp/dispatcher/dispatcher.go
  Dispatcher.Route(ctx, rawMessage)
  │
  ├─ payload.ExtractFirstMessage(raw)          → parse JSON, extrai primeira mensagem
  ├─ Timestamp validation                       → rejeita mensagens > 5 min antigas
  ├─ dedup.InsertIfAbsent(WAMID)               → tabela channel_processed_messages
  │    → ignora silenciosamente se WAMID já existe (dedup 30d)
  ├─ identity.EstablishPrincipal.Execute()      → resolve whatsapp_number → user_id
  │    → cria user se não existe (UpsertUserByWhatsApp)
  │    → lê user_identities + users
  ├─ ratelimit.Allow(userID)                    → 600 req/min por usuário, burst 100
  │    → publica auth.failed na outbox se bloqueado
  ├─ Route decision:
  │    activation token no texto? → onboardingRoute()
  │    usuário não reconhecido?  → onboardingRoute()
  │    else                       → agentRoute()
  │
  ├─ agentRoute():
  │    Publica para outbox_events:
  │    { event_type: "agent.whatsappinboundmessage",
  │      aggregate_id: userID,
  │      payload: { msg_id, text, from, channel } }
  │
  └─ HTTP 200 → ACK para Meta (SLA máximo 20s)
```

### 3.2 Processamento Assíncrono (Worker)

```
[cmd/worker — Outbox Dispatcher Job]
  Tick a cada 2s (OUTBOX_DISPATCHER_TICK_INTERVAL)
  │
  ├─ SELECT id, event_type, payload FROM outbox_events
  │    WHERE status = 1 (Pending)
  │    ORDER BY created_at ASC LIMIT 50 (batch)
  │
  ├─ Marca status = 2 (Processing) em tx
  ├─ events.Dispatcher.Dispatch(event_type, payload)
  │    → lookup handler registrado via Register(event_type, handler)
  │
  ▼
internal/agent/infrastructure/messaging/database/consumers/
  WhatsAppInboundConsumer.Handle(ctx, event)
  │
  ▼
internal/agent/application/services/intent_router.go
  IntentRouter.RouteWhatsApp(ctx, principal, msg)
  │
  ├─ tryResumeInbound()   → verifica runs suspensos (ordem determinística):
  │    1. continuePendingExpenseConfirmation()  → pendingexpense.Draft ativo?
  │    2. continuePendingApproval()             → ConfirmState HITL ativo?
  │    3. continuePendingPlan()                 → PlanState suspenso?
  │    4. continueBudgetSession()               → BudgetSession ativa?
  │    Se qualquer resume → executa e retorna
  │
  ├─ ParseInbound.Execute()
  │    internal/agent/application/usecases/parse_inbound.go
  │    │
  │    ├─ sanitize.Sanitizer — limpa input (trunca em 2000 chars)
  │    ├─ OpenRouter inference:
  │    │    internal/agent/infrastructure/providers/openrouter/client.go
  │    │    Primary:  google/gemini-2.5-flash-lite  (LLMClassParse)
  │    │    Fallback: mistralai/mistral-small-3.2-24b
  │    │    JSON schema strict mode → structured output
  │    │    Max tokens: 768 | Prose: 200
  │    │    Circuit breaker: 5 failures / 30s window / 60s cooldown
  │    ├─ Decode JSON → ParsedIntent{ Kind, Confidence(0-1), Intent, Plan, Raw }
  │    ├─ If Confidence < AGENT_POLICY_MIN_CONFIDENCE (0.8) → PolicyEvaluator bloqueia write
  │    └─ Returns ParseInboundOutput
  │
  ▼
internal/agent/application/services/daily_ledger_agent.go
  DailyLedgerAgent.Handle(ctx, principal, channel, peer, text, messageID)
  │
  ├─ ThreadGateway.GetOrCreate(userID, channel)  → tabela agent_threads
  ├─ RunGateway.Create(threadID, workflow, ...)   → tabela agent_runs
  │
  ├─ Classifica intent.Kind:
  │
  │  [READ — queries, resumos, listagens]
  │    WorkflowRegistry.Resolve(kind) → IntentWorkflow.Execute(ToolInput)
  │    → Tool.Execute() → binding → usecase → repository → PostgreSQL
  │    → Reply formatado
  │
  │  [WRITE STANDARD — income, card purchase simples]
  │    WorkflowRegistry.Resolve(kind) → Workflow.Execute(ToolInput)
  │    → fluxo adapter fino: Tool → binding → usecase → DB
  │
  │  [WRITE KERNEL — expense com categoria, card purchase complexo]
  │    kernelEngine.Start(ctx, kernelDef, correlationKey, ExpenseState{...})
  │    → internal/platform/workflow/engine.go
  │    → 14 steps sequenciais (ver seção 7)
  │    → suspend/resume em workflow_runs + workflow_steps
  │
  │  [WRITE DESTRUCTIVE — delete last, edit last, delete card, budget commit]
  │    confirmEngine.Start(ctx, confirmDef, correlationKey, ConfirmState{...})
  │    → Step confirm_gate → Suspend com prompt "Confirma operação? Sim/Não"
  │    → Aguarda Resume com texto do usuário
  │    → Se "sim" → executor correspondente → DB
  │    → Se "não"/ambíguo/TTL expirado → cancela
  │
  │  [PLAN — multi-step intents]
  │    planEngine.Start() → PlanState → executa steps em sequência
  │
  │  [BUDGET SESSION — configuração interativa de orçamento]
  │    budgetSessionEngine.Start() → sessão conversacional
  │
  ▼
WhatsAppGateway.SendTextMessage(userID, channel, reply)
  internal/agent/infrastructure/binding/
  → HTTP client → Meta WhatsApp Cloud API
  → Persiste message_id em whatsapp_message_status

[Worker — Outbox]
  Marca outbox_events.status = 3 (Published)
  Retry até 3x com backoff exponencial em falha
  Dead-letter após max_attempts: métrica outbox_dead_letter_total
```

---

## 4. Módulos de Negócio (Bounded Contexts)

Cada módulo segue a estrutura:

```
internal/<modulo>/
├── domain/
│   ├── entities/       # Agregados e entidades
│   ├── services/       # Decide* puro (DMMF): sem IO, deterministico
│   ├── valueobjects/   # Smart constructors, validação de invariante
│   ├── commands/       # CQRS commands
│   └── events/         # Domain events
├── application/
│   ├── usecases/       # Orquestra domain + infrastructure
│   ├── dtos/           # Input (com Validate()) + Output
│   ├── interfaces/     # Contratos de repositório (mocks gerados aqui)
│   └── binding/        # Wiring para o agent (quando aplicável)
├── infrastructure/
│   ├── http/server/handlers/        # Adapter fino: handler → usecase
│   ├── repositories/postgres/       # Implementações de repositório
│   ├── messaging/database/
│   │   ├── consumers/               # Event consumers (Outbox)
│   │   └── producers/               # Domain event → Outbox envelope
│   └── jobs/handlers/               # Background job handlers (cron)
├── e2e/features/                    # Cucumber BDD tests
└── module.go                        # Bootstrap: HTTP routes, event handlers, jobs
```

### 4.1 identity

**Responsabilidade:** Usuários, autenticação, resolução de principal (WhatsApp → user_id), housekeeping de auth_events.

| Item | Detalhe |
|------|---------|
| Auth | HMAC-SHA256 gateway middleware |
| Key Use Cases | EstablishPrincipal, UpsertUserByWhatsApp, FindUserByWhatsApp, SignUp, SignIn, ResolvePreferredChannel |
| HTTP Endpoints | POST /auth/signup, POST /auth/signin |
| HTTP Endpoints WhatsApp | GET+POST /api/v1/whatsapp (verify + inbound + status) |
| Jobs | AuthEventsHousekeeping @daily |
| Consumers | SubscriptionActivatedConsumer → atualiza entitlements do usuário |
| Eventos Publicados | identity.subscriptionactivated, identity.authfailed |
| Tabelas | users, user_whatsapp_history, user_identities, auth_events |

### 4.2 billing

**Responsabilidade:** Integração Kiwify (payment processor), lifecycle de subscriptions, reconciliação, grace period PAST_DUE 3 dias, housekeeping.

| Item | Detalhe |
|------|---------|
| Integração | Kiwify API (OAuth token, rate limit 100 req/min, webhook 60 req/min) |
| Webhook | POST /webhooks/kiwify — HMAC verification + payload parsing |
| Key Use Cases | ProcessKiwifyWebhook, ReconcileSubscriptions, ExpireGracePeriod, AnonymizeUser |
| Jobs | ReconciliationJob @hourly, GracePeriodExpirer @daily, AnonymizationJob @daily |
| Consumers | BillingEventConsumer |
| Eventos Publicados | billing.subscriptionapproved, billing.subscriptionlate, billing.subscriptioncanceled |
| Tabelas | billing_subscriptions, billing_plans |

### 4.3 onboarding

**Responsabilidade:** Magic token (TTL 7 dias), ativação via WhatsApp, fluxo conversacional LLM-driven de 8 etapas no kernel, abandonment tracking, email de ativação.

| Item | Detalhe |
|------|---------|
| HTTP | POST /activate, GET /state, POST /checkout |
| Rate Limits | state: 30/min, checkout: 10/min |
| Canal | Meta WhatsApp Cloud API (client em infrastructure/http/client/meta/) |
| LLM | claude-haiku-4.5 (AGENT_ONBOARDING_LLM_MODEL) para steps conversacionais |
| Kernel | Engine[OnboardingState]: 8 steps mapeando Cap.08 do produto |
| Jobs | TokenExpirationJob @daily, OutreachSchedulerJob @every 2h, AbandonmentJob @hourly |
| Consumers | SubscriptionBoundSessionConsumer, PaidWithoutTokenConsumer, ActivationEmailConsumer |
| Tabelas | onboarding_tokens, onboarding_sessions |

### 4.4 categories

**Responsabilidade:** Catálogo estático de categorias de despesa/receita, busca textual, ETag para cache HTTP.

| Item | Detalhe |
|------|---------|
| HTTP | GET /categories (com ETag) |
| Busca | TriG index em category_dictionary para busca textual/semântica |
| Use Cases | ListCategories, SearchDictionary (resolução de hints do agent → category_id) |
| Jobs | Nenhum (referência estática) |
| Tabelas | categories, category_dictionary |

### 4.5 card

**Responsabilidade:** CRUD de cartões de crédito, listagem paginada por cursor, alertas de fatura, compliance PCI RF-16.

| Item | Detalhe |
|------|---------|
| PCI RF-16 | Zero PAN / CVV / CVC / track / PIN em qualquer arquivo Go |
| HTTP | GET/POST/PUT/DELETE /cards, GET /cards/{id}/invoices |
| Paginação | Cursor-based (não offset) |
| Use Cases | CreateCard, ListCards, GetInvoice, ListInvoices |
| Jobs | InvoiceDueAlertJob @daily (janela 3 dias) |
| Consumers | OnboardingCardConsumer (cria cartão padrão ao concluir onboarding) |
| Tabelas | cards, transactions_card_purchases, transactions_card_invoices |

### 4.6 budgets

**Responsabilidade:** Orçamentos mensais com allocations por categoria, registro de despesas, resumo mensal, threshold alerts, reaper de drafts pendentes, purge de dados antigos.

| Item | Detalhe |
|------|---------|
| Thresholds | Category 80%, Goal 50%, Card 85% |
| HTTP | POST /budgets, PUT /budgets/{id}, POST /expenses |
| Use Cases | CreateBudget, ActivateBudget, RecordExpense, GetMonthlySummary |
| Jobs | PendingDraftReaperJob @every 30s (TTL 24h), AbandonedDraftJob @daily 03:00, PurgeRetentionJob @daily 04:00 (batch 500), ThresholdAlertJob @hourly |
| Consumers | TransactionCreatedConsumer, ExpenseCommittedConsumer, CardPurchaseCreatedConsumer, ThresholdAlertNotifier |
| Tabelas | budgets, budgets_allocations, budgets_expenses |

### 4.7 transactions

**Responsabilidade:** Ledger financeiro com padrão DMMF Decide*, idempotência 24h, resumo mensal, recorrência materializada diariamente, timezone America/Sao_Paulo.

| Item | Detalhe |
|------|---------|
| Padrão | DMMF: Decide* puro (DecideCreate, DecideUpdate, DecideMaterializeForDay) |
| HTTP | POST /transactions, PUT /transactions/{id}, DELETE /transactions/{id}, GET /transactions/summary |
| Use Cases | RecordTransaction, UpdateTransaction, DeleteTransaction, QueryTransactions, MonthlySummary |
| Jobs | RecurringMaterializerJob @daily, MonthlySummaryRecomputeJob @daily (lookback 48h) |
| Consumers | MonthlySummaryRecomputeConsumer |
| Eventos Publicados | transactions.transactioncreated, transactions.transactionupdated, transactions.cardpurchasecreated |
| Tabelas | transactions |

### 4.8 agent

**Responsabilidade:** Integração LLM via OpenRouter, padrão Workflow/Tool canônico, WorkflowRegistry, kernel de steps durável (14 steps), Thread/Run/WorkingMemory auditáveis, dispatch multicanal.

| Item | Detalhe |
|------|---------|
| LLM Primary | google/gemini-2.5-flash-lite |
| LLM Fallback | mistralai/mistral-small-3.2-24b |
| Max Tokens | 768 output, 2000 chars input |
| Confidence Min | 0.8 (AGENT_POLICY_MIN_CONFIDENCE) |
| Circuit Breaker | 5 falhas / 30s window / 60s cooldown |
| Consumers | WhatsAppInboundConsumer, OnboardingBoundConsumer, OnboardingCompletedConsumer |
| Jobs | (sem jobs próprios — processamento via consumer) |
| Tabelas | agent_sessions, agent_decisions, agent_runs, agent_threads, agent_working_memory, agent_observations |

---

## 5. Platform — Capacidades Transversais

`internal/platform/` é consumida por todos os módulos. **Não contém regra de negócio.**

### 5.1 database

```
internal/platform/database/
├── postgres/         # Driver pgx/sqlx, pool de conexões
├── uow/
│   ├── uow.go        # Interface UnitOfWork: DBTX(), Do(ctx, fn) error
│   └── do.go         # Genérico Do[T](ctx, uow, fn) → (T, error)
└── mocks/
```

Padrão: leitura via `Repository` injetado (DI), escrita via `UoW.Do()` com `RepositoryFactory`.

### 5.2 outbox

```
internal/platform/outbox/
├── outbox.go         # Envelope{ID, Type, AggregateType, AggregateID, Payload(JSONB)}
├── publisher.go      # Insere na mesma Tx do business data
├── dispatcher.go     # Job: SELECT pending → lock → dispatch → mark published
├── reaper.go         # Libera Processing travados após timeout
└── housekeeping.go   # Remove Published antigos (retention_days)
```

Configuração: `OUTBOX_DISPATCHER_TICK_INTERVAL=2s`, batch 50, retry 3x backoff exponencial.

### 5.3 events

```
internal/platform/events/
└── dispatcher.go     # Thread-safe (sync.RWMutex): Register/Dispatch/Remove por event_type
```

Consumers registram-se no bootstrap do worker via `module.EventHandlers`.

### 5.4 workflow (Kernel Genérico)

```
internal/platform/workflow/
├── engine.go         # Engine[S any]: Start(), Resume()
├── store.go          # Snapshot{RunID, Workflow, CorrelationKey, Status, Cursor, State(JSON), Version}
├── step.go           # Step[S]: interface com Execute(ctx, S) → (S, StepResult)
├── definition.go     # Definition[S]: lista ordenada de Steps
├── codec.go          # Codec[S]: Marshal/Unmarshal/MergePatch (RFC 7386)
└── infrastructure/postgres/  # Store adapter: workflow_runs, workflow_steps
```

**Regras críticas (R-WF-KERNEL-001):**
- Zero import de domínio (`internal/agent`, `internal/transactions`, etc.)
- Estados fechados: `RunStatus` (Running/Suspended/Succeeded/Failed), `StepStatus`, `SuspendReason`
- `Resume()` aplica delta merge-patch RFC 7386 sobre `Snapshot.State` — nunca substitui inteiro
- LLM proibido no kernel
- Zero comentários em .go de produção

### 5.5 whatsapp

```
internal/platform/whatsapp/
├── handlers/
│   ├── inbound_handler.go    # POST /messages: HMAC verify → Dispatcher.Route()
│   ├── verify_handler.go     # GET /messages: webhook token challenge (Meta verification)
│   └── status_handler.go     # POST /statuses: atualiza whatsapp_message_status
├── dispatcher/
│   └── dispatcher.go         # Route: dedup → principal → ratelimit → onboard|agent
├── dedup/                    # InsertIfAbsent(WAMID), HousekeepingJob @daily (30d TTL)
│   └── postgres/
├── status/postgres/          # Persiste status de entrega (sent/delivered/read/failed)
├── signature/
│   ├── hmac.go               # HMAC-SHA256 validation
│   └── raw_body_buffer.go    # Preserva raw body para validação
└── ratelimit/                # Per-user rate limiter: 600/min, burst 100
```

### 5.6 Demais Pacotes Platform

| Pacote | Função |
|--------|--------|
| `idempotency/` | Chaves de idempotência 24h TTL (tabela idempotency_keys) |
| `worker/` | Job scheduler (robfig/cron v3), interface Consumer |
| `http/server/health/` | GET /healthz, GET /readyz |
| `http/server/openapi/` | Docs em /__docs |
| `httpclient/` | HTTP client com timeout e retry |
| `notification/adapters/` | Email, SMS, Push notification adapters |
| `channels/` | Routing de canal: WhatsApp / Telegram / Email |
| `money/` | Precisão decimal monetária (cents) |
| `id/` | Geração de UUIDs |
| `stringsutil/` | Utilitários de string |
| `sqlnull/` | Mapeamento SQL NULL ↔ Go types |
| `testcontainer/` | Testcontainers helper para integration tests |

---

## 6. Workflow Kernel em Detalhe

### 6.1 Ciclo de Vida de um Run

```
Engine[S].Start(ctx, definition, correlationKey, initialState)
  │
  ├─ Insere workflow_runs: Status=Running, Cursor=0, State=marshal(initialState)
  │
  ├─ Loop: for cursor < len(steps)
  │    Step[i].Execute(ctx, currentState) → (newState, StepResult)
  │    StepResult:
  │      Continue   → cursor++, persiste newState
  │      Suspend    → Status=Suspended, para execução
  │      Complete   → Status=Succeeded, encerra
  │      Fail(err)  → Status=Failed, encerra
  │    Persiste StepRecord em workflow_steps (RunID, StepID, Seq, Status, DurationMs)
  │
  └─ Retorna RunResult{Status, State, SuspendedAt}

Engine[S].Resume(ctx, definition, correlationKey, resumePayload []byte)
  │
  ├─ Lê Snapshot de workflow_runs WHERE correlationKey AND status=Suspended
  ├─ MergePatch(snapshot.State, resumePayload) → RFC 7386 delta merge
  ├─ Recomeça do cursor salvo
  └─ Mesmo loop de steps acima
```

### 6.2 Tabelas

```sql
workflow_runs (
  run_id          UUID PRIMARY KEY,
  workflow        TEXT NOT NULL,
  correlation_key TEXT NOT NULL,   -- "{userID}:{channel}"
  status          TEXT NOT NULL,   -- RunStatus fechado
  cursor          INT NOT NULL,
  state           JSONB NOT NULL,
  attempts        INT NOT NULL DEFAULT 0,
  max_attempts    INT NOT NULL DEFAULT 3,
  version         INT NOT NULL,    -- CAS para conflitos
  created_at      TIMESTAMPTZ,
  updated_at      TIMESTAMPTZ,
  ended_at        TIMESTAMPTZ
)

workflow_steps (
  id          UUID PRIMARY KEY,
  run_id      UUID REFERENCES workflow_runs,
  step_id     TEXT NOT NULL,
  seq         INT NOT NULL,
  status      TEXT NOT NULL,   -- StepStatus fechado
  duration_ms INT,
  error       TEXT,
  created_at  TIMESTAMPTZ
)
```

---

## 7. Agent: Workflows, Tools e Kernel Steps

### 7.1 WorkflowRegistry

```
internal/agent/application/workflow/registry.go
  IntentRegistry{ byKind map[intent.Kind]IntentWorkflow }

  Resolve(kind) → IntentWorkflow
  IntentWorkflow.Execute(ctx, ToolInput) → ToolResult
    ToolInput{ UserID, Channel, Intent, Text, MessageID }
    ToolResult{ Outcome ToolOutcome, Reply string }
```

Workflows e tools são adaptadores finos: `Tool.Execute()` → `binding` → `usecase` → repositório. Zero regra de negócio, SQL ou branching de domínio.

### 7.2 Kernel Steps (14 steps — ExpenseState)

```
internal/agent/application/workflow/steps/

Ordem de execução:
 1. replay.go            → ReplayStep: busca decision_id em agent_decisions (idempotência)
 2. audit_begin.go       → AuditBeginStep: inicia registro em agent_decisions
 3. authorize.go         → AuthorizeStep: valida userID == principal
 4. policy.go            → PolicyStep: confidence >= AGENT_POLICY_MIN_CONFIDENCE?
 5. resolve_category.go  → ResolveCategoryStep: busca category_id via SearchDictionary
                            → SUSPEND se CategoryAmbiguousError ou CategoryNeedsConfirmationError
                            → salva pendingexpense.Draft{AwaitingKind, TransactionKind, Candidates}
 6. resolve_candidates.go → confirma candidatos após resume
 7. prepare_target.go    → PrepareTargetStep: monta objeto final (entity command)
 8. format.go            → FormatStep: constrói string de reply
 9. persist.go           → PersistStep: chama ExpenseRecorder ou CardPurchaseLogger
                            → binding → usecase → UoW → PostgreSQL (transação atômica)
10. confirm_gate.go      → (para writes destrutivos dentro do kernel)
11. execute_destructive.go → executa operação destrutiva confirmada
12. confirm_guard.go     → validação final pós-confirmação
13. state.go             → StateStep: atualiza agent_working_memory com sumário
14. (housekeeping)       → finaliza agent_decisions, fecha Run
```

**ExpenseState (estrutura de estado JSON no Snapshot):**

```
ExpenseState {
  Kind           intent.Kind
  UserID         string
  Channel        string
  MessageID      string
  DecisionID     uuid.UUID
  Outcome        tools.ToolOutcome    // tipo fechado
  Reply          string
  AmountCents    int64
  CategoryPath   string
  CardName       string
  TransactionID  uuid.UUID
  PendingDraft   pendingexpense.Draft  // quando suspenso em categoria
}
```

### 7.3 ConfirmState Machine (Gates HITL)

```
internal/agent/domain/confirmation/

Operações (OperationKind — tipo fechado):
  OperationDeleteLast    → apagar último lançamento
  OperationEditLast      → editar último lançamento
  OperationDeleteCard    → remover cartão
  OperationDeleteByRef   → apagar por busca
  OperationEditByRef     → editar por busca
  OperationBudgetCommit  → ativar orçamento (gate de budget no commit)

Fluxo:
  confirmEngine.Start(ConfirmState{ OperationKind, AwaitingApproval=AwaitingConfirm })
    → Step confirm_gate → SUSPEND com prompt "Confirma operação? Sim/Não"

  User responde → Resume({ "ResumeText": "sim" })
    → merge-patch sobre Snapshot.State
    → Parse resposta:
       "sim"|"confirmar"|"ok"|"pode" → executa operação → RunStatus=Succeeded
       "não"|"cancelar"              → descarta → RunStatus=Succeeded
       ambíguo 1ª vez               → re-prompt (RepromptCount 0→1), re-suspend
       ambíguo 2ª vez               → cancela → RunStatus=Succeeded
       TTL expirado                 → cancela, devolve handled=false → ParseInbound
```

**Regras críticas:** Nunca retornar pergunta sem persistir ConfirmState com AwaitingConfirm. LLM proibido no confirm_gate. OperationKind e AwaitingApproval são tipos fechados (nunca string livre).

### 7.4 Thread / Run / WorkingMemory

```
internal/agent/domain/entities/

Thread{
  ID      uuid.UUID
  UserID  string
  Channel string       // "whatsapp"
}
  → tabela agent_threads
  → ThreadGateway.GetOrCreate(userID, channel)

Run{
  ID         uuid.UUID
  ThreadID   uuid.UUID
  Workflow   string
  ToolName   string
  IntentKind intent.Kind
  Status     RunStatus   // tipo fechado: running|succeeded|failed
  Outcome    ToolOutcome
  DurationMs int64
}
  → tabela agent_runs
  → RunGateway.Create() / Complete()

WorkingMemory{
  UserID    string
  Content   string      // Markdown estruturado
  UpdatedAt time.Time
}
  → tabela agent_working_memory
  → Incluída no system prompt de ParseInbound quando disponível
  → Atualizada no StateStep após cada lançamento
```

**WorkingMemory e Thread são exclusivos de `internal/agent`.** Outros módulos não têm esses conceitos.

### 7.5 PendingStep (Categoria)

Quando `resolve_category` detecta ambiguidade:

1. Salva `pendingexpense.Draft{ AwaitingKind: category_confirm|category_choice, TransactionKind, Candidates }` em `agent_sessions`
2. Retorna `OutcomeClarify` com prompt
3. Na próxima mensagem, `continuePendingExpenseConfirmation()` intercepta **antes** de `ParseInbound`
4. Resume o kernel com categoria escolhida
5. Draft é limpo (`Clear()`) imediatamente após execução ou cancelamento

---

## 8. Banco de Dados

### 8.1 Schema e Migrations

```
migrations/
├── 000001_initial_schema.up.sql    # Todas as tabelas + índices + FK
└── 000002_seed_reference_data.up.sql  # categories, system defaults
```

Embedded em `migrations/embed.go` via `//go:embed *.sql`. Aplicado com advisory lock para evitar race em múltiplas instâncias.

### 8.2 Tabelas por Bounded Context

**Identity**
```sql
mecontrola.users (
  id UUID, whatsapp_number TEXT, email TEXT, display_name TEXT,
  status TEXT CHECK('ACTIVE','DELETED'), deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
)
mecontrola.user_whatsapp_history (id, user_id, whatsapp_number, changed_at)
mecontrola.user_identities (user_id, channel TEXT, external_id, created_at)
mecontrola.auth_events (id, user_id, event_type, channel, created_at)
```

**Billing**
```sql
mecontrola.billing_plans (id, name, slug, price_cents, interval)
mecontrola.billing_subscriptions (
  id, user_id, plan_id, status TEXT, gateway_subscription_id,
  activated_at, expires_at, grace_expires_at, canceled_at,
  created_at, updated_at
)
```

**Onboarding**
```sql
mecontrola.onboarding_tokens (
  id, user_id, token_hash, status TEXT, expires_at,
  activated_at, created_at
)
mecontrola.onboarding_sessions (
  id, user_id, channel, phase TEXT,
  completed_at, abandoned_at, created_at, updated_at
)
```

**Categories**
```sql
mecontrola.categories (
  id, name, slug, percentage NUMERIC, parent_id,
  is_system BOOL, created_at
)
mecontrola.category_dictionary (
  id, category_id, term TEXT,
  search_vector TSVECTOR   -- TriG index para busca textual
)
```

**Card**
```sql
mecontrola.cards (
  id, user_id, nickname, name,
  closing_day INT, due_day INT, limit_cents BIGINT,
  created_at, updated_at, deleted_at
)
mecontrola.transactions_card_purchases (
  id, user_id, card_id, category_id, description,
  amount_cents BIGINT, installments INT,
  purchase_date DATE, created_at
)
mecontrola.transactions_card_invoices (
  id, card_id, user_id, reference_month DATE,
  total_cents BIGINT, status TEXT, due_date DATE,
  created_at, updated_at
)
```

**Transactions**
```sql
mecontrola.transactions (
  id, user_id, category_id, card_id,
  direction TEXT CHECK('income','outcome'),
  payment_method TEXT, amount_cents BIGINT,
  description TEXT, tags TEXT[],
  occurred_at DATE, frequency TEXT,
  recurrence_rule JSONB, deleted_at,
  created_at, updated_at
)
```

**Budgets**
```sql
mecontrola.budgets (
  id, user_id, total_cents BIGINT, status TEXT,
  reference_month DATE, activated_at,
  created_at, updated_at
)
mecontrola.budgets_allocations (
  id, budget_id, category_id, allocated_cents BIGINT,
  spent_cents BIGINT, created_at, updated_at
)
mecontrola.budgets_expenses (
  id, budget_id, allocation_id, transaction_id,
  amount_cents BIGINT, occurred_at, created_at
)
```

**Agent**
```sql
mecontrola.agent_sessions (
  id, user_id, channel, pending_draft JSONB,
  created_at, updated_at
)
mecontrola.agent_decisions (
  id, user_id, intent_kind TEXT, outcome TEXT,
  confidence NUMERIC, reply TEXT, message_id TEXT,
  prompt_sha256 TEXT, created_at
)
mecontrola.agent_runs (
  id, thread_id, workflow TEXT, tool_name TEXT,
  intent_kind TEXT, status TEXT, outcome TEXT,
  duration_ms BIGINT, error TEXT,
  created_at, ended_at
)
mecontrola.agent_threads (
  id, user_id, channel TEXT,
  created_at, updated_at
)
mecontrola.agent_working_memory (
  user_id TEXT PRIMARY KEY, content TEXT,
  updated_at TIMESTAMPTZ
)
mecontrola.agent_observations (
  id, user_id, observation_text TEXT, created_at
)
```

**Workflow Kernel**
```sql
mecontrola.workflow_runs (
  run_id UUID PRIMARY KEY, workflow TEXT,
  correlation_key TEXT, status TEXT,
  cursor INT, state JSONB, attempts INT,
  max_attempts INT, version INT,
  created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, ended_at TIMESTAMPTZ
)
mecontrola.workflow_steps (
  id UUID, run_id UUID REFERENCES workflow_runs,
  step_id TEXT, seq INT, status TEXT,
  duration_ms INT, error TEXT, created_at TIMESTAMPTZ
)
```

**Platform**
```sql
mecontrola.outbox_events (
  id UUID PRIMARY KEY, event_type TEXT,
  aggregate_type TEXT, aggregate_id TEXT,
  aggregate_user_id TEXT, payload JSONB,
  metadata JSONB,
  status INT,   -- 1=Pending, 2=Processing, 3=Published, 4=Failed
  attempts INT, max_attempts INT,
  created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, published_at TIMESTAMPTZ
)
mecontrola.channel_processed_messages (
  message_id TEXT PRIMARY KEY,   -- WAMID do WhatsApp
  channel TEXT, processed_at TIMESTAMPTZ
)
mecontrola.whatsapp_message_status (
  message_id TEXT PRIMARY KEY,
  status TEXT, updated_at TIMESTAMPTZ
)
mecontrola.idempotency_keys (
  key TEXT PRIMARY KEY,
  response_body JSONB, status_code INT,
  expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ
)
```

---

## 9. Outbox Pattern — Eventos de Domínio

Todos os eventos de domínio são publicados via `outbox.Publisher` **na mesma transação** que a mutação de negócio, garantindo atomicidade.

### 9.1 Eventos por Módulo

| Módulo | event_type | aggregate_type |
|--------|-----------|----------------|
| identity | identity.subscriptionactivated | user |
| identity | identity.authfailed | user |
| billing | billing.subscriptionapproved | subscription |
| billing | billing.subscriptionlate | subscription |
| billing | billing.subscriptioncanceled | subscription |
| onboarding | onboarding.completed | user |
| onboarding | onboarding.bound | user |
| transactions | transactions.transactioncreated | transaction |
| transactions | transactions.transactionupdated | transaction |
| transactions | transactions.cardpurchasecreated | card_purchase |
| budgets | budgets.thresholdcrossed | budget |
| budgets | budgets.expensecommitted | budget |
| agent | agent.whatsappinboundmessage | user |

### 9.2 Consumers por Evento

| event_type | Consumer | Módulo |
|-----------|---------|--------|
| identity.subscriptionactivated | SubscriptionEventProjector | identity |
| onboarding.bound | SubscriptionBoundSessionConsumer | onboarding |
| onboarding.completed | OnboardingCompletedConsumer | agent |
| transactions.transactioncreated | TransactionCreatedConsumer | budgets |
| transactions.transactioncreated | MonthlySummaryRecomputeConsumer | transactions |
| transactions.cardpurchasecreated | CardPurchaseCreatedConsumer | budgets |
| budgets.thresholdcrossed | ThresholdAlertNotifier | budgets |
| agent.whatsappinboundmessage | WhatsAppInboundConsumer | agent |

---

## 10. HTTP — Rotas Registradas

### 10.1 Server Bootstrap (`cmd/server/server.go`)

```
Chi Router (devkit-go/pkg/http_server)
  Middleware: OTel traces, Prometheus metrics, CORS, recover
  │
  ├─ /health                  → health.NewReadinessRouter()
  ├─ /__docs                  → OpenAPI docs (opcional)
  │
  ├─ /api/v1/
  │   ├─ /auth/               → identity: signup, signin
  │   ├─ /users/              → identity: profile, preferences
  │   ├─ /categories/         → categories: list, search
  │   ├─ /cards/              → card: CRUD, invoices
  │   ├─ /transactions/       → transactions: CRUD, summary
  │   ├─ /budgets/            → budgets: CRUD, expenses, summary
  │   └─ /onboarding/         → onboarding: activate, state, checkout
  │
  ├─ /api/v1/whatsapp/
  │   ├─ GET  /messages        → VerifyHandler (webhook challenge Meta)
  │   ├─ POST /messages        → InboundHandler (mensagem recebida)
  │   └─ POST /statuses        → StatusHandler (status de entrega)
  │
  └─ /webhooks/
      └─ /kiwify              → billing: ProcessKiwifyWebhook
```

### 10.2 Middleware de Autenticação

```
identity/infrastructure/http/server/middleware/
  GatewayAuthMiddleware   → HMAC-SHA256 do gateway (API routes)
  JWTMiddleware           → JWT para rotas de usuário autenticado
```

---

## 11. Observabilidade

### 11.1 Stack

```
Aplicação (OTel SDK Go)
  │
  │  gRPC :4317 (ou HTTP :4318)
  ▼
grafana/otel-lgtm:0.7.5 (OtelCollector)
  ├─ Prometheus  :9090  → métricas (scrape /metrics da app)
  ├─ Loki        :3100  → logs JSON estruturados
  └─ Tempo              → traces distribuídos (in-memory)

Grafana :3000
  ├─ Datasource: Prometheus
  ├─ Datasource: Loki
  ├─ Datasource: Tempo
  └─ Dashboards provisionados (deployment/dashboards/):
       agent-runtime-overview.json   → métricas do agent (intents, outcomes, latência)
       mecontrola-api.json           → HTTP (latência, status codes, throughput)
       mecontrola-infra.json         → PostgreSQL, pgBouncer, memória, CPU
       mecontrola-ops.json           → outbox, jobs, consumers
```

### 11.2 Configuração OTel na App

```
OTEL_EXPORTER_OTLP_ENDPOINT=otel-lgtm:4317
OTEL_TRACE_SAMPLE_RATE=0.1     # 10% em produção
```

Bootstrap via devkit-go em `cmd/server/server.go` e `cmd/worker/worker.go`.

### 11.3 Métricas Chave

| Métrica | Labels | Origem |
|---------|--------|--------|
| `outbox_dead_letter_total` | event_type | platform/outbox |
| `outbox_pending_jobs` (gauge) | — | platform/outbox |
| `whatsapp_dispatcher_route_total` | outcome | platform/whatsapp |
| `whatsapp_webhook_rate_limit_exceeded_total` | — | platform/whatsapp |
| `agent_intent_parsed_total` | kind, outcome | agent |
| `agent_intent_routed_total` | kind, channel | agent |
| `agent_authz_denied_total` | reason | agent |
| `agent_policy_blocks_total` | kind | agent |
| `agent_idempotency_replay_total` | kind | agent |
| `workflow_runs_total` | workflow, status | platform/workflow |
| `workflow_run_duration_seconds` | workflow | platform/workflow |
| `workflow_steps_total` | workflow, step, status | platform/workflow |
| HTTP métricas | method, route, status | devkit-go (automático) |

**Cardinalidade controlada:** Nenhum label de alta cardinalidade (`user_id`, `category_id`, `correlation_key`) em métricas de plataforma ou agent.

---

## 12. Infraestrutura de Deploy

### 12.1 Imagem Docker

```
deployment/docker/Dockerfile

Stage builder:  golang:1.26.4-alpine
  CGO_ENABLED=0, GOFLAGS=-trimpath, ldflags: -s -w
  Compila: mecontrola (server|worker|migrate via Cobra)

Stage runtime: gcr.io/distroless/static-debian12:nonroot
  UID/GID: 65532 (nonroot)
  Zero shell, zero package manager
  Entrypoint: /usr/local/bin/mecontrola
```

### 12.2 Compose Files

| Arquivo | Uso | Destaques |
|---------|-----|-----------|
| `compose.yml` | Base | postgres:16-alpine, pgBouncer, migrate, server, worker, caddy |
| `compose.local.yml` | Dev local | + otel-lgtm, + mailpit (email), sem limits |
| `compose.prod.yml` | Produção | read-only rootfs, resource limits, Docker secrets, sem LGTM stack |
| `compose.swarm.yml` | Docker Swarm | replicas, update policy, placement constraints |

Resource limits em produção: server 1 CPU/1GB, worker 0.5 CPU/512MB.
Log driver: json-file, 100MB/arquivo, 10 arquivos, compressed.

### 12.3 Serviços de Infraestrutura

```
PostgreSQL 16-alpine      :5432  → volume persistente, SSL configurável
pgBouncer v1.25.2         :6432  → pool mode transaction, 200 clients, 20 pool size
Caddy 2-alpine            :80/443 → TLS automático (ACME), rate limiting
pgBackrest                → backup S3, encryption (cipher-pass), @daily/@hourly
Terraform                 → AWS: VPC, EC2, S3, Route53, Security Groups
```

---

## 13. Autenticação e Segurança

### 13.1 Autenticação HTTP

```
Gateway (API requests):
  Authorization: <HMAC-SHA256-signature>
  middleware/gateway_auth.go → valida com IDENTITY_GATEWAY_SECRET

JWT (usuário autenticado):
  Authorization: Bearer <token>
  middleware/jwt.go → valida com secret, extrai user_id
```

### 13.2 WhatsApp Webhook

```
X-Hub-Signature-256: sha256=<hex>
→ HMAC-SHA256(WHATSAPP_APP_SECRET, rawBody)
→ 401 se falhar
RawBodyBuffer preserva body original antes do decode JSON
```

### 13.3 PCI RF-16 (Card Module)

Zero PAN / CVV / CVC / track data / PIN em qualquer arquivo Go. Auditado via `task card:audit` com gates R0–R7 + busca por padrões proibidos.

### 13.4 Container Security

```yaml
read_only: true
tmpfs: ["/tmp"]
user: "65532:65532"
cap_drop: [ALL]
security_opt: [no-new-privileges:true]
```

### 13.5 Rate Limiting

| Endpoint | Limite |
|----------|--------|
| WhatsApp por usuário | 600 req/min, burst 100 |
| Onboarding /state | 30 req/min |
| Onboarding /checkout | 10 req/min |
| Kiwify webhook | 60 req/min |

---

## 14. Configuração (`configs/config.go`)

Parsing centralizado via Viper + `.env`. Structs principais:

```
AppConfig          { Environment, Mode }
DatabaseConfig     { Host(pgbouncer), Port(6432), Name, User, Pass, MaxConns(10), SSL }
HTTPConfig         { Port(8080), CORS origins, Timeouts }
OTelConfig         { OTLPEndpoint, Protocol(grpc), TraceSampleRate(0.1), LogFormat(json) }
WhatsAppConfig     { VerifyToken, AppSecret, PhoneNumberID, BusinessAccountID }
AgentConfig        { PrimaryModel, FallbackModel, MaxTokens(768), MinConfidence(0.8),
                     CircuitBreakerFailures(5), CircuitBreakerWindow(30s), Cooldown(60s) }
BillingConfig      { KiwifyAPIKey, WebhookSecret, GracePeriodDays(3) }
OnboardingConfig   { TokenTTLDays(7), LLMModel(claude-haiku-4.5), EmailProvider }
OutboxConfig       { DispatcherEnabled, TickInterval(2s), BatchSize(50), MaxAttempts(3),
                     ReaperTimeout(30m), HousekeepingRetentionDays }
WorkflowConfig     { MaxAttempts(3), RetryBase(200ms), RetryMax(5s),
                     HousekeepingEnabled, HousekeepingSchedule(@daily) }
BudgetsConfig      { ThresholdCategory(80%), ThresholdGoal(50%), ThresholdCard(85%),
                     PurgeRetentionDays, PurgeJobBatchSize(500) }
TransactionsConfig { Timezone(America/Sao_Paulo), IdempotencyTTL(24h) }
EmailConfig        { SMTP, ResendAPIKey }
```

Referência completa em `.env.example` (60+ variáveis).

---

## 15. CI/CD e Governança

### 15.1 GitHub Actions

```
.github/workflows/

ci-cd.yml → push main + PR:
  lint      → golangci-lint (23KB ruleset)
  test      → go test -race -short
  security  → govulncheck + Trivy (imagem)
  build     → docker build multi-stage
  sign      → cosign (assinatura de imagem)

e2e.yml → push main:
  e2e → cucumber/godog (BDD por módulo)
```

### 15.2 Task Orchestration

```bash
task run                # build + executa server local
task check              # lint + unit tests + security
task test:unit          # go test -race -short
task test:integration   # go test -tags=integration (requer Docker)
task lint:run           # golangci-lint
task security:vulncheck # govulncheck
task mocks:generate     # mockery (regenera mocks)
task migrate:up         # aplica migrations
task deploy:compose     # docker compose up (prod)
task swarm:deploy       # docker swarm deploy
task card:audit         # auditoria PCI R0–R7 + RF-16
task ngrok:start        # tunnel local para webhook Meta
```

### 15.3 Regras [HARD] em Vigor

| Regra | Escopo | Resumo |
|-------|--------|--------|
| R-ADAPTER-001 | handlers/, consumers/, producers/, jobs/handlers/ | Zero comentários Go; adapter fino handler→usecase; sem SQL direto |
| R-WF-KERNEL-001 | internal/platform/workflow/ | Kernel genérico; sem import de domínio; estados fechados; sem LLM |
| R-AGENT-WF-001 | internal/agent/ | WorkflowRegistry obrigatório; Thread+Run em toda execução; LLM só em ParseInbound |
| R-TXN-WORKFLOWS-001 | internal/transactions/ | Decide* puro; validação só em smart constructors; producers só mapeiam |
| R-TESTING-001 | */application/usecases/*_test.go | testify/suite whitebox; fake.NewProvider(); IIFE por mock |
| R-DTO-VALIDATE-001 | */application/dtos/input/ | Todo input DTO tem Validate(); use case chama após span.End() |

---

## 16. Diagrama de Componentes

```
┌─────────────────────────────────────────────────────────────────┐
│                    Meta WhatsApp Cloud API                      │
└──────────────────────────┬──────────────────────────────────────┘
                           │ POST /api/v1/whatsapp/messages
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    cmd/server (HTTP :8080)                       │
│  Chi Router                                                     │
│  ├─ identity, categories, billing, onboarding, card handlers   │
│  ├─ transactions, budgets handlers                              │
│  └─ WhatsApp: Verify|Inbound|Status handlers                   │
└──────────────────────────┬──────────────────────────────────────┘
                           │ InboundHandler
                           │ → HMAC verify
                           │ → Dispatcher (dedup + principal + ratelimit)
                           │ → Publica outbox_events (Pending)
                           │ → HTTP 200 ACK
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│              PostgreSQL 16 via pgBouncer (:6432)                │
│  outbox_events (status=Pending)                                 │
└──────────────────────────┬──────────────────────────────────────┘
                           │ SELECT + lock (tick 2s, batch 50)
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    cmd/worker                                    │
│  OutboxDispatcherJob                                            │
│  → events.Dispatcher.Dispatch(event_type)                      │
│  → WhatsAppInboundConsumer                                      │
│       → IntentRouter                                            │
│            → tryResumeInbound() [pending expense/approval/plan] │
│            → ParseInbound (OpenRouter LLM)                      │
│                 Primary:  gemini-2.5-flash-lite                 │
│                 Fallback: mistral-small-3.2-24b                 │
│            → DailyLedgerAgent                                   │
│                 ├─ READ  → WorkflowRegistry → Tool → binding    │
│                 ├─ WRITE → Workflow → Tool → binding → UoW      │
│                 ├─ KERNEL → Engine[ExpenseState] → 14 steps     │
│                 │           → persist step → UoW → PostgreSQL   │
│                 └─ DESTRUCTIVE → Engine[ConfirmState]           │
│                                  → suspend → approve → execute  │
│  + outros consumers (budgets, transactions, card, onboarding)  │
│  + jobs agendados (cron): reaper, reconciliation, alerts, purge │
└──────────────────┬───────────────────────┬──────────────────────┘
                   │ writes (UoW + Tx)      │ reply
                   ▼                        ▼
┌──────────────────────────┐   ┌───────────────────────────────┐
│  PostgreSQL 16            │   │  Meta WhatsApp Cloud API      │
│  schema mecontrola        │   │  SendTextMessage → usuário    │
│  users, cards             │   └───────────────────────────────┘
│  transactions, budgets    │
│  agent_threads/runs/mem   │
│  workflow_runs/steps      │
│  outbox_events            │
└──────────────┬────────────┘
               │ métricas/traces/logs
               ▼
┌─────────────────────────────┐
│  OTel Collector             │
│  → Prometheus :9090         │
│  → Loki :3100               │
│  → Tempo                    │
│  Grafana :3000              │
│  4 dashboards provisionados │
└─────────────────────────────┘
```

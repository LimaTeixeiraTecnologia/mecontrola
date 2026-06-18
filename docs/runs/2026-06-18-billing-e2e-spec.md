# Plano de Cobertura 100% — Módulo `internal/billing`

**Data:** 2026-06-18
**Skill obrigatória na execução:** `go-implementation`
**Referências obrigatórias por camada:** ver Matriz de Referências (Seção 7)

---

## 1. Inventário Descoberto

### 1.1 HTTP Endpoint

| Método | Rota | Handler | Status esperado |
|--------|------|---------|-----------------|
| POST | `/webhooks/kiwify` | `KiwifyWebhookHandler.Handle` | 202 Accepted |

### 1.2 Use Cases

| Arquivo | Struct | Interface interna |
|---------|--------|------------------|
| `process_sale_approved.go` | `ProcessSaleApproved` | `processSaleApproved` |
| `process_subscription_renewed.go` | `ProcessSubscriptionRenewed` | `processSubscriptionRenewed` |
| `process_subscription_late.go` | `ProcessSubscriptionLate` | `processSubscriptionLate` |
| `process_subscription_canceled.go` | `ProcessSubscriptionCanceled` | `processSubscriptionCanceled` |
| `process_refund_or_chargeback.go` | `ProcessRefundOrChargeback` | `processRefundOrChargeback` |
| `process_kiwify_webhook.go` | `ProcessKiwifyWebhook` | orquestrador |
| `process_subscription_grace_expired.go` | `ProcessSubscriptionGraceExpired` | `processGraceExpiredUseCase` |
| `reconcile_subscriptions.go` | `ReconcileSubscriptions` | — |
| `run_reconciliation.go` | `RunReconciliation` | `runReconciliationUseCase` |
| `cleanup_kiwify_events.go` | `CleanupKiwifyEvents` | `cleanupKiwifyEventsUseCase` |
| `send_subscription_notification.go` | `SendSubscriptionNotification` | `sendSubscriptionNotificationUseCase` |

### 1.3 Domain Services (`Decide*`)

| Arquivo | Funções |
|---------|---------|
| `domain/services/decisions.go` | `DecideRenewal`, `DecidePastDue`, `DecideCancellation` |
| `domain/services/transitions.go` | `CanTransition`, `IsRegression`, `TargetStatus` |

### 1.4 Producers (Outbox)

7 event types publicados via `outbox.Publisher`:

| Evento | Método |
|--------|--------|
| `billing.subscription.activated` | `PublishActivated` |
| `billing.subscription.activated_without_token` | `PublishActivatedWithoutToken` |
| `billing.subscription.renewed` | `PublishRenewed` |
| `billing.subscription.past_due` | `PublishPastDue` |
| `billing.subscription.canceled` | `PublishCanceled` |
| `billing.subscription.refunded` | `PublishRefunded` |
| `billing.subscription.expired_after_grace` | `PublishExpired` |

### 1.5 Consumers

| Arquivo | Handler | Eventos consumidos |
|---------|---------|-------------------|
| `notification_handler.go` | `NotificationHandler` | `past_due`, `refunded`, `expired_after_grace` |

### 1.6 Jobs

| Arquivo | Job name | Schedule | Idempotência |
|---------|----------|----------|-------------|
| `grace_expiration_job.go` | `billing-grace-expiration` | `@every 30m` | batch `ListPastDueGraceExpired` → `ProcessSubscriptionGraceExpired` |
| `reconciliation_job.go` | `billing-reconciliation` | configurable | checkpoint watermark |
| `kiwify_events_housekeeping_job.go` | `billing-kiwify-events-housekeeping` | `@daily` | delete by `received_at < threshold` |

### 1.7 Tabelas

`billing_plans`, `billing_subscriptions`, `billing_processed_events`, `billing_kiwify_events`, `billing_reconciliation_checkpoints`

### 1.8 Testes Existentes (41 arquivos)

**Já cobertos (não reescrever):**

| Camada | Arquivos existentes |
|--------|---------------------|
| Domain | `subscription_test.go`, `status_test.go`, `plan_test.go`, `funnel_token_test.go`, `kiwify_subscription_id_test.go`, `grace_window_test.go`, `transitions_test.go`, `decisions_test.go` |
| Application UC | `process_sale_approved_test.go`, `process_subscription_renewed_test.go`, `process_subscription_late_test.go`, `process_subscription_canceled_test.go`, `process_refund_or_chargeback_test.go`, `process_subscription_grace_expired_test.go`, `process_kiwify_webhook_test.go`, `reconcile_subscriptions_test.go`, `funnel_token_test.go` |
| HTTP handler | `kiwify_webhook_handler_test.go`, `kiwify_webhook_integration_test.go` |
| Middleware | `hmac_signature_test.go`, `raw_body_buffer_test.go` |
| HTTP client | `client_test.go` |
| Jobs | `reconciliation_job_test.go`, `kiwify_events_housekeeping_job_test.go`, `reconciliation_integration_test.go` |
| Producers | `subscription_event_publisher_test.go`, `subscription_event_publisher_integration_test.go` |
| Repositories | `subscription_repository_integration_test.go`, `plan_repository_integration_test.go`, `kiwify_event_repository_integration_test.go`, `processed_event_repository_integration_test.go`, `reconciliation_checkpoint_repository_integration_test.go`, `testutil_test.go` |
| E2E | `f01..f09`, `suite_test.go`, `ctx_test.go`, `steps_webhook_test.go`, `steps_shared_test.go`, `steps_outbox_test.go`, `steps_consumer_test.go`, `steps_job_test.go` |
| Module | `module_test.go` |

---

## 2. Gaps de Cobertura

A tabela abaixo lista o que FALTA para fechar 100% do módulo:

| # | Camada | Tipo | Arquivo a criar | Razão |
|---|--------|------|-----------------|-------|
| G1 | Application UC | Unit | `run_reconciliation_test.go` | `RunReconciliation` sem teste; orquestra `ReconcileSubscriptions` via job trigger |
| G2 | Application UC | Unit | `cleanup_kiwify_events_test.go` | `CleanupKiwifyEvents` sem teste; delega para `KiwifyEventRepository.DeleteOlderThan` |
| G3 | Application UC | Unit | `send_subscription_notification_test.go` | `SendSubscriptionNotification` sem teste; usa `NotificationSender` |
| G4 | Jobs | Unit | `grace_expiration_job_test.go` | `grace_expiration_job.go` não tem `_test.go`; path unit isolado |
| G5 | Jobs | Integration | `grace_expiration_job_integration_test.go` | sem asserção de banco real para batch de expiração |
| G6 | Consumer | Unit | `notification_handler_test.go` | `notification_handler.go` sem teste unit com mockery |
| G7 | Consumer | Integration | `notification_handler_integration_test.go` | idempotência por `event_id` não verificada contra Postgres real |

---

## 3. Plano Detalhado por Gap

### G1 — `run_reconciliation_test.go`

**Arquivo:** `internal/billing/application/usecases/run_reconciliation_test.go`
**Build tag:** nenhuma (teste unit puro, sem Docker)

**Cenários:**

| Cenário | Setup | Assert |
|---------|-------|--------|
| happy path | mock `ReconcileSubscriptions` retorna `nil` | `Execute(ctx)` retorna `nil`; mock `.EXPECT()…Once()` satisfeito |
| erro propagado | mock retorna `errXxx` | `Execute(ctx)` retorna o mesmo erro via `%w` |

**Mocks a usar:** `mocks.ReconcileSubscriptions` (ou interface interna equivalente declarada em `run_reconciliation.go`).

---

### G2 — `cleanup_kiwify_events_test.go`

**Arquivo:** `internal/billing/application/usecases/cleanup_kiwify_events_test.go`
**Build tag:** nenhuma

**Cenários:**

| Cenário | Setup | Assert |
|---------|-------|--------|
| happy path | mock `KiwifyEventRepository.DeleteOlderThan` retorna `(n, nil)` | retorna `nil`; linhas deletadas logadas/propagadas conforme implementação |
| erro de repositório | `DeleteOlderThan` retorna `(0, errDB)` | `Execute(ctx)` retorna erro; `errors.Is` ou comparação de sentinela |

---

### G3 — `send_subscription_notification_test.go`

**Arquivo:** `internal/billing/application/usecases/send_subscription_notification_test.go`
**Build tag:** nenhuma

**Cenários:**

| Cenário | Setup | Assert |
|---------|-------|--------|
| notificação enviada com sucesso | mock `NotificationSender.NotifyTransition` retorna `nil` | retorna `nil`; sender chamado exatamente 1× com input mapeado |
| sender retorna erro | `NotifyTransition` retorna `errSend` | erro propagado; `errors.Is` |
| input com status terminal (sem transição aplicável) | input com status que não gera notificação (regra de negócio do handler) | comportamento conforme implementação — documentar se skip ou erro |

---

### G4 — `grace_expiration_job_test.go` (unit)

**Arquivo:** `internal/billing/infrastructure/jobs/handlers/grace_expiration_job_test.go`
**Build tag:** nenhuma

**Cenários:**

| Cenário | Setup | Assert |
|---------|-------|--------|
| execução normal | mock `processGraceExpiredUseCase.Execute` retorna `nil` | job retorna `nil`; use case chamado 1× |
| use case retorna erro | `Execute` retorna `errUC` | job propaga o erro |
| contexto cancelado antes de executar | `ctx` já cancelado | resposta rápida com `ctx.Err()` ou erro de contexto |

**Padrão de construção:** seguir `reconciliation_job_test.go` como canônico.

---

### G5 — `grace_expiration_job_integration_test.go`

**Arquivo:** `internal/billing/infrastructure/jobs/handlers/grace_expiration_job_integration_test.go`
**Build tag:** `//go:build integration`

**Cenários:**

| Cenário | Pré-condição no banco | Ação | Verificação de banco |
|---------|----------------------|------|----------------------|
| batch de expiração: todas as `past_due` com `grace_end < now` expiram | inserir N assinaturas `past_due` com `grace_end` no passado via SQL direto | executar job | `SELECT COUNT(*) FROM billing_subscriptions WHERE status = 'past_due'` = 0; `SELECT COUNT(*) FROM billing_subscriptions WHERE status = 'expired'` = N; `SELECT COUNT(*) FROM outbox_events WHERE event_type = 'billing.subscription.expired_after_grace'` = N |
| execução dupla é idempotente | mesmas N assinaturas já expiradas | executar job segunda vez | contagens inalteradas; `outbox_events` não duplicou |
| nenhuma assinatura expirada | banco sem `past_due` com `grace_end < now` | executar job | retorna `nil`; sem linhas inseridas em `outbox_events` |

**Helper sugerido:**

```go
func insertPastDueSubscription(t *testing.T, db database.DBTX, graceEnd time.Time) uuid.UUID
func countSubscriptionsByStatus(t *testing.T, db database.DBTX, status string) int
func countOutboxByEventType(t *testing.T, db database.DBTX, eventType string) int
```

---

### G6 — `notification_handler_test.go` (unit)

**Arquivo:** `internal/billing/infrastructure/messaging/database/consumers/notification_handler_test.go`
**Build tag:** nenhuma

**Cenários:**

| Cenário | Setup | Assert |
|---------|-------|--------|
| evento `past_due` processado | mock `SendSubscriptionNotification.Execute` retorna `nil` | handler retorna `nil`; use case chamado 1× com input correto |
| evento `refunded` processado | idem | idem |
| evento `expired_after_grace` processado | idem | idem |
| use case retorna erro | mock retorna `errUC` | handler propaga erro |
| evento com `event_id` desconhecido | handler configurado apenas para tipos conhecidos | comportamento conforme implementação (skip ou erro) |

**Mocks a usar:** `mocks.SendSubscriptionNotification` (ou interface interna `sendSubscriptionNotificationUseCase`).

---

### G7 — `notification_handler_integration_test.go`

**Arquivo:** `internal/billing/infrastructure/messaging/database/consumers/notification_handler_integration_test.go`
**Build tag:** `//go:build integration`

**Cenários:**

| Cenário | Pré-condição | Ação | Verificação de banco |
|---------|-------------|------|----------------------|
| processa evento `past_due` → notificação enviada | assinatura `past_due` no banco; `NotificationSender` stub que grava chamadas | entregar evento via outbox row | `SELECT COUNT(*) FROM outbox_events WHERE status = 'published'` = 1 |
| idempotência: reprocessar mesmo `event_id` não duplica | evento já marcado `published` | reprocessar envelope com mesmo `ID` | `SELECT COUNT(*)` de notificações enviadas = 1 (stub contabiliza) |
| evento com tipo não registrado | handler configurado para `past_due` | entregar evento `billing.subscription.activated` | handler retorna `nil` sem chamar notificação |

**Helper sugerido:**

```go
type stubNotificationSender struct{ calls []input.SendSubscriptionNotificationInput }
func (s *stubNotificationSender) NotifyTransition(ctx context.Context, in input.SendSubscriptionNotificationInput) error
func countPublishedOutboxByType(t *testing.T, db database.DBTX, eventType string) int
```

---

## 4. E2E — Revisão das Features Existentes

As 9 features (`f01..f09`) já cobrem os fluxos principais. Nenhuma feature nova é necessária.

**Revisão de completude recomendada (sem criar features novas):**

| Feature | Verificação de banco esperada | Status |
|---------|------------------------------|--------|
| `f01_sale_approved.feature` | `billing_subscriptions` tem 1 linha; `outbox_events` tem 1 linha `activated` | confirmar em `steps_shared_test.go` |
| `f07_outbox_producer.feature` | campos `aggregate_type`, `aggregate_id`, `aggregate_user_id` verificados via `SELECT` direto | confirmar em `steps_outbox_test.go` |
| `f08_consumer_notification.feature` | `NotificationSender` stub verificado 1× chamado + outbox `published` | confirmar em `steps_consumer_test.go` |
| `f09_grace_expiration_job.feature` | `billing_subscriptions.status = expired` + `outbox_events` count | confirmar em `steps_job_test.go` |

Se alguma verificação estiver ausente nos steps, adicionar asserção SQL no step correspondente (não criar nova feature).

---

## 5. Gherkin — Nenhuma Feature Nova Necessária

Todas as jornadas de negócio estão cobertas em `f01..f09`. O plano não acrescenta features.

Se durante a execução for detectado que uma feature existente não verifica estado de banco, o step correspondente deve ser **modificado** para adicionar a asserção, não uma feature nova.

---

## 6. Estratégia de Validação de Banco (Evidence Strategy)

### 6.1 Padrão de Helper

Cada suite de integração/E2E deve declarar helpers locais:

```go
func countRows(t *testing.T, db database.DBTX, table, where string, args ...any) int {
    t.Helper()
    var n int
    err := db.QueryRowContext(context.Background(),
        "SELECT COUNT(*) FROM "+table+" WHERE "+where, args...).Scan(&n)
    require.NoError(t, err)
    return n
}
```

Espelhar `countOutboxByType` de `establish_principal_integration_test.go` e `countAuthEvents` de `auth_events_consumer_integration_test.go`.

### 6.2 Asserções obrigatórias por operação

| Operação | Asserção SQL obrigatória |
|----------|--------------------------|
| Criar assinatura | `COUNT(*) WHERE id = $1` = 1 |
| Atualizar status | `SELECT status WHERE id = $1` = valor esperado |
| Soft-delete (`deleted_at`) | `SELECT deleted_at WHERE id = $1 IS NOT NULL` |
| Publicar evento outbox | `COUNT(*) FROM outbox_events WHERE event_type = $1 AND aggregate_id = $2` = N |
| Idempotência consumer | `COUNT(*)` estável após 2º processamento |
| Rollback transacional | `COUNT(*) FROM outbox_events WHERE aggregate_id = $1` = 0 |

### 6.3 Isolamento de banco

Cada teste de integração usa `NewTestDatabase` de `internal/platform/database/postgres/test_helper.go` com `t.Cleanup` para drop.
Transactions são isoladas por teste quando possível; fixtures via SQL direto com `db.ExecContext`.

---

## 7. Matriz de Referências go-implementation por Gap

| Gap | Referências obrigatórias | Sob demanda |
|-----|--------------------------|-------------|
| G1, G2, G3 (UC unit) | `architecture.md`, `testing-unit.md` | — |
| G4 (job unit) | `architecture.md`, `job-handler.md`, `testing-unit.md` | — |
| G5 (job integration) | `architecture.md`, `job-handler.md`, `testing-integration.md` | `examples-testing.md` (se suite stateful) |
| G6 (consumer unit) | `architecture.md`, `consumer.md`, `testing-unit.md` | — |
| G7 (consumer integration) | `architecture.md`, `consumer.md`, `testing-integration.md` | `examples-testing.md`, `graceful-lifecycle.md` |

Máximo 4 referências simultâneas. Se conflito, priorizar `architecture.md`, referência específica do tipo de adapter e `testing-integration.md`.

---

## 8. Gates de Definition of Done

Antes de declarar 100% coberto, TODOS os gates abaixo devem passar e ser reportados como evidência:

```bash
# 1. Testes unitários com race detector
task test:unit

# 2. Testes de integração (requer Docker)
task test:integration

# 3. Testes E2E
task test:e2e

# 4. Zero comentários em .go de produção (R-ADAPTER-001.1)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/billing/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentarios proibidos" && exit 1 || true

# 5. Sem SQL direto em adapters (R-ADAPTER-001.2)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/billing/infrastructure/http/server/handlers/ \
  internal/billing/infrastructure/messaging/database/consumers/ \
  internal/billing/infrastructure/messaging/database/producers/ \
  internal/billing/infrastructure/jobs/handlers/ \
  && echo "FAIL: SQL direto em adapter" && exit 1 || true

# 6. Lint
golangci-lint run ./internal/billing/...

# 7. Build + vet
go build ./internal/billing/...
go vet ./internal/billing/...
```

---

## 9. Sequência de Execução (Orquestração por Subagents)

Conforme regra do repositório, trabalho sequencial no main loop é proibido para cobertura ampla.
Cada gap é executado em paralelo por subagent independente:

| Subagent | Responsabilidade | Outputs esperados |
|----------|-----------------|-------------------|
| SA-G1 | `run_reconciliation_test.go` | arquivo criado, `go test` verde |
| SA-G2 | `cleanup_kiwify_events_test.go` | arquivo criado, `go test` verde |
| SA-G3 | `send_subscription_notification_test.go` | arquivo criado, `go test` verde |
| SA-G4 | `grace_expiration_job_test.go` | arquivo criado, `go test` verde |
| SA-G5 | `grace_expiration_job_integration_test.go` + helpers | arquivo criado, banco verificado |
| SA-G6 | `notification_handler_test.go` | arquivo criado, `go test` verde |
| SA-G7 | `notification_handler_integration_test.go` + stub | arquivo criado, banco verificado |
| SA-E2E | revisar steps existentes f07/f08/f09 para verificação de banco | steps atualizados se gaps detectados |

Síntese final: consolidar evidências de todos os subagents antes de declarar o módulo 100% coberto.

---

## 10. Restrições de Implementação (lembrete)

- Zero comentários em `.go` de produção — inegociável (R-ADAPTER-001.1).
- Gherkin e regex dos steps em PT-BR; métodos/steps Go em inglês.
- Build tag `//go:build integration` em todo teste que sobe container.
- Testes unit rodam com `-short` sem Docker.
- Sem `var _ Interface = (*Type)(nil)` — R6.4.
- Sem `Clock` interface — usar `time.Now().UTC()` ou receber `time.Time` por parâmetro — R6.7.
- Nenhum falso positivo é aceitável: se teste quebra, corrigir o código, não o teste.

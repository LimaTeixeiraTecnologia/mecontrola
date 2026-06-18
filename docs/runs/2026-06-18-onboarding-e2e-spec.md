# Plano de Cobertura 100% — Módulo `internal/onboarding`

Data: 2026-06-18
Skill obrigatória: `go-implementation`
Status: planejamento (sem modificação de arquivos)

---

## 1. Inventário Descoberto

### 1.1 Endpoints HTTP

| Método | Rota | Handler | Status possíveis |
|--------|------|---------|-----------------|
| POST | `/api/v1/onboarding/checkout` | `CreateCheckoutHandler.Handle` | 201, 400, 500, 503 |
| OPTIONS | `/api/v1/onboarding/checkout` | CORS preflight | 204 |
| GET | `/api/v1/onboarding/tokens/{token}/state` | `TokenStateHandler.Handle` | 200, 500 |
| OPTIONS | `/api/v1/onboarding/tokens/{token}/state` | CORS preflight | 204 |

Middleware registrado: `RateLimitMiddleware` (429 quando excedido).

### 1.2 Use Cases (13)

| Use Case | Caminho | Caller |
|----------|---------|--------|
| `CreateCheckoutSession` | `application/usecases/` | HTTP handler |
| `GetTokenState` | `application/usecases/` | HTTP handler |
| `MarkTokenPaid` | `application/usecases/` | `SubscriptionPaidConsumer` |
| `SendActivationEmail` | `application/usecases/` | `ActivationEmailConsumer` |
| `ConsumeMagicToken` | `application/usecases/` | Message processors (WA/TG) |
| `TryFallbackActivation` | `application/usecases/` | Message processors |
| `ActivateTelegramByToken` | `application/usecases/` | `TelegramMessageProcessor` |
| `ProcessOnboardingMessage` | `application/usecases/` | WA + TG processors |
| `StartBudgetConfiguration` | `application/usecases/` | `SubscriptionBoundSessionConsumer` |
| `SendOutreach` | `application/usecases/` | `OutreachJob` |
| `ExpireTokens` | `application/usecases/` | `TokenExpirationJob` |
| `HandlePaidWithoutToken` | `application/usecases/` | `PaidWithoutTokenConsumer` |
| `CleanupOnboardingTables` | `application/usecases/` | `MetaProcessedMessagesCleanupJob` |

### 1.3 Domínio

- **Entidades:** `MagicToken`, `OnboardingSession`, `SupportSignal`
- **Value Objects:** `TokenStatus`, `OnboardingState`, `SupportSignalKind`, `ActivationPath`
- **Workflows (Decide*):** `MagicTokenWorkflow`, `OnboardingWorkflow`, `DirectTelegramActivationWorkflow`
- **Eventos de domínio:** `IncomeRegistered`, `CardRegistered`, `SplitsCalculated`, `OnboardingCompleted`

### 1.4 Repositórios

| Interface | Arquivo | Tabela | Teste integração |
|-----------|---------|--------|-----------------|
| `MagicTokenRepository` | `magic_token_repository.go` | `mecontrola.onboarding_tokens` | **AUSENTE** |
| `OnboardingSessionRepository` | `onboarding_session_repository.go` | `mecontrola.onboarding_sessions` | Parcial (1 método) |
| `SupportSignalRepository` | `support_signal_repository.go` | `mecontrola.support_signals` | **AUSENTE** |
| `OnboardingCleanupRepository` | `onboarding_cleanup_repository.go` | `meta_processed_messages`, `consumer_lookup_attempts` | **AUSENTE** |

### 1.5 Consumers (4)

| Consumer | Event Type | Use Case | Teste unit |
|----------|-----------|----------|------------|
| `SubscriptionPaidConsumer` | `billing.subscription.activated` | `MarkTokenPaid` | Sim (4 cenários) |
| `ActivationEmailConsumer` | `billing.subscription.activated` | `SendActivationEmail` | **AUSENTE** |
| `PaidWithoutTokenConsumer` | `billing.subscription.activated_without_token` | `HandlePaidWithoutToken` | Sim (4 cenários) |
| `SubscriptionBoundSessionConsumer` | `onboarding.subscription_bound` | `StartBudgetConfiguration` | Sim (4 cenários) |

### 1.6 Outbox Publisher

| Evento | Event Type | Arquivo | Teste integração |
|--------|-----------|---------|-----------------|
| `SubscriptionBound` | `onboarding.subscription_bound` | `application/events/subscription_bound.go` | Apenas unit (`subscription_bound_test.go`) — sem verificação de inserção em `outbox_events` |

### 1.7 Jobs (3)

| Job | Schedule | Use Case | Teste integração |
|-----|---------|----------|-----------------|
| `OutreachJob` | `5 * * * *` | `SendOutreach` | Coberto via E2E |
| `TokenExpirationJob` | config | `ExpireTokens` | Coberto via E2E |
| `MetaProcessedMessagesCleanupJob` | config | `CleanupOnboardingTables` | Coberto via E2E |

### 1.8 E2E existente (26 cenários em 6 feature files)

| Feature file | Cenários | Cobertura |
|-------------|----------|-----------|
| `checkout_http.feature` | 4 | 201, JSON inválido, plano desconhecido, CORS |
| `token_state_http.feature` | 4 | PAID pronto, PENDING, EXPIRED, não encontrado |
| `billing_consumers.feature` | 2 | `subscription.activated` → paid + email; `activated_without_token` → signal |
| `activation_processors.feature` | 2 | Ativação WA; Ativação Telegram direto |
| `jobs.feature` | 3 | Outreach, expiração, cleanup dedup |
| `robustness.feature` | 11 | Fallback, reuse, Telegram bloqueado, outreach falha, conversa renda |

---

## 2. Gaps de Cobertura (Matriz de Obrigações)

| Camada | Gap | Severidade | Arquivo a criar |
|--------|-----|-----------|----------------|
| Consumer unit | `ActivationEmailConsumer` sem `_test.go` | **ALTA** | `activation_email_consumer_test.go` |
| Repositório integração | `MagicTokenRepository` sem integration test | **ALTA** | `magic_token_repository_integration_test.go` |
| Repositório integração | `SupportSignalRepository` sem integration test | **MÉDIA** | `support_signal_repository_integration_test.go` |
| Repositório integração | `OnboardingCleanupRepository` sem integration test | **MÉDIA** | `onboarding_cleanup_repository_integration_test.go` |
| Outbox integração | `subscription_bound.go` não valida inserção em `outbox_events` | **ALTA** | `subscription_bound_integration_test.go` |
| E2E | Rate limiter (429) sem cenário Gherkin | **MÉDIA** | extensão de `robustness.feature` |
| E2E | `ActivationEmailConsumer` sem cenário isolado de falha | **MÉDIA** | extensão de `billing_consumers.feature` |
| E2E | `OnboardingSessionRepository.Upsert` com `version` increment | **BAIXA** | extensão de integration test existente |

---

## 3. Novos Cenários Gherkin Propostos

### 3.1 Extensão: `billing_consumers.feature`

```gherkin
# language: pt
Funcionalidade: Consumers de eventos de billing no onboarding

  # ... cenários existentes preservados ...

  Cenário: consumer de email propaga erro do use case SendActivationEmail
    Dado que existe um evento "billing.subscription.activated" com funnel_token válido
    E que o use case SendActivationEmail retorna erro de gateway de email
    Quando o consumer ActivationEmail processa o evento
    Então o consumer deve retornar o erro sem engolir

  Cenário: consumer de email ignora evento sem customer_email
    Dado que existe um evento "billing.subscription.activated" com funnel_token válido
    E que o payload não contém o campo customer_email
    Quando o consumer ActivationEmail processa o evento
    Então o consumer deve concluir sem erro e sem chamar o gateway de email

  Cenário: consumer de email rejeita payload de tipo inesperado
    Dado que existe um evento de tipo desconhecido para o consumer de email
    Quando o consumer ActivationEmail tenta processar o evento
    Então o consumer deve retornar erro de payload inesperado
```

### 3.2 Extensão: `robustness.feature`

```gherkin
# language: pt
Funcionalidade: Robustez e limites do módulo onboarding

  # ... cenários existentes preservados ...

  Cenário: requisições em excesso ao endpoint de checkout retornam 429
    Dado que o ambiente de teste para onboarding está pronto
    E que o rate limiter está configurado com limite de 2 requisições por minuto
    Quando o usuário envia 3 requisições POST para "/api/v1/onboarding/checkout" em sequência
    Então a terceira resposta HTTP deve ser 429 Too Many Requests

  Cenário: requisições em excesso ao endpoint de estado do token retornam 429
    Dado que o ambiente de teste para onboarding está pronto
    E que o rate limiter está configurado com limite de 2 requisições por minuto
    Quando o usuário envia 3 requisições GET para o token state em sequência
    Então a terceira resposta HTTP deve ser 429 Too Many Requests
```

### 3.3 Novo: `outbox_publisher.feature`

```gherkin
# language: pt
Funcionalidade: Publicação de eventos onboarding no outbox

  Cenário: ativação via token persiste evento subscription_bound na mesma transação
    Dado que o ambiente de teste para onboarding está pronto
    E que existe um magic_token no status PAID para o usuário "user-a"
    Quando o use case ConsumeMagicToken é executado com o token válido
    Então a tabela outbox_events deve conter exatamente 1 linha com event_type "onboarding.subscription_bound"
    E o campo aggregate_id deve corresponder ao id do magic_token
    E o campo user_id deve corresponder ao usuário que consumiu o token

  Cenário: rollback da transação não persiste evento no outbox
    Dado que o ambiente de teste para onboarding está pronto
    E que existe um magic_token no status PAID para o usuário "user-b"
    E que o repositório está configurado para falhar após a escrita do evento
    Quando o use case ConsumeMagicToken é executado com o token válido
    Então a tabela outbox_events deve estar vazia para o aggregate_id do token
    E a linha do magic_token não deve ter o status CONSUMED

  Cenário: reprocessamento do mesmo event_id não duplica linha no outbox
    Dado que o ambiente de teste para onboarding está pronto
    E que o evento com event_id "evt-xpto" já foi publicado no outbox
    Quando o publisher tenta publicar novamente o mesmo event_id "evt-xpto"
    Então a tabela outbox_events deve conter exatamente 1 linha com event_id "evt-xpto"
```

---

## 4. Definições de Steps Go (assinaturas em Inglês, regex PT-BR)

### 4.1 Steps para `billing_consumers.feature` — ActivationEmailConsumer

```go
// Arquivo: internal/onboarding/e2e/feature_billing_outbox_steps_test.go (extensão)

func (s *OnboardingSuite) registerActivationEmailConsumerSteps(ctx *godog.ScenarioContext) {
    ctx.Step(
        `^que existe um evento "([^"]*)" com funnel_token válido$`,
        s.givenBillingEventWithFunnelToken,
    )
    ctx.Step(
        `^que o use case SendActivationEmail retorna erro de gateway de email$`,
        s.givenSendActivationEmailGatewayFails,
    )
    ctx.Step(
        `^que o payload não contém o campo customer_email$`,
        s.givenPayloadWithoutCustomerEmail,
    )
    ctx.Step(
        `^que existe um evento de tipo desconhecido para o consumer de email$`,
        s.givenUnknownEventTypeForEmailConsumer,
    )
    ctx.Step(
        `^o consumer ActivationEmail processa o evento$`,
        s.whenActivationEmailConsumerHandles,
    )
    ctx.Step(
        `^o consumer ActivationEmail tenta processar o evento$`,
        s.whenActivationEmailConsumerHandles,
    )
    ctx.Step(
        `^o consumer deve retornar o erro sem engolir$`,
        s.thenConsumerReturnsError,
    )
    ctx.Step(
        `^o consumer deve concluir sem erro e sem chamar o gateway de email$`,
        s.thenConsumerSucceedsWithoutEmailSent,
    )
    ctx.Step(
        `^o consumer deve retornar erro de payload inesperado$`,
        s.thenConsumerReturnsUnexpectedPayloadError,
    )
}

func (s *OnboardingSuite) givenBillingEventWithFunnelToken(eventType string) error
func (s *OnboardingSuite) givenSendActivationEmailGatewayFails() error
func (s *OnboardingSuite) givenPayloadWithoutCustomerEmail() error
func (s *OnboardingSuite) givenUnknownEventTypeForEmailConsumer() error
func (s *OnboardingSuite) whenActivationEmailConsumerHandles(ctx context.Context) error
func (s *OnboardingSuite) thenConsumerReturnsError(ctx context.Context) error
func (s *OnboardingSuite) thenConsumerSucceedsWithoutEmailSent(ctx context.Context) error
func (s *OnboardingSuite) thenConsumerReturnsUnexpectedPayloadError(ctx context.Context) error
```

### 4.2 Steps para `robustness.feature` — Rate Limiter

```go
// Arquivo: internal/onboarding/e2e/feature_robustness_steps_test.go (extensão)

func (s *OnboardingSuite) registerRateLimitSteps(ctx *godog.ScenarioContext) {
    ctx.Step(
        `^que o rate limiter está configurado com limite de (\d+) requisições por minuto$`,
        s.givenRateLimiterWithRequestsPerMinute,
    )
    ctx.Step(
        `^o usuário envia (\d+) requisições POST para "([^"]*)" em sequência$`,
        s.whenUserSendsNPostRequests,
    )
    ctx.Step(
        `^o usuário envia (\d+) requisições GET para o token state em sequência$`,
        s.whenUserSendsNGetTokenStateRequests,
    )
    ctx.Step(
        `^a terceira resposta HTTP deve ser 429 Too Many Requests$`,
        s.thenLastResponseIs429,
    )
}

func (s *OnboardingSuite) givenRateLimiterWithRequestsPerMinute(limit int) error
func (s *OnboardingSuite) whenUserSendsNPostRequests(n int, path string) error
func (s *OnboardingSuite) whenUserSendsNGetTokenStateRequests(n int) error
func (s *OnboardingSuite) thenLastResponseIs429(ctx context.Context) error
```

### 4.3 Steps para `outbox_publisher.feature`

```go
// Arquivo: internal/onboarding/e2e/feature_outbox_publisher_steps_test.go (NOVO)

func (s *OnboardingSuite) registerOutboxPublisherSteps(ctx *godog.ScenarioContext) {
    ctx.Step(
        `^que existe um magic_token no status PAID para o usuário "([^"]*)"$`,
        s.givenPaidMagicTokenForUser,
    )
    ctx.Step(
        `^o use case ConsumeMagicToken é executado com o token válido$`,
        s.whenConsumeMagicTokenExecutes,
    )
    ctx.Step(
        `^a tabela outbox_events deve conter exatamente (\d+) linha com event_type "([^"]*)"$`,
        s.thenOutboxContainsNRowsWithEventType,
    )
    ctx.Step(
        `^o campo aggregate_id deve corresponder ao id do magic_token$`,
        s.thenOutboxAggregateIDMatchesToken,
    )
    ctx.Step(
        `^o campo user_id deve corresponder ao usuário que consumiu o token$`,
        s.thenOutboxUserIDMatchesConsumer,
    )
    ctx.Step(
        `^que o repositório está configurado para falhar após a escrita do evento$`,
        s.givenRepositoryFailsAfterEventWrite,
    )
    ctx.Step(
        `^a tabela outbox_events deve estar vazia para o aggregate_id do token$`,
        s.thenOutboxEmptyForAggregateID,
    )
    ctx.Step(
        `^a linha do magic_token não deve ter o status CONSUMED$`,
        s.thenTokenStatusIsNotConsumed,
    )
    ctx.Step(
        `^que o evento com event_id "([^"]*)" já foi publicado no outbox$`,
        s.givenOutboxEventWithEventID,
    )
    ctx.Step(
        `^o publisher tenta publicar novamente o mesmo event_id "([^"]*)"$`,
        s.whenPublisherPublishesDuplicateEventID,
    )
    ctx.Step(
        `^a tabela outbox_events deve conter exatamente (\d+) linha com event_id "([^"]*)"$`,
        s.thenOutboxContainsNRowsWithEventID,
    )
}

func (s *OnboardingSuite) givenPaidMagicTokenForUser(userKey string) error
func (s *OnboardingSuite) whenConsumeMagicTokenExecutes(ctx context.Context) error
func (s *OnboardingSuite) thenOutboxContainsNRowsWithEventType(n int, eventType string) error
func (s *OnboardingSuite) thenOutboxAggregateIDMatchesToken(ctx context.Context) error
func (s *OnboardingSuite) thenOutboxUserIDMatchesConsumer(ctx context.Context) error
func (s *OnboardingSuite) givenRepositoryFailsAfterEventWrite() error
func (s *OnboardingSuite) thenOutboxEmptyForAggregateID(ctx context.Context) error
func (s *OnboardingSuite) thenTokenStatusIsNotConsumed(ctx context.Context) error
func (s *OnboardingSuite) givenOutboxEventWithEventID(eventID string) error
func (s *OnboardingSuite) whenPublisherPublishesDuplicateEventID(eventID string) error
func (s *OnboardingSuite) thenOutboxContainsNRowsWithEventID(n int, eventID string) error
```

---

## 5. Estrutura de Pastas

```
internal/onboarding/
├── application/
│   ├── events/
│   │   ├── subscription_bound.go
│   │   ├── subscription_bound_test.go
│   │   └── subscription_bound_integration_test.go   ← NOVO (//go:build integration)
│   └── usecases/
│       └── activate_telegram_by_token_integration_test.go  (existente)
├── infrastructure/
│   ├── messaging/
│   │   └── database/
│   │       └── consumers/
│   │           └── activation_email_consumer_test.go  ← NOVO (unit, sem build tag)
│   └── repositories/
│       └── postgres/
│           ├── magic_token_repository_integration_test.go      ← NOVO (//go:build integration)
│           ├── support_signal_repository_integration_test.go   ← NOVO (//go:build integration)
│           ├── onboarding_cleanup_repository_integration_test.go ← NOVO (//go:build integration)
│           └── onboarding_session_repository_integration_test.go (existente, expandir)
└── e2e/
    ├── features/
    │   ├── checkout_http.feature           (existente)
    │   ├── token_state_http.feature        (existente)
    │   ├── billing_consumers.feature       (existente — adicionar 3 cenários)
    │   ├── activation_processors.feature   (existente)
    │   ├── jobs.feature                    (existente)
    │   ├── robustness.feature              (existente — adicionar 2 cenários rate limit)
    │   └── outbox_publisher.feature        ← NOVO (3 cenários)
    ├── suite_test.go                       (existente)
    ├── support_world_test.go               (existente)
    ├── support_runtime_test.go             (existente)
    ├── feature_shared_steps_test.go        (existente)
    ├── feature_checkout_http_steps_test.go (existente)
    ├── feature_token_state_http_steps_test.go (existente)
    ├── feature_billing_outbox_steps_test.go   (existente — adicionar steps email consumer)
    ├── feature_activation_processor_steps_test.go (existente)
    ├── feature_jobs_steps_test.go          (existente)
    ├── feature_robustness_steps_test.go    (existente — adicionar steps rate limit)
    └── feature_outbox_publisher_steps_test.go  ← NOVO
```

**Total de arquivos novos:** 7
**Arquivos existentes que recebem extensão:** 3 (`billing_consumers.feature`, `robustness.feature`, `feature_billing_outbox_steps_test.go`, `feature_robustness_steps_test.go`)

---

## 6. Estratégia de Evidência de Validação

### 6.1 Helpers de banco obrigatórios por suite

Cada suite de integração/E2E deve expor helpers que executam `QueryRowContext` diretamente nas
tabelas, sem passar pelo repositório sob teste:

```go
// Padrão: countOutboxByEventType(t, db, eventType) int
// Padrão: latestMagicTokenByID(t, db, tokenID) magicTokenRow
// Padrão: countSupportSignalsByKind(t, db, kind) int
// Padrão: sessionStateForUser(t, db, userID) string
```

### 6.2 Verificação por operação

| Operação | Asserção obrigatória no banco |
|----------|------------------------------|
| `MagicTokenRepository.Insert` | `COUNT(*) = 1` na `onboarding_tokens` pelo `id` |
| `MagicTokenRepository.UpdateMarkPaid` | `status = 'PAID'` + `paid_at IS NOT NULL` + `subscription_id` correto |
| `MagicTokenRepository.UpdateMarkConsumed` | `status = 'CONSUMED'` + `consumed_at IS NOT NULL` + `consumed_by_user_id` correto |
| `MagicTokenRepository.BulkExpire` | Todos os tokens expirados têm `status = 'EXPIRED'` |
| `MagicTokenRepository.UpdateMarkOutreachSent` | `outreach_sent_at IS NOT NULL` |
| `OnboardingSessionRepository.Upsert` | `state` correto + `updated_at` atualizado |
| `OnboardingSessionRepository.MarkActive` | `state = 'active'` |
| `SupportSignalRepository.Insert` | `COUNT(*) = 1` por `id` + `kind` correto |
| `OnboardingCleanupRepository.DeleteMetaProcessedOlderThan` | `COUNT(*) = 0` para registros `< before` |
| Publisher `SubscriptionBound` | `outbox_events`: `event_type`, `aggregate_id`, `user_id`, `payload` corretos |
| Publisher rollback | `COUNT(*) = 0` em `outbox_events` para `aggregate_id` |
| Publisher idempotência | `COUNT(*) = 1` para mesmo `event_id` após segundo `publish` |

### 6.3 Outbox — padrão de asserção

```go
func countOutboxByEventType(t *testing.T, db *sql.DB, eventType, aggregateID string) int {
    t.Helper()
    var count int
    err := db.QueryRowContext(
        context.Background(),
        `SELECT COUNT(*) FROM mecontrola.outbox_events
         WHERE event_type = $1 AND aggregate_id = $2`,
        eventType, aggregateID,
    ).Scan(&count)
    require.NoError(t, err)
    return count
}
```

### 6.4 Idempotência por `event_id` — padrão de asserção

```go
func countOutboxByEventID(t *testing.T, db *sql.DB, eventID string) int {
    t.Helper()
    var count int
    err := db.QueryRowContext(
        context.Background(),
        `SELECT COUNT(*) FROM mecontrola.outbox_events WHERE event_id = $1`,
        eventID,
    ).Scan(&count)
    require.NoError(t, err)
    return count
}
```

---

## 7. Plano de Orquestração com Subagents

Quando a implementação for autorizada, spawnar **1 subagent por camada em paralelo**:

| Subagent | Responsabilidade | Arquivos a criar |
|----------|-----------------|-----------------|
| A — Consumer unit | `activation_email_consumer_test.go` (unit, sem build tag) | 1 arquivo |
| B — MagicToken repo | `magic_token_repository_integration_test.go` | 1 arquivo |
| C — SupportSignal repo | `support_signal_repository_integration_test.go` + `onboarding_cleanup_repository_integration_test.go` | 2 arquivos |
| D — Outbox integration | `subscription_bound_integration_test.go` | 1 arquivo |
| E — E2E novos cenários | `outbox_publisher.feature` + `feature_outbox_publisher_steps_test.go` | 2 arquivos |
| F — E2E extensões | Extensões em `billing_consumers.feature`, `robustness.feature` e steps correspondentes | 4 extensões |

Síntese final: consolidar evidências (contagem de testes novos, gates R-ADAPTER-001.1/001.2, zero comentários, idempotência validada) antes de declarar 100% coberto.

---

## 8. Definition of Done

Antes de declarar o módulo fechado, todos os gates abaixo devem ser reportados como verdes:

```bash
# Unit tests (sem Docker)
task test:unit -- ./internal/onboarding/...

# Integration tests (requer Docker)
task test:integration -- ./internal/onboarding/...

# E2E tests (requer Docker)
task test:e2e -- ./internal/onboarding/...

# Lint e vet
golangci-lint run ./internal/onboarding/...
go vet ./internal/onboarding/...

# Zero comentários em produção
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/onboarding/ configs/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL" && exit 1 || true

# Sem SQL direto em adapters
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/onboarding/infrastructure/http/server/handlers/ \
  internal/onboarding/infrastructure/messaging/database/consumers/ \
  internal/onboarding/infrastructure/jobs/handlers/ \
  && echo "FAIL" && exit 1 || true
```

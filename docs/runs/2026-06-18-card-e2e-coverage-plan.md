# Plano de Cobertura 100% — Módulo `internal/card`

**Data**: 2026-06-18
**Skill obrigatória**: `go-implementation` (Etapas 1–5 do SKILL.md)

---

## 1. Estado Atual da Cobertura

### ✅ Já Implementado (não replicar)

| Camada | Arquivos | Cenários |
|--------|----------|----------|
| Domain unit | `card_test.go`, `billing_cycle_test.go`, `decide_create_card_test.go`, `decide_update_card_test.go`, `decide_invoice_due_alerts_test.go`, `timezone_test.go`, VOs (`card_limit`, `card_name`, `nickname`, `billing_cycle_vo`) | Todos os smart constructors, `Decide*` puro |
| Application unit | `create_card_test.go`, `get_card_test.go`, `list_cards_test.go`, `update_card_test.go`, `update_card_limit_test.go`, `soft_delete_card_test.go`, `invoice_for_test.go`, `notify_invoice_due_test.go`, `evaluate_invoice_due_alerts_test.go`, `card_mapper_test.go`, `cursor_test.go` | Todos os 10 use cases (happy + cada erro) |
| HTTP handler unit | `create_test.go`, `list_test.go`, `get_test.go`, `update_test.go`, `update_card_limit_test.go`, `delete_test.go`, `invoice_for_test.go`, `pii_regression_test.go`, `contract_test.go`, `openapi_test.go` | Todos os 7 endpoints (201, 200, 204, 400, 401, 404, 409, 422) |
| Repository integration | `card_repository_integration_test.go`, `invoice_due_phase5_integration_test.go` | Insert→Get, SoftDelete, UpdateLimit OCC, paginação cursor 250 itens, concorrência nickname, FindCardsWithInvoiceDueWithin, InsertSent/IsNotified/MarkNotified |
| Consumer unit | `invoice_due_notifier_test.go` (2), `onboarding_card_consumer_test.go` (5) | Happy path + erros de decode, invalid payload, malformed JSON |
| E2E godog | `f04`–`f09` — 51 cenários | HTTP CRUD completo, fatura edge cases, job→outbox payload assertion, consumer pipelines |

---

## 2. Lacunas Reais (Matriz de Cobertura — itens faltantes)

| # | Camada | Arquivo a criar | Gap |
|---|--------|-----------------|-----|
| L1 | Producer / Outbox | `infrastructure/messaging/database/producers/invoice_due_publisher_integration_test.go` | Nenhum teste verifica que a linha em `outbox_events` é escrita na mesma tx e que rollback a remove |
| L2 | Job handler | `infrastructure/jobs/handlers/invoice_due_alerts_job_integration_test.go` | Nenhum teste verifica idempotência (2ª execução não duplica `card_invoice_alerts_sent`) |
| L3 | Consumer integration | `infrastructure/messaging/database/consumers/onboarding_card_consumer_integration_test.go` | Testes existentes são unit (mock); falta integração com BD real e idempotência por `event_id` |
| L4 | Consumer integration | `infrastructure/messaging/database/consumers/invoice_due_notifier_integration_test.go` | Idem — sem verificação de `card_invoice_alerts_sent.notified_at` em BD real |
| L5 | E2E godog | `e2e/features/f09_card_consumer.feature` (augment) | Faltam 2 cenários: reprocessar `event_id` de onboarding não duplica cartão; InvoiceDueNotifier idempotente |

---

## 3. Especificação dos Arquivos a Criar

### L1 — Producer Integration Test

**Arquivo**: `internal/card/infrastructure/messaging/database/producers/invoice_due_publisher_integration_test.go`

**Build tag**: `//go:build integration`

**Cenários obrigatórios**:

1. **Publicar evento na mesma transação persiste linha em `outbox_events`**
   - Setup: criar user + card no BD
   - Executar: `publisher.Publish(ctx, tx, alert, now)` dentro de uma `*sql.Tx` commitada
   - Assert:
     ```sql
     SELECT COUNT(*) FROM mecontrola.outbox_events
     WHERE event_type = 'card.invoice_due.v1'
       AND aggregate_id = $cardID
       AND aggregate_user_id = $userID
     ```
     → `COUNT == 1`
   - Assert: `event_type`, `aggregate_type`, `aggregate_id`, `aggregate_user_id` corretos

2. **Rollback da transação não persiste evento**
   - Executar: `publisher.Publish(ctx, tx, alert, now)` dentro de `*sql.Tx` com `tx.Rollback()`
   - Assert: `COUNT == 0` em `outbox_events`

**Helpers de asserção** (inline na suite):
```go
func countOutboxByTypeAndCard(t *testing.T, db *sqlx.DB, eventType, cardID, userID string) int
```

---

### L2 — Job Handler Integration Test

**Arquivo**: `internal/card/infrastructure/jobs/handlers/invoice_due_alerts_job_integration_test.go`

**Build tag**: `//go:build integration`

**Cenários obrigatórios**:

1. **Execução com cartão na janela → enfileira evento e registra `card_invoice_alerts_sent`**
   - Setup: cartão com `due_day = dia_de_hoje + 2`; usuário ACTIVE
   - Executar: `job.Run(ctx)`
   - Assert:
     ```sql
     SELECT COUNT(*) FROM mecontrola.card_invoice_alerts_sent
     WHERE card_id = $1 AND user_id = $2
     ```
     → `COUNT == 1`
   - Assert:
     ```sql
     SELECT COUNT(*) FROM mecontrola.outbox_events
     WHERE event_type = 'card.invoice_due.v1' AND aggregate_id = $1
     ```
     → `COUNT == 1`

2. **Execução dupla é idempotente (sem linha duplicada)**
   - `job.Run(ctx)` duas vezes seguidas com mesmo contexto/BD
   - Assert: `COUNT == 1` em `card_invoice_alerts_sent` (ON CONFLICT DO NOTHING)
   - Assert: outbox pode ter 1 ou 2 eventos (publisher idempotente via `event_id` único) — verificar que `card_invoice_alerts_sent` não duplicou

3. **Sem candidatos → executa sem erro e sem linhas novas**
   - Nenhum cartão com `due_day` na janela
   - `job.Run(ctx)` retorna `nil`
   - Assert: `COUNT == 0` em `card_invoice_alerts_sent`

**Helpers**:
```go
func countAlertsSent(t *testing.T, db *sqlx.DB, cardID, userID uuid.UUID) int
func countOutboxByTypeAndCard(t *testing.T, db *sqlx.DB, eventType, cardID, userID string) int
```

---

### L3 — OnboardingCardConsumer Integration Test

**Arquivo**: `internal/card/infrastructure/messaging/database/consumers/onboarding_card_consumer_integration_test.go`

**Build tag**: `//go:build integration`

**Cenários obrigatórios**:

1. **Processar evento cria cartão no BD**
   - Construir `Envelope` com `EventType = "onboarding.card_registered"`, payload com `UserID`, `Name`, `LimitCents`, `ClosingDay`, `DueDay`
   - `consumer.Handle(ctx, event)`
   - Assert:
     ```sql
     SELECT COUNT(*) FROM mecontrola.cards WHERE name = $1 AND user_id = $2 AND deleted_at IS NULL
     ```
     → `COUNT == 1`

2. **Reprocessar mesmo `event_id` não cria duplicata (idempotência)**
   - Chamar `Handle` duas vezes com o mesmo `Envelope.ID`
   - O segundo call deve retornar sucesso (via idempotência do use case) OU erro de idempotência controlado
   - Assert: `COUNT == 1` (sem duplicata)

3. **Evento com payload inválido não cria linha**
   - `Envelope.Payload` = JSON malformado
   - `Handle` retorna erro
   - Assert: `COUNT == 0`

**Helpers**:
```go
func countCardsByName(t *testing.T, db *sqlx.DB, name string, userID uuid.UUID) int
```

---

### L4 — InvoiceDueNotifier Integration Test

**Arquivo**: `internal/card/infrastructure/messaging/database/consumers/invoice_due_notifier_integration_test.go`

**Build tag**: `//go:build integration`

**Cenários obrigatórios**:

1. **Processar evento marca `notified_at` em `card_invoice_alerts_sent`**
   - Setup: inserir row em `card_invoice_alerts_sent` (sem `notified_at`)
   - `consumer.Handle(ctx, event)` com payload `card.invoice_due.v1`
   - Assert:
     ```sql
     SELECT notified_at IS NOT NULL
     FROM mecontrola.card_invoice_alerts_sent
     WHERE card_id = $1 AND user_id = $2
     ```
     → `true`

2. **Notificação já enviada não chama gateway novamente (idempotência)**
   - `Handle` duas vezes com mesmo `event_id`
   - Assert: gateway recebeu exatamente 1 chamada (mock ou stub de contagem)
   - Assert: `COUNT(notified_at IS NOT NULL) == 1`

3. **Usuário sem canal configurado → outcome `no_channel`, sem erro fatal**
   - Resolver retorna `false`
   - Assert: `Handle` retorna `nil` (sem erro)
   - Assert: nenhuma linha `notified_at` atualizada

**Helpers** (compartilhados com L3):
```go
func insertAlertSentPending(t *testing.T, db *sqlx.DB, userID, cardID uuid.UUID, refDueDate time.Time)
func isAlertNotified(t *testing.T, db *sqlx.DB, userID, cardID uuid.UUID, refDueDate time.Time) bool
```

---

### L5 — Novos Cenários E2E (augment f09)

**Arquivo a augmentar**: `internal/card/e2e/features/f09_card_consumer.feature`

```gherkin
# language: pt
Funcionalidade: Consumo de eventos do módulo de cartão

  Contexto:
    Dado existe um usuário autenticado

  Cenário: Consumer onboarding cria cartão a partir de evento
    Quando o consumer recebe o evento "onboarding.card_registered" com nome "Cartão Onboarding", limite 100000, fechamento 5 e vencimento 12
    Então o cartão deve estar persistido no banco para o usuário

  Cenário: Reprocessar evento de onboarding com mesmo event_id não cria cartão duplicado
    Quando o consumer recebe o evento "onboarding.card_registered" com nome "Cartão Idem", limite 50000, fechamento 3 e vencimento 10
    E o mesmo evento de onboarding é reprocessado com o mesmo event_id
    Então o banco deve conter exatamente 1 cartão com aquele nome para o usuário

  Cenário: InvoiceDueNotifier envia notificação ao receber evento de vencimento
    Dado que o usuário possui um cartão criado com nome "Notif Fatura", fechamento 5, vencimento 12 e limite 100000
    Quando o consumer recebe o evento "card.invoice_due.v1" para o cartão criado com vencimento em 2 dias
    Então a gateway de canal deve ter recebido ao menos 1 mensagem para o usuário

  Cenário: Consumer InvoiceDueNotifier é idempotente para vencimento já notificado
    Dado que o usuário possui um cartão criado com nome "Notif Idem", fechamento 5, vencimento 12 e limite 100000
    Quando o consumer recebe o evento "card.invoice_due.v1" para o cartão criado com vencimento em 2 dias
    E o mesmo evento de vencimento é reprocessado
    Então a gateway de canal deve ter recebido exatamente 1 mensagem para o usuário
```

**Assinaturas de steps novos** (em `steps_consumer_test.go`):

```go
// Regex PT-BR: "o mesmo evento de onboarding é reprocessado com o mesmo event_id"
func (e *cardE2ECtx) reprocessSameOnboardingEvent() error

// Regex PT-BR: "o banco deve conter exatamente (\d+) cartão com aquele nome para o usuário"
func (e *cardE2ECtx) assertExactCardCountByName(expected int) error

// Regex PT-BR: "o mesmo evento de vencimento é reprocessado"
func (e *cardE2ECtx) reprocessSameInvoiceDueEvent() error

// Regex PT-BR: "a gateway de canal deve ter recebido exatamente (\d+) mensagem para o usuário"
func (e *cardE2ECtx) assertGatewayReceivedExactMessages(expected int) error
```

**Ajuste em `cardE2ECtx`** (adicionar campos):
```go
lastOnboardingEventID  string
lastInvoiceDueEventID  string
```

**Lógica de `reprocessSameOnboardingEvent`**: reusar `e.lastOnboardingEventID` e `e.cardName` para construir o mesmo `Envelope` e chamar `h.Handle` novamente.

**Lógica de `assertExactCardCountByName`**:
```go
SELECT COUNT(*) FROM mecontrola.cards WHERE name = $1 AND user_id = $2 AND deleted_at IS NULL
```

**Lógica de `assertGatewayReceivedExactMessages`**: contar `e.channelGateway.messages` filtrados por `ExternalID == e2eUserPhone`.

---

## 4. Estrutura de Pastas Após a Implementação

```
internal/card/
├── e2e/
│   ├── features/
│   │   ├── f04_card_create.feature          ← existente
│   │   ├── f05_card_read_list.feature       ← existente
│   │   ├── f06_card_update.feature          ← existente
│   │   ├── f07_card_delete.feature          ← existente
│   │   ├── f08_card_invoice.feature         ← existente
│   │   └── f09_card_consumer.feature        ← AUGMENT (+2 cenários)
│   ├── steps_consumer_test.go               ← AUGMENT (+4 funções)
│   └── ...                                  ← outros steps existentes
│
└── infrastructure/
    ├── jobs/handlers/
    │   ├── invoice_due_alerts_job.go        ← existente
    │   └── invoice_due_alerts_job_integration_test.go  ← NOVO (L2)
    │
    └── messaging/database/
        ├── consumers/
        │   ├── invoice_due_notifier.go                       ← existente
        │   ├── invoice_due_notifier_test.go                  ← existente (unit)
        │   ├── invoice_due_notifier_integration_test.go      ← NOVO (L4)
        │   ├── onboarding_card_consumer.go                   ← existente
        │   ├── onboarding_card_consumer_test.go              ← existente (unit)
        │   └── onboarding_card_consumer_integration_test.go  ← NOVO (L3)
        │
        └── producers/
            ├── invoice_due_publisher.go                      ← existente
            └── invoice_due_publisher_integration_test.go     ← NOVO (L1)
```

---

## 5. Estratégia de Evidência de Validação

### 5.1 Outbox — Padrão de Verificação

```go
// countOutboxByTypeAndCard — executa SELECT direto na tabela após a operação
func countOutboxByTypeAndCard(t *testing.T, db *sqlx.DB, eventType, cardID, userID string) int {
    t.Helper()
    var n int
    err := db.QueryRowContext(context.Background(),
        `SELECT COUNT(*) FROM mecontrola.outbox_events
         WHERE event_type = $1 AND aggregate_id = $2 AND aggregate_user_id = $3`,
        eventType, cardID, userID,
    ).Scan(&n)
    require.NoError(t, err)
    return n
}
```

- **Commit → `COUNT == 1`**: prova que o evento foi persistido na mesma tx
- **Rollback → `COUNT == 0`**: prova que atomicidade funciona (sem evento "fantasma")

### 5.2 Job — Padrão de Idempotência

```go
func countAlertsSent(t *testing.T, db *sqlx.DB, cardID, userID uuid.UUID) int {
    t.Helper()
    var n int
    _ = db.QueryRowContext(context.Background(),
        `SELECT COUNT(*) FROM mecontrola.card_invoice_alerts_sent WHERE card_id = $1 AND user_id = $2`,
        cardID, userID,
    ).Scan(&n)
    return n
}
```

- Run 1 → `count == 1`; Run 2 → `count == 1` (ON CONFLICT DO NOTHING)

### 5.3 Consumer — Padrão de Verificação de BD

- Após `Handle(ctx, event)`: `SELECT COUNT(*) FROM mecontrola.cards WHERE name = $1 AND user_id = $2` → `COUNT == before+1`
- Após segundo `Handle` com mesmo `event_id`: `COUNT == 1` (sem duplicata)

### 5.4 DB Isolado por Teste

Todos os testes de integração/E2E usam:
```go
db, _ := postgres.NewTestDatabase(t)
// t.Cleanup remove o banco ao final
```

---

## 6. Regras Obrigatórias de Implementação

| Regra | Verificação |
|-------|-------------|
| `//go:build integration` em todos os testes L1–L4 | `grep -rn "//go:build" <arquivo>` |
| Zero comentários em `.go` de produção | Gate R-ADAPTER-001.1 |
| Sem SQL direto em adapter | Gate R-ADAPTER-001.2 |
| `errors.Join` para múltiplos erros | R7.6 |
| `context.Context` em toda fronteira de IO | R6 |
| Sem `panic` em produção | R5.12 |
| Gherkin em PT-BR | Verificação visual |
| Métodos/steps Go em inglês | Verificação visual |

---

## 7. Orquestração — Agentes em Paralelo

Conforme regra do repositório, execução paralela obrigatória por camada:

| Agente | Escopo |
|--------|--------|
| agent-producer | L1: `invoice_due_publisher_integration_test.go` |
| agent-job | L2: `invoice_due_alerts_job_integration_test.go` |
| agent-consumer-onboarding | L3: `onboarding_card_consumer_integration_test.go` |
| agent-consumer-notifier | L4: `invoice_due_notifier_integration_test.go` |
| agent-e2e | L5: augment `f09_card_consumer.feature` + steps |

Síntese final: consolidar evidências (`COUNT` pré/pós, outbox state, gateway messages) antes de declarar 100% coberto.

---

## 8. Definition of Done

Antes de declarar o módulo 100% coberto, todos os gates abaixo devem passar:

```bash
# Unitários
task test:unit

# Integração (requer Docker)
task test:integration

# E2E (requer Docker)
task test:e2e

# Linting
golangci-lint run ./internal/card/...
go vet ./internal/card/...

# R-ADAPTER-001.1: zero comentários em .go de produção
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/card/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL" || echo "OK"

# R-ADAPTER-001.2: sem SQL direto em adapter
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/card/infrastructure/http/server/handlers/ \
  internal/card/infrastructure/messaging/database/consumers/ \
  internal/card/infrastructure/messaging/database/producers/ \
  internal/card/infrastructure/jobs/handlers/ \
  && echo "FAIL" || echo "OK"
```

---

## 9. Referências Canônicas

- `.agents/skills/go-implementation/SKILL.md` — Etapas 1–5 obrigatórias
- `.agents/skills/go-implementation/references/testing-integration.md` — padrão de suite stateful
- `.agents/skills/go-implementation/references/messaging.md` — producer/consumer
- `internal/platform/database/postgres/test_helper.go` — `NewTestDatabase`
- `internal/card/e2e/suite_test.go` — padrão de wiring E2E existente
- `internal/card/e2e/steps_consumer_test.go` — padrão de step Go + regex PT-BR
- `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher_integration_test.go` — exemplo canônico de producer integration test
- `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer_integration_test.go` — exemplo canônico de consumer integration test com idempotência

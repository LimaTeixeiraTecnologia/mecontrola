# Run — Validate() error em todos os Input DTOs

- Data: 2026-06-22
- Skill: go-implementation (task_type: cross-cutting, validation_profile: boundary)
- Regra criada: R-DTO-VALIDATE-001 em `.claude/rules/input-dto-validate.md`

## Objetivo

Adicionar `Validate() error` em todos os structs de `internal/<modulo>/application/dtos/input/`,
tornando cada DTO auto-validável como primeira etapa do pipeline (DMMF Princípio 5).

## Padrão Canônico

```go
func (i *InputType) Validate() error {
    var errs []error
    if i.UserID == uuid.Nil {
        errs = append(errs, fmt.Errorf("user_id: %w", domain.ErrMissingUserID))
    }
    if _, err := valueobjects.NewCardName(i.Name); err != nil {
        errs = append(errs, fmt.Errorf("name: %w", err))
    }
    return errors.Join(errs...)
}
```

- `errors.Join` — coleta TODOS os erros simultaneamente (nunca fail-fast)
- Campo nomeado em cada mensagem
- Receiver pointer `*T`
- Puro: sem IO, sem `context.Context` (DMMF P6)
- Delega a VO smart constructors (DMMF P1)

## Decisões de Negócio

| Módulo | Campo | Decisão |
|--------|-------|---------|
| transactions | `RawCreateCardPurchase.InstallmentsTotal` | Exigir ≥ 1 (novo contrato, breaking change aceito) |
| identity | `RecordGatewayAuthFailureInput.Reason` | NÃO validar no DTO — whitelist permanece no use case (evita drift de manutenção) |
| identity | `ConsumeMagicTokenInput.Token` | NÃO validar — use case trata como ConsumeOutcomeNotFound |
| billing | `ProcessRefundOrChargebackInput.Trigger` | NÃO obrigatório — use case defaulta para "order_refunded" |
| billing | `ProcessSubscriptionCanceled/LateInput.KiwifySubID` | NÃO obrigatório — fallback para OrderID no use case |
| budgets | `UpsertExpenseInput.OccurredAt` | NÃO validar zero — use case usa time.Now() como default |

## Módulos Executados (7 agentes paralelos)

### categories
- Migrar `SearchDictionaryInput.Validate()` de fail-fast para `errors.Join`
- Novo: `GetCategoryInput`, `ListCategoriesInput`, `ListDictionaryInput`
- Use cases: 4

### card
- Novos sentinels em `domain/errors.go`: `ErrMissingUserID`, `ErrMissingCardID`, `ErrInvalidLimit`
- `isCardValidationError` atualizado em `usecases/errors.go`
- DTOs: `CountCards`, `CreateCard`, `GetCard`, `InvoiceFor`, `ListCards`, `SoftDeleteCard`, `UpdateCard`, `UpdateCardLimit`
- Use cases: 8

### identity
- Novo `errors.go` com 7 sentinels
- DTOs: 11 (incluindo consumer DTOs)
- `ClientIPRaw`, `RequestID` NÃO validados (degradação graciosa intencional)
- Use cases: 11

### onboarding
- Novo `errors.go` com 7 sentinels
- DTOs: 4
- Guards inline removidos: `create_checkout_session.go` e `mark_token_paid.go`
- Use cases: 4

### billing
- Novo `errors.go` com 12 sentinels
- DTOs: 8
- Use cases: 8

### budgets
- Novo `errors.go` com 10 sentinels
- DTOs: 7 (`AlertQuery` excluído — campos todos tipados, construído internamente)
- Use cases: 9 (2 use cases com `Execute` + `ExecuteWithTx`)

### transactions
- Novo `errors.go` com 12 sentinels
- DTOs: 6 (`Raw*` apenas com checks de superfície — R-TXN-002)
- Use cases: 6

## Arquivos Criados

- `internal/billing/application/dtos/input/errors.go`
- `internal/budgets/application/dtos/input/errors.go`
- `internal/identity/application/dtos/input/errors.go`
- `internal/onboarding/application/dtos/input/errors.go`
- `internal/transactions/application/dtos/input/errors.go`
- `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001)

## Evidências de Validação

```
go build ./...           → OK (sem erros)
go test ./internal/...   → todos os módulos OK
zero comentários gate    → OK
R-DTO-VALIDATE-001 gate  → 48/49 DTOs com Validate() (1 excluído: AlertQuery)
```

## Totais

- DTOs com `Validate()` novo/migrado: 48
- Use cases com `in.Validate()` inserido: ~50
- Arquivos `errors.go` criados: 5
- Regras criadas: 1 (R-DTO-VALIDATE-001)

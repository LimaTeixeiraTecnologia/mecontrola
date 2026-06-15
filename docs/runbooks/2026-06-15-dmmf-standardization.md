# DMMF Standardization — Production-Ready Audit

**Skill:** go-implementation (Etapas 1-5)
**Go version:** 1.26.4
**Referência DMMF:** internal/transactions + internal/budgets

## Contexto

Auditoria pós-production-readiness (2026-06-15). Os bloqueadores críticos (panic recovery, job
timeouts, HMAC middleware, /readiness) foram corrigidos na sessão anterior. Esta etapa foca nas
violações R1 identificadas pelo checklist build.md: funções standalone em camadas de domínio e
aplicação que deveriam ser métodos de struct.

## Violações R1 Identificadas

### Cluster A — `card/domain/services/billing_cycle.go`
- `InvoiceFor(...)` standalone → método de `BillingCycleService`
- Helpers privados (`computeCycle`, `advanceMonth`, `clamp`, `daysInMonth`) — mantidos como funções privadas (implementação interna)
- **Caller:** `card/application/usecases/invoice_for.go`
- **Teste:** `card/domain/services/billing_cycle_test.go` (package services_test)

### Cluster B — `card/application/mappers/card_mapper.go`
- `ToInvoiceOutput`, `ToCardOutput`, `ToCardListOutput` standalone → métodos de `Mapper`
- Criar `card/application/mappers/mapper.go` com `type Mapper struct{}` + `var M = Mapper{}`
- **Callers:** get_card.go, invoice_for.go, list_cards.go, create_card.go (2x) — todos usam `mappers.ToXxx(...)`
- **Teste:** card_mapper_test.go

### Cluster C — `card/application/pagination/cursor.go`
- `Encode`, `Decode` standalone → métodos de `Coder`
- Criar `var C = Coder{}` como conveniência
- **Callers:** list_cards.go (usa Decode), list_cards_test.go, cursor_test.go

### Cluster D — `card/infrastructure/observability/redact.go`
- `RedactCardLogFields`, `RedactOutputCardLogFields` standalone → métodos de `Redactor`
- **Callers:** handlers/create.go, handlers/update.go (usam cardobs.RedactXxx)

### Cluster E — `identity/domain/services/verify_gateway_request.go`
- `VerifyGatewayRequest(...)` standalone → método de `GatewayRequestVerifier`
- `canonical(...)` — mantido privado
- **Caller:** infrastructure/http/server/middleware/require_gateway_auth.go
- **Teste:** verify_gateway_request_test.go (package services — teste interno, acessa canonical diretamente)

### Cluster F — `budgets/domain/services/` standalone
- `alert_workflow.go`: `IsRetroactiveAlert`, `DecideAlertForInsert` → `AlertWorkflow` struct
- `allocation_distributor.go`: `Distribute` → `AllocationDistributor` struct
- `threshold_evaluator.go`: `EvaluateThresholds` → `ThresholdEvaluator` struct
- **Callers:** application/usecases/activate_budget.go, evaluate_alert.go; budget_clone_for_recurrence.go (mesma package)

## Skills Necessárias

- go-implementation (ativa)
- Referências carregadas: architecture.md, build.md

## Etapa 5 — Validação

```bash
# R0: sem init()
grep -rn "^func init()" --include="*.go" internal/ cmd/

# R1: funções standalone (esperar ZERO após fix)
grep -rn "^func [^(]" --include="*.go" internal/ | \
  grep -v "_test.go" | grep -v "mocks/" | \
  grep -vE "func (New|main|Test|Build)" | grep -vE "^.*:func New"

# Build + testes
go build ./...
go vet ./...
go test ./internal/card/... ./internal/identity/... ./internal/budgets/... -count=1

# Zero comentários
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/ configs/ cmd/ | grep -Ev "(//go:|//nolint:|// Code generated)"
```

# ADR-005 — `billing.Subscription` implementa `identity.Subscription` por satisfação estrutural

## Metadados

- **Título:** Contrato cross-module entre billing.Subscription e identity.Subscription
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de domínio
- **Relacionados:** `prd-billing-pipeline/prd.md` (F-3, RF-18), `prd-identity-foundation/techspec.md` ADR-003, `techspec.md` §Modelos de Domínio

## Contexto

E1 (identity) entrega a função pura `IsEntitled(sub identity.Subscription, now time.Time) bool` em `internal/identity/domain/services/entitlement.go`. A interface `identity.domain.services.Subscription` (contrato mínimo) tem 3 métodos:

```go
type Subscription interface {
    Status() SubscriptionStatus
    CurrentPeriodEnd() time.Time
    GracePeriodEnd() time.Time
}
```

`SubscriptionStatus` é enum (iota+1) com 7 valores (`Unknown`, `Trialing`, `Active`, `PastDue`, `CanceledPending`, `Expired`, `Refunded`).

Billing (E2) define seu próprio agregado `billing.entities.Subscription` com tudo que negócio precisa (provider, external_id, plan_code, period, refund_amount, etc.). Esse agregado precisa ser consumido por `IsEntitled` sem importar `identity/infrastructure` (depguard rule `billing-no-identity-infrastructure`) e sem dependência circular (identity → billing seria ruim — identity é raiz do roadmap).

## Decisão

`billing.entities.Subscription` implementa `identity.domain.services.Subscription` por **satisfação estrutural** (Go duck typing) — sem importar/declarar implementação explícita. Os métodos do agregado billing têm assinatura compatível:

```go
// billing/domain/entities/subscription.go
func (s *Subscription) Status() valueobjects.SubscriptionStatus  // VO billing
func (s *Subscription) PeriodEnd() time.Time
func (s *Subscription) GracePeriodEnd() time.Time
```

**Adaptação de tipo**: `billing.valueobjects.SubscriptionStatus` (uint8) e `identity.domain.services.SubscriptionStatus` (uint8) **são semanticamente idênticos** — mesma ordenação, mesmos 7 valores. Para evitar dois tipos com o mesmo conteúdo, a interface `identity.Subscription` é refinada para usar **tipo subjacente equivalente** (`uint8`) e billing fornece método `Status() services.SubscriptionStatus` que retorna o tipo de identity diretamente, importando `identity/domain/services` (permitido).

Resultado: billing importa `identity/domain/services` apenas para tipos (`SubscriptionStatus`, `Subscription` interface) — sem ciclo, sem violar depguard. `identity` não importa nada de billing.

**Renomeação do método PRD**: PRD usa "PeriodEnd" / "GracePeriodEnd" na interface; identity techspec usa "CurrentPeriodEnd" / "GracePeriodEnd". A interface oficial em identity é `CurrentPeriodEnd()`. Billing implementa `CurrentPeriodEnd()` como alias semântico de `PeriodEnd()`.

## Alternativas Consideradas

### Adapter struct (`billingSubscriptionAdapter`) wrappa billing.Subscription para implementar identity.Subscription

- Vantagem: separação explícita; permite billing.Subscription evoluir independentemente da interface identity.
- Desvantagem: ~30 linhas de boilerplate; alocação extra a cada `IsEntitled` chamada.
- Rejeitada por overhead em hot path (`Check` é < 5ms p99).

### `identity` declara interface aceitando `interface{ Status() string; ... }` (strings)

- Vantagem: zero acoplamento de tipo.
- Desvantagem: validação de domínio com string é ruim (typo silencioso); perde type safety.
- Rejeitada por trade-off contra correção.

## Consequências

### Benefícios Esperados

- Zero boilerplate de adapter.
- Type safety preservado.
- `IsEntitled` consumido diretamente: `identity.NewEntitlementChecker().IsEntitled(sub, now)` onde `sub` é `*billing.entities.Subscription`.
- Depguard rules verdes.

### Trade-offs e Custos

- Acoplamento de tipo: `billing.entities.Subscription.Status()` retorna `identity/domain/services.SubscriptionStatus`. Se identity refatorar o enum, billing precisa atualizar. **Mitigação:** identity é raiz; enum estável documentado em identity AGENTS.md.

### Riscos e Mitigações

- **Risco:** dois enums com mesmos valores divergem com o tempo (e.g., billing adiciona `TRIALING_EXTENDED`). **Mitigação:** método `Status()` retorna tipo de identity; estados não-mapeáveis seriam rejeitados em compile time se enum não tiver o valor.

## Plano de Implementação

1. Confirmar com E1 que a interface `identity.domain.services.Subscription` usa o método `CurrentPeriodEnd()` (já está no techspec de identity).
2. Em `billing/domain/entities/subscription.go`, implementar `func (s *Subscription) CurrentPeriodEnd() time.Time { return s.periodEnd }`.
3. Em `billing/domain/valueobjects/subscription_status.go`, retornar `services.SubscriptionStatus` (importado de `identity/domain/services`) em `Status()`.
4. Teste em billing: verificar `var _ services.Subscription = (*entities.Subscription)(nil)` está implícito — usar teste de uso real `services.NewEntitlementChecker().IsEntitled(billingSub, now)` em vez de declaração explícita (R6.4 proíbe `var _`).

## Monitoramento e Validação

- Compile time enforça a satisfação — qualquer drift quebra build.
- Teste de integração executa `IsEntitled(sub, now)` no caminho do `EntitlementService.Check`.

## Impacto em Documentação e Operação

- `internal/billing/AGENTS.md` documenta que `Subscription` satisfaz contrato de identity sem importação explícita de `var _`.
- `internal/identity/AGENTS.md` documenta que o enum é contrato compartilhado.

## Revisão Futura

- Se billing precisar de estado fora dos 6 canônicos, abrir nova ADR para evoluir o enum em identity (impacto em E1 + E2).

# ADR-002 — `Subscription` mínima como interface em `internal/identity/domain`

## Metadados

- **Título:** Forma do contrato `Subscription` consumido por `IsEntitled`
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** Time MeControla (owner: Jailton Junior)
- **Relacionados:**
  - PRD: [`prd.md`](./prd.md) — RF-12, RF-13, F-09
  - Tech Spec: [`techspec.md`](./techspec.md)
  - PRD Q em aberto fechada: **Q-02**
  - Decisões correlatas: [ADR-001](./adr-001-reason-string-type.md)

## Contexto

`IsEntitled` precisa decidir entitlement a partir de três campos do agregado `Subscription`: `status`, `period_end`, `grace_period_end`. O agregado completo de `Subscription` vive em E2 (módulo `internal/billing`), que ainda não foi implementado.

Restrições:

- **Não pode haver cross-module em runtime.** `internal/identity` não pode importar `internal/billing` (regra `depguard` ativa e `AGENTS.md`).
- **`IsEntitled` precisa existir antes de E2.** O E1 destrava E2 — invertendo, `identity` seria bloqueado por E2.
- **`AGENTS.md` impõe interface no consumidor (R6).** Quem consome o contrato (identity) declara a interface; quem implementa (billing) satisfaz.
- **Comunicação cross-module só pode acontecer via interface declarada pelo consumidor, domain event/outbox ou contrato explícito.**

Sinal histórico: o padrão `internal/platform/events.Event` (consumidor declara `Handler`, dispatcher recebe `Event`) confirma a convenção do projeto.

## Decisão

`Subscription` será declarado em `internal/identity/domain/entitlement.go` como **interface mínima**:

```go
type SubscriptionStatus string

const (
    SubscriptionTrialing         SubscriptionStatus = "TRIALING"
    SubscriptionActive           SubscriptionStatus = "ACTIVE"
    SubscriptionPastDue          SubscriptionStatus = "PAST_DUE"
    SubscriptionCanceledPending  SubscriptionStatus = "CANCELED_PENDING"
    SubscriptionExpired          SubscriptionStatus = "EXPIRED"
    SubscriptionRefunded         SubscriptionStatus = "REFUNDED"
)

type Subscription interface {
    Status() SubscriptionStatus
    PeriodEnd() time.Time
    GracePeriodEnd() time.Time
}
```

`IsEntitled` recebe `Subscription` (interface) ou `nil`:

```go
func IsEntitled(sub Subscription, now time.Time) (bool, Reason)
```

`GracePeriodEnd()` retorna `time.Time` zero (`time.Time{}`) quando ausente — a função trata zero como "sem grace period" sem ramificação extra. Isso evita expor `*time.Time` na interface e mantém o consumidor agnóstico de ponteiros.

## Alternativas Consideradas

### A) Struct concreta `Subscription` em `identity/domain`

- **Vantagens:** mais simples para `IsEntitled` (acesso direto a campos); zero overhead de invocação dinâmica.
- **Desvantagens:**
  - E2 precisaria mapear seu agregado completo para o struct antes de chamar — copy redundante a cada decisão.
  - Mudanças no agregado de E2 (adição de campos) podem exigir alteração no struct de identity.
  - Acopla a forma de armazenamento de E2 ao formato de leitura de identity.
- **Motivo de não escolher:** quebra o princípio "interface no consumidor" e gera fricção em E2.

### B) Função genérica `IsEntitled[T any](sub T, getStatus func(T) Status, ...)`

- **Vantagens:** evita interface dinâmica.
- **Desvantagens:** explode a superfície da API e foge do padrão do repositório.
- **Motivo de não escolher:** complexidade desproporcional.

### C) `IsEntitled` no domínio de E2 (billing) com `identity` consumindo via cross-module

- **Vantagens:** mantém regra de entitlement perto do agregado dono.
- **Desvantagens:**
  - Inverte a dependência: identity passaria a importar billing — quebra `depguard` e ordem do roadmap.
  - Handlers de WhatsApp em E3 acabariam dependendo de billing para obter `Reason`.
- **Motivo de não escolher:** viola fronteiras do roadmap e o "1 user = 1 subscription ativa" passa a depender de E2 estar pronto.

## Consequências

### Benefícios Esperados

- **Identity destrava E2 e E3 sem cross-module.**
- **E2 satisfaz a interface com qualquer struct** — `func (s *billingSubscription) Status() identity.SubscriptionStatus { ... }`.
- **Testes de `IsEntitled` ficam triviais** com `mockery` ou fake mínimo no `_test.go`.
- **Padrão idiomático Go** alinhado a R6.

### Trade-offs e Custos

- Pequena alocação por chamada quando o consumidor passa um `*struct` (a interface boxed costuma ser elidida pelo compilador em chamadas inline).
- `Subscription` no domínio de identity precisa documentar claramente que o nome **não** é o agregado de billing, apenas seu contrato de leitura para decisão de entitlement.

### Riscos e Mitigações

- **Risco:** confusão semântica com agregado de E2 (mesmo nome).
  - **Mitigação:** documentar em `doc.go` que `identity.Subscription` é o contrato mínimo, não o agregado; quando ambíguo, qualificar imports (`identity.Subscription` vs `billing.Subscription`).
- **Risco:** E2 implementa `Status()` retornando string crua (e.g., `"active"` minúsculo).
  - **Mitigação:** `SubscriptionStatus` é tipo nomeado com constantes — o compilador exige conversão explícita, exposta no PR.

## Plano de Implementação

1. Declarar `SubscriptionStatus`, suas constantes e `Subscription` em `internal/identity/domain/entitlement.go`.
2. `IsEntitled(sub Subscription, now time.Time) (bool, Reason)` consome apenas os 3 métodos.
3. Testes em `internal/identity/domain/entitlement_test.go` usam fake interno:
   ```go
   type fakeSub struct {
       status SubscriptionStatus
       end    time.Time
       grace  time.Time
   }
   func (f fakeSub) Status() SubscriptionStatus { return f.status }
   func (f fakeSub) PeriodEnd() time.Time       { return f.end }
   func (f fakeSub) GracePeriodEnd() time.Time  { return f.grace }
   ```
4. Em E2, o agregado de billing implementa os 3 métodos; nenhum mapeamento intermediário é necessário.

## Monitoramento e Validação

- **Validação imediata:** `go build ./internal/identity/...` e `go test -race -count=1 ./internal/identity/domain/...`.
- **Validação em E2:** quando E2 implementar `Subscription` real, basta `var _ identity.Subscription = (*billingSub)(nil)` em **teste** (R6.4 proíbe esse assertion em código de produção).
- **Sinal de drift:** se `IsEntitled` precisar de um quarto campo, o contrato cresce e este ADR deve ser revisitado.

## Impacto em Documentação e Operação

- `internal/identity/doc.go` descreve o contrato `Subscription` e o motivo da escolha.
- README do módulo cita ADR-002 para qualquer extensão futura.

## Revisão Futura

- Revisitar quando E2 implementar o agregado real de `Subscription` (validar shape).
- Revisitar se entitlement passar a depender de campos derivados (e.g., `paid_until`, `external_subscription_id`).
- Revisitar se o número de implementações da interface crescer além de uma (em geral é sinal de design errado para interface no consumidor).

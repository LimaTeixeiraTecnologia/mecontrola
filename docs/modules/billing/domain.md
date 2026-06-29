# Domínio — internal/billing

Módulo responsável pelo ciclo de vida de assinaturas pagas no mecontrola: ativação, renovação, inadimplência, cancelamento, estorno e expiração após janela de carência, com proteção contra regressão de estado por eventos fora de ordem.

## Aggregate Root: Subscription

### Campos

| Campo | Tipo Go | Nullable / Zero-value | Propósito | Invariante |
|---|---|---|---|---|
| `id` | `string` | vazio em `NewSubscription` | Identificador persistido | Atribuído apenas pelo repositório; jamais alterado após hidratação |
| `userID` | `string` | vazio em `Hydrate` | Vínculo com o usuário da plataforma | Preenchido somente via `HydrateWithUser` |
| `plan` | `valueobjects.Plan` | zero-value de Plan | Código e duração do plano contratado | Deve ter `durationDays > 0` e `PlanCode` suportado |
| `funnelToken` | `valueobjects.FunnelToken` | zero-value de FunnelToken | Token de funil de venda (rastreio Kiwify) | String não vazia após TrimSpace |
| `status` | `valueobjects.Status` | `0` (sem status) | Estado atual da assinatura na máquina de estados | Apenas constantes `Status*` definidas; nunca `0` em assinatura ativa |
| `periodStart` | `time.Time` | zero-value | Início do período de vigência atual | Não alterado pelos métodos de ciclo de vida atuais |
| `periodEnd` | `time.Time` | zero-value | Fim do período de vigência atual | Recalculado em `applyActive`; base = `max(periodEnd, occurredAt)` |
| `graceEnd` | `time.Time` | zero-value | Prazo final da janela de carência | Definido apenas em `applyPastDue`; zerado em todos os demais applies |
| `lastEventAt` | `time.Time` | zero-value | Timestamp do último evento aplicado | Usado para detecção de regressão; atualizado em todo apply |

### Construtores

| Construtor | Parâmetros | Validações executadas | Retorno de erro |
|---|---|---|---|
| `NewSubscription` | `plan Plan`, `funnelToken FunnelToken` | Nenhuma (VOs já validados na criação) | — |
| `Hydrate` | `id`, `funnelToken`, `plan`, `status`, `periodStart`, `periodEnd`, `graceEnd`, `lastEventAt` | Nenhuma (reconstrói do repositório) | — |
| `HydrateWithUser` | `id`, `userID`, `funnelToken`, `plan`, `status`, `periodStart`, `periodEnd`, `graceEnd`, `lastEventAt` | Nenhuma (reconstrói do repositório com user) | — |

`NewSubscription` cria assinatura sem `id` e sem `status` definido (estado inicial neutro, pronto para `Activate`). Os construtores `Hydrate*` não validam os valores recebidos — a invariante é garantida pelo repositório e pelos métodos de ciclo de vida durante a primeira escrita.

### Métodos de Ciclo de Vida

| Método | Trigger interno | Pré-condição | Efeito colateral | Erro retornado |
|---|---|---|---|---|
| `Activate` | `TriggerSaleApproved` | Transição válida para `StatusActive` | `status=ACTIVE`, `periodEnd=base+plan.Duration()`, `graceEnd=zero`, `lastEventAt=occurredAt` | `ErrOccurredAtRequired`, `ErrTransitionNotAllowed` |
| `Renew` | `TriggerSubscriptionRenewed` | Transição válida para `StatusActive` | Mesmo que `Activate`; base = `max(periodEnd, occurredAt)` — preserva tempo não consumido | `ErrOccurredAtRequired`, `ErrTransitionNotAllowed` |
| `MarkPastDue` | `TriggerSubscriptionLate` | Transição válida para `StatusPastDue` | `status=PAST_DUE`, `graceEnd=occurredAt+graceDuration`, `lastEventAt=occurredAt` | `ErrOccurredAtRequired`, `ErrTransitionNotAllowed` |
| `MarkCanceled` | `TriggerSubscriptionCanceled` | Transição válida para `StatusCanceledPending` | `status=CANCELED_PENDING`, `graceEnd=zero`, `lastEventAt=occurredAt` | `ErrOccurredAtRequired`, `ErrTransitionNotAllowed` |
| `MarkRefunded` | `TriggerRefunded` | Transição válida para `StatusRefunded` | `status=REFUNDED`, `graceEnd=zero`, `lastEventAt=occurredAt`; **idempotente**: se já `REFUNDED` e `occurredAt <= lastEventAt`, nenhum campo é alterado | `ErrOccurredAtRequired`, `ErrTransitionNotAllowed` |
| `MarkExpiredAfterGrace` | `TriggerGraceExpired` | `status == StatusPastDue` (único estado de origem) | `status=EXPIRED`, `graceEnd=zero`, `lastEventAt=occurredAt` | `ErrOccurredAtRequired`, `ErrTransitionNotAllowed` |

Todos os métodos delegam a `applyStatusTransition`, que:
1. Rejeita `occurredAt` zero com `ErrOccurredAtRequired`.
2. Consulta `TransitionService.TargetStatus` para validar a transição.
3. Despacha para o `apply*` específico do status alvo.
4. Retorna `ErrTransitionNotAllowed` para qualquer combinação não mapeada.

### Invariantes Guardadas

- `occurredAt` nunca pode ser `time.Time{}` zero — checado em `applyStatusTransition` antes de qualquer mutação.
- `periodEnd` em `applyActive`: a base é `max(s.periodEnd, occurredAt)`, garantindo que uma renovação antecipada não perde o tempo restante do período em curso.
- `applyRefunded` é idempotente: múltiplos eventos `REFUNDED` com `occurredAt <= lastEventAt` não produzem efeito.
- `StatusTrialing` não possui saídas válidas na `transitionTable` — nenhum trigger produz transição a partir de `TRIALING`.
- `StatusRefunded` é terminal: a linha 6 da `transitionTable` é completamente `false`, impedindo qualquer saída; a única exceção é o caso especial de idempotência no próprio `TriggerRefunded`.

---

## Máquina de Estados

### Diagrama

```
                     ┌─────────────────────┐
                     │   [zero / sem status] │
                     │   (NewSubscription)   │
                     └──────────┬────────────┘
                                │ SaleApproved
                                │ SubscriptionRenewed
                                ▼
          ┌─────────┐     ┌──────────────┐   SubscriptionLate    ┌──────────────┐
          │TRIALING │     │    ACTIVE    │──────────────────────▶│   PAST_DUE   │
          │ (stuck) │     │              │◀──────────────────────│              │
          └─────────┘     │              │  SaleApproved /       │              │
                          │              │  SubscriptionRenewed  └──────┬───────┘
                          │              │                              │         │
                          │              │◀─────────────────────────────┘         │ GraceExpired
                          │              │  SaleApproved /                        ▼
                          │              │  SubscriptionRenewed          ┌──────────────┐
                          └──────┬───────┘                               │   EXPIRED    │
                                 │                                       └──────┬───────┘
                    Canceled     │  Refunded (qualquer estado)                 │ Refunded
                                 ▼                                             ▼
                      ┌──────────────────┐   Refunded           ┌──────────────────────┐
                      │ CANCELED_PENDING │────────────────────▶ │      REFUNDED        │
                      └──────────────────┘                       │  (terminal/idempot.) │
                                                                  └──────────────────────┘
```

`TRIALING` não possui transições de saída definidas na `transitionTable` — é um estado sem caminho de evolução no código atual.

### Tabela de Transições Permitidas

| De | Para | Trigger | Condição adicional |
|---|---|---|---|
| `[zero]` | `ACTIVE` | `SaleApproved`, `SubscriptionRenewed` | `status == 0` (assinatura nunca ativada) |
| `ACTIVE` | `ACTIVE` | `SaleApproved`, `SubscriptionRenewed` | `CanTransition(Active→Active) = true` |
| `ACTIVE` | `PAST_DUE` | `SubscriptionLate` | `CanTransition(Active→PastDue) = true` |
| `ACTIVE` | `CANCELED_PENDING` | `SubscriptionCanceled` | `CanTransition(Active→CanceledPending) = true` |
| `ACTIVE` | `REFUNDED` | `Refunded` | `CanTransition(Active→Refunded) = true` |
| `PAST_DUE` | `ACTIVE` | `SaleApproved`, `SubscriptionRenewed` | `CanTransition(PastDue→Active) = true` |
| `PAST_DUE` | `PAST_DUE` | `SubscriptionLate` | `CanTransition(PastDue→PastDue) = true` |
| `PAST_DUE` | `CANCELED_PENDING` | `SubscriptionCanceled` | `CanTransition(PastDue→CanceledPending) = true` |
| `PAST_DUE` | `EXPIRED` | `GraceExpired` | `current == StatusPastDue` (verificação direta) |
| `PAST_DUE` | `REFUNDED` | `Refunded` | `CanTransition(PastDue→Refunded) = true` |
| `CANCELED_PENDING` | `ACTIVE` | `SaleApproved`, `SubscriptionRenewed` | `CanTransition(CanceledPending→Active) = true` |
| `CANCELED_PENDING` | `REFUNDED` | `Refunded` | `CanTransition(CanceledPending→Refunded) = true` |
| `EXPIRED` | `ACTIVE` | `SaleApproved`, `SubscriptionRenewed` | `CanTransition(Expired→Active) = true` |
| `EXPIRED` | `REFUNDED` | `Refunded` | `CanTransition(Expired→Refunded) = true` |
| `REFUNDED` | `REFUNDED` | `Refunded` | Caso especial de idempotência; `applyRefunded` é no-op se `occurredAt <= lastEventAt` |

### Tabela de Transições Proibidas

| De | Para | Trigger tentado | Motivo |
|---|---|---|---|
| `TRIALING` | qualquer | qualquer | Linha 1 da `transitionTable` é completamente `false` |
| `ACTIVE` | `EXPIRED` | `GraceExpired` | `GraceExpired` só é aceito quando `current == StatusPastDue` |
| `CANCELED_PENDING` | `PAST_DUE` | `SubscriptionLate` | `CanTransition(CanceledPending→PastDue) = false` |
| `CANCELED_PENDING` | `CANCELED_PENDING` | `SubscriptionCanceled` | `CanTransition(CanceledPending→CanceledPending) = false` |
| `EXPIRED` | `PAST_DUE` | `SubscriptionLate` | `CanTransition(Expired→PastDue) = false` |
| `EXPIRED` | `CANCELED_PENDING` | `SubscriptionCanceled` | `CanTransition(Expired→CanceledPending) = false` |
| `REFUNDED` | qualquer (≠ self) | qualquer | Linha 6 da `transitionTable` é completamente `false`; `REFUNDED` é terminal |

---

## Detecção de Regressão (TransitionService / DecisionService)

O `TransitionService` expõe métodos `Decide*` que retornam um `Decision` antes de qualquer mutação no aggregate, permitindo que o chamador ignore eventos fora de ordem sem retornar erro.

### Tipo Decision

```
Decision uint8
  DecisionApply           = 1   // aplicar a transição normalmente
  DecisionSkipAsRegression = 2  // ignorar: evento mais antigo que o último processado
```

### Regras de Regressão

| Método | Trigger consultado | Condição de regressão (`IsRegression = true`) | Decisão retornada |
|---|---|---|---|
| `DecideRenewal` | `SubscriptionRenewed` | `occurredAt <= lastEventAt` E `lastEventAt` não é zero E a transição mudaria o status | `DecisionSkipAsRegression` |
| `DecidePastDue` | `SubscriptionLate` | `occurredAt <= lastEventAt` E `lastEventAt` não é zero E a transição mudaria o status | `DecisionSkipAsRegression` |
| `DecideCancellation` | `SubscriptionCanceled` | `occurredAt <= lastEventAt` E `lastEventAt` não é zero E a transição mudaria o status | `DecisionSkipAsRegression` |
| _(sem método)_ | `Refunded` | `IsRegression` retorna **sempre `false`** — estornos nunca são descartados por timestamp | `DecisionApply` (sempre) |
| _(sem método)_ | `GraceExpired` | `IsRegression` retorna **sempre `false`** — expiração por carência nunca é descartada | `DecisionApply` (sempre) |

**Regra de `IsRegression`:**

```
IsRegression = true quando:
  trigger ∉ {TriggerRefunded, TriggerGraceExpired}
  E lastEventAt ≠ zero
  E occurredAt ≤ lastEventAt
  E TargetStatus(current, trigger) ≠ current
```

---

## Value Objects

### Status

| Constante | Valor uint8 | Wire string | `IsActiveForBilling` | `IsTerminal` | Semântica |
|---|---|---|---|---|---|
| `StatusTrialing` | 1 | `"TRIALING"` | `false` | `false` | Estado de avaliação/teste; sem caminho de saída definido |
| `StatusActive` | 2 | `"ACTIVE"` | `true` | `false` | Assinatura vigente; usuário tem acesso pleno |
| `StatusPastDue` | 3 | `"PAST_DUE"` | `true` | `false` | Pagamento em atraso; assinatura ainda válida durante janela de carência |
| `StatusCanceledPending` | 4 | `"CANCELED_PENDING"` | `true` | `false` | Cancelamento solicitado; vigência ainda corre até o fim do período pago |
| `StatusExpired` | 5 | `"EXPIRED"` | `false` | `true` | Carência esgotada; acesso encerrado definitivamente |
| `StatusRefunded` | 6 | `"REFUNDED"` | `false` | `true` | Estorno processado; terminal e idempotente |

**Construtor/parser:**

| Função | Parâmetro | Validação | Erro retornado |
|---|---|---|---|
| `ParseStatus` | `s string` | Lookup em `statusByWire`; rejeita desconhecido | `ErrUnknownStatus` com fmt `%q: %w` |

`statusByWire` é construído uma única vez via `var` de inicialização (inversão de `statusTable`); lookup é O(1).

### Plan

| Constante `PlanCode` | Valor string | Semântica |
|---|---|---|
| `PlanCodeMonthly` | `"MONTHLY"` | Plano mensal |
| `PlanCodeQuarterly` | `"QUARTERLY"` | Plano trimestral |
| `PlanCodeAnnual` | `"ANNUAL"` | Plano anual |

**Construtor:**

| Função | Parâmetros | Validação | Erro retornado |
|---|---|---|---|
| `NewPlan` | `code string`, `durationDays int` | `PlanCode.IsSupported()` rejeita código desconhecido; `durationDays > 0` obrigatório | `ErrPlanCodeInvalid`, `ErrPlanDurationInvalid` |

**Métodos:**

| Método | Retorno | Comportamento |
|---|---|---|
| `Code()` | `PlanCode` | Retorna o código tipado |
| `DurationDays()` | `int` | Retorna os dias de duração armazenados |
| `Duration()` | `time.Duration` | Converte `durationDays` para `time.Duration`: `days * 24h` |

### FunnelToken

| Função | Parâmetro | Validação | Erro retornado |
|---|---|---|---|
| `NewFunnelToken` | `raw string` | `strings.TrimSpace(raw) != ""` | `ErrFunnelTokenEmpty` |

### KiwifySubscriptionID

| Função | Parâmetro | Validação | Erro retornado |
|---|---|---|---|
| `NewKiwifySubscriptionID` | `raw string` | `strings.TrimSpace(raw) != ""` | `ErrKiwifySubscriptionIDEmpty` |

### GraceWindow

| Constante | Valor | Tipo base | Uso |
|---|---|---|---|
| `DefaultGraceWindow` | `3 * 24 * time.Hour` (72 h) | `time.Duration` | Passado como `graceDuration` para `MarkPastDue`; define `graceEnd = occurredAt + 72h` |

---

## Erros Públicos

| Variável | Pacote | Contexto de uso |
|---|---|---|
| `ErrOccurredAtRequired` | `entities` | `applyStatusTransition` quando `occurredAt.IsZero()` |
| `ErrTransitionNotAllowed` | `entities` | `applyStatusTransition` quando trigger não produz target válido |
| `ErrUnknownStatus` | `valueobjects` | `ParseStatus` quando wire string não está em `statusByWire` |
| `ErrPlanCodeInvalid` | `valueobjects` | `NewPlan` quando código não está em `{MONTHLY, QUARTERLY, ANNUAL}` |
| `ErrPlanDurationInvalid` | `valueobjects` | `NewPlan` quando `durationDays <= 0` |
| `ErrFunnelTokenEmpty` | `valueobjects` | `NewFunnelToken` quando string pós-trim é vazia |
| `ErrKiwifySubscriptionIDEmpty` | `valueobjects` | `NewKiwifySubscriptionID` quando string pós-trim é vazia |

---

## Referências

- [Use Cases](usecases.md)
- [Schema](schema.md)
- [Eventos](events.md)

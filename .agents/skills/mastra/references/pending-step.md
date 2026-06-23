# Pending Step — clarificação de categoria (resume antes do parse)

R-AGENT-WF-001.7: quando uma clarificação de categoria é necessária, o estado de retomada **deve**
ser salvo como `pendingexpense.Draft` antes de retornar `OutcomeClarify`. O draft é o "suspend/resume"
Mastra adaptado ao Go.

## Tipos (fechados)

`domain/pendingexpense/draft.go`:

```go
type AwaitingKind string
const (
    AwaitingCategoryConfirm AwaitingKind = "category_confirm" // sim/não
    AwaitingCategoryChoice  AwaitingKind = "category_choice"  // 1/2/3 entre candidatos
)

type TransactionKind string
const (
    TransactionKindExpense      TransactionKind = "expense"
    TransactionKindIncome       TransactionKind = "income"
    TransactionKindCardPurchase TransactionKind = "card_purchase"
)

type Draft struct {
    AmountCents, Installments                      int...
    Merchant, PaymentMethod, Direction, OccurredAt string
    CategoryID, CategoryPath, CardHint             string
    Candidates                                     []string
    AwaitingKind                                   AwaitingKind
    TransactionKind                                TransactionKind
}
```

`Encode`/`Decode` serializam para JSONB em `agent_sessions`.

## Fluxo Save → Resume → Clear

1. **Save** — `categoryClarification` detecta `CategoryAmbiguousError`
   (`AwaitingCategoryChoice` + `Candidates`) ou `CategoryNeedsConfirmationError`
   (`AwaitingCategoryConfirm`); `savePendingDraft(...)` grava o `Draft` e retorna `OutcomeClarify`.
2. **Resume** — `continuePendingExpenseConfirmation(ctx, userID, channel, text)` roda **antes** do
   `ParseInbound`: se há draft, interpreta a resposta do usuário (escolha/confirmação) sem chamar LLM.
3. **Clear** — `clearPendingDraft(...)` apaga o draft imediatamente após executar ou cancelar.
   Nunca deixar draft órfão.

Cobre `KindRecordExpense`, `KindRecordIncome`, `KindRecordCardPurchase`. Para `card_purchase` a
retomada usa `ForceCategory` no command. O kind de retomada é derivado de `TransactionKind`
(`buildPendingDraft`), não de um switch sobre `intent.Kind`.

## Ao estender

- Novo tipo de clarificação → adicionar valor fechado a `AwaitingKind` (ver `state-as-type.md`) e
  cobrir o resume.
- **Proibido** retornar `OutcomeClarify` sem salvar o `Draft` (viola R-AGENT-WF-001.7).
- Pending Step é exclusivo do `internal/agent`.

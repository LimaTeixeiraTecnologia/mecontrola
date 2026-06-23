# State-as-Type — enums fechados e smart constructors (DMMF)

R-AGENT-WF-001.3: `ToolOutcome`, `RunStatus`, `AwaitingKind`, `TransactionKind` (e `intent.Kind`)
são **tipos fechados**. Nunca string livre em assinatura pública nem branching sobre string crua.
Persistência em coluna TEXT é permitida via `String()`; a fronteira de código permanece tipada.

## Inventário e onde mora

| Tipo              | Valores                                              | Arquivo |
| ----------------- | --------------------------------------------------- | ------- |
| `ToolOutcome`     | `routed, fallback, parse_error, usecase_error, missing_resolver, reply_failed, empty_text, authz_denied, clarify, policy_blocked, replay` | `application/tools/tool.go` |
| `RunStatus`       | `running, succeeded, failed`                        | `domain/entities/run.go` |
| `AwaitingKind`    | `category_confirm, category_choice`                 | `domain/pendingexpense/draft.go` |
| `TransactionKind` | `expense, income, card_purchase`                    | `domain/pendingexpense/draft.go` |
| `intent.Kind`     | 21 kinds fechados                                   | `domain/intent/intent.go` |
| `Confidence`      | `[0,1]` com smart constructor                       | `domain/valueobjects/confidence.go` |
| `DecisionStatus`  | `pending, executed, rejected, awaiting_confirmation`| `domain/valueobjects/decision_status.go` |

## Padrão canônico (ex.: `ToolOutcome`)

```go
type ToolOutcome int

const (
    OutcomeRouted ToolOutcome = iota + 1
    OutcomeFallback
    // ...
)

func (o ToolOutcome) String() string { /* switch exaustivo */ }

func ParseOutcome(raw string) (ToolOutcome, error) { /* string -> tipo, erro se inválido */ }
```

Elementos obrigatórios:
- `iota + 1` (zero-value inválido, força construção explícita).
- `String()` com switch exaustivo.
- `Parse...(raw string) (T, error)` como smart constructor que **rejeita** valor desconhecido
  (ex.: `ErrToolOutcomeUnknown`, `ErrRunStatusInvalid`).

## Como adicionar um novo valor

1. Adicionar a constante ao bloco `const`.
2. Cobrir o novo valor em `String()` e no `Parse...`.
3. Atualizar todo switch exaustivo que consome o tipo (o compilador não força — buscar usos).
4. Se o valor cruza fronteira de métrica, confirmar que continua sendo label enum-fechado
   (R-AGENT-WF-001.5 / R-TXN-004): nunca `user_id`/`category_id` como label.
5. Teste de unidade cobrindo `String()`↔`Parse...` ida e volta + erro no valor inválido.

## Anti-padrões proibidos (hard)

- `string` solta em assinatura de Tool/Workflow/Run para representar outcome/status/kind.
- Construir o valor a partir de string externa sem passar pelo smart constructor.
- `Result[T,E]` customizado, `Either`, currying, DSL de pipeline, mônadas (ver `governance.md`).

# State-as-Type — enums fechados e smart constructors (DMMF)

Estados de fronteira são **tipos fechados**: nunca string livre em assinatura pública nem branching sobre
string crua. Persistência em coluna TEXT é permitida via `String()`; a fronteira de código permanece tipada.
Vale para o kernel (R-WF-KERNEL-001.3) e para o primitivo de agent (R-AGENT-WF-001.3).

## Inventário e onde mora

| Tipo | Valores | Arquivo |
| --- | --- | --- |
| `agent.RunStatus` | `running, succeeded, failed` | `internal/platform/agent/types.go` |
| `agent.ToolOutcome` | `routed, clarify, usecaseError, missingResolver, replay` | `internal/platform/agent/types.go` |
| `agent.AwaitingKind` | `none, confirm` | `internal/platform/agent/types.go` |
| `agent.ExecutionMode` | `sync, stream` | `internal/platform/agent/types.go` |
| `workflow.RunStatus` | `running, suspended, succeeded, failed` | `internal/platform/workflow/step.go` |
| `workflow.StepStatus` | `completed, suspended, failed, skipped` | `internal/platform/workflow/step.go` |
| `workflow.SuspendReason` | `awaiting_input` | `internal/platform/workflow/step.go` |
| `memory.MessageRole` | `user, assistant, tool, system` | `internal/platform/memory/types.go` |
| `scorer.ScorerKind` | `code_based, llm_judged` | `internal/platform/scorer/types.go` |
| `scorer.SamplingType` | `ratio, always, never` | `internal/platform/scorer/types.go` |

Nota: `agent.RunStatus` e `workflow.RunStatus` são **tipos distintos** (mecanismo do kernel vs Run semântico
do agent — addendum R-AGENT-WF-001.6-A); não os compartilhe.

## Padrão canônico (ex.: `agent.ToolOutcome`)

```go
type ToolOutcome int

const (
    ToolOutcomeRouted ToolOutcome = iota + 1
    ToolOutcomeClarify
    ToolOutcomeUsecaseError
    ToolOutcomeMissingResolver
    ToolOutcomeReplay
)

func (o ToolOutcome) String() string { /* switch exaustivo */ }
func (o ToolOutcome) IsValid() bool  { return o >= ToolOutcomeRouted && o <= ToolOutcomeReplay }
func ParseToolOutcome(s string) (ToolOutcome, error) { /* string -> tipo, erro se inválido */ }
```

Elementos obrigatórios:
- `iota + 1` (zero-value inválido, força construção explícita).
- `String()` com switch exaustivo.
- `IsValid()` por faixa.
- `Parse...(s string) (T, error)` como smart constructor que **rejeita** valor desconhecido
  (ex.: `errInvalidToolOutcome`, `errInvalidRunStatus`).

`MessageRole` é a variante string-based (tipo nomeado `string` com `IsValid()` por `switch`), também fechado.

## Como adicionar um novo valor

1. Adicionar a constante ao bloco `const` (preservando ordem para `IsValid()` por faixa).
2. Cobrir o novo valor em `String()` e no `Parse...`.
3. Atualizar todo switch exaustivo que consome o tipo (o compilador não força — buscar usos).
4. Ajustar `IsValid()` (limite superior da faixa) se aplicável.
5. Se cruza fronteira de métrica, confirmar que continua label enum-fechado (R-WF-KERNEL-001.4 / R-TXN-004):
   nunca `user_id`/`resource_id`/`correlation_key`/`category_id` como label.
6. Teste cobrindo `String()`↔`Parse...` ida e volta + erro no valor inválido.

## Anti-padrões proibidos (hard)

- `string` solta em assinatura pública para representar status/outcome/kind/role.
- Construir o valor a partir de string externa sem passar pelo smart constructor.
- `Result[T,E]` customizado, `Either`, currying, DSL de pipeline, mônadas (ver `governance.md`).

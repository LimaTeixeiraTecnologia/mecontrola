# Adicionar um novo Workflow / Tool — a receita

Seam único: `buildRegistry()` em
`internal/agent/application/services/agent_workflows.go`. Toda extensão de comportamento entra aqui.
**Nunca** adicione `case intent.Kind` ao switch de `daily_ledger_agent.go`.

Carregue `go-implementation` antes de tocar qualquer `.go`.

## Os 6 passos

### 1. Declarar o `intent.Kind` (se for um kind novo)

`internal/agent/domain/intent/intent.go`: adicionar a constante ao bloco `const (... Kind = iota+1)`,
e atualizar `String()`, `ParseKind()` e — se for escrita — `IsWrite()`. Kind é tipo fechado: sem
string livre. Ver `state-as-type.md`.

### 2. Implementar a Tool como struct fina sob `application/tools/`

Criar uma struct em `internal/agent/application/tools/` (agrupada por categoria — p.ex.
`transactions_tools.go`, `budget_tools.go`, `cards_tools.go`) que implementa `tools.Tool`
(`Name() / Descriptor() / Execute`). O `Execute` apenas mapeia `intent.Intent` → DTO/command, chama o
**binding → usecase**, mapeia o retorno para `tools.ToolResult{Reply, Outcome, Kind}` e delega
clarificação ao `ClarificationResolver`. Sem regra de negócio, sem SQL, sem branching de domínio.

Os contratos de binding consumidos (interfaces + structs Input/Result) vivem em `tools/contracts.go`
— interface no consumidor (R6.3); o pacote `tools` é a fonte única, sem aliases em `services`.
Métrica via `Recorder` injetado (`tools/recorder.go`). Padrão existente: `RecordExpenseTool`,
`ListCardsTool`.

```go
type FooTool struct {
    foo  FooBinding
    rec  *Recorder
    o11y observability.Observability
}

func NewFooTool(foo FooBinding, rec *Recorder, o11y observability.Observability) *FooTool {
    return &FooTool{foo: foo, rec: rec, o11y: o11y}
}

func (t *FooTool) Name() string         { return "foo" }
func (t *FooTool) Descriptor() ToolSpec { return ToolSpec{Name: "foo", IntentKind: intent.KindFoo} }

func (t *FooTool) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
    // mapear in.Intent → input, chamar t.foo (binding → usecase), mapear → ToolResult
}
```

### 3. Instanciar a Tool no `buildRegistry()`

Dentro de `buildRegistry()` em `agent_workflows.go`, instanciar a struct (não mais closure `routeTool`):

```go
fooTool := tools.NewFooTool(a.fooBinding, a.recorder, a.o11y)
```

### 4. Vincular a um Workflow

Adicionar `workflow.KindTool{Kind: intent.KindFoo, Tool: fooTool}` a um `NewWorkflow(...)` existente
(`transactions`/`budget`/`cards`) **ou** criar um novo workflow:

```go
fooWorkflow, err := workflow.NewWorkflow("foo", guard, // guard p/ escrita; nil p/ só leitura
    workflow.KindTool{Kind: intent.KindFoo, Tool: fooTool},
)
if err != nil {
    return nil, err
}
```

- **Escrita** → passar o `guard` compartilhado (`a.newWriteGuard()`), que aplica authz/replay/policy/audit.
- **Leitura/fallback** → `guard` pode ser `nil`; `composite.Execute` pula a guarda quando
  `!kind.IsWrite() || guard == nil`.

### 5. Registrar o kind como roteável

Adicionar `intent.KindFoo` ao slice de `routableKinds()` e, se criou workflow novo, incluí-lo na
chamada final `workflow.NewRegistry(routableKinds(), ..., fooWorkflow)`.

### 6. Wiring (se novas dependências)

Se a Tool precisar de um binding/usecase ainda não injetado: declarar a interface do binding em
`tools/contracts.go`, adicionar o campo ao `DailyLedgerAgent`, passá-lo ao construtor `NewFooTool`
em `buildRegistry()` e fiar a implementação em `internal/agent/module.go` (DI manual, sem framework,
sem `init()`).

## Checklist de fronteira (R-AGENT-WF-001.2 / R-ADAPTER-001)

- [ ] Tool (struct sob `application/tools/`) sem regra de negócio, SQL (`QueryContext`/`ExecContext`/...) ou branching de domínio.
- [ ] Zero comentários em `.go` (exceto `//go:`, `//nolint:`, `// Code generated`).
- [ ] `Outcome` é `ToolOutcome` fechado; `Kind` é `intent.Kind` fechado.
- [ ] Escrita passa pelo `guard`; leitura não duplica authz/policy.
- [ ] Sem chamada de LLM dentro da tool/workflow (exceto exceções sancionadas — ver `parse-llm-boundary.md`).
- [ ] Teste de unidade no padrão testify/suite (R-TESTING-001).

Valide com `rules-checklist.md`.

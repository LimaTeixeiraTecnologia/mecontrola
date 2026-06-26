# Adicionar ou estender uma capability do agent — a receita

O `buildRegistry()` em `internal/agent/application/services/agent_workflows.go` continua sendo o
seam de roteamento para kinds roteáveis, mas ele **não é o único seam** do módulo. Antes de mudar o
código, classifique qual caminho está sendo estendido:

1. **Registry seam** — `buildRegistry()` + `routableKinds()` em
   `internal/agent/application/services/agent_workflows.go`.
2. **Kernel write seam** — `buildKernelDefinition()` em
   `internal/agent/application/services/agent_workflows.go` + `NewTransactionsWriteDefinition(...)`
   em `internal/agent/application/workflow/transactions_write.go`.
3. **Confirmation seam** — `NewDestructiveConfirmDefinition(...)` em
   `internal/agent/application/workflow/destructive_confirm.go`.
4. **Plan seam** — `NewPlanExecutor(...)` em
   `internal/agent/application/workflow/plan_executor.go`.
5. **Resume chain seam** — ordem em
   `internal/agent/application/services/daily_ledger_agent.go`:
   `continuePendingExpenseConfirmation(...) -> continuePendingPlan(...) -> continuePendingApproval(...)`.

Fonte única de classificação, auditoria e métricas: o catálogo canônico em
`internal/agent/application/capability/` (`BuildCatalog()`, `CapabilitySpec`, `Catalog.Lookup`,
`Catalog.List`, `Catalog.Classify`). Toda extensão aplicável deve registrar a `CapabilitySpec`
correspondente.

**Nunca** adicione `case intent.Kind` ao switch de `daily_ledger_agent.go`.

Carregue `go-implementation` antes de tocar qualquer `.go`.

## Checklist de extensão

### 1. Novo kind

- Declarar o `intent.Kind` em `internal/agent/domain/intent/intent.go`: adicionar a constante ao
  bloco `const (... Kind = iota+1)`, e atualizar `String()`, `ParseKind()` e — se for escrita —
  `IsWrite()`. Kind é tipo fechado: sem string livre. Ver `state-as-type.md`.
- Registrar a `CapabilitySpec` no catálogo canônico em
  `internal/agent/application/capability/build.go`.
- Se o kind for roteável, registrar em `buildRegistry()` e atualizar `routableKinds()`.

### 2. Nova tool

- Implementar a Tool como struct fina sob `internal/agent/application/tools/`, agrupada pela
  categoria já existente, implementando `tools.Tool` (`Name() / Descriptor() / Execute`).
- O `Execute` apenas mapeia `intent.Intent` -> DTO/command, chama binding -> usecase, mapeia o
  retorno para `tools.ToolResult{Reply, Outcome, Kind}` e delega clarificação ao
  `ClarificationResolver`.
- Registrar a `CapabilitySpec` no catálogo canônico em
  `internal/agent/application/capability/build.go`.
- Instanciar a tool no seam correto de roteamento (`buildRegistry()`) quando ela atender um kind
  roteável.

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

### 3. Novo workflow

- Se a capability pede um owner novo ou um agrupamento novo de kinds, criar o workflow em
  `buildRegistry()` com `workflow.NewWorkflow(...)` ou `agentwf.NewIntentWorkflow(...)`, conforme o
  padrão real do arquivo.
- Registrar a `CapabilitySpec` no catálogo canônico em
  `internal/agent/application/capability/build.go` com `WorkflowID` coerente.
- Se criou workflow novo, incluir o workflow na chamada final de `workflow.NewRegistry(...)`.

Dentro de `buildRegistry()` em `agent_workflows.go`, instanciar a struct (não mais closure `routeTool`):

```go
fooTool := tools.NewFooTool(a.fooBinding, a.recorder, a.o11y)
```

### 4. Novo pending state

- Para clarificação/suspend-resume nova, evoluir o tipo fechado apropriado (`AwaitingKind`,
  `TransactionKind`, `confirmation.ConfirmState`, `PlanState` ou draft análogo) e cobrir save ->
  resume -> clear.
- Registrar a `CapabilitySpec` no catálogo canônico em
  `internal/agent/application/capability/build.go` quando o pending state pertencer a uma
  capability roteável ou destrutiva existente.
- Preservar a ordem da cadeia de resume em `daily_ledger_agent.go`:
  `continuePendingExpenseConfirmation(...) -> continuePendingPlan(...) -> continuePendingApproval(...)`.

### 5. Novo gate de confirmação

- Para operação destrutiva ou sensível, evoluir `NewDestructiveConfirmDefinition(...)` em
  `internal/agent/application/workflow/destructive_confirm.go` e os mapas/deps associados, sem
  empurrar regra de negócio para fora do workflow de confirmação.
- Registrar a `CapabilitySpec` no catálogo canônico em
  `internal/agent/application/capability/build.go` com `RequiresConfirmation=true`.
- Se houver kind novo associado, também registrar no seam de roteamento adequado.

### 6. Novo plan step

- Evoluir `internal/agent/application/workflow/plan_executor.go` quando o comportamento for
  multi-step e depender de `NewPlanExecutor(...)`, `PlanStepDispatcher` ou serialização de
  `PlanStepItem`.
- Registrar a `CapabilitySpec` no catálogo canônico em
  `internal/agent/application/capability/build.go` para cada capability executada pelo plano.
- Se o plan step introduzir kind roteável novo, registrar também em `buildRegistry()` e
  `routableKinds()`.

## Receita do registry seam

Use esta receita quando a mudança cair no seam de roteamento (`buildRegistry()`).

### 1. Implementar a Tool como struct fina sob `application/tools/`

Criar uma struct em `internal/agent/application/tools/` (agrupada por categoria — p.ex.
`transactions_tools.go`, `budget_tools.go`, `cards_tools.go`) que implementa `tools.Tool`
(`Name() / Descriptor() / Execute`). O `Execute` apenas mapeia `intent.Intent` -> DTO/command,
chama binding -> usecase, mapeia o retorno para `tools.ToolResult{Reply, Outcome, Kind}` e delega
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
    // mapear in.Intent -> input, chamar t.foo (binding -> usecase), mapear -> ToolResult
}
```

### 2. Instanciar a Tool no `buildRegistry()`

Dentro de `buildRegistry()` em `agent_workflows.go`, instanciar a struct:

```go
fooTool := tools.NewFooTool(a.fooBinding, a.recorder, a.o11y)
```

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

### 3. Registrar a tool no workflow

Adicionar `intent.KindFoo` ao slice de `routableKinds()` e, se criou workflow novo, incluí-lo na
chamada final `workflow.NewRegistry(routableKinds(), ..., fooWorkflow)`.

### 4. Registrar o kind como roteável + `CapabilitySpec`

- Adicionar `intent.KindFoo` ao slice de `routableKinds()`.
- Registrar a `CapabilitySpec` correspondente em
  `internal/agent/application/capability/build.go`.

### 5. Wiring (se novas dependências)

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

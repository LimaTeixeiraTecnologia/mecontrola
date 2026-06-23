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

### 2. Escrever o handler fino `routeXxx`

Método de `*DailyLedgerAgent` que apenas mapeia `intent.Intent` → DTO/command, chama o
**binding → usecase** e devolve `RouteResult{Reply, Outcome, Kind}`. Sem regra de negócio, sem SQL,
sem branching de domínio. Padrão existente, p.ex. `routeLogExpense`, `routeListCards`.

### 3. Embrulhar como Tool via `routeTool`

Dentro de `buildRegistry()`:

```go
fooTool := a.routeTool("foo", intent.KindFoo, func(ctx context.Context, in tools.ToolInput) RouteResult {
    return a.routeFoo(ctx, in.UserID, in.Channel, in.Intent)
})
```

`routeTool(name, kind, route)` já cria o `tools.ToolSpec` e adapta `RouteResult` ↔ `tools.ToolResult`.

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

Se o handler precisar de um binding/usecase ainda não injetado, adicione o campo ao
`DailyLedgerAgent` e fie em `internal/agent/module.go` (DI manual, sem framework, sem `init()`).

## Checklist de fronteira (R-AGENT-WF-001.2 / R-ADAPTER-001)

- [ ] Tool/handler sem regra de negócio, SQL (`QueryContext`/`ExecContext`/...) ou branching de domínio.
- [ ] Zero comentários em `.go` (exceto `//go:`, `//nolint:`, `// Code generated`).
- [ ] `Outcome` é `ToolOutcome` fechado; `Kind` é `intent.Kind` fechado.
- [ ] Escrita passa pelo `guard`; leitura não duplica authz/policy.
- [ ] Sem chamada de LLM dentro da tool/workflow (exceto exceções sancionadas — ver `parse-llm-boundary.md`).
- [ ] Teste de unidade no padrão testify/suite (R-TESTING-001).

Valide com `rules-checklist.md`.

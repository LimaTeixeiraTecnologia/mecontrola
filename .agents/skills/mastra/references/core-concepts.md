# Core Concepts — primitivos Mastra no Go do internal/agent

Use para situar cada conceito Mastra no código Go real antes de implementar.

## Agent

Dois objetos formam o "Agent":

- `AgentRuntime` (`application/services/agent_runtime.go`) — fronteira de execução Mastra-style:
  resolve Thread, abre/fecha Run, emite métricas. `Execute(ctx, principal, channel, peer, text, messageID)`.
- `DailyLedgerAgent` (`application/services/daily_ledger_agent.go`) — orquestrador fino: detém o
  `*workflow.Registry`, roda guarda/format compartilhados e delega resolução de kind ao registry.
  **Não** contém `switch case intent.Kind` de domínio.

## Workflow

`workflow.Workflow` é a interface; `composite` é a implementação
(`application/workflow/composite.go`):

```go
type Workflow interface {
    ID() string
    Handles(kind intent.Kind) bool
    Execute(ctx context.Context, in tools.ToolInput) (tools.ToolResult, error)
}
```

Um workflow agrupa `KindTool{Kind, Tool}` e aplica o `WriteGuard` quando `kind.IsWrite()`. Há 4
workflows: `transactions`, `budget`, `cards`, `conversational`. O `conversational` recebe `guard=nil`
(só leitura/fallback).

## Tool

`tools.Tool` (`application/tools/tool.go`) é adapter fino de responsabilidade única:

```go
type Tool interface {
    Name() string
    Descriptor() ToolSpec
    Execute(ctx context.Context, in ToolInput) (ToolResult, error)
}
```

Construída via `tools.NewTool(spec, exec)`. No mecontrola, cada tool é montada por
`DailyLedgerAgent.routeTool(name, kind, route)`, que envolve um método `routeXxx` (o handler que
chama binding→usecase). **Proibido** regra de negócio, SQL ou branching de domínio dentro da tool.

`ToolInput` carrega `UserID`, `Channel`, `Intent`, `MessageID`, `Text`, `Confidence`, `Parsed`.
`ToolResult` carrega `Reply`, `Outcome` (`ToolOutcome`), `Kind`.

## Thread

`entities.Thread` (`domain/entities/thread.go`) com identidade canônica `(user_id, channel)` —
espelha Mastra `resourceId=user_id`, `threadId=channel`. Resolvida por
`ThreadGateway.GetOrCreate(ctx, userID, channel)` no `AgentRuntime`.

## Run

`entities.Run` (`domain/entities/run.go`) — execução auditável. `RunStatus` fechado
(`running|succeeded|failed`). `StartRun` → `Resolve` (workflow/tool/kind) → `Finish(outcome, ok, errText)`.
Persistida por `RunGateway` (`Insert`/`Finish`). Ver `thread-run-runtime.md`.

## WorkingMemory

`entities.WorkingMemory` (escopo `resource`, por `user_id`, markdown). O `ContextBuilder`
(`application/prompting/context_builder.go`) concatena persona + budgets + working memory no
`SystemPrompt` usado pelo parser. Ausência de working memory não é erro. Ver `parse-llm-boundary.md`.

## Pending Step

`pendingexpense.Draft` (`domain/pendingexpense/draft.go`) — estado de espera salvo quando uma
clarificação de categoria é necessária. `AwaitingKind` (`category_confirm|category_choice`) e
`TransactionKind` (`expense|income|card_purchase`) são tipos fechados. Resume ocorre **antes** do
`ParseInbound` via `continuePendingExpenseConfirmation`. Ver `pending-step.md`.

## Model router (equivalente Go)

Mastra usa `"provider/model-name"` no model router TS. No Go, modelos são resolvidos por env
(`AGENT_*_LLM_MODEL`) via OpenRouter (`infrastructure/providers/openrouter/`) com `FallbackChain`
(primário + fallbacks) e `CircuitBreaker`. Onboarding usa modelo dedicado por decisão de projeto.

# Refatoração do módulo `internal/agent` — Tools sob `application/tools/` + Runtime sempre-on

- Data: 2026-06-23
- Branch: `refactor/internal-agent-tools`
- Prompt de origem: `docs/prompts/refactor_internal_agent.md`
- Skills obrigatórias declaradas: `go-implementation` (Etapas 1–5, R0–R7) + `mastra`
- Tipo: refatoração de **preservação de comportamento** (sem feature nova; replies, outcomes e métricas inalterados)

## Contexto

O alvo Workflow + Tool + Registry + Runtime (Mastra) já existia e roteava sem switch de domínio
(`Handle → registry.Resolve(kind) → Workflow.Execute → Tool.Execute → binding → usecase`). Os gaps
reais eram: (1) os 21 métodos `route*` viviam em `daily_ledger_agent.go` (1128 linhas) e eram
embrulhados como closures via `routeTool()`, não como `Tool` finas sob `application/tools/` (§4 do
prompt); e (2) o `AgentRuntime` (Thread→Run auditável) era condicional ao flag `RuntimeEnabled`.

Decisões do usuário: escopo **completo** (extração estrutural literal §4 + gaps + gates + report) e
**Runtime sempre-on**.

## Resultado

- Cada uma das 21 ações virou uma `Tool` fina (struct nomeada `Name/Descriptor/Execute`) em arquivo
  próprio sob `internal/agent/application/tools/`.
- `daily_ledger_agent.go`: **1128 → 448 linhas** (orquestra registry + write guard + resume +
  fallback compartilhado).
- `AgentRuntime` agora ativa sempre que o session store está presente; o flag `AGENT_RUNTIME_ENABLED`
  foi removido.

### Quebra do ciclo de import (tools ⇏ services)

`services` importa `tools`. Para os tools consumirem as 21 interfaces de binding sem ciclo, as
**definições canônicas migraram para o pacote `tools`** (`contracts.go`). `tools` passou a ser a
**única fonte de verdade** — `IntentRouterDeps`, os campos de `DailyLedgerAgent`, `module.go`, os
adapters de `infrastructure/binding/`, usecases, onboarding, workflow e os testes referenciam
diretamente `tools.<Nome>`. Os ~61 type/var aliases de compatibilidade que existiam em `services`
foram **removidos** (move §4 100% limpo, sem superfície dupla). `RouteResult` permaneceu em
`services`; o registry já converte `ToolResult ↔ RouteResult`. `errors.As`/`errors.Is` sobre
`*tools.CategoryAmbiguousError` e os sentinelas `tools.Err*` seguem funcionando.

## Arquivos

### Criados (pacote `tools`)
- `contracts.go` — 21 interfaces de binding + structs Input/Result
- `errors.go` — sentinelas + `CategoryAmbiguousError` / `CategoryNeedsConfirmationError`
- `formatting.go` — todas as funções `format*` (migradas de `services/formatting.go`)
- `text.go` — constantes de texto (algumas exportadas para o resume em `services`)
- `retry.go` — `WithReadRetry` (exportada) + `isTransientReadError`
- `recorder.go` — `Recorder` injetável encapsulando o counter `routedTotal` (labels enums: kind/channel/outcome)
- `clarification.go` — `ClarificationResolver` (categoria ambígua/confirmação + cartão; salva
  `pendingexpense.Draft{AwaitingKind}` **antes** de `OutcomeClarify` — R-AGENT-WF-001.7)
- `competence.go` — helper `currentCompetence(loc)`
- `budget_session.go` — `BudgetSessionRunner` (Start/Continue/advance — fonte única entre tool e resume)
- `transactions_tools.go`, `budget_tools.go`, `cards_tools.go`, `conversational_tool.go` — as 21 structs Tool

### Alterados
- `application/services/agent_workflows.go` — `buildRegistry()` instancia as structs Tool (não mais
  closures); `routeTool`/`toToolResult` removidos; `newWriteGuard()` mantido em `services`
- `application/services/daily_ledger_agent.go` — removidos os 21 `route*`, clarifications, budget
  session methods e helpers; mantidos `Handle`, `dispatchWrite`, `authorizeWrite`, `replayDecision`,
  `beginDecisionAudit`, resume (`continuePendingExpenseConfirmation`, `continuePendingBudgetSession`),
  `delegateFallback`, `record`, `buildRegistry`, `newWriteGuard`
- `application/services/intent_router.go` — definições movidas viraram type/var aliases
- `module.go` — `attachRuntime` sem o gate de flag (sempre-on quando o session store existe; mantido
  o caminho explícito `session_store_missing`)
- `configs/config.go` — removido o campo morto `RuntimeEnabled` (`AGENT_RUNTIME_ENABLED`)
- `application/services/formatting.go` — **deletado** (conteúdo migrou para `tools`)
- Testes internos movidos para `tools` (`budget_cancel_internal_test.go`,
  `category_clarification_internal_test.go`, retry) com `package tools`

### Decisão sobre budget session
`BudgetSessionRunner` em `tools/budget_session.go` é fonte única de verdade. A tool `ConfigureBudget`
dispara `Start`; o resume em `Handle` (antes do parse) chama `Continue`, sem duplicar
`advanceBudgetSession`.

## Validação (todas executadas localmente)

| Gate | Comando | Resultado |
|------|---------|-----------|
| Build | `go build ./...` | OK (exit 0) |
| Vet | `go vet ./internal/agent/...` | OK (exit 0) |
| Testes + race | `go test -race -count=1 ./internal/agent/...` | OK (todos os pacotes `ok`, 0 falhas) |
| Gate 1 — switch de domínio | grep `case intent.Kind` em `daily_ledger_agent.go` | OK (cases=0) |
| Gate 2 — zero comentários | grep `^//` em `tools/` + `workflow/` | OK |
| Gate 3 — sem SQL direto | grep `QueryContext/ExecContext/...` em `tools/` + `workflow/` | OK |
| Lint | `golangci-lint run` em `tools/` + `services/` | 0 issues |

## Aderência às regras hard

- **R-AGENT-WF-001.1** — fluxo canônico preservado; nenhum `case` de domínio em `daily_ledger_agent.go`.
- **R-AGENT-WF-001.2** — tools finas: sem regra de negócio nova, SQL ou branching de domínio (prechecks
  de adapter como `AmountCents()==0` e seleção de formato preservados como mapeamento).
- **R-AGENT-WF-001.3** — `ToolOutcome`/`RunStatus`/`AwaitingKind`/`TransactionKind` permanecem tipos fechados.
- **R-AGENT-WF-001.4** — LLM apenas em `ParseInbound`; `conversational` é a exceção sancionada.
- **R-AGENT-WF-001.5/.6** — Run auditável e Thread-first agora sempre ativos quando o store existe.
- **R-AGENT-WF-001.7** — `ClarificationResolver` salva `Draft{AwaitingKind}` antes de `OutcomeClarify`.
- **R-ADAPTER-001.1** — zero comentários em `.go` de produção.
- **Go**: sem `init()`, sem `panic`, sem `var _ Iface = (*T)(nil)`, sem abstração de clock; `errors`
  com `%w`/`errors.Join`; aliases não introduzem assertions estáticas proibidas.

## Cobertura de testes

Os 21 tools são exercitados end-to-end pelos testes existentes `services/intent_router_*` (que agora
dirigem `registry → workflow → tool`), preservados e verdes. O pacote `tools` mantém testes internos
de `clarification`, `budget cancel`, `retry`, `registry` e `tool`. O ciclo Thread→Run é coberto por
`agent_runtime_test.go` e `agent_runtime_integration_test.go`.

## Riscos residuais e suposições

- **Cobertura unitária por tool**: optou-se por não duplicar 21 suítes dedicadas, pois a cobertura
  comportamental já existe no nível `intent_router_*` (adapters finos). Suposição: cobertura de
  integração é suficiente para esses adapters; suítes dedicadas podem ser adicionadas se desejado.
- **Aliases removidos**: a camada de compat (`type X = tools.X`) foi eliminada; `tools` é a única
  fonte de verdade. Verificado: 0 declarações de alias remanescentes, delta líquido de asserts de
  teste = 0 (sem perda de cobertura), `go test -race ./...` = 135 pacotes OK.
- **Runtime sempre-on**: depende do session store (`sessionDB`) presente no bootstrap; quando ausente,
  o caminho `session_store_missing` registra modo degradado explicitamente.
- Nenhum commit foi feito; mudanças permanecem na branch `refactor/internal-agent-tools`.

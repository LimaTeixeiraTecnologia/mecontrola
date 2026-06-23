> **Skill obrigatória:** a skill `go-implementation` é OBRIGATÓRIA para toda alteração Go desta iniciativa
> (carregar `SKILL.md`, executar Etapas 1–5 e o checklist R0–R7 de `references/build.md`, verificar `go.mod`
> antes de usar APIs novas). Alterações não-Go (regras, docs, migrations SQL, templates) seguem `AGENTS.md`,
> `agent-governance` e as regras transversais em `.claude/rules/`.

# Evolução do DailyAgent — Workflow/Tool como padrão canônico (inspirado em Mastra)

## Context

O módulo `internal/agent` do mecontrola já implementa um agente financeiro conversacional
(`DailyLedgerAgent`) cujo fluxo real é `IntentRouter → switch(intent) → binding adapter → usecase
financeiro → domain → repository`. O LLM nunca toca repositório; toda escrita passa por usecases
existentes (transactions/card/budgets) via `infrastructure/binding/`, com `auth.WithPrincipal`
injetando `user_id`. Já há persistência agent-owned (`agent_sessions`, `agent_decisions`,
`agent_working_memory`, `agent_observations`), audit trail idempotente por `message_id`, fallback
chain de LLM com circuit breaker, memória conversacional e publicação via outbox.

Hoje as "tools" existem só como **catálogo estático tool→intent** (`application/tools/registry.go`,
`usecases/tool_catalog.go`); não há interface `Tool` executável nem `Workflow` de primeira classe.
O roteamento é um `switch` gigante em `daily_ledger_agent.go`.

**Objetivo (revisado pelo usuário):** tornar **Workflow + Tool o padrão canônico e obrigatório**
do `internal/agent`. O `switch` deixa de ser o mecanismo de roteamento e passa a delegar para um
**WorkflowRegistry**; cada intent é atendido por um `Workflow → Tool → binding → usecase`. Novos
comportamentos entram como Tool/Workflow, **nunca** como novo `case`. Sobre isso, montar o Runtime
Mastra (Thread/Run auditável) e fechar os gaps funcionais. MVP simples: **um único DailyAgent**;
separação só via Intent Router + Workflows + Tools. Sem duplicar entidades financeiras; reuso
máximo de bindings/usecases. Production-ready, idiomático, sem falso positivo.

**Decisões do usuário (2026-06-23):**
- **`go-implementation` é obrigatória** em toda alteração Go: carregar `SKILL.md`, rodar Etapas 1–5
  e o checklist R0–R7 (`references/build.md`). Verificar `go.mod` antes de usar APIs novas.
- **DMMF (Domain Modeling Made Functional)** aplicado onde fizer sentido: smart constructors,
  *state-as-type* / discriminated unions, *workflow pipeline* (steps), tornar estados ilegais
  irrepresentáveis. Respeitar `domain-modeling.md` e a precedência em `.claude/rules/governance.md`
  (DMMF prevalece sobre Uber para tipo/estado; proibido Result/Either custom, currying, DSL).
- **Atualizar as regras do projeto** para codificar o padrão Workflow/Tool do agent (nova regra
  `R-AGENT-WF-001`) e referenciá-la em `CLAUDE.md`/`AGENTS.md`.
- Editar percentual de categoria → **novo usecase dedicado em `internal/budgets`** (`Decide*` puro
  + testes no próprio módulo).
- AgentRuntime → **wrapper opt-in** sobre o entrypoint atual, com fallback ao caminho legado.
- Escopo → **tudo**: padrão Workflow/Tool canônico + gaps + runtime Thread/Run + regras.

> Primeiro passo da implementação: espelhar este plano completo em
> `docs/runs/2026-06-23-evolucao-dailyagent-mastra.md` (preferência vigente), declarando a skill
> `go-implementation` como obrigatória.

## Decisão arquitetural central — Workflow/Tool canônico

Migrar o roteamento para um padrão de primeira classe, **preservando comportamento** e usando a
suíte de testes existente como rede de segurança:

```
IntentRouter → AgentRuntime(opt-in) → WorkflowRegistry.Resolve(kind) → Workflow.Execute
                                              → Tool.Execute → binding → usecase → domain → repo
```

- **WorkflowRegistry** resolve o `intent.Kind` para o Workflow responsável (transactions/budget/
  cards/conversational). É a substituição estruturada do `switch`.
- Cada **Workflow** compõe **Tools** determinísticas (1 responsabilidade cada). A lógica de
  pre-write (authz + replay + policy + decision audit, hoje em `dispatchWrite`) é extraída para um
  **step de guarda** reutilizável aplicado pelos workflows de escrita — não duplicado por tool.
- Cada **Tool** é adapter fino que **reusa os bindings existentes** (`ExpenseRecorder`,
  `CardCreator`, `CardUpdater`, ...). Tool não tem regra de negócio, SQL, nem branching de domínio
  (R-ADAPTER-001 vale para tools).
- O LLM (parse) permanece encapsulado num step a montante (`ParseInbound`) — fora dos workflows
  (fiel ao Mastra: "LLM dentro de um step").

Migração incremental e behavior-preserving: o `switch` é desmontado por área, um Workflow por vez,
mantendo `RouteResult` idêntico (testes de não-regressão por kind). Ao final, `daily_ledger_agent.go`
fica fino: orquestra registry + guarda + formatação compartilhada.

## DMMF aplicado (onde faz sentido)

- **Smart constructors**: novos `intent.Kind` (constructors que validam invariantes) — já é o padrão.
- **State-as-type / discriminated union**: `RunStatus` (running|succeeded|failed) e `ToolOutcome`
  como tipos fechados (não strings livres), tornando outcome ilegal irrepresentável. `Tool.Execute`
  retorna `ToolResult` com `Outcome` tipado.
- **Workflow pipeline**: Workflow como sequência de steps com contrato in/out explícito; a parte
  pura (construção de intent/command) via smart constructors, a parte de IO isolada nas tools.
- **Tornar estados ilegais irrepresentáveis**: `UpdateCardFields` com ponteiros (nil = não alterar)
  e invariante "≥1 campo a alterar"; `Run` sem `endedAt` enquanto `running`.
- Proibido (governança): Result/Either custom, currying, DSL de pipeline. Erros via `errors.Join`
  + `fmt.Errorf("ctx: %w", err)`.

## Abstrações novas (assinaturas idiomáticas; código real é zero-comentário)

- **Tool** — `internal/agent/application/tools/tool.go`
  ```go
  type ToolOutcome int // routed, clarify, usecaseError, missingResolver, ...
  type Tool interface {
      Name() string
      Descriptor() ToolSpec
      Execute(ctx context.Context, in ToolInput) (ToolResult, error)
  }
  type ToolInput  struct { UserID uuid.UUID; Channel string; Intent intent.Intent }
  type ToolResult struct { Reply string; Outcome ToolOutcome; Kind intent.Kind }
  ```
- **Workflow / Registry** — `internal/agent/application/workflow/{workflow.go,registry.go}`
  ```go
  type Workflow interface {
      ID() string
      Handles(kind intent.Kind) bool
      Execute(ctx context.Context, in ToolInput) (ToolResult, error)
  }
  type Registry interface { Resolve(kind intent.Kind) (Workflow, bool) }
  ```
  Implementações: `transactions_workflow.go`, `budget_workflow.go`, `cards_workflow.go`,
  `conversational_workflow.go`. Step de guarda de escrita: `write_guard.go` (authz+replay+policy+
  audit), reutilizando `decision_audit.go` e `policy_evaluator.go`.
- **Thread / Run** — `internal/agent/domain/entities/{thread.go,run.go}`, smart-constructed:
  `StartRun(...)`, `Finish(outcome, ok, errText) Run`, `DurationMs() int64`, `RunStatus` tipado.
- **AgentRuntime** — `internal/agent/application/services/agent_runtime.go`: Thread/Run lifecycle +
  spans/métricas, envolvendo o entrypoint atual.
- **ContextBuilder** — `internal/agent/application/prompting/context_builder.go`: função **pura**
  `BuildContext(ContextInput) ContextResult` (turn history + working memory + observations).

## Atualização de regras do projeto

- **Novo**: `.claude/rules/agent-workflows-tools.md` — `R-AGENT-WF-001` (severidade hard, escopo
  `internal/agent/`). Conteúdo: (1) roteamento obrigatório `Workflow → Tool → binding → usecase`;
  proibido novo `case` de domínio no switch / lógica de roteamento fora de Workflow; (2) Tool tem
  1 responsabilidade, zero regra/SQL/branching de domínio (herda R-ADAPTER-001); (3) `ToolOutcome`/
  `RunStatus` como tipos fechados (DMMF state-as-type); (4) LLM só no step de parse; (5) Run sempre
  auditável (thread_id, run_id, workflow, tool, status, duração, erro); (6) gate de verificação
  (grep) garantindo que `daily_ledger_agent.go` não cresça em cases de domínio.
- **Editar**: `CLAUDE.md` e `AGENTS.md` — referenciar `R-AGENT-WF-001` e reforçar
  `go-implementation` obrigatória + DMMF para o módulo agent. `.claude/rules/governance.md` —
  registrar a nova regra na seção "Regras de Modulo".

## Mapa intent → workflow → tool → usecase

| # | Funcionalidade | Intent kind | Workflow | Tool | Usecase / binding | Status |
|---|---|---|---|---|---|---|
| 1 | Lançar compra (cartão) | `KindRecordCardPurchase` | transactions | recordCardPurchaseTool | `CardPurchaseLogger` | EXISTE (migrar) |
| 2 | Lançar salário | `KindRecordIncome` | transactions | recordIncomeTool | `ExpenseRecorder` | EXISTE (migrar) |
| 3 | Ver orçamento detalhado | `KindMonthlySummary` | budget | monthlySummaryTool | `MonthlySummaryReader` | EXISTE + descrição |
| 4 | Ver orçamento resumido | `KindHowAmIDoing` | budget | summaryTool | `MonthlySummaryReader` (resumido) | EXISTE (migrar) |
| 5 | Editar orçamento | `KindConfigureBudget` | budget | configureBudgetTool | `BudgetConversation`/`Committer` | EXISTE (migrar) |
| 6 | Editar percentual de categoria | `KindEditCategoryPercentage` (NOVO) | budget | editCategoryPercentTool (NOVO) | **novo usecase budgets** + binding | NOVO |
| 7 | Editar lançamento | `KindEditLastTransaction` | transactions | editLastTxTool | `LastTransactionEditor` | EXISTE (migrar) |
| 8 | Apagar lançamento | `KindDeleteLastTransaction` | transactions | deleteLastTxTool | `LastTransactionDeleter` | EXISTE (migrar) |
| 9 | Cadastrar cartão | `KindCreateCard` | cards | createCardTool | `CardCreator` | EXISTE (migrar) |
| 10 | Listar cartões | `KindListCards` | cards | listCardsTool | `CardLister` | EXISTE (migrar) |
| 11 | Editar cartão | `KindUpdateCard` (NOVO) | cards | updateCardTool (NOVO) | `card.UpdateCardUC` (DTO ponteiros) | wiring NOVO |
| 12 | Apagar cartão | `KindDeleteCard` (NOVO) | cards | deleteCardTool (NOVO) | `card.SoftDeleteCardUC` | wiring NOVO |

Novos kinds em `internal/agent/domain/intent/intent.go` — **append no fim do enum iota** (seguro:
`agent_decisions.intent_kind`/`agent_runs.intent_kind` são TEXT via `String()`). Branches em
`String()`, `ParseKind()`; os 3 entram em `IsWrite()`. Smart constructors:
`NewUpdateCard(UpdateCardFields{CardName; Nickname,Name *string; ClosingDay,DueDay *int})`
(CardName não vazio; ≥1 campo; dias [1..31]), `NewDeleteCard(cardName)`,
`NewEditCategoryPercentage(CategoryName; Percentage[0..100])`. Cartão resolvido por nome→ID via
`CardLister` antes de `UpdateCardUC`/`SoftDeleteCardUC` (exigem ID+UserID); ambiguidade → clarify.

## Persistência nova (golang-migrate, `migrations/`)

Complementar, sem duplicar `agent_decisions` (audit de escrita LLM) nem `agent_sessions` (efêmero).
- `000017_create_agent_threads.{up,down}.sql` — `agent_threads(id, user_id FK users CASCADE,
  channel, created_at, updated_at)`, `UNIQUE(user_id, channel)`, CHECK len(channel) 1..32.
- `000018_create_agent_runs.{up,down}.sql` — `agent_runs(id, thread_id FK CASCADE, user_id,
  channel, message_id, agent_id, workflow, tool_name, intent_kind, outcome, status CHECK
  in('running','succeeded','failed'), error, decision_id FK agent_decisions SET NULL, started_at,
  ended_at, duration_ms)`, índices `(thread_id, started_at DESC)`, `(user_id, started_at DESC)`.
  Cabeçalho com `SET LOCAL lock_timeout/statement_timeout` como nas migrations existentes.

Ports em `application/interfaces/{agent_thread_repository.go,agent_run_repository.go}`; repos
postgres em `infrastructure/repositories/postgres/` (factory por DBTX; leitura por DI / escrita via
uow — padrão de `agent_decision_repository.go`).

## Observabilidade

Spans run→workflow→tool→usecase (padrão `agent.<camada>.<op>`): `agent.runtime.execute` →
`agent.workflow.{transactions,budget,cards,conversational}` → `agent.tool.<tool_name>` → usecases.
Métricas (baixa cardinalidade, sem user_id/category_id; aditivas aos counters atuais):
`agent_runs_total{agent_id,channel,workflow,status}`,
`agent_run_duration_seconds{agent_id,channel,workflow}`,
`agent_tool_invocations_total{tool,outcome}`.

## Plano incremental (cada fase compila e testa isolada; go-implementation Etapas 1–5 por fase)

- **Fase 0 — Regras + descrições**: criar `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001);
  editar `CLAUDE.md`/`AGENTS.md`/`governance.md`. Ajustar descrições detalhado/resumido em
  `tool_catalog.go`/`registry.go`/`prompting/prompts.go`.
- **Fase 1 — Fundação Workflow/Tool (DMMF)**: `application/tools/tool.go` (`Tool`, `ToolOutcome`,
  `ToolInput/Result`); `application/workflow/{workflow.go,registry.go,write_guard.go}`;
  `Registry` ganha `byName`/`Resolve`. Migrar **um** workflow piloto (cards) do switch para o
  registry, preservando `RouteResult` (testes de não-regressão). `daily_ledger_agent.go` passa a
  delegar esse kind ao registry.
- **Fase 2 — Migrar demais áreas**: transactions, budget, conversational como Workflows/Tools;
  esvaziar o switch de cases de domínio; `write_guard` reutilizado por todas as escritas.
- **Fase 3 — Gaps funcionais no novo padrão**: 3 novos kinds + constructors (DMMF) e testes;
  `internal/budgets` ganha **usecase de edição parcial de percentual** (`Decide*` puro + DTO
  `Validate()` + testes) exposto em `module.go`; bindings `CardUpdaterAdapter`/`CardDeleterAdapter`
  /`CategoryPercentageEditor`; tools `updateCardTool`/`deleteCardTool`/`editCategoryPercentTool`;
  formatadores; wiring em `agent/module.go`.
- **Fase 4 — Thread/Run + persistência**: migrations 000017/000018; `domain/entities/{thread,run}.go`
  (+testes); ports + repos postgres (+integration tests).
- **Fase 5 — AgentRuntime (opt-in) + ContextBuilder**: `agent_runtime.go`; gateways em
  `interfaces/`; `prompting/context_builder.go` (puro); `module.go` constrói/expõe runtime;
  consumer WhatsApp chama runtime quando habilitado, com fallback. Não-regressão: `RouteResult`
  idêntico com/sem runtime; Run persistido com status/outcome/duração; escrita referencia
  `decision_id`.

## Estratégia de testes

testify/suite whitebox; `SetupTest` com `fake.NewProvider()`; `dependencies` struct com IIFE por
mock; SUT dentro de `s.Run`. Cobrir: constructors dos novos kinds; `ToolOutcome`/`RunStatus`
(tipos fechados); cada Tool (mapeamento intent→DTO, wrap de erro, outcome); `write_guard`
(authz_denied/policy_blocked/replay reusando `daily_ledger_replay_test.go`,
`intent_router_authz_retry_test.go`); Workflows (Resolve + Execute por kind, paridade com
comportamento anterior); `Thread`/`Run` (invariantes, `Finish`/`DurationMs`); runtime
(não-regressão + Run persistido); integração postgres dos repos thread/run; novo usecase budgets
(`Decide*` sem mock + suite); E2E `internal/agent/e2e/cards_steps_test.go` (editar/apagar cartão).

## Riscos

1. **Refator do switch para registry**: maior superfície de mudança — mitigado por migração por
   área (um workflow por vez), paridade de `RouteResult` validada por testes existentes, e
   `write_guard` extraído sem alterar a lógica de `dispatchWrite`.
2. **Ordem do enum `intent.Kind`**: append-only (persistência TEXT). Não reordenar.
3. **Resolução de cartão por nome** (editar/apagar): ambiguidade → `clarify`.
4. **UpdateCard com ponteiros**: parser emite só campos a alterar; JSON Schema da tool não-required;
   nil = não alterar.
5. **Usecase de percentual em budgets**: re-normalização das allocations; invariantes no `Decide*`;
   cobrir soma > 100%.
6. **AgentRuntime duplo-caminho**: ambos testados até estabilizar; remover legado depois.
7. **Cardinalidade de métricas**: `workflow`/`tool`/`status`/`outcome` sempre de enums fechados.

## Verificação end-to-end

- Por fase: `task build`/`go build ./...`, `task test`, lint; integration tests com testcontainers
  Postgres para repos novos.
- Gates HARD (go-implementation R0–R7 + regras): zero comentários (R-ADAPTER-001.1 e R-AGENT-WF-001),
  sem SQL/regra em tool/adapter, lógica de domínio só em `Decide*`/smart constructors, `Validate()`
  em todo input DTO novo, grep do gate de R-AGENT-WF-001 (switch sem cases de domínio).
- `task migrate` aplica 000017/000018; conferir schema e rollback (`down`).
- Smoke conversacional (fixtures e2e / WhatsApp sandbox): editar/apagar cartão, editar percentual,
  detalhado vs resumido; conferir Run em `agent_runs` (duração/outcome) e `decision_id` em escrita.
- Observabilidade: spans `agent.runtime.execute → agent.workflow.* → agent.tool.*` no otel-lgtm;
  métricas `agent_runs_total`/`agent_tool_invocations_total` sem labels de alta cardinalidade.

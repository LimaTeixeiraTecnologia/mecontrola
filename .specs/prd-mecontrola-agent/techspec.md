<!-- spec-hash-prd: c8ae9e47917a5181897f19e006928ce22f79a94ca74d69fecfc70dd2b4063acd -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — `MeControlaAgent`

PRD consumido: `.specs/prd-mecontrola-agent/prd.md` (spec-version 4, RF-01..RF-39, decisões D-01..D-24). Decisões de arquitetura adicionais D-25..D-28 (ADR-007/008).

## Skills e Referências Obrigatórias (inegociável)

A implementação DEVE, sem exceção, ser conduzida sob:

- **`.claude/skills/go-implementation/`** — Etapas 1–5 do SKILL.md, Regras Estritas R0–R7 `[HARD]`, e o Checklist de Validação de `references/build.md`. Carregar `references/architecture.md` antes de qualquer edição; selecionar no máximo 4 referências por escopo.
- **`.agents/skills/mastra/`** — base canônica para construir o agente sobre `internal/platform/{agent,llm,memory,workflow,tool,scorer}`, usando `internal/agents` (port weather) como molde estrutural. Todo Agent/Tool/Workflow/Step/Thread/Run/WorkingMemory/Scorer segue o mapeamento desta skill.
- **Domain Modeling Made Functional (DMMF — Scott Wlaschin)** — modelagem como tipos: smart constructors, discriminated unions, **state-as-type** (`OnboardingPhase`, `AwaitingKind`, `OperationKind` fechados), funções `Decide*` puras e determinísticas, "make illegal states unrepresentable". **Anti-padrões proibidos `[HARD]`**: `Result[T,E]`/`Either` customizado, currying, DSL de pipeline, monades. DMMF prevalece sobre estilo idiomático genérico para regras de tipo e estado (`.claude/rules/governance.md`).

Essas três fontes são autoritativas e equivalem às regras `[HARD]` do repositório; nenhum desvio é permitido por conveniência.

## Resumo Executivo

O `MeControlaAgent` substitui integralmente o consumidor weather em `internal/agents`, reusando seu **molde estrutural** (layout `application/{agents,tools,workflows,scorers}` + `module.go` + `infrastructure/messaging/.../consumers`) e o substrato `internal/platform/{agent,llm,memory,workflow,tool,scorer}` **sem reimplementar mecanismo**. O agente conversa em português no WhatsApp como parceiro financeiro: conduz um **onboarding obrigatório de 8 etapas** modelado como **workflow durável** (`workflow.Engine[S]`, suspend/resume por merge-patch) e atende a **operação diária** por linguagem natural via **tools finas** (`tool.NewTool[I,O]`) que delegam aos use cases reais de `internal/{categories,card,budgets,transactions}`.

Duas invariantes de correção guiam o design: (1) **fronteira de ferramentas** — todo lançamento é escrito **somente** via `internal/transactions`, e o orçamento (resumo/alertas) é atualizado automaticamente pelos consumers de outbox já existentes em `internal/budgets` (D-13); (2) **escrita exatamente-uma-vez por intenção** — como o middleware de idempotência atual só persiste respostas 4xx (`internal/platform/idempotency/middleware.go:136-154`), o agente mantém um **ledger de idempotência próprio** (agent-owned) chaveado por `(wamid, item_seq, operation)` que torna retries/loops idempotentes (D-19, ADR-004). Operações destrutivas exigem **confirmação humana** com aviso de impacto, reemitindo o contrato HITL de `R-AGENT-WF-001.7-A` para este consumidor (D-21, ADR-005).

A entrega inclui o **cutover**: remoção total do weather-agent (`agent.go`, tools/workflows/scorers weather, `interfaces.WeatherClient`, `infrastructure/weather`) sem resíduo, preservando intacto o `internal/onboarding` (ativação de conta por magic token, separada do agente — D-01).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Layout-alvo em `internal/agents` (reusa o molde, troca o conteúdo de domínio):

- **`application/agents/mecontrola_agent.go`** (novo; substitui `agent.go`) — `BuildMeControlaAgent(provider, tools, hooks, o11y)`; system prompt com identidade/tom/emojis/regras de comunicação (RF-06..RF-09); registra as tools de operação diária. Único agente no `AgentRegistry`.
- **`application/tools/`** (novo) — tools finas de operação diária, uma responsabilidade cada (ADR-003): `register_expense`, `register_income`, `register_card_purchase`, `query_month`, `query_plan`, `edit_entry`, `delete_entry`, `adjust_allocation`, `classify_category`. Cada `tool.NewTool[I,O]` delega a um binding consumer-side; zero regra/SQL/branching (R-AGENT-WF-001.2).
- **`application/workflows/onboarding_workflow.go`** (novo; substitui `workflow.go` weather) — `Definition[OnboardingState]` com `Sequence` de 8 steps; `Durable:true`, suspend após cada pergunta, resume por merge-patch (ADR-002).
- **`application/scorers/`** (novo) — `BuildMeControlaScorers(provider)`: tool-call-accuracy, completeness e categorization (LLM-judged) (RF-39, ADR-006).
- **`application/interfaces/`** (novo) — interfaces consumer-side: `CategoriesReader`, `CardManager`, `BudgetPlanner`, `TransactionsLedger` (R6: interface no consumidor).
- **`application/usecases/`** — mantém `HandleInbound` (inalterado na assinatura: `runtime.Execute`); adiciona `ResolveOnboardingOrAgent` (decide entre rodar onboarding ou operação diária por estado derivado — RF-30.1) e `IdempotentWrite` helper (ADR-004).
- **`infrastructure/binding/`** (novo) — adapters que implementam as interfaces consumer-side chamando os use cases reais dos módulos (espelha `budgets/infrastructure/repositories/postgres/categories_reader_adapter.go`).
- **`infrastructure/persistence/`** (novo) — repositório do **ledger de idempotência** agent-owned (tabela `agents_write_ledger`) e do **estado de confirmação destrutiva** (reaproveita o `Snapshot` do kernel; sem side-store de domínio).
- **`module.go`** (reescrito) — `Deps` passa a receber os módulos de domínio já construídos; monta agente + tools + workflow + scorers + bindings + runtime; mantém o registro do `EmbeddingIndexHandler` e do `WhatsAppInboundConsumer`.

Inalterados (consumidos, não reimplementados): `internal/platform/*`, `internal/{categories,card,budgets,transactions}`, `internal/identity`, `internal/onboarding`, `internal/platform/whatsapp/*` (dispatcher, dedup, ratelimit, gateway).

### Fluxo de Dados

Inbound (idêntico ao atual até o consumer): `WhatsApp → handler HTTP → assinatura → dispatcher (dedup wamid, principal via identity, rate limit) → agentRoute publica outbox agents.whatsapp.inbound.v1 → WhatsAppInboundConsumer → HandleInbound.Execute → AgentRuntime.Execute`.

Dentro do runtime (substrato, inalterado): resolve `Thread(resourceID=UserID, threadID=Peer)` → abre `Run` auditável → injeta working memory + últimas 20 mensagens no system prompt (`runtime.go:165-185`) → `Agent.Execute` (loop tool-calling) → `MessageStore.Append` → fecha `Run`.

Decisão onboarding × operação (novo, dentro de `HandleInbound`/agente): antes do loop principal, `ResolveOnboardingOrAgent` verifica (a) estado de confirmação destrutiva pendente no `Snapshot` (resume antes de qualquer parse — ADR-005); (b) onboarding em andamento/necessário (Snapshot do `OnboardingWorkflow` suspenso, ou estado derivado "não onboardado" = sem objetivo na working memory e sem orçamento ativo — RF-30.1) → `Engine.Resume/Start`; senão (c) operação diária via loop tool-calling do agente.

Escrita de lançamento (operação diária): tool `register_*` → `IdempotentWrite` (consulta `agents_write_ledger` por `(wamid,item_seq,op)`) → se ausente, chama o use case de `internal/transactions` → grava resource_id no ledger → retorna. O domínio publica outbox `transactions.transaction.created.v1` / `card_purchase.created.v1`, consumido por `internal/budgets` que popula `budgets_expenses` (resumo/alertas) — **sem ação do agente** (D-13).

## Contratos de Comunicação entre Módulos

A comunicação ponta a ponta tem **5 saltos**, cada um com contrato explícito e estilo (síncrono vs assíncrono) escolhido por propósito. Nenhum ciclo entre módulos: o domínio não conhece o agente; o agente só escreve em `transactions` e o orçamento reage por evento.

### Diagrama de sequência (inbound → resposta)

```
WhatsApp(Meta)   Server(HTTP)         Outbox        Worker/Agente        Domínio          Budgets
     │  POST+HMAC     │                   │               │                 │                 │
     │───────────────▶│ [1] assinatura    │               │                 │                 │
     │                │ dispatcher:       │               │                 │                 │
     │                │  dedup(wamid)     │               │                 │                 │
     │                │  principal(identity)              │                 │                 │
     │                │  rateLimit        │               │                 │                 │
     │                │ [2] publish ──────▶ inbound.v1    │                 │                 │
     │◀───200─────────│  (RouteOutcome)   │               │                 │                 │
     │                │                   │──── consume ──▶ [3] HandleInbound│                 │
     │                │                   │               │  AgentRuntime:  │                 │
     │                │                   │               │  Thread→Run     │                 │
     │                │                   │               │  WM+histórico   │                 │
     │                │                   │               │ [4] tool→binding▶ usecase (sync)  │
     │                │                   │               │                 │ create_tx       │
     │                │                   │               │                 │─[4b] tx.created─▶ consumer
     │                │                   │               │                 │                 │ upsert_expense
     │                │                   │               │                 │                 │ (budgets_expenses)
     │◀── [5] SendTextMessage ────────────┼───────────────┤ outcome.Content │                 │
     │  (Meta API, sync)                  │               │                 │                 │
```

### Tabela de contratos

| # | De → Para | Estilo | Contrato (tipo fechado / envelope / interface) | Âncora |
|---|-----------|--------|-----------------------------------------------|--------|
| 1 | Canal → dispatcher | In-process | HMAC SHA-256 + `RouteOutcome` (enum) | `whatsapp/dispatcher/dispatcher.go:104` (`Route`), dedup `:129` |
| 2 | dispatcher → agente | **Assíncrono (outbox)** | `agents.whatsapp.inbound.v1` `{user_id,peer,text,message_id}`, idempotente por `event_id`; `OutcomeAgent` | `agents/module.go:37,186-230` (publish `:222`) |
| 3 | consumer → runtime | In-process | `InboundRequest → Outcome{RunID,Content,Status:RunStatus}` | `agents/.../handle_inbound.go:30`; runtime `runtime.go:165-185` |
| 4 | agente → domínio | **Síncrono** | Interfaces consumer-side (`TransactionsLedger`/`BudgetPlanner`/`CardManager`/`CategoriesReader`) → usecases | techspec ADR-003; padrão `categories_reader_adapter.go` |
| 4b | transactions → budgets | **Assíncrono (outbox)** | `transactions.transaction.created.v1` / `transactions.card_purchase.created.v1`, idempotente por `event_id` | `budgets/module.go:134-149` |
| 5 | agente → usuário | **Síncrono** | `whatsAppGateway.SendTextMessage(toE164,text)` (texto-only) | `onboarding/.../whatsapp_gateway.go:39` |

### Invariantes de comunicação

- **Identidade resolvida uma vez**: o `principal` (user_id) é resolvido no dispatcher (`identity`) e propagado via `auth.WithPrincipal`; nenhum módulo a jusante re-resolve telefone→usuário. O agente usa `resourceID=user_id`, `threadID=peer`.
- **Fronteiras tipadas (state-as-type, DMMF)**: `RouteOutcome`, `RunStatus`, `ToolOutcome` são enums fechados — nunca string solta entre camadas.
- **Mão única, sem ciclo**: o agente escreve **apenas** em `transactions`; o orçamento (resumo/alertas) reage por evento de outbox (D-13). Módulos de domínio não importam `internal/agents`.
- **Idempotência em todo ponto de entrada**: dedup por `wamid` (dispatcher) + ledger de escrita `(wamid,item_seq,operation)` (ADR-004) + consumers idempotentes por `event_id`.

### Consistência eventual e falha parcial (notas operacionais)

- **Resumo do orçamento é eventualmente consistente**: como o salto 4b é assíncrono, logo após "✅ registrei" o `budgets_expenses` pode não estar populado. O agente DEVE responder consultas de orçamento a partir do estado consolidado e, quando o usuário registra e pergunta no mesmo turno, pode compor a resposta considerando o lançamento recém-confirmado (sem reler como fonte de verdade). Documentar no runbook; observável via lag do consumer.
- **Múltiplos lançamentos não são atômicos**: itens de uma mesma mensagem são escritas síncronas separadas (sem batch). Falha do n-ésimo não desfaz os anteriores; cada item é idempotente e o resumo final reporta sucessos/falhas individualmente (RF-21.2). Não há transação transversal entre módulos (por design: isolamento; o ledger é agent-owned).
- **Latência percebida**: depende do worker consumir o evento de inbound (salto 2→3). Observável via `agents_inbound_total{outcome}` + métricas de outbox.

## Design de Implementação

### Interfaces Chave

Bindings consumer-side (definidos em `internal/agents/application/interfaces`, implementados em `infrastructure/binding`):

```go
type TransactionsLedger interface {
	CreateTransaction(ctx context.Context, in RawTransaction) (EntryRef, error)
	CreateCardPurchase(ctx context.Context, in RawCardPurchase) (EntryRef, error)
	UpdateTransaction(ctx context.Context, in RawUpdateTransaction) (EntryRef, error)
	DeleteTransaction(ctx context.Context, ref EntryRef, version int64) error
	UpdateCardPurchase(ctx context.Context, in RawUpdateCardPurchase) (EntryRef, error)
	DeleteCardPurchase(ctx context.Context, ref EntryRef, version int64) error
	ListMonthlyEntries(ctx context.Context, userID uuid.UUID, refMonth string, cursor string, limit int) ([]MonthlyEntry, error)
	GetMonthlySummary(ctx context.Context, userID uuid.UUID, refMonth string) (MonthlySummary, error)
}

type BudgetPlanner interface {
	CreateBudget(ctx context.Context, in DraftBudget) (BudgetRef, error)
	ActivateBudget(ctx context.Context, userID uuid.UUID, competence string) error
	CreateRecurrence(ctx context.Context, userID uuid.UUID, competence string, months int) error
	EditCategoryPercentage(ctx context.Context, userID uuid.UUID, competence, rootSlug string, percentage int) error
	GetMonthlySummary(ctx context.Context, userID uuid.UUID, competence string) (BudgetSummary, error)
	ListAlerts(ctx context.Context, userID uuid.UUID) ([]Alert, error)
}

type CardManager interface {
	CreateCard(ctx context.Context, in NewCard) (CardRef, error)
	ListCards(ctx context.Context, userID uuid.UUID) ([]Card, error)
	SoftDeleteCard(ctx context.Context, cardID, userID uuid.UUID) error
	HasOpenInstallments(ctx context.Context, cardID, userID uuid.UUID) (bool, error)
}
// NewCard no onboarding coleta apenas Nickname + DueDay (RF-15). O adapter preenche os
// obrigatórios do domínio (create_card.go:18-35): Name=Nickname, LimitCents=0, ClosingDay=DueDay
// (default determinístico; impacta competência de parcelas via BillingCycleResolver — RF-15.2).

type CategoriesReader interface {
	SearchDictionary(ctx context.Context, term, kind string) ([]CategoryCandidate, error)
	ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error)
}
```

Estado fechado do onboarding (DMMF state-as-type; `internal/agents/application/workflows`):

```go
type OnboardingPhase int

const (
	PhaseWelcome OnboardingPhase = iota + 1
	PhaseGoal
	PhaseMonthlyIncome
	PhaseCards
	PhaseMethodology
	PhaseDistribution
	PhaseSummary
	PhaseConclusion
)

type OnboardingState struct {
	Phase        OnboardingPhase   `json:"phase"`
	Goal         string            `json:"goal"`
	IncomeCents  int64             `json:"incomeCents"`
	CardsDone    bool              `json:"cardsDone"`
	Allocations  map[string]int    `json:"allocations"`
	CardNickname string            `json:"cardNickname"`
	CardDueDay   int               `json:"cardDueDay"`
	Recurrence   bool              `json:"recurrence"`
	ResumeText   string            `json:"resumeText"`
}
```

Estado fechado de confirmação destrutiva (reemissão do contrato `R-AGENT-WF-001.7-A` — ADR-005):

```go
type AwaitingKind int

const (
	AwaitingNone AwaitingKind = iota + 1
	AwaitingConfirm
)

type OperationKind int

const (
	OpDeleteEntry OperationKind = iota + 1
	OpEditEntry
	OpDeleteCard
)

type ConfirmState struct {
	Awaiting     AwaitingKind  `json:"awaiting"`
	Operation    OperationKind `json:"operation"`
	TargetRef    string        `json:"targetRef"`
	ImpactNote   string        `json:"impactNote"`
	RepromptDone bool          `json:"repromptDone"`
	MessageID    string        `json:"messageId"`
}
```

### Modelos de Dados

- **`agents_write_ledger`** (nova migration; agent-owned, ADR-004): `(id uuid pk, user_id uuid, wamid text, item_seq int, operation text, resource_id uuid, resource_kind text, created_at timestamptz)`, com **unique `(wamid, item_seq, operation)`**. Garante exatamente-uma-vez por intenção; replay retorna o `resource_id` existente (`ToolOutcomeReplay`).
- **Estado de onboarding e de confirmação**: persistidos no `Snapshot` do kernel (`workflow_runs`/`workflow_steps`), **fonte única de verdade**; sem tabela de domínio adicional (R-WF-KERNEL-001 / R-AGENT-WF-001.7).
- **Objetivo financeiro**: persistido na working memory (`platform_resources`, escopo `resourceID=UserID`) como markdown estruturado, injetado no system prompt (RF-28). Um objetivo principal por usuário (D-04).
- Schemas de `transactions`, `budgets`, `card`, `categories`, `platform_*`: **inalterados**.

### Endpoints de API

Nenhum endpoint HTTP novo de produto. O agente opera via **canal WhatsApp existente** (texto-only); a resposta volta por `whatsAppGateway.SendTextMessage`. Os use cases de domínio já expõem suas rotas REST atuais (não tocadas).

Ajuste de exposição (não é endpoint, é binding): o `TransactionsModule` constrói mas **não expõe** `UpdateCardPurchase`, `DeleteCardPurchase`, `GetMonthlySummary`, `ListMonthlyEntries` (`internal/transactions/module.go:165-248`). A techspec **adiciona esses campos públicos ao `TransactionsModule`** para permitir o binding (ADR-003). `CardModule` e `BudgetsModule` já expõem o necessário.

## Pontos de Integração

- **OpenRouter** (LLM único, `llm.NewOpenRouterProvider`): loop tool-calling do agente, step `Stream` de workflow para mensagens de onboarding e texto livre, e scorer LLM-judged. Sem fallback chain/circuit breaker (R-AGENT-WF-001.4). **Modelo: `openai/gpt-4o-mini`** (default vigente), com **gate `RUN_REAL_LLM`** validando tool-calling + structured output `Strict:true` no chain real antes de produção (D-28, ADR-007). Memória de risco: `Strict` quebra em haiku/gpt-5-nano; flash-lite é flaky em tool-calling — não usar.
- **Teto de tool rounds configurável**: o cap atual é fixo (`maxToolRounds = 5`, `internal/platform/agent/agent.go:15`). Estender o primitivo com `WithMaxToolRounds(n)` (AgentOption) e usar **default 12** no `MeControlaAgent` para acomodar múltiplos lançamentos por mensagem (D-22) sem estourar `ErrMaxToolRounds`; o ledger de idempotência (ADR-004) impede duplicatas mesmo com mais rounds (D-25, ADR-007).
- **Onboarding I/O via LLM sancionado**: mensagens de cada etapa geradas por step que chama `agent.Stream` (call-site sancionada); respostas do usuário (renda, objetivo, alocações) extraídas por **structured output** (`llm.StructuredContract[T]`, `Strict:true`) — determinismo garantido pelo schema, LLM apenas nas call-sites sancionadas (D-27, ADR-002/ADR-007).
- **Timezone de negócio America/Sao_Paulo**: "hoje" (data-default, D-20) e a competência (`ref_month`) derivam de `time.Now().UTC()` convertido para `America/Sao_Paulo` (intenção declarada do domínio em `ref_month.go:10`), inline no ponto de uso (sem abstrair tempo). Risco sinalizado: `card_billing_snapshot.go:40` usa `time.UTC` — inconsistência pré-existente do domínio, fora do escopo de correção (D-26, ADR-008).
- **Módulos de domínio** (`categories`, `card`, `budgets`, `transactions`): via bindings consumer-side + adapters, construídos no wiring `cmd/server` na ordem `categories → card → budgets → transactions → agents` (já vigente; agents passa a receber os 4 módulos).
- **Outbox/consumers existentes**: a propagação lançamento→orçamento é responsabilidade dos consumers de `internal/budgets` (`budgets/module.go:134-149`); o agente não publica nem consome esses eventos.
- **WhatsApp**: dispatcher (dedup, principal, rate limit) e gateway inalterados; mídia continua descartada (texto-only, D-09).

## Abordagem de Testes

### Testes Unitários

- **Tools** (`application/tools/*_test.go`): testify/suite whitebox, `fake.NewProvider()`, IIFE por mock dos bindings (R-TESTING-001). Cenários: sucesso, erro de binding, validação de input, replay idempotente (ledger hit), data ausente → hoje (D-20).
- **Onboarding workflow** (`application/workflows/onboarding_workflow_test.go`): testes de cada step puro e da máquina de fases; transições, suspend/resume por merge-patch (`{"ResumeText":"sim"}`), distribuição que não fecha 100% → re-pergunta (RF-14), reuso de estado pré-existente (D-23), recorrência 12m (D-15). Steps de decisão devem ser puros/determinísticos (DMMF `Decide*`).
- **ConfirmState/HITL** (ADR-005): confirmação explícita executa; ambíguo re-pergunta uma vez e cancela; resume antes do parse; limpeza determinística (sem run órfão).
- **IdempotentWrite/ledger**: dupla chamada com mesmo `(wamid,item_seq,op)` cria um único recurso; segunda retorna replay.
- **Binding adapters**: mapeamento DTO↔use case; wrapping de erro `%w`.

### Testes de Integração

> Critérios: (a) fronteiras de IO críticas (Postgres: ledger, snapshot, working memory) onde mocks não garantem correção — **sim**; (b) risco de unit passar e integração falhar (propagação lançamento→orçamento via outbox) — **sim**. ⇒ integration tests recomendados.

`//go:build integration` com testcontainers Postgres: (1) escrita idempotente real (unique constraint do ledger sob concorrência); (2) onboarding end-to-end persistindo/retomando Snapshot; (3) lançamento via tool → consumer de budgets popula `budgets_expenses` → `GetMonthlySummary` reflete o gasto (prova de D-13, sem dupla contagem).

### Testes E2E

Variante real atrás de `RUN_REAL_LLM` (espelha o weather): mensagem WhatsApp simulada percorrendo dispatcher→consumer→runtime→tool→gateway, validando jornada de onboarding e um registro de despesa com resumo. Determinístico no CI sem a flag (provider fake).

## Sequenciamento de Desenvolvimento

### Ordem de Build

0. **Extensão do substrato**: `WithMaxToolRounds(n)` AgentOption em `internal/platform/agent` (default preservado em 5; `MeControlaAgent` usa 12). Mudança mínima e genérica no primitivo (ADR-007).
1. **Expor use cases faltantes no `TransactionsModule`** (`UpdateCardPurchase`, `DeleteCardPurchase`, `GetMonthlySummary`, `ListMonthlyEntries`) — desbloqueia bindings (ADR-003).
2. **Interfaces consumer-side + adapters de binding** (`categories/card/budgets/transactions`) com testes unitários.
3. **Ledger de idempotência** (migration + repositório + `IdempotentWrite`) (ADR-004).
4. **Tools de operação diária** (register/query/edit/delete/adjust/classify), finas, com idempotência e data-default; testes.
5. **HITL de operações destrutivas** (`ConfirmState`, resume-antes-do-parse, re-prompt único) (ADR-005).
6. **Onboarding workflow durável** (8 fases, suspend/resume, distribuição 100%, recorrência 12m, reuso de estado) (ADR-002).
7. **Agente + system prompt + scorers** (`BuildMeControlaAgent`, `BuildMeControlaScorers`) (ADR-006).
8. **`module.go` + wiring `cmd/server`/`cmd/worker`** (Deps com os 4 módulos; registrar consumer e embedding handler).
9. **Cutover**: remover weather (`agent.go`, tools/workflows/scorers weather, `WeatherClient`, `infrastructure/weather`, config `WeatherClient` em Deps) sem resíduo; ajustar e2e (ADR-001). Gates de governança verdes.

### Dependências Técnicas

- Migration `agents_write_ledger` aplicada antes do passo 4.
- `internal/onboarding` (ativação) preservado e intacto (D-01).
- Schema `platform_*` e `workflow_runs/steps` já existentes (sem nova migration de kernel).

## Monitoramento e Observabilidade

Reaproveita a stack otel-lgtm e as métricas do substrato (`agent_runs_total`, `agent_run_duration_seconds`, `agent_tool_invocations_total`, `workflow_*`, `scorer_*`, `agent_llm_*`). Métricas novas com **cardinalidade controlada** (R-AGENT-WF-001.5 / R-TXN-004): `agents_inbound_total{outcome}`, `agents_onboarding_phase_total{phase}`, `agents_write_total{operation,outcome}` (outcome ∈ created|replay|usecase_error|clarify), `agents_destructive_confirm_total{operation,result}`. **Proibido** `user_id`/`category_id`/`correlation_key` como label. Run auditável por interação (RF-37) com `thread_id`/`run_id`/`status`/`duration_ms`. Logs em pt-br nos pontos de erro de binding/tool.

## Considerações Técnicas

### Decisões Chave (ADRs)

- **ADR-001** — Cutover: substituir o weather por `MeControlaAgent` reusando o molde estrutural; remoção total sem resíduo. `adr-001-cutover-substituicao-weather.md`.
- **ADR-002** — Onboarding de 8 etapas como workflow durável com `OnboardingPhase` fechado e reuso de estado. `adr-002-onboarding-workflow-duravel.md`.
- **ADR-003** — Fronteira de ferramentas + binding consumer-side + exposição de use cases faltantes. `adr-003-fronteira-tools-binding.md`.
- **ADR-004** — Idempotência exatamente-uma-vez via ledger agent-owned. `adr-004-idempotencia-escrita-agent-owned.md`.
- **ADR-005** — HITL de operações destrutivas (reemissão de `R-AGENT-WF-001.7-A`). `adr-005-hitl-operacoes-destrutivas.md`.
- **ADR-006** — Scorers mínimos do MVP. `adr-006-scorers-mvp.md`.
- **ADR-007** — Contrato de execução LLM: `maxToolRounds` configurável (default 12), modelo `openai/gpt-4o-mini` + gate `RUN_REAL_LLM`, structured I/O do onboarding. `adr-007-contrato-execucao-llm.md`.
- **ADR-008** — Timezone de negócio `America/Sao_Paulo` para data-default e competência. `adr-008-timezone-competencia.md`.

### Riscos Conhecidos

- **Idempotência do middleware (4xx-only)**: não corrigimos o middleware (raio de impacto amplo); o ledger agent-owned cobre o caso do agente (ADR-004). Risco residual: outros chamadores REST continuam sem idempotência de sucesso — fora do escopo deste PRD, registrado.
- **Delete de cartão com parcelas em aberto deixa órfãos no domínio**: mitigado no agente por confirmação + aviso (`HasOpenInstallments`), não por proteção de domínio (ADR-005). A integridade definitiva exigiria mudança em `internal/card` — fora de escopo, sinalizado.
- **Múltiplos lançamentos sequenciais sem batch**: latência/atomicidade parcial (um item pode falhar). Mitigação: cada item é idempotente e o resumo reporta sucessos/falhas individualmente (RF-21.2).
- **Working memory como string única**: objetivo concorrente sobrescreveria; mitigado por D-04 (um objetivo principal) e atualização estruturada.
- **Distribuição/Decisão dentro de step**: manter `Decide*` puro; risco de vazar IO no step — coberto por revisão R-WF-KERNEL/R-AGENT-WF e testes determinísticos.
- **Cap de tool rounds vs custo**: default 12 aumenta custo/latência em mensagens grandes; mitigado por idempotência (sem dano em retry) e por amostragem de scorers. `ErrMaxToolRounds` ainda protege contra loop infinito.
- **Inconsistência de timezone no domínio** (`ref_month.go` America/Sao_Paulo vs `card_billing_snapshot.go` UTC): o agente adota America/Sao_Paulo; a divergência do `card` permanece e é sinalizada para correção futura fora deste escopo (ADR-008).
- **Modelo e `Strict:true`**: gpt-4o-mini precisa de validação real (gate `RUN_REAL_LLM`) para tool-calling + json_schema strict; fallback de modelo não existe (provider único) — se inadequado, trocar o modelo via config é a mitigação (ADR-007).
- **`ClosingDay` derivado do vencimento (RF-15.2)**: coletar só apelido+vencimento força `ClosingDay := DueDay` como default; isso desloca a competência das parcelas no `BillingCycleResolver` em relação a um cartão com fechamento distinto do vencimento. Mitigação: documentar a simplificação; permitir o usuário ajustar o cartão depois (via `internal/card`); refinar a regra de derivação se necessário. Não há perda de dado — apenas aproximação do ciclo.

### Conformidade com Padrões

`.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001: registry, tool fina, estados fechados, LLM nas call-sites sancionadas, Thread-first, pending step, HITL .7-A reemitido), `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001: kernel genérico, merge-patch no resume), `.claude/rules/go-adapters.md` (R-ADAPTER-001: zero comentários, adapters finos, sem SQL direto fora de postgres), `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001: `Validate()` após span), `.claude/rules/go-testing.md` (R-TESTING-001: testify/suite whitebox, IIFE, `fake.NewProvider`), `.claude/rules/governance.md` (precedência DMMF state-as-type), go-implementation R0–R7 (sem `init`/`panic`, `context` nas fronteiras, `errors.Join`/`%w`, goroutines canceláveis, sem abstrair tempo, sem `var _ Iface=(*T)(nil)`, `defer func(){_=rows.Close()}()`).

### Arquivos Relevantes e Dependentes

- Substrato (consumir): `internal/platform/agent/{ports.go,runtime.go,agent.go,registry.go,types.go}`, `internal/platform/tool/tool.go`, `internal/platform/workflow/{engine.go,step.go,store.go,codec.go,combinators.go}`, `internal/platform/scorer/*`, `internal/platform/memory/{ports.go,types.go}`, `internal/platform/llm/*`.
- Molde a substituir/remover: `internal/agents/application/{agents/agent.go,tools/tool.go,workflows/workflow.go,scorers/scorers.go}`, `internal/agents/application/interfaces` (WeatherClient), `internal/agents/infrastructure/weather`, `internal/agents/domain` (weather).
- Domínio a consumir: `internal/categories/module.go`, `internal/card/module.go`, `internal/budgets/module.go` (+ `module.go:134-149` consumers), `internal/transactions/module.go` (expor UCs faltantes).
- Wiring: `cmd/server/server.go` (ordem de módulos, `agents.Deps`), `cmd/worker/worker.go`, `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`.
- Migrations: nova `agents_write_ledger`.

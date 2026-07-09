<!-- spec-hash-prd: ed471323c1cc317f89481eb38b494ea82bc10065436a33006d5cc9f19db1f9b8 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Orçamento Retroativo Conversacional e Mês por Extenso

Skills mandatórias aplicadas: `go-implementation`, `mastra`, `domain-modeling-production`, `design-patterns-mandatory`.
Regras vinculantes: R-AGENT-WF-001, R-WF-KERNEL-001, R-ADAPTER-001, R-DTO-VALIDATE-001, R-TESTING-001, R-TXN-004 (cardinalidade), governança DMMF state-as-type.
Módulo Go raiz: `github.com/LimaTeixeiraTecnologia/mecontrola`.

## Resumo Executivo

A fatia entrega quatro capacidades coesas sobre o consumidor `internal/agents` e o domínio `internal/budgets`, sem tocar o kernel genérico `internal/platform/workflow`:

1. **Criação conversacional de orçamento com HITL durável** — nova Tool fina `create_budget` que **inicia** um workflow durável `budget-creation` (espelhando `BuildOnboardingWorkflow`/`BuildDestructiveConfirmWorkflow`), com estado de espera fechado `BudgetAwaitingSlot`, coleta multi-turno (total → distribuição → confirmação), persistência de snapshot antes de cada pergunta, resume por merge-patch antes do parse, e persistência via `planner.CreateBudget` + `planner.ActivateBudget` (portas já existentes, hoje só consumidas pelo onboarding). **A coleta de total e distribuição espelha exatamente os steps do onboarding** (`BuildIncomeStep` linha 600, `BuildMethodologyStep` linha 684): cada step suspende com um prompt e, no resume, invoca `a.Execute(ctx, Request{Schema: strict})` para extrair o valor estruturado, seguido de um `Decide*` puro que valida e faz reprompt — reusando `DecideIncomeCents`, `_allocationInputSystemPrompt`, `allocationInputSchema`, `DecideAllocationsBP`, `ParseAllocationInputKind` e o default `_defaultDistributionBP` (40/10/10/10/30, oferecido para aceitar ou customizar). Um `BudgetCreationContinuer` é inserido na cadeia `try*` do `WhatsAppInboundConsumer`, **antes** do agente/ParseInbound.
2. **Resolução determinística de mês** — tipo fechado `MonthReference` (união discriminada) resolvido por função pura `DecideCompetence(ref, now)` em `internal/budgets`, consumido pelas tools de competência (`query_month`, `query_plan`, `create_budget`, retrospectiva). Substitui a delegação ao LLM (bug "mês passado" → "setembro de 2023") e a "REGRA ABSOLUTA DE DATA" que hoje rejeita mês relativo.
3. **Mês por extenso** — função pura `FormatCompetencePtBR` em `internal/budgets/domain/valueobjects`, aplicada às saídas do agente; armazenamento ISO `YYYY-MM` inalterado.
4. **Retrospectiva planejado vs realizado** — composição por instrução das tools de leitura existentes (`query_plan` já retorna planejado+realizado+% por raiz; `query_month` cobre realizado sem orçamento); nenhuma tool nova, nenhuma nova fonte de verdade.

O acerto é medido por gate real-LLM (`RUN_REAL_LLM=1`) estatístico ≥ 0.90 sobre cenários-chave, mais a eliminação do fallback genérico neste caminho.

Decisões materiais em ADRs: [ADR-001](adr-001-workflow-budget-creation-hitl.md), [ADR-002](adr-002-resolucao-deterministica-mes-monthreference.md), [ADR-003](adr-003-formatacao-mes-por-extenso.md), [ADR-004](adr-004-retrospectiva-composicao-tools-existentes.md), [ADR-005](adr-005-mensagem-especifica-e-observabilidade.md).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Novos:

- `internal/budgets/domain/valueobjects/month_reference.go` — **NOVO**. `MonthReference` (união discriminada fechada) + `DecideCompetence(ref MonthReference, now time.Time) (valueobjects.Competence, ClarifyReason, error)` puro.
- `internal/budgets/domain/valueobjects/competence.go` — **MODIFICADO**. Adiciona `Prev() Competence` (simétrico a `Next()`) e `FormatCompetencePtBR(c Competence) string` (ou método `Humanize()`), ambos puros.
- `internal/agents/application/tools/create_budget.go` — **NOVO**. Tool fina que inicia o workflow `budget-creation`.
- `internal/agents/application/tools/create_budget_input.go` (ou inline) — **NOVO**. DTO com `Validate() error`.
- `internal/agents/application/workflows/budget_creation_state.go` — **NOVO**. `BudgetCreationState` + `BudgetAwaitingSlot` (tipo fechado) + `BudgetCreationStatus`.
- `internal/agents/application/workflows/budget_creation_decisions.go` — **NOVO**. `Decide*` puros de coleta/confirmação (`now time.Time` injetado).
- `internal/agents/application/workflows/budget_creation_workflow.go` — **NOVO**. `BuildBudgetCreationWorkflow(...) workflow.Definition[BudgetCreationState]` + `BuildBudgetCreationReaper(...)`.
- `internal/agents/application/usecases/budget_creation_continuer.go` — **NOVO**. `BudgetCreationContinuer` (Load→Resume se suspenso; senão não-handled).
- `internal/agents/application/workflows/month_reference_mapping.go` (ou em tools) — **NOVO**. Mapeia o payload estruturado do LLM → `MonthReference` do domínio.

Modificados:

- `internal/agents/module.go` — registra `BuildCreateBudgetTool(engine, def)` em `buildFinancialTools()` (24 tools); constrói `BuildBudgetCreationWorkflow(meControlaAgent, planner)` e o `Engine[BudgetCreationState]`; instancia `BudgetCreationContinuer`, seu reaper (`staleAfter` 35min) e o job; injeta o continuer no consumer. **`create_budget` NÃO entra no `WithWriteToolSet`** — é starter de workflow (como `delete_entry`/`update_card`), não write direto; a persistência ocorre no slot de confirmação do workflow.
- `internal/agents/application/agents/mecontrola_agent.go` — instrução: (a) remove/substitui a "REGRA ABSOLUTA DE DATA" que rejeita mês relativo; (b) instrui a emitir `MonthReference` estruturado; (c) instrui a oferecer criação apenas via `create_budget`; (d) mapa de mês por extenso e composição da retrospectiva; (e) mensagem de orçamento não encontrado passa a citar mês por extenso.
- `internal/agents/application/tools/query_month.go` e `query_plan.go` — passam a aceitar `MonthReference` estruturado e resolver via `DecideCompetence(ref, now)` na borda (`now = time.Now().In(loc)`); mantêm fallback para mês corrente quando `ref` ausente.
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` — insere `tryBudgetCreation(...)` na cadeia `try*` antes do agente; mensagem específica de indisponibilidade distinta do `fallbackReply`.

### Fluxo de Dados (criação)

```
WhatsApp inbound → WhatsAppInboundConsumer.handle
  → tryPendingEntry / tryDestructiveConfirm / tryBudgetCreation (NOVO, antes do agente)
     ├─ se há run budget-creation SUSPENSO p/ o resourceId → Continuer.Resume(merge-patch → step)
     └─ senão → não-handled → segue para o agente (ParseInbound / tool-calling)
  → Agente resolve intenção → tool create_budget.exec
     → DecideCompetence(ref, now) → competence YYYY-MM (ou clarify, sem iniciar workflow)
     → engine.Start(budget-creation, key=resourceId, initial=BudgetCreationState{Competence, Awaiting:Total|Distribution})
     → StepFunc suspende pedindo total/distribuição (snapshot persistido ANTES da pergunta)
  → turnos seguintes capturados por tryBudgetCreation (por resourceId) → Resume
     → step extrai via a.Execute(Schema strict) + Decide* puro (valida/reprompt) → próximo slot
  → confirm "sim" (isSim determinístico): planner.CreateBudget(DraftBudget) + planner.ActivateBudget → run succeeded, msg por extenso
```

## Design de Implementação

### Interfaces Chave

Domínio — resolução de mês (pura, sem IO, `now` injetado):

```go
package valueobjects

type MonthRefKind int

const (
	MonthRefCurrent MonthRefKind = iota + 1
	MonthRefPrevious
	MonthRefNext
	MonthRefExplicit
	MonthRefNamedWithoutYear
	MonthRefUnknown
)

type MonthReference struct {
	Kind  MonthRefKind
	Year  int
	Month int
}

type ClarifyReason int

const (
	ClarifyNone ClarifyReason = iota + 1
	ClarifyMissingYear
	ClarifyUnrecognized
)

func DecideCompetence(ref MonthReference, now time.Time) (Competence, ClarifyReason, error)
```

Regra pura: `MonthRefCurrent → CompetenceFromTime(now)`, `MonthRefPrevious → .Prev()`, `MonthRefNext → .Next()`, `MonthRefExplicit → NewCompetence(fmt YYYY-MM)`, `MonthRefNamedWithoutYear → (zero, ClarifyMissingYear, nil)`, `MonthRefUnknown → (zero, ClarifyUnrecognized, nil)`. `now` deve chegar já em `America/Sao_Paulo` (convertido na borda). Sem `time.Now()` interno (proíbe abstração de tempo — `time.Now().In(loc)` fica na tool exec).

Tool `create_budget` (adapter fino, inicia workflow — sem regra/SQL/branching de domínio):

```go
type CreateBudgetToolInput struct {
	MonthRefKind string `json:"monthRefKind"`
	Year         int    `json:"year,omitempty"`
	Month        int    `json:"month,omitempty"`
	TotalCents   int64  `json:"totalCents,omitempty"`
}

func (i *CreateBudgetToolInput) Validate() error // errors.Join, nomeia campos

func BuildCreateBudgetTool(engine workflow.Engine[BudgetCreationState], def workflow.Definition[BudgetCreationState]) tool.ToolHandle
// exec: DecideCompetence(ref, now) → se clarify, retorna pergunta sem iniciar; senão engine.Start(key=resourceID)

func BuildBudgetCreationWorkflow(a agent.Agent, planner interfaces.BudgetPlanner) workflow.Definition[BudgetCreationState]
// a agent.Agent injetado nos steps de coleta (extração estruturada), espelhando os steps do onboarding;
// planner só é chamado no slot de confirmação (CreateBudget + ActivateBudget).
```

Estado de espera fechado do workflow:

```go
type BudgetAwaitingSlot int

const (
	AwaitingBudgetTotal BudgetAwaitingSlot = iota + 1
	AwaitingBudgetDistribution
	AwaitingBudgetConfirm
)

type BudgetCreationStatus int

const (
	BudgetCreationActive BudgetCreationStatus = iota + 1
	BudgetCreationCompleted
	BudgetCreationCancelled
	BudgetCreationExpired
)
// ambos com String()/IsValid()/Parse* (padrão confirm_state.go / pending_entry_state.go)
```

Continuer (espelha `PendingEntryContinuer`/`DestructiveConfirmContinuer`):

```go
type BudgetCreationContinuer struct { /* engine, def, o11y */ }
func (uc *BudgetCreationContinuer) Continue(ctx context.Context, resourceID, text, messageID string) (BudgetContinueResult, error)
```

### Modelos de Dados

`BudgetCreationState` (persistido como `Snapshot.State` JSON pelo kernel; campos alinhados a `ConfirmState`/`PendingEntryState`):

```go
type BudgetCreationState struct {
	Status        BudgetCreationStatus
	Awaiting      BudgetAwaitingSlot
	UserID        uuid.UUID
	Competence    string           // YYYY-MM já resolvido por DecideCompetence antes do Start
	TotalCents    int64
	Allocations   map[string]int   // rootSlug (expense.*) → basisPoints; default 40/10/10/10/30 até o usuário aceitar/customizar
	ResumeText    string
	ResponseText  string
	RepromptCount int
	MessageID     string
	SuspendedAt   time.Time
	Expired       bool
}
```

Constantes de tempo (espelham pending-entry): `budgetCreationTTL = 30 * time.Minute` (avaliado no resume) e reaper `budgetCreationStaleAfter = 35 * time.Minute`. TTL expirado no resume → cancela sem efeito, `handled=false` (texto segue ao ParseInbound).

Persistência de orçamento: **nenhum schema novo**. Reutiliza `mecontrola.budgets` / `budgets_allocations` via `planner.CreateBudget`→`CreateBudget.Execute` (`CreateBudgetInput`) e `planner.ActivateBudget`→`ActivateBudget.Execute`. Constraints intactas: `budgets_user_comp_uk` (unicidade → `ErrBudgetConflict` no create), `budgets_competence_chk` (regex YYYY-MM), `budgets_allocations_basis_points_chk` (0..10000), soma exatamente 10000 no `Budget.Activate`.

Persistência do snapshot: tabela do kernel `internal/platform/workflow/infrastructure/postgres` (`Store.Insert/Load/Save`), inalterada. Workflow novo apenas adiciona um `workflow` string próprio (`"budget-creation"`).

### Contrato das 5 categorias raiz

Distribuição sempre sobre os 5 slugs canônicos fixos (`internal/budgets/domain/valueobjects/root_slug.go`, `CanonicalOrder()`): `expense.custo_fixo`, `expense.conhecimento`, `expense.prazeres`, `expense.metas`, `expense.liberdade_financeira`. O step de distribuição acumula basis points por slug e só transita para confirmação quando a soma = 10000 (RF-04). `CreateBudgetCommand` tolera soma ≤ 10000; a igualdade estrita é imposta em `Budget.Activate`.

## Pontos de Integração

- **OpenRouter (único provider)** — LLM apenas nas call-sites sancionadas. Os steps de coleta do workflow `budget-creation` invocam `a.Execute(ctx, Request{Schema: strict})` para **extrair** total/distribuição estruturados — exatamente o padrão sancionado dos steps do onboarding (`BuildIncomeStep`/`BuildMethodologyStep`), enquadrado em R-AGENT-WF-001.4 #2 (step de workflow que invoca o Agent). São **determinísticos e sem LLM**: o resolvedor de mês (`DecideCompetence`), os validadores `Decide*` (`DecideIncomeCents`/`DecideAllocationsBP`), o gate de confirmação (`isSim`/`isNao`) e o kernel (`internal/platform/workflow`, R-WF-KERNEL-001.5).
- **Portas do módulo budgets** — `BudgetPlanner.CreateBudget`/`ActivateBudget` já existem e são mockadas (`.mockery.yml` linha 137). Nenhuma interface nova de domínio; adapter `budget_planner_adapter.go` já mapeia DTOs.

## Abordagem de Testes

### Testes Unitários

- `DecideCompetence` — tabela pura cobrindo Current/Previous/Next em viradas de ano (dez→jan, jan→dez) em `America/Sao_Paulo`, Explicit válido/ inválido, NamedWithoutYear→ClarifyMissingYear, Unknown→ClarifyUnrecognized. Sem mock; `now` injetado (padrão `DecidePendingResume`).
- `Competence.Prev()` / `FormatCompetencePtBR` — tabela pura (12 meses + anos), incluindo "junho de 2026", "janeiro de 2025".
- Decisões do workflow (`budget_creation_decisions.go`) — tabela pura: total inválido→reprompt; distribuição incompleta (soma≠10000) bloqueia via `DecideAllocationsBP` (reuso do onboarding); soma=10000 transita; confirm sim/não/ambíguo→reprompt-único→cancel; TTL 30min expira; replay de messageID. A extração estruturada (LLM) é mockada nos unit tests via `agent.Agent` fake; a validação pura é testada sem mock.
- Tool `create_budget` — testify/suite **whitebox** (R-TESTING-001): `SetupTest` com `fake.NewProvider()`, `dependencies` com IIFE por mock (`mocks.BudgetPlanner`, engine fake), SUT em `s.Run`; cenários: inicia workflow, competência clarify não inicia, input inválido (`Validate`).
- `Validate()` do input DTO — `errors.Join`, nomeia campos (R-DTO-VALIDATE-001).

### Testes de Integração

Critérios atendidos (≥2): fronteiras de IO críticas (Postgres do kernel de workflow, unicidade de orçamento) e histórico de falsos-verdes em harness que dirige workflow em vez de tools (memória de review). Adotar:

- `//go:build integration` com Postgres real (padrão do repo) — criação retroativa persistida e ativada; unicidade `(user_id, competence)` não duplica; draft de mês futuro tratado como existente; confirmação negada limpa estado e encerra run; TTL/reaper não deixa run suspenso.

### Testes E2E (real-LLM, gate estatístico)

- Suíte `RUN_REAL_LLM=1` (modelo `AGENT_HARNESS_MODEL`, default `openai/gpt-4o-mini`) espelhando `onboarding_workflow_integration_test.go` e `mecontrola_agent_integration_test.go`. **Gate estatístico ≥ 0.90**: N execuções por cenário, asserção sobre taxa de acerto agregada (o gate não existe no código hoje — é criado aqui, precedente em reviews anteriores). Cenários: criação com distribuição, retroativo (junho/2026), mês antigo (jan/2025), "mês passado"→junho/2026 por extenso, mês sem ano→clarifica, retrospectiva com/sem orçamento, competência já existente, confirmação negada, falha de persistência → mensagem específica.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Domínio de mês** (`internal/budgets`): `MonthReference` + `DecideCompetence` + `Prev()` + `FormatCompetencePtBR`, com testes puros. Sem dependência dos demais.
2. **Estado + decisões do workflow** (`budget_creation_state.go`, `budget_creation_decisions.go`), com testes puros.
3. **Workflow + Continuer + Reaper** (`budget_creation_workflow.go`, `budget_creation_continuer.go`), reusando `workflow.Engine`/`Store`/`Codec`/`StaleSuspendedReaper` e os helpers de extração/validação do onboarding (`DecideIncomeCents`, `_allocationInputSystemPrompt`, `allocationInputSchema`, `DecideAllocationsBP`, `ParseAllocationInputKind`, `_defaultDistributionBP`); `agent.Agent` injetado nos steps de coleta.
4. **Tool `create_budget`** + DTO `Validate`, testes whitebox; mapeamento `MonthReference` do payload.
5. **Wiring** em `module.go` (registro da tool, construção do workflow+engine, continuer, reaper/job, injeção no consumer) + `tryBudgetCreation` + mensagem específica.
6. **Resolver nas tools existentes** (`query_month`/`query_plan`) + instrução do agente (remove REGRA ABSOLUTA DE DATA, adiciona MonthReference, mês por extenso, composição da retrospectiva).
7. **Integração + E2E real-LLM (gate ≥0.90)**.

### Dependências Técnicas

- Postgres (kernel de workflow + budgets) para integração.
- `OPENROUTER_API_KEY` + `.env` para E2E real-LLM.
- `task mocks` se alguma nova interface for adicionada ao `.mockery.yml` (não previsto: `BudgetPlanner` já é mockado).

## Monitoramento e Observabilidade

- Nova série `agent_tool_invocations_total{tool="create_budget"}` passa a existir (hoje ausente — sinal de sucesso do PRD).
- Run auditável do workflow via kernel (`workflow`, `step`, `status`, `outcome`), labels de cardinalidade controlada (R-TXN-004 / R-WF-KERNEL-001.4): **proibido** `user_id`/`competence`/`correlation_key` como label.
- Reaper `budget-creation` emite contador de runs expirados (padrão `StaleSuspendedReaper.reaped`).
- **Correção de observabilidade (RF-30):** na falha de persistência, o step retorna o erro para o kernel gravar `Snapshot.LastError` e o run registrar `error` não-vazio (o incidente teve `platform_runs.error` vazio). A mensagem ao usuário é específica de indisponibilidade, distinta do `fallbackReply`.

## Considerações Técnicas

### Decisões Chave

- [ADR-001](adr-001-workflow-budget-creation-hitl.md) — `create_budget` inicia workflow durável HITL; escrita/ativação no step de confirmação; Continuer no `try*`.
- [ADR-002](adr-002-resolucao-deterministica-mes-monthreference.md) — `MonthReference` união discriminada + `DecideCompetence` puro; substitui delegação ao LLM e a regra que rejeita mês relativo.
- [ADR-003](adr-003-formatacao-mes-por-extenso.md) — formatação por extenso como função pura em budgets; sem Strategy (anti-over-engineering).
- [ADR-004](adr-004-retrospectiva-composicao-tools-existentes.md) — retrospectiva por composição de `query_plan`/`query_month`; sem tool nova.
- [ADR-005](adr-005-mensagem-especifica-e-observabilidade.md) — mensagem específica de indisponibilidade + persistência do erro (RF-26, RF-30).

### Riscos Conhecidos

- **R1 — Merge-patch e mapas:** `Allocations map[string]int` no resume via merge-patch RFC 7386: chave com `null` remove; sobrescrita de chave existente substitui valor escalar (não soma). Mitigação: o step trata a distribuição acumulada como estado autoritativo e só aplica deltas explícitos; testes de resume parcial (`{"ResumeText":"..."}`) preservam `Allocations`.
- **R2 — Regressão de roteamento do LLM:** ao instruir `MonthReference` e `create_budget`, o LLM pode emitir referência errada. Mitigação: `DecideCompetence` é a autoridade (LLM só classifica o tipo); gate real-LLM ≥0.90; instrução por exemplo (lição de reviews anteriores sobre single-shot mascarar acurácia). A extração de total/distribuição reusa o schema e o `Decide*` já validados em produção pelo onboarding, reduzindo risco de parsing.
- **R3 — Precedência no `try*`:** um run `budget-creation` suspenso deve capturar o próximo inbound antes do agente; ordem incorreta reabre o LLM. Mitigação: `tryBudgetCreation` antes do agente, chave por `resourceId`; exclusão mútua com pending-entry/confirm (um estado de espera ativo por vez, como já garantido para os demais).
- **R4 — Draft de mês futuro:** unicidade trata competência com draft como existente; `CreateBudget.Execute` retorna `ErrBudgetConflict`. Mitigação: o step mapeia conflito para mensagem "já existe" e encerra sem duplicar/ativar (RF-11/RF-12).
- **R5 — Fuso na virada de ano:** `Previous`/`Next` em dez/jan. Mitigação: `Prev()`/`Next()` via `AddDate` sobre `CompetenceFromTime(now_in_saopaulo)`; testes de tabela nas viradas.
- **R6 — TTL órfão:** run suspenso deve encerrar. Mitigação: reaper dedicado `budget-creation` + limpeza determinística no step (nunca permanece `RunStatusSuspended` após confirm/cancel/expire).

### Conformidade com Padrões

- **R-AGENT-WF-001** — roteamento por registry (sem `switch case intent.Kind`); Tool fina; estados fechados (`BudgetAwaitingSlot`, `ToolOutcome`, `RunStatus`); LLM só nas call-sites sancionadas; Run auditável; Thread→Run; pending step persistido antes da confirmação; resume antes do parse; limpeza determinística (Addendum .7-A, contrato comportamental reemitido para este consumidor HITL).
- **R-WF-KERNEL-001** — kernel intocado; nenhum import de domínio no kernel; merge-patch no resume; sem LLM/SQL no kernel.
- **R-ADAPTER-001** — zero comentários em `.go` de produção; tool/consumer finos; sem SQL direto.
- **R-DTO-VALIDATE-001** — input DTO da tool com `Validate()`/`errors.Join` nomeando campos.
- **R-TESTING-001** — testify/suite whitebox, `fake.NewProvider()`, dependencies+IIFE, SUT em `s.Run`, mocks via `.mockery.yml`.
- **DMMF** — `Decide*` puros; state-as-type; união discriminada; sem `Result[T,E]`/currying/DSL.

### Arquivos Relevantes e Dependentes

- `internal/agents/module.go` (linhas 289–324 buildFinancialTools; 228 WithWriteToolSet; 239–242 reapers) — wiring.
- `internal/agents/application/interfaces/budget_planner.go` (CreateBudget:10, ActivateBudget:12) / `types.go` (DraftBudget:82, AllocationDraft) / `infrastructure/binding/budget_planner_adapter.go` (53–112).
- `internal/agents/application/workflows/{destructive_confirm_workflow.go, confirm_state.go, pending_entry_workflow.go, pending_entry_state.go, pending_entry_decisions.go}` — molde do workflow HITL (confirmação + estado fechado).
- `internal/agents/application/workflows/onboarding_workflow.go` (BuildIncomeStep:600, BuildMethodologyStep:684, BuildDistributionStep:730, DecideIncomeCents, DecideAllocationsBP, _defaultDistributionBP:35-56, canonicalSlugs) — molde da coleta via extração LLM-no-step + `Decide*` puro; helpers de distribuição reutilizados.
- `internal/platform/workflow/{engine.go, store.go, codec.go, step.go, combinators.go, reaper.go}` — kernel reutilizado (sem alteração).
- `internal/platform/agent/{runtime.go:175–238, agent.go:142, types.go:48–100, identity_context.go}` — outcome/identidade.
- `internal/budgets/domain/{valueobjects/competence.go, valueobjects/root_slug.go, entities/budget.go, commands/create_budget.go, commands/activate_budget.go, services/allocation_distributor.go}`.
- `internal/budgets/application/usecases/{create_budget.go, activate_budget.go, get_monthly_summary.go, edit_category_percentage.go}`.
- `internal/agents/application/tools/{query_month.go, query_plan.go, adjust_allocation.go, suggest_allocation.go}` — molde de tool + pontos de resolução de mês.
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (263 fallbackReply; cadeia try*).
- `internal/agents/application/agents/mecontrola_agent.go` (60 REGRA ABSOLUTA DE DATA; 197 competência; 199–204 mapa; 223 orçamento não encontrado).
- `.mockery.yml` (137 BudgetPlanner) / `Taskfile.yml` (task mocks).

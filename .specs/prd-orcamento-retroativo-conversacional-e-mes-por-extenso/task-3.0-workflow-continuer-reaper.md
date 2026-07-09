# Tarefa 3.0: Workflow budget-creation + Continuer + Reaper (coleta espelha onboarding)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Construir o workflow durável `budget-creation` (`workflow.Definition[BudgetCreationState]`, `Durable:true`, `MaxAttempts:1`), o `BudgetCreationContinuer` e o reaper dedicado. A coleta de total e distribuição **espelha os steps do onboarding** (`BuildIncomeStep`, `BuildMethodologyStep`): cada step suspende com prompt e, no resume, invoca `a.Execute(ctx, Request{Schema: strict})` para extrair o valor estruturado, validando com `Decide*` puro e fazendo reprompt. A persistência (create + activate) ocorre no slot de confirmação, delegando às portas existentes. Nenhuma regra/SQL/branching de domínio na orquestração.

<requirements>
- RF-01 (persistência via fluxo): a criação é executada por um fluxo real (create + activate).
- RF-02: workflow durável com suspend/resume e HITL.
- RF-03: distribuição por diálogo completo por categoria antes de criar; oferece default 40/10/10/10/30 (aceitar/customizar), sem reaproveitar perfil.
- RF-04: ativa só com total>0 e soma=10000 bps; soma≠100% pede ajuste, não ativa.
- RF-05: competência retroativa sem limite; futura permitida; única validação de tempo é o formato YYYY-MM.
- RF-08: confirmação "sim" → create+activate, resposta cita mês por extenso.
- RF-09: "não"/"cancela" → nada criado, estado limpo, run encerrado.
- RF-11: unicidade (user_id, competence) — informa e não duplica.
- RF-12: competência com draft de mês futuro → tratada como existente (não ativa via este fluxo).
</requirements>

## Subtarefas

- [ ] 3.1 `BuildBudgetCreationWorkflow(a agent.Agent, planner interfaces.BudgetPlanner)` com `StepFunc` root ramificando por `BudgetAwaitingSlot`.
- [ ] 3.2 Slot total: suspende; no resume extrai via `a.Execute` (reusa `incomeSchema`/`incomeExtract`) + `DecideIncomeCents`; reprompt em falha.
- [ ] 3.3 Slot distribuição: oferece `_defaultDistributionBP`; extrai via `_allocationInputSystemPrompt`/`allocationInputSchema` + `ParseAllocationInputKind` + `DecideAllocationsBP`; só transita com soma=10000.
- [ ] 3.4 Slot confirmação: `isSim` → `planner.CreateBudget(DraftBudget)` + `planner.ActivateBudget`; `ErrBudgetConflict` → mensagem "já existe" e encerra sem duplicar; `isNao` → cancela; mensagem de sucesso com mês por extenso (`FormatCompetencePtBR`).
- [ ] 3.5 `BudgetCreationContinuer.Continue(ctx, resourceID, text, messageID)`: Load→Resume (merge-patch) se suspenso; senão não-handled; chave por `resourceId`.
- [ ] 3.6 `BuildBudgetCreationReaper` (`staleAfter` 35min) reusando `StaleSuspendedReaper`.
- [ ] 3.7 Testes: unit (steps com `agent.Agent` fake + `mocks.BudgetPlanner`) e integração Postgres (persistência/unicidade/draft/limpeza) sob `//go:build integration`.

## Detalhes de Implementação

Ver techspec.md → "Design de Implementação", "Contrato das 5 categorias raiz" e ADR-001. Reusar `workflow.Engine`/`Store`/`Codec`/`StaleSuspendedReaper` e helpers do onboarding (`onboarding_workflow.go`: DecideIncomeCents, DecideAllocationsBP, _defaultDistributionBP, canonicalSlugs, allocationInputSchema, ParseAllocationInputKind). `CreateBudgetCommand` tolera soma ≤10000; igualdade estrita imposta em `Budget.Activate`. `create_budget` NÃO entra em `WithWriteToolSet` (starter de workflow).

## Critérios de Sucesso

- `go build`, `go vet`, `go test -race`, lint verdes no pacote `workflows` e no módulo agents.
- Nenhum run permanece `RunStatusSuspended` após confirm/cancel/expire; reaper cobre órfãos.
- Unicidade não duplica; draft futuro tratado como existente; retroativo sem bloqueio de antiguidade.
- Zero comentários; sem SQL direto no step (delega a portas).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — construção de workflow durável HITL, Continuer no `try*`, resume por merge-patch e reaper sobre o substrato `internal/platform/{agent,workflow}`.
- `design-patterns-mandatory` — gate de desenho da orquestração (StepFunc + ramificação por estado) reusando Registry/Command; evitar over-engineering de combinadores.

## Testes da Tarefa

- [ ] Testes unitários (steps com Agent fake + mocks.BudgetPlanner; transições/reprompt/conflito).
- [ ] Testes de integração Postgres (`//go:build integration`): persistência, unicidade, draft futuro, limpeza determinística.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/budget_creation_workflow.go` (novo)
- `internal/agents/application/usecases/budget_creation_continuer.go` (novo)
- `internal/agents/application/workflows/onboarding_workflow.go` (helpers reutilizados — referência)
- `internal/agents/application/interfaces/budget_planner.go` / `infrastructure/binding/budget_planner_adapter.go` (portas)
- `internal/platform/workflow/{engine.go, reaper.go}` (kernel — referência)
- `internal/agents/application/workflows/*budget_creation*_test.go` (novos)

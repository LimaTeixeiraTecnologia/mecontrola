# Tarefa 4.0: Workflow `budget-edit` — estado fechado + `Decide*` puras

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Definir o estado durável do workflow de edição como tipos fechados (DMMF state-as-type) e as funções `Decide*` puras que governam coleta, confirmação, reprompt, expiração e replay. Espelha `budget_creation_state.go`/`budget_creation_decisions.go`, sem padrão GoF (ADR-001).

<requirements>
- `BudgetEditState` (JSON tags) com campos: `Operation`, `Awaiting`, `Status`, `UserID`, `Competence`, `CurrentTotalCents`, `NewTotalCents`, `TargetRootSlug`, `TargetPercentage`, `Allocations map[string]int`, `ResumeText`, `ResponseText`, `RepromptCount`, `MessageID`, `IncomingMessageID`, `SuspendedAt`, `Expired`.
- Tipos fechados com `String()`/`Parse*`/`IsValid()`:
  - `BudgetEditOperation`: `EditTotal | AdjustCategory | Redistribute`.
  - `BudgetEditAwaiting`: `AwaitingEditValue | AwaitingEditConfirm`.
  - `BudgetEditStatus`: `Active | Completed | Cancelled | Expired`.
- `Decide*` puras (sem IO/ctx): `DecideBudgetEditTotal` (≤0 → reprompt), `DecideBudgetEditDistribution` (soma ≠ 10000 → reprompt), `DecideBudgetEditAdjust` (categoria válida + % 0–100, 0% permitido), `DecideBudgetEditConfirmation` (sim/não/ambíguo/expira/replay), `DecideBudgetEditPendingResume`.
- Constantes `budgetEditTTL = 30 * time.Minute`, `budgetEditMaxReprompts = 1`.
- Reusar regex compartilhado `reConfirmYes`/`reConfirmNo`/`isCancelMessage` (cancelamento em qualquer passo — RF-27).
- Estados nunca como string livre; sem comentários.
</requirements>

## Subtarefas
- [ ] 4.1 `budget_edit_state.go`: struct + 3 tipos fechados + helpers.
- [ ] 4.2 `budget_edit_decisions.go`: `Decide*` + constantes + `BudgetEditAction`.
- [ ] 4.3 Testes suite table-driven das `Decide*` (feliz/alternativo/erro; expiração 31min; replay; reprompt 1x→2x cancela).

## Detalhes de Implementação
Ver techspec.md "Modelos de Dados" + ADR-002. Moldes: `budget_creation_state.go:11-110`, `budget_creation_decisions.go:8-110`, `pending_entry_decisions.go:121-150` (regex/cancel).

## Critérios de Sucesso
- Tipos fechados com `Parse*` rejeitando valor inválido; `Decide*` deterministas e testadas sem mock.
- Cobertura das transições da matriz conversacional E1–E4/F1–F2.
- `build`/`vet`/`test -race`/`lint` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — estado/decisões do workflow durável sobre o substrato de plataforma (agent/workflow), tipos fechados de fronteira.
- `domain-modeling-production` — state-as-type e funções `Decide*` puras determinísticas (DMMF).
- `design-patterns-mandatory` — gate de desenho da máquina de estados (selector já executado: `reject`/não aplicar padrão; registrar a decisão no desenho).

## Testes da Tarefa
- [ ] Testes unitários (suite table-driven das `Decide*` e dos tipos fechados)
- [ ] Testes de integração (não aplicável nesta tarefa)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/budget_edit_state.go` (novo)
- `internal/agents/application/workflows/budget_edit_decisions.go` (novo)

# Tarefa 7.0: Wiring `module.go` + consumer resolver + reaper job

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Conectar o workflow `budget-edit` ao runtime: engine, agent, definição, tool, continuer, reaper e o resolver no consumer inbound (na ordem correta da `tryResumeChain`).

<requirements>
- `internal/agents/module.go`: criar `budgetEditEngine := workflow.NewEngine[workflows.BudgetEditState](workflowStore, deps.O11y)`; `budgetEditAgent` + `budgetEditDef := workflows.BuildBudgetEditWorkflow(...)`; passar engine+def a `buildFinancialTools` (registrar `edit_budget`); `continueBudgetEdit := usecases.NewBudgetEditContinuer(...)`; `budgetEditReaper := workflows.BuildBudgetEditReaper(...)` no slice de Jobs; `consumers.WithBudgetEditResolver(continueBudgetEdit)` em `consumerOpts`.
- `whatsapp_inbound_consumer.go`: interface `budgetEditResolver`; campo na struct; `WithBudgetEditResolver` option; `tryContinueBudgetEdit` adicionado à `tryResumeChain` **após** `tryContinueBudgetCreation` e **antes** de `tryResolveOnboarding` (preserva o curto-circuito que garante exclusividade — ADR-004); handler emite métrica de outcome e envia reply.
- Nenhuma mudança de assinatura pública além do necessário; zero comentários; cardinalidade de métrica controlada (sem `user_id`/`category_id`).
- Manter verdes os invariantes de inventário de tools ao registrar `edit_budget` em `buildFinancialTools`: `postdeploy/regression_contract_test.go:104` (`RegisteredTools` len = 25) e `module_wiring_source_test.go:64` (`buildFinancialTools` registra exatamente `len(RegisteredTools)`). Coordenar com a edição de `RegisteredTools` feita em 6.0 (remove `adjust_allocation`, adiciona `edit_budget`; net 25).
</requirements>

## Subtarefas
- [ ] 7.1 Wiring no `module.go` (engine/agent/def/tool/continuer/reaper/resolver).
- [ ] 7.2 Atualizar assinatura de `buildFinancialTools` e registrar `edit_budget`.
- [ ] 7.3 Consumer: interface + option + `tryContinueBudgetEdit` na cadeia (ordem correta).
- [ ] 7.4 Testes de boot do módulo + consumer (resolver na cadeia).

## Detalhes de Implementação
Ver techspec.md "Visão Geral dos Componentes" + ADR-004. Moldes: `module.go` (linhas de wiring do budget-creation: engine, def, continuer, reaper, resolver, buildFinancialTools), `whatsapp_inbound_consumer.go:183-201` (`tryResumeChain`) + molde `tryContinueBudgetCreation`.

## Critérios de Sucesso
- Módulo compila e sobe com o `budget-edit` registrado; `edit_budget` disponível ao agente; reaper no slice de Jobs.
- Resume chain resolve `budget-edit` na ordem correta; exclusividade preservada.
- `build`/`vet`/`test -race`/`lint` verdes; teste de boot verde.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — wiring do runtime de agente (engine/def/tool/continuer/reaper) e resolver no consumer inbound, preservando o contrato do substrato.

## Testes da Tarefa
- [ ] Testes unitários (boot do módulo; resolver presente na cadeia)
- [ ] Testes de integração (`module_boot_integration_test.go` estende cobertura do `budget-edit`)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`

# Tarefa 6.0: Tool `edit_budget` (prefill + pré-check) + remoção de `adjust_allocation`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a tool starter `edit_budget` que resolve a competência, pré-checa existência/estado e inicia o workflow `budget-edit` (com prefill), e **remover** a tool imediata `adjust_allocation` (mutação sem confirmação — ADR-002).

<requirements>
- `edit_budget.go`: input com `operation` (enum obrigatório `EditTotal|AdjustCategory|Redistribute`), referência de mês (`monthRefKind`,`year`,`month`) e valores opcionais de prefill (`newTotalCents`; `targetRootSlug`+`targetPercentage`). `Validate()` por operação (R-DTO-VALIDATE-001).
- Resolver competência via `budgetsvo.DecideCompetence` (RF-06..RF-09); `ClarifyMissingYear`/`ClarifyUnrecognized` → outcome `clarify` (RF-07/RF-08).
- Pré-check `planner.GetMonthlySummary`: `ErrBudgetNotFound` → outcome `offer_create` (RF-10); orçamento `AutoDraft` sem alocações → outcome `offer_create` (RF-13/R2). Caso ok, preencher `CurrentTotalCents`.
- Iniciar `engine.Start(budget-edit, BudgetEditKey(resourceID), state)`; `ErrRunAlreadyExists` → outcome `pending_flow_exists` (RF-33). Se prefill válido → estado inicia em `AwaitingEditConfirm`; senão `AwaitingEditValue`.
- `userID` do inbound (`agent.InboundIdentityFromContext`/`RuntimeFrom`) — só o dono edita (RF-37).
- Desambiguação de operação vaga e pedido combinado: instrução do agente pede a operação antes de chamar a tool (RF-02/RF-03); tool exige `operation` concreto.
- **Remover** `adjust_allocation.go` e atualizar TODOS os dependentes vivos (blast radius verificado): registro em `buildFinancialTools` (module.go); inventário `RegisteredTools` em `application/postdeploy/regression_contract.go:10` (trocar `adjust_allocation`→`edit_budget`, mantendo contagem = 25); mapa de write-tools/args em `application/scorers/behavioral_scorers.go:18,34,66` e `application/scorers/mecontrola_scorers.go:30`; guard `successWithoutToolWriteTools` em `application/agents/guards/success_without_tool.go:28`; prompt do agente em `application/agents/mecontrola_agent.go:167,169,246` (descrever a edição com HITL via `edit_budget`, remover instruções `adjust_allocation`); golden `application/golden/cases_journey.go:45,46,55,56`; e os testes de `adjust_allocation`.
- Manter verdes os invariantes de inventário: `postdeploy/regression_contract_test.go:104` (`RegisteredTools` len = 25) e `postdeploy/module_wiring_source_test.go:64` (`buildFinancialTools` registra exatamente `len(RegisteredTools)`). Net: remove 1 tool, adiciona 1 → contagem preservada só se `RegisteredTools` for editado.
- Copy PT-BR reaproveitando o tom da criação; sem inventar respostas fora do runtime (RF-39). Adapter fino, sem regra de negócio (R-AGENT-WF-001.2). Zero comentários.
</requirements>

## Subtarefas
- [ ] 6.1 `edit_budget.go`: schemas, `Validate()` por operação, exec (competência + pré-check + Start + prefill).
- [ ] 6.2 Outcomes fechados (`started|clarify|offer_create|pending_flow_exists`).
- [ ] 6.3 Remover `adjust_allocation.go` + registro; atualizar `RegisteredTools`, scorers, guard, prompt do agente e golden `cases_journey.go` (blast radius completo).
- [ ] 6.4 Manter verdes os invariantes de inventário (`regression_contract_test.go:104`, `module_wiring_source_test.go:64`).
- [ ] 6.5 Testes suite da tool.

## Detalhes de Implementação
Ver techspec.md "Prefill"/"Fluxo de Dados" + ADR-002/ADR-004. Moldes: `create_budget.go:19-187` (starter), `competence_reference.go:50` (copy offer create), `types.go:102` (`BudgetSummary.AutoDraft/Allocations`).

## Critérios de Sucesso
- `edit_budget` inicia o workflow certo por operação; prefill pula coleta; clarify/offer_create/pending_flow_exists corretos.
- `adjust_allocation` removida; nenhum caminho de escrita sem confirmação permanece; testes/golden ajustados.
- `build`/`vet`/`test -race`/`lint` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — tool starter fina que inicia workflow durável, pré-check e roteamento por registry (sem regra de negócio), copy WhatsApp.

## Testes da Tarefa
- [ ] Testes unitários (suite: cada operação, clarify, offer_create, pending_flow_exists, prefill)
- [ ] Testes de integração (cobertos por 5.0/8.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/edit_budget.go` (novo)
- `internal/agents/application/tools/adjust_allocation.go` (remover)
- `internal/agents/application/postdeploy/regression_contract.go` (`RegisteredTools`)
- `internal/agents/application/scorers/behavioral_scorers.go`, `scorers/mecontrola_scorers.go`
- `internal/agents/application/agents/guards/success_without_tool.go`
- `internal/agents/application/agents/mecontrola_agent.go` (prompt)
- `internal/agents/application/golden/cases_journey.go`
- `internal/agents/application/postdeploy/regression_contract_test.go`, `module_wiring_source_test.go` (invariantes)

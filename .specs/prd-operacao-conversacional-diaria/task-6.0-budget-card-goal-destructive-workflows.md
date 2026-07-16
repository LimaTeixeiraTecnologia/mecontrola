# Tarefa 6.0: Workflows budget-manage, card-manage, goal-edit, destructive-confirm

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os quatro workflows duráveis restantes da topologia em `internal/agents/application/workflows/`: `budget-manage` (criação retroativa, alterar total e alterar distribuição), `card-manage` (cadastrar e editar cartão), `goal-edit` (alterar objetivo na WorkingMemory) e `destructive-confirm` (excluir cartão/recorrência com aviso de impacto). Cada um com estado fechado próprio discriminado por `OperationKind` fechado (`map[OperationKind]decideFn`), `Decide*` puros e confirmação universal antes de gravar. Mensagens consumidas do catálogo (task 4.0).

<requirements>
- RF-04, RF-10, RF-17, RF-18, RF-19, RF-20, RF-21, RF-22, RF-31.
- ADR-001: topologia por forma de interação; `budget-manage` com OperationKind `create_retroactive`/`edit_total`/`edit_distribution`; `card-manage`, `goal-edit`, `destructive-confirm`; discriminação por `map[OperationKind]decideFn`, nunca `switch case intent.Kind`.
- R-AGENT-WF-001: fluxo Workflow→Tool→binding→usecase; `Decide*` puros; estados fechados (state-as-type); confirmação/pending step antes de gravar; resume por merge-patch; Run auditável; cardinalidade controlada.
- R-WF-KERNEL-001: consome `workflow.Engine[S]`/`Store` do kernel sem reimplementar mecanismo.
- R-ADAPTER-001.1: zero comentários em `.go` de produção.
- R-TESTING-001: suite canônica testify/suite nos testes de usecase.
- Dependências: task 3.0 (`EditBudgetTotal`) e task 4.0 (catálogo de mensagens).
</requirements>

## Subtarefas

- [ ] 6.1 `budget-manage`: estado fechado, `OperationKind` `create_retroactive`/`edit_total`/`edit_distribution`, consumindo `EditBudgetTotal` (task 3.0), `EditCategoryPercentage` e a criação retroativa; confirmação universal antes de gravar.
- [ ] 6.2 `card-manage`: cadastrar e editar cartão, com confirmação universal inclusive para apelido/banco.
- [ ] 6.3 `goal-edit`: alterar objetivo via read-modify-write da seção `## Objetivo Financeiro` na WorkingMemory, preservando as demais seções, através dos usecases de memory.
- [ ] 6.4 `destructive-confirm`: excluir cartão/recorrência com aviso de impacto via `BuildImpactNote`, passando pelo gate de confirmação.
- [ ] 6.5 Todos com `Decide*` puros, estados fechados e confirmação universal antes de gravar; mensagens do catálogo (task 4.0).
- [ ] 6.6 Testes unitários dos `Decide*` por workflow + suite canônica testify/suite para os usecases.

## Detalhes de Implementação

Ver `techspec.md` (RF-04, RF-10, RF-17, RF-18, RF-19, RF-20, RF-21, RF-22, RF-31) e `adr-001-workflow-topology.md` desta pasta — **referenciar em vez de duplicar**.

Pontos-chave do ADR-001:
- Cada workflow é uma `workflow.Definition[S]` com estado fechado próprio; discriminação interna por `OperationKind` fechado com `map[OperationKind]decideFn`; proibido `switch case intent.Kind` (R-AGENT-WF-001.1).
- `budget-manage` agrega `create_retroactive`/`edit_total`/`edit_distribution`; `goal-edit` altera o objetivo na WorkingMemory; `destructive-confirm` inclui aviso de impacto.
- Confirmação universal antes de gravar; pending step persistido no snapshot antes de pedir confirmação; resume por merge-patch; reuso do padrão durável (suspend/resume, reaper).
- `goal-edit` faz read-modify-write preservando as seções não editadas da WorkingMemory.

## Critérios de Sucesso

- Cada fluxo (budget/card/goal/destructive) confirma antes de gravar.
- Mensagens verbatim vêm do catálogo (task 4.0).
- `goal-edit` preserva as demais seções da WorkingMemory ao alterar `## Objetivo Financeiro`.
- Exclusão de cartão/recorrência passa pelo gate de confirmação com aviso de impacto (`BuildImpactNote`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — workflows duráveis (budget/card/goal/destructive) sobre o substrato de agente.
- `domain-modeling-production` — estados fechados e Decide* puros (state-as-type).
- `design-patterns-mandatory` — gate de desenho dos quatro workflows.

## Testes da Tarefa

- [ ] Testes unitários (por workflow: `Decide*` de cada operação)
- [ ] Testes de integração (suspend/resume de cada workflow)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/budget_manage_state.go`, `budget_manage_decisions.go`, `budget_manage_workflow.go`
- `internal/agents/application/workflows/card_manage_state.go`, `card_manage_decisions.go`, `card_manage_workflow.go`
- `internal/agents/application/workflows/goal_edit_state.go`, `goal_edit_decisions.go`, `goal_edit_workflow.go`
- `internal/agents/application/workflows/destructive_confirm_state.go`, `destructive_confirm_decisions.go`, `destructive_confirm_workflow.go`
- `internal/agents/application/workflows/*_test.go`
- `internal/agents/application/messages/catalog.go` (consumo; task 4.0)

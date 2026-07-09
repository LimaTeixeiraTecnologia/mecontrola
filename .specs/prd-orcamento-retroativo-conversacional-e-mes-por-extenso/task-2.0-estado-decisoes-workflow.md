# Tarefa 2.0: Estado e decisões do workflow budget-creation (tipos fechados + Decide* puros)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Modelar o estado durável e as decisões puras do workflow de criação de orçamento, em `internal/agents/application/workflows`, espelhando `confirm_state.go`/`pending_entry_state.go`/`pending_entry_decisions.go`. Tudo como tipos fechados (state-as-type) e funções `Decide*` puras (`now` injetado), sem IO/LLM. Persistência do estado de espera acontece via o kernel (esta tarefa só define o estado e as decisões).

<requirements>
- RF-06: estado de espera do diálogo como tipo fechado (`BudgetAwaitingSlot`: Total, Distribution, Confirm) apto a ser salvo no `Snapshot` antes de cada pergunta.
- RF-07: confirmação explícita + limpeza determinística (confirm/cancel/expire encerram; nunca permanece suspenso).
- RF-28: estados de fronteira como tipos fechados, nunca string livre.
</requirements>

## Subtarefas

- [ ] 2.1 `BudgetCreationState` (campos: Status, Awaiting, UserID, Competence, TotalCents, Allocations map[string]int, ResumeText, ResponseText, RepromptCount, MessageID, SuspendedAt, Expired).
- [ ] 2.2 Tipos fechados `BudgetAwaitingSlot` e `BudgetCreationStatus` com `String()`/`IsValid()`/`Parse*`.
- [ ] 2.3 `Decide*` de coleta: total inválido → reprompt; distribuição incompleta (soma≠10000) bloqueia; soma=10000 transita.
- [ ] 2.4 `Decide*` de confirmação: `isSim`/`isNao` determinístico; ambíguo → reprompt único → cancel; TTL 30min (avaliado no resume) → expire; replay de `messageID`.
- [ ] 2.5 Testes de tabela puros (sem mock) para todas as transições e limpeza determinística.

## Detalhes de Implementação

Ver techspec.md → "Modelos de Dados" (`BudgetCreationState`, constantes de tempo `budgetCreationTTL=30min`) e ADR-001. Reusar o padrão de `DecidePendingResume`/`DecideConfirmation` (recebem `now time.Time`). A validação de soma de distribuição reusa `DecideAllocationsBP` na tarefa 3.0 (aqui, a decisão de transição de slot é pura sobre o estado).

## Critérios de Sucesso

- `go build`, `go vet`, `go test -race`, lint verdes no pacote `workflows`.
- Nenhum estado como string livre; `IsValid()` cobre todas as constantes.
- Nenhum caminho deixa o estado sem encerramento após confirm/cancel/expire.
- Zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — estado de espera como tipo fechado (state-as-type) e `Decide*` puros de transição/confirmação.
- `mastra` — contrato de estado durável/PendingStep do substrato de agente (Snapshot, resume por merge-patch, limpeza determinística).

## Testes da Tarefa

- [ ] Testes unitários de tabela (puros) para coleta, confirmação, TTL, replay e limpeza.
- [ ] Testes de integração (não aplicável — lógica pura).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/budget_creation_state.go` (novo)
- `internal/agents/application/workflows/budget_creation_decisions.go` (novo)
- `internal/agents/application/workflows/budget_creation_*_test.go` (novos)

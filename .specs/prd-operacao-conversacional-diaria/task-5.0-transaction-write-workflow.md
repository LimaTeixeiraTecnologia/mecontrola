# Tarefa 5.0: Workflow transaction-write (registro/edição/recorrência)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o workflow durável `transaction-write` em `internal/agents/application/workflows/` cobrindo registro de despesa/receita, edição de lançamento e criação de recorrência, com slot-filling e confirmação universal antes de gravar. Estado fechado próprio (`TransactionWriteState`) discriminado por `OperationKind` fechado via mapa `map[OperationKind]decideFn` (sem `switch case intent.Kind`). Reencarna as invariantes de produção: idempotência wamid+item_seq, guarda anti-falso-sucesso, reclassificação de categoria por kind, TTL/reaper, teto de reprompt e resume por merge-patch.

<requirements>
- RF-01, RF-02, RF-04, RF-06, RF-07, RF-09, RF-10, RF-11, RF-13, RF-14, RF-15, RF-16, RF-31.
- ADR-001: topologia por forma de interação; `OperationKind` fechado com `map[OperationKind]decideFn`, nunca `switch case intent.Kind`.
- R-AGENT-WF-001: fluxo Workflow→Tool→binding→usecase; `Decide*` puros; estados fechados (state-as-type); LLM só nas call-sites sancionadas; resume por merge-patch; pending step antes de confirmação; Run auditável; cardinalidade controlada.
- R-WF-KERNEL-001: consome `workflow.Engine[S]`/`Store` do kernel sem reimplementar mecanismo.
- R-ADAPTER-001.1: zero comentários em `.go` de produção.
- R-TESTING-001: suite canônica testify/suite nos testes de usecase.
- Dependências: task 1.0 (evento/enum), task 2.0 (`SearchEditCandidates`), task 4.0 (catálogo de mensagens).
</requirements>

## Subtarefas

- [ ] 5.1 Estado fechado `TransactionWriteState` (campos `Awaiting*` e `OperationKind` ∈ `register_expense`/`register_income`/`edit_entry`/`create_recurrence`) em `internal/agents/application/workflows/`, como tipos fechados com constantes enumeradas (DMMF state-as-type).
- [ ] 5.2 `Decide*` PUROS (slot-filling, confirmação, pós-escrita) sem IO nem `context.Context`, discriminados por `OperationKind` via `map[OperationKind]decideFn`.
- [ ] 5.3 Reencarnar invariantes: idempotência wamid+item_seq via `agents_write_ledger` (reusar `write_ledger`/`idempotent_write`); guarda anti-falso-sucesso (`DecidePostWrite` → `StepStatusFailed` + métrica); reclassificação de categoria por kind; TTL/reaper; teto de reprompt; resume por merge-patch (snapshot como fonte única).
- [ ] 5.4 Edição: consumir `SearchEditCandidates` (task 2.0) para localizar candidatos; permitir alterar valor, categoria/subcategoria e forma de pagamento (respeitando `guardPaymentMethodMigration` — crédito<->não-crédito bloqueado, mensagem do catálogo).
- [ ] 5.5 Mensagens de confirmação/sucesso/esclarecimento consumidas do catálogo (task 4.0), sem geradores locais.
- [ ] 5.6 Testes unitários dos `Decide*` + suite canônica testify/suite para o usecase do workflow.

## Detalhes de Implementação

Ver `techspec.md` (RF-01, RF-02, RF-04, RF-06, RF-07, RF-09, RF-10, RF-11, RF-13, RF-14, RF-15, RF-16, RF-31) e `adr-001-workflow-topology.md` desta pasta — **referenciar em vez de duplicar**.

Pontos-chave do ADR-001:
- `transaction-write` é uma `workflow.Definition[S]` com estado fechado próprio; registro de despesa/receita/recorrência e edição com slot-filling e confirmação universal antes de gravar.
- Discriminação interna por `OperationKind` fechado com `map[OperationKind]decideFn`; proibido `switch case intent.Kind` (R-AGENT-WF-001.1).
- Só pede o slot ausente; confirmação universal antes de qualquer escrita; pending step persistido no snapshot antes de pedir confirmação; resume aplica merge-patch antes do parse.
- Idempotência, guarda de falso-sucesso, TTL e reprompt são invariantes de produção reencarnadas; escrita só via binding→usecase de `internal/transactions`.

## Critérios de Sucesso

- Registro de despesa/receita, edição e criação de recorrência confirmam antes de gravar.
- Mensagens verbatim vêm do catálogo (task 4.0).
- Escrita idempotente por wamid+item_seq; replay não gera segunda mutação.
- Zero falso-sucesso: falha de escrita resulta em `StepStatusFailed` + métrica, nunca mensagem de sucesso.
- Slot-filling pede apenas o slot ausente.
- Edição de forma de pagamento respeita `guardPaymentMethodMigration`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — workflow durável transaction-write sobre o kernel e o substrato de agente.
- `domain-modeling-production` — estado fechado e funções Decide* puras (state-as-type).
- `design-patterns-mandatory` — gate de desenho do workflow e das operações.

## Testes da Tarefa

- [ ] Testes unitários (`Decide*`, idempotência, falso-sucesso, reclassify por kind, TTL)
- [ ] Testes de integração (suspend/resume + write duplicado)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/transaction_write_state.go`
- `internal/agents/application/workflows/transaction_write_decisions.go`
- `internal/agents/application/workflows/transaction_write_workflow.go`
- `internal/agents/application/workflows/transaction_write_*_test.go`
- `internal/agents/application/messages/catalog.go` (consumo; task 4.0)

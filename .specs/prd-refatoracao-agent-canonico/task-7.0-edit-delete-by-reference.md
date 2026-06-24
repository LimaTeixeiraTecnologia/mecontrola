# Tarefa 7.0: Editar/apagar por referência + desambiguação (search + HITL)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar editar/apagar lançamento por referência (descrição) com desambiguação de múltiplos
resultados, reusando o workflow `destructive_confirm`. Adiciona porta de busca no `transactions` (dono
do dado), steps `resolve_candidates`/`select_target`, tipos fechados `AwaitingSelect`/`TargetCandidate`/
`OperationDeleteByRef`/`OperationEditByRef` e executors by-ref. Contrato HITL ADR-003 as-is.

<requirements>
- RF-21: consumo de outro módulo pela porta de entrada mais adequada (search via usecase do transactions).
- RF-22: tools do agent permanecem adapters finos (sem regra/SQL/branching de domínio).
- RF-24: agente não replica regra de negócio (busca/ILIKE no dono; optimistic lock no transactions).
- RF-31: alteração/exclusão seguem Localizar → Exibir → Confirmar → Executar → Confirmar sucesso (HITL).
- RF-32: gate HITL suspende com estado durável (kernel), retoma idempotente, limpa após efetivar/cancelar/expirar.
- RF-36: editar/apagar por referência (localizar por descrição via porta de leitura, exibir, HITL).
- RF-37: múltiplos resultados → escolha (pending step de desambiguação com AwaitingKind fechado) antes do confirm.
- RF-38: contrato HITL ADR-003 as-is (sim/não, reprompt único, TTL, replay sem duplicar).
</requirements>

## Subtarefas

- [ ] 7.1 No `internal/transactions`: VO `NewSearchQuery` (smart constructor, R-TXN-002), método repo `SearchByDescription` (SQL `ILIKE` + user_id + deleted_at IS NULL + LIMIT), usecase `SearchTransactions`.
- [ ] 7.2 Binding fino no agent: `TransactionSearcher` (contract) + `TransactionSearcherAdapter`.
- [ ] 7.3 Estender `confirmation.ConfirmState`: `AwaitingSelect`, `TargetCandidate{TxID,Version,Description,AmountCents,OccurredAt}`, `Target{TxID,Version,Desc,Amount}`, `NewAmount`, `SearchQuery`; `OperationDeleteByRef`/`OperationEditByRef` (tipos fechados + String/IsValid/Parse).
- [ ] 7.4 Steps `resolve_candidates` (chama searcher; popula Candidates) e `select_target` (0→shortcut; 1→auto; N→suspende AwaitingSelect; resume parseia índice; reprompt único) — inseridos antes do `confirm_gate` em `destructive_confirm`.
- [ ] 7.5 Kinds `KindDeleteTransactionByRef`/`KindEditTransactionByRef` (state-as-type) + entradas em `intentToOperationKind`/`resolveIntentKindFromOperation` (sem crescer switch — R-AGENT-WF-001.1); parse popula `SearchQuery` e `NewAmount`.
- [ ] 7.6 Executors by-ref usam `Target*`+version (optimistic lock) reusando `LastTransactionEditor`/`Deleter`; **corrigir bug latente** do `NewAmount` não preenchido no executor de edit; mapear `ErrTransactionVersionConflict` para mensagem amigável.
- [ ] 7.7 Candidatos persistidos no snapshot (não re-buscados no resume — R-WF-KERNEL-001.7).

## Detalhes de Implementação

Ver `adr-008-edit-delete-by-reference.md` (componentes novos + sequência de steps) e techspec
§"Busca por referência", §"Desambiguação". DMMF: tipos fechados; `select_target` é função pura.

## Critérios de Sucesso

- `Apaga o Uber` (1→confirma→apaga), `Apaga o mercado` (N→escolhe→confirma), `O Uber foi 42 e não 35` (edita) verdes em e2e.
- Nenhuma efetivação sem item único selecionado e confirmado; índice estável suspend↔resume.
- Busca por ILIKE no transactions (gate de fronteira de dados verde); zero SQL/loop no agent.
- `agent_target_select_total{outcome}` com cardinalidade controlada.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — adiciona steps/workflow/tool e estado de espera HITL no `internal/agent` (R-AGENT-WF-001.7-A).

## Testes da Tarefa

- [ ] Testes unitários (`select_target` 0/1/N + índice válido/inválido + reprompt; `NewSearchQuery` invariantes; executors by-ref; version conflict).
- [ ] Testes de integração (`SearchByDescription` ILIKE/limit/isolamento por user; suspend AwaitingSelect→resume índice→confirm→execute durável).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/domain/valueobjects/search_query.go` (novo), `application/usecases/search_transactions.go` (novo), `application/interfaces/transaction_repository.go`, `infrastructure/repositories/postgres/transaction_repository.go`
- `internal/agent/domain/confirmation/draft.go`, `application/workflow/{destructive_confirm.go,steps/*}`
- `internal/agent/domain/intent/intent.go`, `application/services/daily_ledger_agent.go`
- `internal/agent/infrastructure/binding/{transaction_query.go,hitl_adapters.go}`

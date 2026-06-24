# ADR-008 — Editar/Apagar Lançamento por Referência com Desambiguação

## Metadados

- **Título:** Localizar lançamento por descrição, desambiguar N resultados e confirmar (HITL), reusando `destructive_confirm`
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Solicitante + plataforma
- **Relacionados:** PRD (RF-36, RF-37, RF-38), Documento Oficial Cap 11, techspec §"Busca por referência" e §"Desambiguação", `R-AGENT-WF-001`, `R-TXN-WORKFLOWS-001`, `R-WF-KERNEL-001.7`

## Contexto

O Documento Oficial (Cap 11) mostra apagar/editar **por referência**: `Apaga o Uber`,
`O Uber foi 42 e não 35`, `Apaga o mercado` → 3 resultados → escolher. Hoje só existem
`DeleteLast`/`EditLast` (último lançamento). Os bindings `LastTransactionDeleter`/`Editor` já recebem
`txID`+`version` explícitos (não "buscam o último"), e `ListTransactions` só filtra por `ref_month`
(sem busca por descrição). A confirmação destrutiva já existe (`destructive_confirm` + `ConfirmState`
+ `prepare_target`/`confirm_gate`/`execute_destructive`, suspend/resume durável, TTL, reprompt único).

## Decisão

Implementar by-ref **reusando ao máximo** o que existe:

1. **Porta de busca no transactions** (dono do dado): VO `NewSearchQuery` (smart constructor,
   R-TXN-002), método de repo `SearchByDescription` (SQL `ILIKE` + `user_id` + `deleted_at IS NULL` +
   `LIMIT` pequeno) e usecase `SearchTransactions`. **Não** sobrecarregar `ListTransactions` (ref_month
   obrigatório + paginação keyset). Busca/ILIKE é **SQL no dono**, nunca loop no agent
   (R-AGENT-WF-001.2).
2. **Adapter fino no agent**: `TransactionSearcher` (contract) + `TransactionSearcherAdapter`
   (binding→usecase), espelhando `TransactionListerAdapter`.
3. **Desambiguação no `destructive_confirm`** (não em `pendingexpense.Draft`, que é fase de categoria):
   estender `ConfirmState` (state-as-type) com `AwaitingSelect`, `TargetCandidate[]`,
   `Target{TxID,Version,Desc,Amount}`, `NewAmount`, `SearchQuery`; adicionar `OperationDeleteByRef`/
   `OperationEditByRef`. Inserir 2 steps **antes** do `confirm_gate`: `resolve_candidates` (chama o
   searcher; popula `Candidates`) e `select_target` (0→shortcut "não encontrei"; 1→auto; N→suspende
   `AwaitingSelect` com lista enumerada). `select_target` é função pura determinística; índice mapeado
   para o candidato **persistido no snapshot** (não re-buscado no resume — R-WF-KERNEL-001.7), com
   reprompt único.
4. **Kinds novos** `KindDeleteTransactionByRef`/`KindEditTransactionByRef` (state-as-type) + entradas
   no mapa `intentToOperationKind` (NÃO cresce switch de domínio — R-AGENT-WF-001.1). Parse popula
   `SearchQuery` de `Merchant()` e `NewAmount` de `AmountCents()`.
5. **Executors by-ref** usam `Target*`+`version` (optimistic lock) reusando
   `LastTransactionEditor`/`Deleter`; corrigir o bug latente de `NewAmount` não preenchido no executor
   de edit.

## Alternativas Consideradas

- **Buscar no agent (carregar mês e filtrar com `strings.Contains`)**: viola R-AGENT-WF-001.2, não
  escala (lançamentos ilimitados/cross-month). Rejeitada.
- **Reusar `pendingexpense.Draft` para escolha**: mistura fase de categoria com seleção de alvo;
  candidatos precisam de `txID`+`version`, não strings. Rejeitada.
- **Workflow separado fora do `destructive_confirm`**: duplica authorize/replay/policy/audit/TTL/
  resume. Rejeitada — mais código e risco.
- **Reusar kinds `DeleteLast`/`EditLast` com merchant preenchido**: discriminar por campo
  vazio/preenchido é branching de domínio frágil (R-AGENT-WF-001.2). Rejeitada — kinds explícitos.

## Consequências

### Benefícios Esperados

- Cobertura fiel ao Documento Oficial (Cap 11) reusando suspend/resume/TTL/HITL/audit existentes;
  delta mínimo (1 porta no transactions + 2 steps + tipos fechados).

### Trade-offs e Custos

- Nova porta no transactions; extensão do `ConfirmState`; novos kinds/operations a testar.

### Riscos e Mitigações

- **Optimistic-lock stale** entre busca e "sim": mapear `ErrTransactionVersionConflict` para mensagem
  amigável ("o lançamento mudou, tente de novo").
- **Índice estável suspend↔resume**: candidatos **persistidos no snapshot**, não re-buscados.
- **ILIKE sem índice**: `LIMIT` + `user_id`; evoluir p/ `pg_trgm` se preciso (encapsulado no repo).
- **Ambiguidade "qual é o novo valor"** em "foi 42 e não 35": resolver no schema/prompt do parse
  (único lugar com LLM). **Rollback:** desabilitar kinds by-ref (volta a last-only).

## Plano de Implementação

1. VO+repo+usecase `SearchTransactions` (transactions). 2. Binding `TransactionSearcher`. 3. Estender
   `ConfirmState` + `AwaitingSelect`/operations. 4. Steps `resolve_candidates`/`select_target` no
   `destructive_confirm`. 5. Kinds + mapa `intentToOperationKind`. 6. Executors by-ref + fix do
   `NewAmount`. 7. Testes unit/integração/e2e (Cap 11).

## Monitoramento e Validação

- `agent_target_select_total{outcome}` (found/none/multi/reprompt/cancel), cardinalidade controlada.
  Sucesso: cenários `Apaga o Uber`/`Apaga o mercado`/`O Uber foi 42` verdes em e2e; nenhuma efetivação
  sem item único selecionado e confirmado.

## Impacto em Documentação e Operação

- Runbook conversacional (exemplos by-ref), prompts de parse, OpenAPI de transactions (nova porta de
  busca, se exposta via HTTP).

## Revisão Futura

- Reavaliar ranking/`pg_trgm` e edição de outros campos (além de valor) conforme uso real.

# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Reconciliação de edição — enriquecer TransactionUpdated e consumir updated no budgets
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Solicitante do produto, engenharia de plataforma
- **Relacionados:** PRD `prd.md` (RF-15, RF-25, RF-26); techspec `techspec.md`; R-TXN-WORKFLOWS-001; Outbox

## Contexto

RF-15 permite editar valor e categoria de um lançamento. O resumo por categoria (RF-25) e o resumo geral (RF-26) usam a projeção `budgets_expenses`, alimentada por consumers de eventos. Hoje o budgets consome `transactions.transaction.created.v1` e `transactions.transaction.deleted.v1`, mas **não** consome `updated`; além disso, o evento `TransactionUpdated` carrega `PaymentMethod`, `AmountCents` e `RefMonth`, mas **não** a subcategoria. Consequência: editar valor ou categoria de um lançamento não reflete no resumo — um defeito de correção que arranha o objetivo de zero falso positivo.

## Decisão

Fechar a reconciliação de forma aditiva e event-driven:

1. Enriquecer o evento de domínio `TransactionUpdated` com `SubcategoryID` (mantendo `AmountCents`/`RefMonth`); o producer/outbox passa a serializar a subcategoria (payload versionado).
2. Adicionar no budgets um consumer de `transactions.transaction.updated.v1` que chama o `UpsertExpense` existente (resolve `RootSlug` pela subcategoria e move valor/competência via `existing.Edit`), espelhando o skip do consumer de created quando não há subcategoria/expense (ex.: receita).
3. Idempotência por `event_id` conforme o padrão de outbox do repositório.

## Alternativas Consideradas

- **budgets soma gasto-por-categoria direto do read model de transactions**: elimina a necessidade do consumer; rejeitada por acoplar budgets->transactions em leitura, mudar a fonte do resumo e descartar a projeção atual (mudança não aditiva, maior raio de risco).
- **Job periódico de reconciliação**: simples; rejeitada por deixar o resumo inconsistente na janela entre a edição e o job (consistência eventual), incompatível com zero falso positivo.

## Consequências

### Benefícios Esperados

- Resumo correto imediatamente após a edição; consistência forte.
- Mudança aditiva e coerente com o padrão event-driven (created/deleted já existem).

### Trade-offs e Custos

- Enriquecer o evento e adicionar consumer amplia o contrato entre dois módulos.

### Riscos e Mitigações

- Risco: eventos `updated` antigos sem subcategoria em trânsito no cutover. Mitigação: consumer tolera ausência (skip como no created) e payload é versionado; ADR-005 drena runs antes do corte.
- Risco: regra de domínio no consumer. Mitigação: consumer é adapter fino que delega ao `UpsertExpense` (R-ADAPTER-001); sem SQL/branching de domínio.

## Plano de Implementação

1. Adicionar `SubcategoryID` a `TransactionUpdated` e ao producer/payload.
2. Criar `transaction_updated_consumer` no budgets delegando ao `UpsertExpense`.
3. Registrar o handler em `internal/budgets/module.go` para `transactions.transaction.updated.v1`.
4. Testes de integração de reconciliação (categoria e valor).

## Monitoramento e Validação

- Counters `budgets_transaction_updated_consumer_{decode_failed,skipped}_total`; span por handle.
- Sucesso: teste de integração move o valor entre categorias raiz após edição; `GetMonthlySummary` reflete a mudança.

## Impacto em Documentação e Operação

- Atualizar o contrato de evento `TransactionUpdated` e o runbook de reconciliação budgets.

## Revisão Futura

- Revisitar se a projeção `budgets_expenses` for consolidada com o read model de transactions no futuro.

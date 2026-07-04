# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Tabela única `transactions` com colunas de cartão e fonte única do resumo mensal
- **Data:** 2026-07-04
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** `prd.md` (RF-12..RF-16a, RF-22/23), `techspec.md`,
  `adr-002-card-purchase-cutover-drop.md`, `.claude/rules/transactions-workflows.md` (R-TXN-003/004)

## Contexto

Hoje transação avulsa vive em `transactions` e compra no crédito em `transactions_card_purchases`;
`card_invoice_items.purchase_id` referencia a segunda. O resumo mensal soma dois caminhos
(`TransactionRepository.SumByMonth` + `CardInvoiceRepository.SumByMonth`). Ao unificar, uma transação
`credit_card` passaria a existir em `transactions` **e** ter itens em `card_invoice_items` — se ambos
os caminhos somarem, há **double-counting**. Além disso, o item precisa referenciar `transactions(id)`.
A listagem por `ref_month` e `ListEntries` (UNION ALL) também misturaria o total da transação
credit_card com suas parcelas.

## Decisão

1. **Uma tabela `transactions`** ganha colunas nullable `card_id`, `installments_total`,
   `card_closing_day`, `card_due_day`, com CHECK de completude quando `payment_method=7` e FK
   `card_id→cards(id)`. A entidade `Transaction` absorve os campos (option-typed).
2. **`card_invoice_items` re-aponta para `transactions`**: `purchase_id → transaction_id`, UNIQUE
   `(transaction_id, installment_index)`, FK `→ transactions(id)`.
3. **`DROP TABLE transactions_card_purchases`** (ADR-002; ledger vazio, sem backfill).
   `transactions_card_invoices` permanece intacta (chave natural `(user_id, card_id, ref_month)`).
4. **Fonte única do resumo mensal**: a transação `credit_card` é **excluída** do ramo `transactions`
   em `SumByMonth` (novo `SumByMonthExcludingCredit`, `WHERE payment_method <> 7`) e em `ListEntries`;
   o valor de crédito entra **apenas** via `CardInvoiceRepository.SumByMonth` (soma de
   `card_invoice_items`). Assim cada real é contado uma única vez.
5. **Leitura de estado por query direta**: novo `GetItemsByTransactionID` substitui a re-derivação de
   refMonths por `BillingCycleResolver` na leitura (elimina fragilidade quando o cartão muda de
   fechamento). O plano de parcelas usa o `billingSnapshot` congelado na criação.

Migration `000003` incremental e idempotente (`IF EXISTS`/`IF NOT EXISTS`), com `down` que recria a
estrutura anterior vazia.

## Alternativas Consideradas

- **Manter tabelas separadas + dual-dispatch (Opção B).** Vantagem: sem migração de schema.
  Desvantagem: use case despacha leitura/escrita entre duas tabelas; listagem exige UNION permanente;
  double-counting continua latente. Rejeitada por complexidade e risco de correção.
- **Guardar `amount_cents=0` para credit_card em `transactions`.** Evitaria double-count mantendo a
  linha no SumByMonth. Desvantagem: amount=0 quebra invariante `amount_cents > 0` e distorce
  relatórios por transação. Rejeitada — exclusão por `payment_method` é mais limpa.

## Consequências

### Benefícios Esperados
- Uma fonte de verdade de escrita e de item de fatura; FK consistente.
- Double-counting eliminado de forma verificável.
- Reuso de `UpsertByMonth`/`ApplyDelta` sem mudança (operam por `(user,card,ref_month)`).

### Trade-offs e Custos
- Migração de schema com CHECK/FK novas; risco de migração (mitigado por ledger vazio + idempotência).
- `SumByMonth` do ramo transactions passa a ter cláusula de exclusão — precisa ser aplicada em
  TODOS os pontos de agregação (reconciler, ListEntries) sob pena de regressão.

### Riscos e Mitigações
- Risco: esquecer a exclusão em algum agregador → double-count. Mitigação: teste de integração
  dedicado + revisão dos pontos `SumByMonth`/`ListEntries`.
- Risco: OCC de fatura em N meses numa operação. Mitigação: escrita serializada por usuário; conflito
  → rollback total + 409 (retry automático fora de escopo, documentado).
- Rollback: migration `down` + revert do PR.

## Plano de Implementação
1. Escrever `000003_unify_transactions.up.sql` / `.down.sql` (idempotentes).
2. Adaptar `Transaction` entity + reconstituição + repos (`GetItemsByTransactionID`, `ReplaceItems`,
   `SumByMonthExcludingCredit`, `ExistsActiveCreditByCard`).
3. Ajustar `ListEntries` e `ReconcileMonthlySummary` para a fonte única.
4. Testes de integração (create/update/delete parcelado, resumo sem double-count, migration up/down).

## Monitoramento e Validação
- Integração: soma do resumo mensal = soma das faturas (sem duplicidade) para credit_card.
- Métricas com labels de baixa cardinalidade (R-TXN-004); sem `card_id`/`user_id`.
- Critério de sucesso: total do mês bate com soma independente das faturas em cenário parcelado.

## Impacto em Documentação e Operação
- Atualizar `docs/runbooks/transactions.md`, dashboard `transactions-overview.json` e
  `docs/alerts/transactions.yaml` para a fonte única e remoção de card-purchase.

## Revisão Futura
- Revisitar se o produto introduzir estorno/transferência (novos eventos que afetam fatura) ou
  múltiplos usuários com volume que exija sharding do resumo mensal.

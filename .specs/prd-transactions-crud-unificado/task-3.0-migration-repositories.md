# Tarefa 3.0: Migration 000003 + repositórios unificados

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Executar o corte de schema do ADR-003 e alinhar os repositórios Postgres à tabela única
`transactions`. A migration incremental `000003_unify_transactions` adiciona as colunas de cartão à
`transactions`, re-aponta `card_invoice_items` para `transactions(id)`, remove o vínculo residual em
`recurring_materializations` e dropa `transactions_card_purchases` — sem backfill, pois a produção
tem ledger vazio (RF-24a). Em paralelo, os repositórios ganham a leitura de itens por transação, a
soma mensal que exclui crédito (fonte única) e a checagem de crédito ativo por cartão, todos operando
sobre a nova estrutura.

Esta tarefa é infraestrutura de persistência: não altera `Decide*`, use cases ou handlers. Toda a
regra de negócio permanece no domínio; aqui só há SQL de adapter e mapeamento de linha ↔ entidade.

<requirements>
- Migration `000003` incremental e idempotente (`IF EXISTS`/`IF NOT EXISTS`, `RENAME` guardado),
  aplicável sobre o baseline atual e reversível pelo `.down.sql`.
- Sem backfill de dados (RF-24a): produção com ledger vazio; drop limpo de `transactions_card_purchases`.
- `transactions_card_invoices` permanece intacta (chave natural `(user_id, card_id, ref_month)`).
- SQL de domínio exclusivamente no adapter Postgres; nenhuma regra de negócio, branching de domínio ou
  cálculo em repositório (herda R-ADAPTER-001.2 / R-TXN-003).
- Zero comentários em Go de produção (R-ADAPTER-001.1); `defer func() { _ = rows.Close() }()` obrigatório.
- Nenhum label de métrica com `user_id`/`card_id`/`category_id` (R-TXN-004).
</requirements>

## Subtarefas

- [ ] 3.1 Escrever `migrations/000003_unify_transactions.up.sql` idempotente:
  (a) `ALTER TABLE transactions ADD COLUMN IF NOT EXISTS card_id, installments_total,
  card_closing_day, card_due_day`; CHECK de completude quando `payment_method=7` (todas as 4 colunas
  presentes) e CHECK de que ficam nulas caso contrário; CHECK de ranges (`installments_total 1..24`,
  `card_closing_day 1..31`, `card_due_day 1..31`); FK `card_id → cards(id)`.
  (b) `card_invoice_items`: `RENAME COLUMN purchase_id → transaction_id`; UNIQUE
  `(transaction_id, installment_index)`; FK `→ transactions(id)`.
  (c) `transactions_recurring_materializations DROP COLUMN IF EXISTS materialized_purchase_id`.
  (d) `DROP TABLE IF EXISTS transactions_card_purchases`.
- [ ] 3.2 Escrever `migrations/000003_unify_transactions.down.sql` reconstruindo a estrutura anterior
  vazia (recria `transactions_card_purchases`, reverte o rename e a FK de `card_invoice_items`,
  recria `materialized_purchase_id`, remove colunas/constraints de cartão de `transactions`).
- [ ] 3.3 `TransactionRepository` (`transaction_repository.go`): scan/insert das novas colunas de
  cartão (nil-safe); novo `GetItemsByTransactionID`; mover `ReplaceItems` para cá com
  `ON CONFLICT (transaction_id, installment_index)`; novo `SumByMonthExcludingCredit`
  (`WHERE payment_method <> 7`); mover `ExistsActiveCreditByCard` (`payment_method = 7`).
- [ ] 3.4 `card_invoice_repository.go`: ajustar upsert/leitura de item para a coluna `transaction_id`
  (rename); `card_invoice_invoices`/`ApplyDelta` OCC permanecem por `(user, card, ref_month)`.
- [ ] 3.5 `monthly_summary_repository.go`: `ListEntries` filtra `payment_method <> 7` no ramo
  `transactions` para não double-contar crédito (crédito soma só via `card_invoice_items`).
- [ ] 3.6 `recurring_materialization_repository.go`: remover leitura/escrita de
  `materialized_purchase_id`.
- [ ] 3.7 Atualizar contratos em `application/interfaces/{transaction_repository.go,
  card_invoice_repository.go}` conforme as assinaturas da techspec (novo/movido).

## Detalhes de Implementação

Ver `techspec.md` seções "Migration", "Interfaces Chave" (assinatura do `TransactionRepository`) e
"Modelos de Dados" (regra de fonte única do resumo mensal). O SQL completo e idempotente e a semântica
de cada passo estão em `adr-003-unified-transactions-schema.md` (decisões 1–5) e o corte de
`transactions_card_purchases` em `adr-002-card-purchase-cutover-drop.md`. DDL de referência das tabelas
atuais: `migrations/000001_initial_schema.up.sql` (linhas ~793–933). Não duplicar o SQL do ADR aqui —
referenciá-lo.

Fonte única do resumo mensal (ADR-003): transação `credit_card` (`payment_method = 7`) fica **fora** de
`SumByMonthExcludingCredit` e do ramo `transactions` de `ListEntries`; o valor de crédito entra apenas
via `CardInvoiceRepository.SumByMonth` (soma de `card_invoice_items`), garantindo contagem única.

## Critérios de Sucesso

- `migrate up` e `migrate down` idempotentes em banco limpo (reexecução não falha).
- CHECKs de completude/ranges e FK `card_id → cards(id)` criados; UNIQUE e FK de
  `card_invoice_items(transaction_id, ...)` criados.
- `transactions_card_purchases` inexistente após `up`; recriada vazia após `down`.
- `card_invoice_items.transaction_id` existente pós-`up`; `materialized_purchase_id` ausente.
- Nenhum SQL de domínio fora do adapter Postgres; producers apenas mapeiam evento (R-TXN-003).
- Zero comentários em Go; `defer func() { _ = rows.Close() }()` em toda iteração de `rows`.
- Resumo mensal não double-conta crédito (crédito somado só via fatura).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — mapeamento de linha ↔ entidade das novas colunas de cartão (nil-safe) e
  construção de query onde a lógica for testável sem banco.
- [ ] Testes de integração (testcontainers-go, `//go:build integration`):
  - migration `up`/`down` em banco limpo: colunas, CHECKs, FK e UNIQUE criados; `transactions_card_purchases`
    inexistente pós-`up` e recriada vazia pós-`down`.
  - upsert de fatura + `ApplyDelta` com OCC sobre `card_invoice_items(transaction_id)`.
  - `GetItemsByTransactionID` retorna os itens vinculados a uma transação.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `migrations/000003_unify_transactions.up.sql` — novo (ADR-003 decisões 1–5).
- `migrations/000003_unify_transactions.down.sql` — novo (reconstrução vazia).
- `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go` — scan/insert
  de colunas de cartão; `GetItemsByTransactionID`; `ReplaceItems` (movido); `SumByMonthExcludingCredit`;
  `ExistsActiveCreditByCard` (movido).
- `internal/transactions/infrastructure/repositories/postgres/card_invoice_repository.go` — coluna
  `transaction_id` no upsert/leitura de item; OCC `ApplyDelta` intacto.
- `internal/transactions/infrastructure/repositories/postgres/monthly_summary_repository.go` —
  `ListEntries` filtrado (`payment_method <> 7` no ramo transactions).
- `internal/transactions/infrastructure/repositories/postgres/recurring_materialization_repository.go` —
  remover `materialized_purchase_id`.
- `internal/transactions/application/interfaces/transaction_repository.go` — novas/movidas assinaturas.
- `internal/transactions/application/interfaces/card_invoice_repository.go` — ajuste de contrato.
- `migrations/000001_initial_schema.up.sql` — referência de DDL (linhas ~793–933).
- Referências de decisão: `techspec.md`, `adr-003-unified-transactions-schema.md`,
  `adr-002-card-purchase-cutover-drop.md`.

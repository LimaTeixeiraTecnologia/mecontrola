# Tarefa 3.0: Tools de leitura (11) sobre budgets/card/transactions

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar 11 tools de leitura em `internal/agents/application/tools/`, uma por arquivo, cada uma como
um `tool.NewTool[I,O]` fino sobre o substrato `internal/platform`. Cada tool declara schema Strict,
tem `exec` determinístico que parseia `userId`/UUID, aplica defaults de campos opcionais (ex.:
`refMonth` corrente em `America/Sao_Paulo`, `limit` default), chama o binding correspondente e mapeia
o retorno para o output tipado. Zero comentários, zero regra de negócio, sem SQL nem branching de
domínio. As tools ainda **não** são registradas no agente — o registro em `buildFinancialTools` é
escopo da tarefa 6.0.

Depende da tarefa 2.0 (interfaces agent-owned + binding adapters + wiring dos use cases).
Paralelizável com as tarefas 4.0 e 5.0.

<requirements>
- RF-09 — `list_cards` (via `CardManager.ListCards`)
- RF-10 — `get_card` (via `CardManager.GetCard`)
- RF-11 — `query_card_invoice` (via `TransactionsLedger.GetCardInvoice`)
- RF-12 — `best_purchase_day` (via `CardManager.BestPurchaseDay`, entra `bank` + `dueDay`)
- RF-13 — `search_transactions` (via `TransactionsLedger.SearchTransactions`)
- RF-14 — `list_recurrences` (via `RecurrenceManager.ListRecurrences`)
- RF-18a — `get_transaction` (via `TransactionsLedger.GetTransaction`) e `get_card_purchase` (via `TransactionsLedger.GetCardPurchase`) — tools DISTINTAS
- RF-18b — `list_card_purchases` (via `TransactionsLedger.ListCardPurchases`)
- RF-18c — `count_cards` (via `CardManager.CountCards`)
- RF-18d — `suggest_allocation` (via `BudgetPlanner.SuggestAllocation`)
- RF-19 — `get_transaction` e `get_card_purchase` são tools distintas, cada uma delegando a um único use case de leitura, sem branching de domínio
</requirements>

## Subtarefas

- [ ] 3.1 `list_cards.go` — `tool.NewTool[I,O]` delegando a `CardManager.ListCards` (input `userId`)
- [ ] 3.2 `get_card.go` — delega a `CardManager.GetCard` (input `userId` + `cardId`)
- [ ] 3.3 `query_card_invoice.go` — delega a `TransactionsLedger.GetCardInvoice` (input `userId` + `cardId`, default `refMonth` corrente America/Sao_Paulo)
- [ ] 3.4 `best_purchase_day.go` — delega a `CardManager.BestPurchaseDay` (input `bank` + `dueDay`)
- [ ] 3.5 `search_transactions.go` — delega a `TransactionsLedger.SearchTransactions` (input `userId` + `query`, default `refMonth` e `limit`)
- [ ] 3.6 `list_recurrences.go` — delega a `RecurrenceManager.ListRecurrences` (input `activeOnly`/`cursor`/`limit` com defaults)
- [ ] 3.7 `get_transaction.go` — delega a `TransactionsLedger.GetTransaction` (input `txId`)
- [ ] 3.8 `get_card_purchase.go` — delega a `TransactionsLedger.GetCardPurchase` (input `purchaseId`)
- [ ] 3.9 `list_card_purchases.go` — delega a `TransactionsLedger.ListCardPurchases` (input `userId` + `cardId`, default `refMonth`/`cursor`/`limit`)
- [ ] 3.10 `count_cards.go` — delega a `CardManager.CountCards` (input `userId`)
- [ ] 3.11 `suggest_allocation.go` — delega a `BudgetPlanner.SuggestAllocation` (input `totalCents` + `allocations`)

## Detalhes de Implementação

Ver `techspec.md` desta pasta: seção **Tools novas (15) — mapeamento tool → capacidade real** (tabela
tool→binding→gate, com as 11 leituras); seção **Design de Implementação → Interfaces Chave** para as
assinaturas dos métodos consumidos; seção **Sequenciamento de Desenvolvimento → Ordem de Build**
(passo 3, "Tools de leitura (11)"); seção **Conformidade com Padrões** (R-ADAPTER-001, R-AGENT-WF-001).
Cada tool segue o molde `tool.NewTool[I,O]` do consumidor de referência `internal/agents`, com schema
Strict e `exec` fino que apenas parseia input, aplica defaults, invoca o binding e mapeia o retorno
com wrapping de erro (`fmt.Errorf("...: %w", err)`).

## Critérios de Sucesso

- Cada uma das 11 tools compila e é um `tool.NewTool[I,O]` fino, sem regra de negócio, sem SQL, sem branching de domínio, zero comentários.
- Cada tool possui teste unitário cobrindo: sucesso (mapeamento correto args→binding e retorno→output), erro do use case com wrapping propagado, UUID/`userId` inválido, e defaults de campos opcionais.
- Nenhuma tool inventa dado: o output contém exclusivamente o que o use case/binding retorna.
- `get_transaction` e `get_card_purchase` permanecem tools distintas, cada uma delegando a um único use case (RF-19).
- As 11 tools **não** estão registradas no agente (registro é escopo da tarefa 6.0).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — cada tool é um tool.NewTool[I,O] fino sobre o substrato internal/platform, seguindo o molde internal/agents.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Testes unitários por tool com o binding mockado (mockery), cobrindo sucesso, erro do use case
(wrapping propagado), UUID inválido e defaults de campos opcionais. Testes de integração: N/A para
esta tarefa (tools de leitura sem gate nem escrita).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/tools/list_cards.go`
- `internal/agents/application/tools/get_card.go`
- `internal/agents/application/tools/query_card_invoice.go`
- `internal/agents/application/tools/best_purchase_day.go`
- `internal/agents/application/tools/search_transactions.go`
- `internal/agents/application/tools/list_recurrences.go`
- `internal/agents/application/tools/get_transaction.go`
- `internal/agents/application/tools/get_card_purchase.go`
- `internal/agents/application/tools/list_card_purchases.go`
- `internal/agents/application/tools/count_cards.go`
- `internal/agents/application/tools/suggest_allocation.go`
- Testes `*_test.go` correspondentes a cada tool acima.

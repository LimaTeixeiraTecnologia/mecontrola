# Tarefa 4.0: Use cases unificados create/update/delete

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Unificar `CreateTransaction`, `UpdateTransaction` e `DeleteTransaction` para hospedar o
fluxo `credit_card` absorvido de `card-purchase`. O use case orquestra (ADR-001): quando
`payment_method=credit_card`, faz o IO sancionado `cardLookup.GetForUser` **antes** do
`Decide*` puro e, no **mesmo** `uow.Do`, persiste a transação, faz upsert das faturas por
mês, aplica os deltas com OCC, substitui os itens e publica o evento no outbox. Para os
demais meios, permanece lançamento simples. A regra de negócio (split, refMonths, deltas)
vive 100% em `Decide*`; o use case não contém branch de regra de domínio.

Depende das Tarefas 1.0 (VOs/commands), 2.0 (`TransactionWorkflow` + `DecideDelete`) e 3.0
(migration + repos `GetItemsByTransactionID`/`ReplaceItems`/`SumByMonthExcludingCredit`/
`ExistsActiveCreditByCard`) já concluídas.

<requirements>
- Injetar em `Create/Update/DeleteTransaction`: `cardLookup interfaces.CardLookup`,
  `cardInvoiceRepo` (via factory) e o publisher já existente.
- `credit_card`: `cardLookup.GetForUser(ctx, cardID, userID)` executa ANTES do `Decide*`
  (ADR-001 = orquestração de fronteira, não regra no use case); passar
  `option.Some(snapshot)` para o `Decide*`. Demais meios: `option.None`.
- Dentro do MESMO `uow.Do`: `repo.Create`/`UpdateWithVersion`/`SoftDelete` +
  `invoiceRepo.UpsertByMonth` + `ApplyDelta` (OCC) + `repo.ReplaceItems` +
  `PublishCreated/Updated/Deleted` (outbox, `EventID` = PK). Falha em qualquer passo →
  rollback total, nenhuma fatura alterada.
- Guard RF-19 (`outcome ⇒ subcategory` obrigatória) presente em `Create` **E** `Update`
  (hoje falta no update — corrigir).
- `DeleteTransaction` passa a usar `workflow.DecideDelete` (não montar `TransactionDeleted`
  inline como hoje).
- Fonte única do resumo (ADR-003): transação `credit_card` fora do `SumByMonth` do ramo
  `transactions` — os agregadores usam `SumByMonthExcludingCredit`; evitar double-counting.
- Corrigir swallow silencioso na leitura dos itens atuais: usar
  `repo.GetItemsByTransactionID(ctx, txID)` direto (sem re-derivação por resolver que
  engole erro), passando `currentItems` para `DecideUpdate`/`DecideDelete`.
- Span aberto antes do `Validate()` (R-DTO-002); zero comentários Go (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 4.1 `CreateTransaction`: injetar `cardLookup` + `cardInvoiceRepo`; IO condicional
  `GetForUser` antes do `DecideCreate(cmd, snap, txID, eventID, now)`; no `uow.Do` estender
  para upsert de fatura + `ApplyDelta` + `ReplaceItems` quando `decision.Items != nil`,
  preservando o path `ON CONFLICT origin DO NOTHING` (reconciliação idempotente).
- [ ] 4.2 `UpdateTransaction`: adicionar guard RF-19 (`outcome ⇒ subcategory`); ler
  `currentItems` via `GetItemsByTransactionID`; IO condicional `GetForUser`;
  `DecideUpdate(current, currentItems, cmd, eventID, now)`; reaplicar deltas em todas as
  faturas afetadas + `ReplaceItems` atômico no mesmo UoW (RF-16).
- [ ] 4.3 `DeleteTransaction`: ler `currentItems` via `GetItemsByTransactionID`; trocar o
  evento inline por `DecideDelete(current, currentItems, eventID, now)`; reverter deltas de
  todas as parcelas/faturas no mesmo UoW (RF-16a); publicar `decision.Event`.
- [ ] 4.4 Garantir que os pontos de agregação de resumo usem `SumByMonthExcludingCredit`
  (fonte única — ADR-003), sem introduzir double-counting no ramo `transactions`.

## Detalhes de Implementação

Ver `techspec.md` — "Fluxo de Dados (create credit_card unificado)", "Interfaces Chave"
(`TransactionDecision{Transaction, Items, InvoiceDeltas, Event}`; assinaturas
`DecideCreate/DecideUpdate/DecideDelete`), "Modelos de Dados" (regra de fonte única do
resumo) e "Validação (fronteira → smart constructor → use case)" (linhas RF-19/RF-20/RF-21).
ADR-001 fixa o despacho por orquestração (IO antes do `Decide*`, `option.Option[CardBillingSnapshot]`
como parâmetro, sem `switch`/regra no use case). ADR-003 fixa a exclusão de `credit_card`
do `SumByMonth` do ramo transactions e a leitura de itens via `GetItemsByTransactionID`.

Não reescrever regra pura: `InstallmentSplitter`/`BillingCycleResolver`/`Decide*` são
reusados via Tarefa 2.0. Absorver o comportamento hoje em `create_card_purchase.go`/
`update_card_purchase.go`/`delete_card_purchase.go` (a remover na Tarefa 6.0); esta tarefa
não apaga esses arquivos, apenas migra a orquestração para os use cases unificados.

## Critérios de Sucesso

- Tudo atômico no `uow.Do`: falha em qualquer passo (create/update/soft-delete, upsert,
  `ApplyDelta`, `ReplaceItems`, publish) → rollback total; nenhuma fatura alterada.
- Idempotência preservada: `ON CONFLICT origin DO NOTHING` no create + `idempotency.Middleware`;
  outbox por `EventID` (PK) mantém exactly-once.
- `credit_card` só com `card_id` + `direction=outcome` (garantido no smart ctor da Tarefa 1.0);
  IO `GetForUser` só ocorre para `credit_card`.
- Zero branch de REGRA de domínio no use case — só orquestração/IO (gate R-TXN-001 verde:
  grep de `amount_cents`/`direction`/`installments`/`payment_method` fora de `Decide*`/`command`/`input.` vazio).
- Span aberto antes do `Validate()` (R-DTO-002); zero comentários Go de produção.
- Guard RF-19 presente em create E update; `DeleteTransaction` sem evento montado inline.
- Resumo mensal sem double-counting: agregadores chamam `SumByMonthExcludingCredit` (ADR-003).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (testify/suite whitebox, `fake.NewProvider`, IIFE por mock — R-TESTING-001):
  - `CreateTransaction`: `outcome` sem subcategory → erro (RF-19).
  - `UpdateTransaction`: `outcome` sem subcategory → erro (RF-19, hoje ausente — regressão).
  - `credit_card` cria itens de fatura + upsert/`ApplyDelta`/`ReplaceItems` chamados; `GetForUser`
    invocado só no path credit_card; não-cartão não chama `cardLookup` nem `invoiceRepo`.
  - `DeleteTransaction`: usa `DecideDelete`; leitura de itens via `GetItemsByTransactionID`;
    reverte deltas.
  - Agregadores usam `SumByMonthExcludingCredit` (mock verifica chamada; sem double-count).
- [ ] Testes de integração: coberta na Tarefa 8.0 (create/update/delete parcelado com Postgres +
  outbox + OCC, rollback total em falha, resumo sem double-count) — referenciar, não duplicar aqui.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/transactions/application/usecases/create_transaction.go` — injeção de `cardLookup`/
  `cardInvoiceRepo`; IO condicional + upsert/`ApplyDelta`/`ReplaceItems` no UoW.
- `internal/transactions/application/usecases/update_transaction.go` — guard RF-19; `currentItems`
  via `GetItemsByTransactionID`; recomposição atômica de deltas (RF-16).
- `internal/transactions/application/usecases/delete_transaction.go` — `DecideDelete`; reversão de
  deltas (RF-16a); sem evento inline.
- `internal/transactions/application/usecases/create_card_purchase.go` — referência a absorver
  (orquestração de fatura/parcela); removido na Tarefa 6.0.
- `internal/transactions/application/usecases/reconcile_monthly_summary.go` — ponto de agregação a
  apontar para `SumByMonthExcludingCredit` (fonte única — ADR-003).
- `internal/transactions/application/usecases/errors.go` — `ErrOutcomeTransactionRequiresSubcategory`,
  `ErrUsecaseUnauthorized` reusados.
- `internal/transactions/application/usecases/helpers.go` — `toCommandRawCreate/Update`,
  `optSubcategoryUUID`, `snapSubName`.
- `internal/transactions/application/interfaces/card_lookup.go` — `CardLookup.GetForUser`.
- `internal/transactions/application/interfaces/card_invoice_repository.go` — `UpsertByMonth`/
  `ApplyDelta`/`SumByMonth`.
- `internal/transactions/application/interfaces/transaction_event_publisher.go` —
  `PublishCreated/Updated/Deleted`.
- `internal/transactions/application/interfaces/category_validator.go` — `Validate` (raiz/filha/kind).
- `internal/transactions/application/usecases/{create,update,delete}_transaction_test.go` — suites
  a estender com os cenários acima.
- `.specs/prd-transactions-crud-unificado/techspec.md`, `adr-001-payment-method-dispatch.md`,
  `adr-003-unified-transactions-schema.md` — fonte de contrato.

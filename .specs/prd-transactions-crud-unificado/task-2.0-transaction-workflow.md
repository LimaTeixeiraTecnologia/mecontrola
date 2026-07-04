# Tarefa 2.0: TransactionWorkflow enriquecido + DecideDelete + entity/eventos

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Enriquecer o domínio puro de `internal/transactions` para absorver a lógica de cartão de crédito
no fluxo de transação, sem reescrever a lógica pura já existente. `TransactionWorkflow.DecideCreate`
e `DecideUpdate` passam a receber `option.Option[valueobjects.CardBillingSnapshot]`; quando presente
(`payment_method=credit_card`), compõem os `Decide*` puros já existentes (`InstallmentSplitter` +
`BillingCycleResolver` do `CardPurchaseWorkflow`) e emitem `[]CardInvoiceItem` + `InvoiceDeltas`.
Adiciona-se `DecideDelete` puro (hoje o delete monta o evento fora de `Decide*` — corrigido). A
entidade `Transaction` e os eventos `TransactionCreated/Updated/Deleted` ganham os campos de cartão.

Escopo estritamente de domínio puro (sem IO, sem `context.Context`, sem `time.Now` — recebe `now`),
conforme R-TXN-001. O wiring nos use cases, o IO de `CardLookup`/fatura e a migration pertencem a
outras tarefas — esta tarefa NÃO os toca.

<requirements>
- `DecideCreate`/`DecideUpdate` recebem `option.Option[valueobjects.CardBillingSnapshot]`; ramo
  credit_card compõe `InstallmentSplitter` + `BillingCycleResolver` REUSANDO o código existente
  (`CardPurchaseWorkflow`), sem reescrever a lógica pura.
- Ramo não-cartão (`option.None`) produz apenas a `Transaction` simples: `Items` e `InvoiceDeltas`
  nil, sem fatura e sem parcela (RF-10).
- Novo `DecideDelete` puro que reverte os deltas de todas as parcelas em todas as faturas afetadas
  (RF-16a) e emite `TransactionDeleted` — substitui a montagem de evento hoje feita fora de `Decide*`.
- `TransactionDecision` ganha `Items []entities.CardInvoiceItem` e `InvoiceDeltas map[string]int64`
  (nil quando não-cartão).
- `entities.Transaction` ganha campos option `cardID`, `installmentsTotal`, `billingSnapshot`
  (congelado na criação) + suporte a reconstituição.
- `entities.TransactionCreated/Updated/Deleted` ganham `RefMonthsAffected []RefMonth`; `Created`
  ganha `Installments []CardPurchaseInstallment` (nil para não-cartão).
- `Decide*` puros, determinísticos: sem `context.Context`, sem IO, sem repositório, sem `time.Now`
  (recebe `now time.Time`), sem geração aleatória de IDs (recebe `txID`/`eventID`). R-TXN-001.
- Split soma exatamente igual ao total (0 divergência de centavos); refMonths por parcela via
  `BillingCycleResolver` (RF-13, RF-15).
- Zero comentários em Go de produção (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 2.1 `domain/entities/transaction.go`: adicionar campos option `cardID`,
  `installmentsTotal`, `billingSnapshot` + acessores e reconstituição (`billingSnapshot` congelado
  na criação, só populado quando credit_card).
- [ ] 2.2 `domain/entities/events.go`: adicionar `RefMonthsAffected []RefMonth` em
  `TransactionCreated/Updated/Deleted`; adicionar `Installments []CardPurchaseInstallment` em
  `TransactionCreated` (nil para não-cartão).
- [ ] 2.3 `domain/services/transaction_workflow.go`: `TransactionDecision` ganha
  `Items []entities.CardInvoiceItem` e `InvoiceDeltas map[string]int64`.
- [ ] 2.4 `DecideCreate` recebe `snap option.Option[valueobjects.CardBillingSnapshot]`; ramo
  credit_card compõe splitter+resolver (reuso) e emite items + deltas + `Installments`; ramo
  não-cartão inalterado (nil).
- [ ] 2.5 `DecideUpdate` recebe `currentItems []entities.CardInvoiceItem` + `snap`; recompõe as
  parcelas e reaplica os deltas em todas as faturas afetadas (RF-16).
- [ ] 2.6 `DecideDelete` puro: reverte os deltas de todas as parcelas/faturas (RF-16a) e emite
  `TransactionDeleted` com `RefMonthsAffected`.
- [ ] 2.7 Testes unitários puros SEM mock cobrindo todos os ramos.

## Detalhes de Implementação

Ver `techspec.md` — seções "Arquitetura do Sistema › Visão Geral dos Componentes" (Domínio e
Value Objects), "Design de Implementação › Interfaces Chave" (assinaturas de `DecideCreate`/
`DecideUpdate`/`DecideDelete` e `TransactionDecision`) e "Modelos de Dados" (campos option da
entidade). Ver `adr-001-payment-method-dispatch.md` (despacho por `option.Option[CardBillingSnapshot]`,
regra 100% em `Decide*`, reuso de `InstallmentSplitter`/`BillingCycleResolver` sem reescrita) e
`adr-003-unified-transactions-schema.md` (fonte única, `billingSnapshot` congelado, refMonths via
snapshot da criação).

Assinaturas-alvo (conforme techspec):

```go
func (w TransactionWorkflow) DecideCreate(cmd commands.CreateTransaction,
    snap option.Option[valueobjects.CardBillingSnapshot], txID, eventID uuid.UUID, now time.Time) TransactionDecision
func (w TransactionWorkflow) DecideUpdate(current entities.Transaction,
    currentItems []entities.CardInvoiceItem, cmd commands.UpdateTransaction, eventID uuid.UUID, now time.Time) TransactionDecision
func (w TransactionWorkflow) DecideDelete(current entities.Transaction,
    currentItems []entities.CardInvoiceItem, eventID uuid.UUID, now time.Time) (TransactionDecision, error)
```

Restrições invioláveis: a lógica pura de parcelamento (`InstallmentSplitter`) e de ciclo de fatura
(`BillingCycleResolver`) do `card_purchase_workflow.go` é REUSADA, não reescrita. RF cobertos:
RF-10, RF-11, RF-12, RF-15, RF-16, RF-16a.

## Critérios de Sucesso

- `DecideCreate`/`DecideUpdate`/`DecideDelete` puros e determinísticos: sem `context.Context`, sem
  IO, sem repositório, sem `time.Now` (recebem `now`), sem geração aleatória de ID.
- Ramo credit_card: soma das parcelas exatamente igual ao total (0 divergência de centavos);
  `RefMonth` por parcela distribuído via `BillingCycleResolver`; `Items` + `InvoiceDeltas`
  populados; `TransactionCreated.Installments` preenchido.
- Ramo não-cartão: `Items` e `InvoiceDeltas` nil; sem fatura e sem parcela.
- `DecideUpdate` recompõe deltas em todas as faturas afetadas; `DecideDelete` reverte todos os
  deltas sem deixar saldo residual.
- Gate R-TXN-001 verde (regra de domínio fora de `Decide*` bloqueia PR).
- Zero comentários nos `.go` de produção tocados.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários puros SEM mock (padrão R-TESTING-001, testify/suite whitebox,
  `fake.NewProvider`): income; outcome simples; credit_card 1x (à vista); credit_card 12x com
  soma das parcelas exatamente = total e refMonths distribuídos; update que recompõe deltas;
  delete que reverte deltas.
- [ ] Testes de integração — não nesta tarefa.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/transactions/domain/services/transaction_workflow.go` — `DecideCreate`/`DecideUpdate`
  enriquecidos + novo `DecideDelete`; `TransactionDecision` com `Items`/`InvoiceDeltas`.
- `internal/transactions/domain/services/card_purchase_workflow.go` — fonte da composição
  splitter+resolver a REUSAR (não reescrever).
- `internal/transactions/domain/services/installment_splitter.go` — split determinístico soma=total.
- `internal/transactions/domain/services/billing_cycle_resolver.go` — refMonth por parcela.
- `internal/transactions/domain/entities/transaction.go` — campos option `cardID`,
  `installmentsTotal`, `billingSnapshot` + reconstituição.
- `internal/transactions/domain/entities/events.go` — `RefMonthsAffected` nos 3 eventos;
  `Installments` no `TransactionCreated`.
- `internal/transactions/domain/entities/card_invoice_item.go` — tipo `CardInvoiceItem` retornado
  na decisão.
- `internal/transactions/domain/valueobjects/card_billing_snapshot.go` — `CardBillingSnapshot`
  recebido via `option.Option`.
- `internal/transactions/domain/services/transaction_workflow_test.go` — testes unitários puros.

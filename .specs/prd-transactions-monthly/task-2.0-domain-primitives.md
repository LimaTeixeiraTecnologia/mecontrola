# Tarefa 2.0: Domain primitives — VOs, option, commands, entities, events

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Cria todas as primitivas de domínio puro (sem IO): tipo genérico `Option[T]`, value objects com smart constructors agressivos, pacote `domain/commands/` exportado (audit fix #1 — resolve import cycle), agregados em `domain/entities/`, e 9 domain events tipados em `entities/events.go`. Base inegociável da camada DMMF seletiva (ADR-006 §1, §3, §4).

<requirements>
- `domain/option/option.go` com `Option[T any]` (`Some`, `None`, `Get() (T, bool)`, `IsPresent()`). ≤ 30 linhas.
- VOs com construtores `New*` que validam invariantes e retornam sentinel errors (`ErrXxx`): `Money` (>0), `InstallmentCount` (1..24), `DayOfMonth` (1..28 para template; 1..31 para snapshot de cartão), `Description` (não-vazia, ≤ N chars), `RefMonth` (parse `YYYY-MM` em fuso `America/Sao_Paulo`), `Direction` (`income`/`outcome` iota+1), `PaymentMethod` (8 valores iota+1; `doc=8` legado), `Frequency` (`monthly`/`yearly`), `CardBillingSnapshot` (closing+due com clamp implícito).
- `UserID`, `CardID`, `CategoryID`, `SubcategoryID` como tipos opacos sobre `uuid.UUID` com `Parse(s string) (T, error)`.
- `domain/commands/` (audit fix #1) com tipos exportados `CreateTransaction`, `UpdateTransaction`, `CreateCardPurchase`, `UpdateCardPurchase`, `CreateRecurringTemplate`, `UpdateRecurringTemplate`, `MaterializeRecurring` + smart constructor `NewXxx(raw RawXxx, principal auth.Principal) (Xxx, error)` que acumula erros com `errors.Join`. Vive no domínio para que `domain/services/` consuma sem import cycle.
- `domain/entities/transaction.go`, `card_purchase.go`, `card_invoice.go`, `card_invoice_item.go`, `recurring_template.go`, `monthly_summary.go` — agregados construídos exclusivamente via factories `New*` que recebem VOs já validados.
- `domain/entities/events.go` com 9 structs (`TransactionCreated/Updated/Deleted`, `CardPurchaseCreated/Updated/Deleted`, `RecurringTemplateCreated/Updated/Deleted`); cada um carrega `EventID uuid.UUID`, `AggregateID uuid.UUID`, `UserID uuid.UUID`, `OccurredAt time.Time` + campos específicos (`RefMonthsAffected []RefMonth`, `InvoiceDeltas map[RefMonth]int64` para `CardPurchaseUpdated`/`Deleted`).
- Tags `json:"..."` nos events para serialização posterior pelo producer.
- Zero IO, zero `context.Context`, zero `time.Now()` interno (R6.7 — instante chega por parâmetro nos eventos).
</requirements>

## Subtarefas

- [ ] 2.1 `domain/option/option.go` + teste 100% coverage.
- [ ] 2.2 12 VOs em `domain/valueobjects/*.go` cada um com sentinel `ErrXxx` + smart constructor + table-driven test.
- [ ] 2.3 7 arquivos em `domain/commands/*.go` com smart constructors usando `errors.Join` (R7.6).
- [ ] 2.4 6 agregados em `domain/entities/*.go` com factory `New*` recebendo VOs já tipados.
- [ ] 2.5 `domain/entities/events.go` com 9 domain events tipados.
- [ ] 2.6 Testes unitários table-driven (`testify/suite` por VO/agregado).

## Detalhes de Implementação

Referência: techspec "Visão Geral dos Componentes" (lista completa de VOs/commands/events); ADR-006 §1/§3/§4 (smart constructors, eventos tipados, Option). `BillingCycleResolver` e workflows ficam para Task 3.0.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/transactions/domain/...` passa com cobertura ≥ 95% no pacote `domain/valueobjects/` e `domain/option/`.
- Nenhum tipo de domínio pode ser instanciado sem passar pelo construtor (testar via API pública e via reflection se necessário).
- `golangci-lint run ./internal/transactions/domain/...` limpo.
- Zero comentários em `.go` de produção (R-ADAPTER-001.1).
- Smart constructor de `commands.CreateCardPurchase` acumula múltiplos erros em uma única chamada (não fail-fast).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários por VO (table-driven, mín. 3 casos: valid, invalid lower, invalid upper).
- [ ] Testes unitários por command construtor (acumulação de erros com `errors.Join`).
- [ ] Testes unitários de Option[T] (Some/None/Get).
- [ ] Sem integration tests (pacote puro).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/domain/option/option.go` (novo)
- `internal/transactions/domain/valueobjects/{money,installment_count,day_of_month,description,ref_month,direction,payment_method,frequency,card_billing_snapshot,user_id,card_id,category_id,subcategory_id}.go` (novos)
- `internal/transactions/domain/commands/{create,update}_{transaction,card_purchase,recurring_template}.go` + `materialize_recurring.go` (novos)
- `internal/transactions/domain/entities/{transaction,card_purchase,card_invoice,card_invoice_item,recurring_template,monthly_summary,events}.go` (novos)
- Testes correspondentes `*_test.go` para cada arquivo.

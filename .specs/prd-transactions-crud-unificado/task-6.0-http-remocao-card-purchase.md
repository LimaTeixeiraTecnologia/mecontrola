# Tarefa 6.0: HTTP unificado + remoção de card-purchase + OpenAPI

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a porta HTTP única de transações e apagar em definitivo a superfície `card-purchase`. Os DTOs
de entrada de transação passam a carregar `card_id` e `installments` (default 1), com `Validate()` de
superfície cobrindo os campos de crédito; o DTO de saída expõe os campos de cartão via `omitempty`. As
5 rotas `/api/v1/card-purchases`, seus 5 handlers, 5 use cases, DTOs e o publisher dedicado são
removidos (RF-24, ADR-002); o `TransactionsRouter` é limpo dos campos e parâmetros correlatos. O
contrato unificado é documentado no `openapi.yaml` com condicional `if payment_method==credit_card
then required [card_id]`, e o `error_mapper` passa a expor um código único de conflito de versão.

Esta tarefa é de camada de adapter e contrato: handlers permanecem finos (decode → usecase → encode),
sem SQL, regra de negócio ou branching de domínio. Toda a regra de crédito (fatura, parcela, split)
vive no domínio/use case já entregue nas tarefas 2.0/4.0; aqui só há validação de superfície,
mapeamento de DTO e remoção de código morto.

<requirements>
- `RawCreateTransaction`/`RawUpdateTransaction` ganham `card_id *uuid.UUID` e `installments int`
  (default 1); `Validate()` (errors.Join, nomeando o campo): `card_id` obrigatório quando
  `payment_method=credit_card`, `installments` no intervalo 1..24 quando `credit_card`, e a regra já
  existente `origin_wamid ⇒ origin_operation` preservada.
- NÃO duplicar semântica de enum (R-DTO-003): validade de `direction`/`payment_method` e o bloqueio de
  `doc` permanecem em `ParsePaymentMethodForCreate`/command; `credit_card ⇒ outcome` permanece no
  command/use case. O DTO valida apenas superfície.
- `Transaction` (output) ganha `card_id`, `installments_total`, `ref_months_affected`, `items`, todos
  `omitempty` (nil/vazio para não-crédito).
- Remover a superfície card-purchase completa: 5 rotas, 5 handlers, 5 use cases, DTOs de entrada e
  `output/card_purchase.go` (movendo `CardInvoiceItemOutput` antes), `CardPurchaseEventPublisher`;
  limpar campos e parâmetros de `TransactionsRouter`/`NewTransactionsRouter`.
- `openapi.yaml`: schema completo de `RawCreateTransaction` com `if/then` (OpenAPI 3.1); remover
  `RawCreateCardPurchase` e as 5 rotas `/card-purchases`.
- `error_mapper.go`: código único de conflito `version_conflict` (não `card_purchase_version_conflict`).
- Handlers finos, sem SQL/regra/branch de domínio (R-ADAPTER-001.2); zero comentários em Go
  (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 6.1 `input/raw_create_transaction.go`: adicionar `CardID *uuid.UUID` (`json:"card_id,omitempty"`)
  e `Installments int` (`json:"installments,omitempty"`, default 1 no consumo); estender `Validate()`
  para `card_id` obrigatório e `installments` 1..24 quando `payment_method` for `credit_card`
  (comparação de string bruta, sem parse de enum — R-DTO-003), preservando `origin_wamid ⇒
  origin_operation`; novos sentinels em `input/errors.go`.
- [ ] 6.2 `input/raw_update_transaction.go`: mesmos campos e regras de superfície de crédito,
  coerentes com o contrato de edição (`installments` opcional; default 1).
- [ ] 6.3 `output/transaction.go`: adicionar `CardID *uuid.UUID`, `InstallmentsTotal int`,
  `RefMonthsAffected []string` e `Items []CardInvoiceItemOutput`, todos `omitempty`; preencher em
  `TransactionFrom` a partir da entidade (nil/vazio quando não-crédito).
- [ ] 6.4 Mover `CardInvoiceItemOutput` para `output/card_invoice.go` (ou destino que sobrevive) e
  então remover `output/card_purchase.go`.
- [ ] 6.5 Remover os 5 handlers `*_card_purchase_handler.go` e limpar `error_mapper.go`: unificar o
  código de conflito de versão em `version_conflict`, removendo os ramos e sentinels exclusivos de
  card-purchase (`ErrCardPurchaseVersionConflict`, `ErrCardPurchaseConflict`, `ErrCardPurchaseNotFound`)
  que deixem de existir após a remoção dos use cases.
- [ ] 6.6 Remover a rota-grupo `/api/v1/card-purchases` e os 5 campos/5 parâmetros de card-purchase de
  `TransactionsRouter` e `NewTransactionsRouter` em `transactions_router.go`.
- [ ] 6.7 Remover os 5 use cases `*_card_purchase.go`, os DTOs `raw_create_card_purchase.go`/
  `raw_update_card_purchase.go` e o `CardPurchaseEventPublisher` (producer dedicado).
- [ ] 6.8 `openapi.yaml`: schema completo de `RawCreateTransaction` com `card_id`/`installments` e
  `if: {properties: {payment_method: {const: credit_card}}} then: {required: [card_id]}`; atualizar o
  schema de resposta `Transaction` com os campos novos `omitempty`; remover `RawCreateCardPurchase` e
  as 5 rotas `/card-purchases`.

## Detalhes de Implementação

Ver `techspec.md` seções "Endpoints de API" (contrato estendido de `POST/PATCH/GET /transactions`,
rotas removidas, condicional OpenAPI 3.1 `if/then`), "Validação (fronteira → smart constructor → use
case)" (a linha `Raw*.Validate()` cobre superfície: `card_id` p/ credit_card e `installments` 1..24 —
RF-03/RF-13/RF-11b) e "Infraestrutura (removido/modificado)" (lista de artefatos card-purchase a
apagar, incluindo mover `CardInvoiceItemOutput`). O corte da superfície e seu racional (ledger vazio,
breaking change) estão em `adr-002-card-purchase-cutover-drop.md` (decisão 1, escopo HTTP). Não
duplicar o contrato do ADR/techspec aqui — referenciá-lo.

Fronteira de responsabilidade (R-DTO-VALIDATE-001 + R-DTO-003): o `Validate()` do DTO valida apenas
superfície (presença de `card_id` e faixa de `installments` para o valor bruto `credit_card`); a
validade do enum `payment_method`, o bloqueio de `doc` e a regra `credit_card ⇒ outcome` permanecem no
`ParsePaymentMethodForCreate`/smart constructor/use case entregues nas tarefas 1.0/4.0. Não replicar
whitelists do use case no DTO (R-DTO-004).

Handlers de transação permanecem finos (herda R-ADAPTER-001.2): decode do `Raw*`, `Validate()`,
delegação ao use case, encode do output — sem SQL, sem branching de domínio.

## Critérios de Sucesso

- As 5 rotas `/api/v1/card-purchases` retornam 404 (rota inexistente) após o corte.
- Contrato unificado documentado no `openapi.yaml`: `RawCreateTransaction` com `card_id`/`installments`
  e condicional `if/then`; `RawCreateCardPurchase` e rotas `/card-purchases` ausentes.
- `Validate()` cobre `card_id` obrigatório e `installments` 1..24 para `credit_card`, com mensagem que
  nomeia o campo; sem duplicar semântica de enum (R-DTO-003).
- Output de transação expõe `card_id`, `installments_total`, `ref_months_affected`, `items`
  (`omitempty`) apenas para crédito.
- `error_mapper` expõe um único código `version_conflict`; nenhum resíduo de
  `card_purchase_version_conflict`.
- Nenhum handler com SQL, regra de negócio ou branching de domínio; zero comentários em Go de produção.
- Build compila sem referências pendentes aos artefatos card-purchase removidos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários:
  - `RawCreateTransaction.Validate()`/`RawUpdateTransaction.Validate()`: `credit_card` sem `card_id` →
    erro nomeando o campo; `credit_card` com `installments` 0/25 → erro; `credit_card` com `card_id` e
    `installments` 1..24 → ok; não-crédito sem `card_id` → ok; `origin_wamid` sem `origin_operation` →
    erro (regressão).
  - Handler de transação (decode → usecase mock): body válido roteia ao use case e encoda o output;
    erro de validação/conflito mapeado ao status/código HTTP correto (`version_conflict`).
- [ ] Testes de integração — cobertos na Tarefa 8.0 (e2e: 404 nas rotas `/card-purchases` removidas;
  jornada HTTP unificada com `credit_card`). Referenciar, não reimplementar aqui.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/transactions/infrastructure/http/server/transactions_router.go` — remover rota-grupo
  `/api/v1/card-purchases`, campos e parâmetros de card-purchase de `TransactionsRouter`/
  `NewTransactionsRouter`.
- `internal/transactions/infrastructure/http/server/handlers/{create,update,delete,get,list}_card_purchase_handler.go`
  — removidos.
- `internal/transactions/infrastructure/http/server/handlers/error_mapper.go` — código único
  `version_conflict`; remover ramos/sentinels de card-purchase.
- `internal/transactions/application/dtos/input/raw_create_transaction.go` — `card_id`, `installments`,
  `Validate()` estendido.
- `internal/transactions/application/dtos/input/raw_update_transaction.go` — idem para edição.
- `internal/transactions/application/dtos/input/{raw_create_card_purchase.go,raw_update_card_purchase.go}`
  — removidos.
- `internal/transactions/application/dtos/input/errors.go` — novos sentinels de superfície de crédito;
  remover sentinels órfãos de card-purchase.
- `internal/transactions/application/dtos/output/transaction.go` — campos `card_id`,
  `installments_total`, `ref_months_affected`, `items` (`omitempty`) e mapeamento em `TransactionFrom`.
- `internal/transactions/application/dtos/output/card_purchase.go` — removido (mover
  `CardInvoiceItemOutput` para `output/card_invoice.go` antes).
- `internal/transactions/application/usecases/*_card_purchase.go` (5) e o `CardPurchaseEventPublisher`
  (producer) — removidos.
- `internal/transactions/openapi.yaml` — schema `RawCreateTransaction` com `if/then`; `Transaction`
  atualizado; `RawCreateCardPurchase` e rotas `/card-purchases` removidos.
- `internal/transactions/module.go` — wiring ajustado à nova assinatura de `NewTransactionsRouter`
  (parte final validada na Tarefa 8.0).
- Referências de decisão: `techspec.md` ("Endpoints de API", "Validação", "Infraestrutura"),
  `adr-002-card-purchase-cutover-drop.md`.

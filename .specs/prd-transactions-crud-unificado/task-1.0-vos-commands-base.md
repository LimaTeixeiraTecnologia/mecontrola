# Tarefa 1.0: VOs e Commands base

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Preparar a camada de domínio pura da unificação: estender o Value Object `PaymentMethod` com os
dois novos meios de pagamento aderentes ao Brasil (`vale_refeicao`, `vale_alimentacao`) e enriquecer
os commands `CreateTransaction`/`UpdateTransaction` com os campos de cartão (`CardID`,
`Installments`) e os invariantes de crédito. É a base pura (sem IO, sem `context.Context`) sobre a
qual o `TransactionWorkflow` (Tarefa 2.0) e os use cases (Tarefa 4.0) serão construídos. Nenhuma
lógica de fatura, parcelamento ou persistência é implementada aqui — apenas tipos fechados e smart
constructors. Ver ADR-001 para o modelo de despacho por `PaymentMethod`.

<requirements>
- RF-03: `CreateTransaction`/`UpdateTransaction` devem carregar os campos base da transação e passar
  a comportar `CardID` e `Installments` quando aplicável.
- RF-05: `PaymentMethod` permanece enum fechado (state-as-type); parse rejeita valor fora do catálogo.
- RF-06: catálogo de criação = `pix`, `ted`, `debit_in_account`, `debit_card`, `cash`, `boleto`,
  `credit_card`, `vale_refeicao`, `vale_alimentacao`.
- RF-07: `doc` permanece legado somente-leitura — legível em `ParsePaymentMethod`/`PaymentMethodFromInt`,
  bloqueado em `ParsePaymentMethodForCreate`.
- RF-08: adicionar `PaymentMethodMealVoucher` (9, `vale_refeicao`) e `PaymentMethodFoodVoucher`
  (10, `vale_alimentacao`) como lançamento simples.
- RF-09: catálogo permanece fechado; nenhum meio excluído é introduzido.
- RF-11a: `credit_card` válido apenas para `direction=outcome`; `credit_card` + `income` rejeitado
  no smart constructor (`ErrCommandCreditCardRequiresOutcome`).
- RF-11b: `credit_card` exige `card_id`; ausência rejeitada no smart constructor
  (`ErrCommandCreditCardRequiresCardID`, já existente).
- RF-13: parcelamento no intervalo fechado 1..24 via reuso de `InstallmentCount`.
- RF-14: parcelamento opcional; `Installments` como `option.Option`, default 1 (resolvido a jusante).
- Conformidade: DMMF state-as-type + smart constructors; validação agregada por `errors.Join`;
  zero comentários Go (R-ADAPTER-001.1); validação de invariante somente em smart constructor
  (R-TXN-002); sem IO/`context.Context` (R-TXN-001, DMMF Princípio 6).
</requirements>

## Subtarefas

- [ ] 1.1 Estender `PaymentMethod` com `PaymentMethodMealVoucher` (9) e `PaymentMethodFoodVoucher`
  (10); atualizar `ParsePaymentMethod` (aceita `vale_refeicao`/`vale_alimentacao`), `String()` e
  `PaymentMethodFromInt` (bound `1..10`). Manter `doc` bloqueado em `ParsePaymentMethodForCreate`.
  Cobre RF-05, RF-06, RF-07, RF-08, RF-09.
- [ ] 1.2 Adicionar `ErrCommandCreditCardRequiresOutcome` em `domain/commands/errors.go` (reusar
  `ErrCommandCreditCardRequiresCardID` já em `errors.go:16`). Cobre RF-11a.
- [ ] 1.3 Enriquecer `CreateTransaction` (struct + `RawCreateTransaction` + `NewCreateTransaction`)
  com `CardID option.Option[valueobjects.CardID]` e `Installments option.Option[valueobjects.InstallmentCount]`;
  smart constructor rejeita `credit_card` sem `card_id` e `credit_card` com `direction != outcome`;
  parse de `Installments` reusa `InstallmentCount` (1..24). Cobre RF-03, RF-11a, RF-11b, RF-13, RF-14.
- [ ] 1.4 Aplicar o mesmo enriquecimento e invariantes em `UpdateTransaction`
  (`domain/commands/update_transaction.go`). Cobre RF-03, RF-11a, RF-11b, RF-13, RF-14.
- [ ] 1.5 Testes unitários dos VOs e smart constructors (padrão testify/suite whitebox, R-TESTING-001).

## Detalhes de Implementação

Ver `techspec.md` — seções "Value Objects (modificado)", "Commands (modificado)" e a tabela
"Validação (fronteira → smart constructor → use case)". Ver `adr-001-payment-method-dispatch.md`
para a decisão de que a diferenciação `credit_card` vs. simples é orquestração a jusante — aqui
apenas os campos e invariantes tipados são introduzidos, sem branch de regra.

Pontos-chave (não duplicar da techspec):
- `PaymentMethodFromInt` passa de bound `1..8` para `1..10`; `doc` (8) permanece legível.
- `credit_card`/`vale_*` continuam validados semanticamente no smart constructor (R-TXN-002/R-DTO-003);
  não replicar essa validação em DTO de input.
- `Installments` e `CardID` são `option.Option` — `None` para meios não-cartão; validação de
  presença de `card_id` e de `outcome` ocorre apenas quando `PaymentMethod == credit_card`.
- Agregação de erros via `errors.Join`, mensagem nomeando o campo (`fmt.Errorf("card_id: %w", err)`).

## Critérios de Sucesso

- Pacotes `domain/valueobjects` e `domain/commands` compilam.
- Testes unitários verdes cobrindo: `vale_refeicao`/`vale_alimentacao` válidos na criação; `doc`
  bloqueado em `ParsePaymentMethodForCreate` e legível em `ParsePaymentMethod`/`PaymentMethodFromInt`;
  `credit_card` sem `card_id` → `ErrCommandCreditCardRequiresCardID`; `credit_card` + `income` →
  `ErrCommandCreditCardRequiresOutcome`; `installments` fora de `1..10`/`1..24` → erro; roundtrip
  `String()`/`PaymentMethodFromInt` para os 10 valores.
- Zero comentários em Go de produção (R-ADAPTER-001.1).
- Validação agregada por `errors.Join`; invariantes de domínio somente em smart constructor (R-TXN-002).
- Sem IO nem `context.Context` na camada tocada (R-TXN-001).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — VO `PaymentMethod` (parse/String/FromInt) e smart constructors
  `NewCreateTransaction`/`NewUpdateTransaction`, padrão testify/suite whitebox (R-TESTING-001).
- [ ] Testes de integração — não aplicável a esta tarefa (domínio puro, sem IO).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/transactions/domain/valueobjects/payment_method.go` — enum `PaymentMethod`, parse, `String`, `PaymentMethodFromInt` (alvo principal).
- `internal/transactions/domain/valueobjects/direction.go` — `Direction` fechado, usado no invariante `credit_card ⇒ outcome`.
- `internal/transactions/domain/valueobjects/installment_count.go` — `InstallmentCount` (1..24) reusado.
- `internal/transactions/domain/valueobjects/ids.go` — `CardID` reusado no campo `CardID`.
- `internal/transactions/domain/commands/create_transaction.go` — `RawCreateTransaction`, `CreateTransaction`, `NewCreateTransaction`.
- `internal/transactions/domain/commands/update_transaction.go` — command de update (mesmo enriquecimento).
- `internal/transactions/domain/commands/errors.go` — `ErrCommandCreditCardRequiresCardID` (existente) + novo `ErrCommandCreditCardRequiresOutcome`.

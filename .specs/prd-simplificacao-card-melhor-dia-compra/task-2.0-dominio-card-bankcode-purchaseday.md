# Tarefa 2.0: Domínio card — `BankCode`, `PurchaseDayService`, entity/deciders/errors

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reescrever o domínio de `internal/card` conforme DMMF: novo VO `BankCode` (preserva texto original,
normaliza no lookup), novo domain service **puro** `PurchaseDayService.Decide` (cálculo fechamento/melhor
dia reutilizando `clamp`/`advanceMonth`), consolidação de identificação em `Nickname` (remover `CardName`),
remoção do VO `CardLimit`/`LimitCents`, adição de `Bank` na entidade e nos deciders, ajuste de erros.

<requirements>
- RF-02: consolidar identificação em `Nickname`; remover VO `CardName` e `ErrInvalidCardName` (ADR-005).
- RF-03/RF-20: VO `BankCode` texto livre; `String()` = original, `LookupKey()` = normalizado; `normalizeBank` determinística.
- RF-06: remover a regra `dueDay = closingDay + 7` do domínio (vínculo inverso passa ao shell via derivação).
- RF-08: `PurchaseDayService.Decide(dueDay, daysBeforeDue, now, tz) PurchaseDay{ClosingDay, BestDay}` puro (sem IO, sem `context`).
- RF-11: tratamento de virada de mês/dias inválidos reutilizando `clamp`/`advanceMonth` de `billing_cycle.go`.
- Remover `LimitCents`, `CardLimit`, `Card.UpdateLimit`, `ErrCardLimit*`; adicionar `Bank` e `ErrInvalidBank`.
</requirements>

## Subtarefas

- [ ] 2.1 Criar `domain/valueobjects/bank_code.go` (`BankCode`, `NewBankCode`, `String`, `LookupKey`) + `normalizeBank` (trim+lowercase+sem acento+espaços→`-`) + testes.
- [ ] 2.2 Criar `domain/services/purchase_day.go` (`PurchaseDay{ClosingDay, BestDay int}`, `PurchaseDayService.Decide`) reutilizando `clamp` + testes de tabela (8 bancos, fallback, wrap, mês curto, Nubank/20→(13,14)).
- [ ] 2.3 Editar `domain/entities/card.go`: remover `Name`/`LimitCents`/`Card.UpdateLimit`; adicionar `Bank valueobjects.BankCode`; ajustar `NewCardInput`/`HydrateCard`/`HydrateCardWithVersion`.
- [ ] 2.4 Editar `domain/services/decide_create_card.go` e `decide_update_card.go`: `CreateCardCommand`/`UpdateCardCommand` sem `Name`/`LimitCents`, com `Bank`; deciders puros.
- [ ] 2.5 Editar `domain/errors.go`: adicionar `ErrInvalidBank`; remover `ErrInvalidCardName`, `ErrCardLimitNegative`, `ErrCardLimitTooLarge`, `ErrCardLimitConflict`.
- [ ] 2.6 Remover `domain/valueobjects/card_limit.go`, `card_name.go` (+ seus `_test.go`).

## Detalhes de Implementação

Ver `techspec.md` §"Design de Implementação" (código de `BankCode` e `PurchaseDayService`), ADR-001/003/005.
Domínio permanece puro (DMMF Princípio 6): `now`/`tz` explícitos, sem `time.Now()`; smart constructors
(Princípio 1). `PurchaseDayService.Decide` usa `clamp` (mesmo pacote `services`); `tz` de `timezone.go`.

## Critérios de Sucesso

- `PurchaseDayService.Decide` determinístico: Nubank/venc.20/7d → `{13,14}`; testes de borda passam.
- `BankCode`: `NewBankCode("Banco do Brasil").String()` = "Banco do Brasil", `.LookupKey()` = "banco-do-brasil"; vazio → `ErrInvalidBank`.
- Compila sem `Name`/`LimitCents`/`CardLimit`/`CardName` no pacote `domain`.
- Deciders puros (sem IO); cobertura unitária sem mock.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `bank_code_test.go`, `purchase_day_test.go` (tabela), `decide_create_card_test.go`/`decide_update_card_test.go` ajustados, `card_test.go` ajustado.
- [ ] Testes de integração: n/a (domínio puro).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/card/domain/valueobjects/bank_code.go` (novo)
- `internal/card/domain/services/purchase_day.go` (novo)
- `internal/card/domain/entities/card.go`, `domain/services/{decide_create_card,decide_update_card,billing_cycle}.go`, `domain/errors.go` (editar)
- `internal/card/domain/valueobjects/{card_limit,card_name}.go` (+tests) (remover)

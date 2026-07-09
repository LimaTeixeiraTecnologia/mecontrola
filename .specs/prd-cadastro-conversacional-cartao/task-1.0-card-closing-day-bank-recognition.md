# Tarefa 1.0: Dia de fechamento explícito opcional e reconhecimento de banco no módulo card

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Mudança **aditiva** (ADR-002) no módulo `internal/card` que desbloqueia o cadastro conversacional
de cartão para bancos não reconhecidos, sem alterar o fluxo de onboarding (RF-09). Estende o input
`CreateCard` com o par sentinela `ClosingDay int` + `ClosingDayProvided bool`, faz branch no usecase
`CreateCard.Execute` por **intenção do chamador** (não por reconhecimento), e adiciona uma leitura
read-only `IsBankRecognized` que reusa a normalização do smart constructor `NewBankCode` para que
reconhecimento e derivação enxerguem o mesmo `code`. Toda validação reusa smart constructors
existentes; zero duplicação de whitelist e zero comentários em Go de produção.

<requirements>
- Estender `internal/card/application/dtos/input/create_card.go` `CreateCard` com `ClosingDay int` e `ClosingDayProvided bool`; `Validate()` valida `ClosingDay` ∈ [1,31] SOMENTE quando `ClosingDayProvided` (`errors.Join`, campo nomeado — R-DTO-001).
- Branch em `internal/card/application/usecases/create_card.go` (linhas ~87-101): se `in.ClosingDayProvided` → `valueobjects.NewBillingCycle(in.ClosingDay, in.DueDay)` diretamente; senão → caminho de derivação atual inalterado (`BankDaysReader.DaysBeforeDue` + `PurchaseDayService.Decide`).
- Adicionar leitura `IsBankRecognized(ctx, bank) (bool, error)` no `infrastructure/postgres` do módulo card (`SELECT EXISTS(...) FROM mecontrola.banks WHERE code = $1`) + read repo/usecase/interface; DEVE reusar a normalização de `valueobjects.NewBankCode` (NFD + hyphen-join, lowercase) ANTES da consulta.
- Onboarding 100% inalterado (RF-09): o branch é por `ClosingDayProvided`, nunca por reconhecimento.
- Range de dia é 1..31 (NÃO 1..28); sem restrição cruzada fechamento×vencimento (RF-11).
- Reuso total dos smart constructors existentes, zero duplicação (R-DTO-004).
- Zero comentários em Go de produção (R-ADAPTER-001.1).
- Cobre RF-07, RF-08, RF-09, RF-10, RF-11, RF-20.
</requirements>

## Subtarefas

- [x] 1.1 Estender `input.CreateCard` com `ClosingDay int` + `ClosingDayProvided bool`; `Validate()` valida `ClosingDay` ∈ [1,31] só quando `ClosingDayProvided`, com `errors.Join` e campo nomeado (R-DTO-001).
- [x] 1.2 Branch em `CreateCard.Execute` (~87-101): `ClosingDayProvided` → `NewBillingCycle(in.ClosingDay, in.DueDay)`; senão → derivação atual inalterada.
- [x] 1.3 Adicionar leitura `IsBankRecognized(ctx, bank) (bool, error)` no adapter postgres (`SELECT EXISTS(...) FROM mecontrola.banks WHERE code = $1`) reusando normalização `NewBankCode` antes da consulta; expor via read repo/usecase/interface do módulo card.
- [x] 1.4 Escrever testes unitários do usecase (provided-path, derive-path inalterado, regressão onboarding, banco reconhecido ignora closing, validações nomeadas) e teste do `IsBankRecognized` cobrindo paridade de normalização.

## Detalhes de Implementação

Ver `techspec.md` — seções "Reconhecimento de Banco e Dia de Fechamento (RF-07/08/09, ADR-002)",
"Modelos de Dados" (`input.CreateCard`), "Sequenciamento de Desenvolvimento" (item 1 da Ordem de
Build) e "Pontos de Integração" (SQL novo apenas no read `IsBankRecognized`). Ver
`adr-002-closing-day-optional-modeling.md` — seções "Decisão" (itens 1, 2, 4, 5), "Alternativas
Consideradas" (rejeição de `*int`/`omitempty` e do branch por reconhecimento) e "Plano de
Implementação".

Pontos críticos (não duplicar prosa da techspec):

- Sentinela sem ponteiro e sem `omitempty` — evita semântica `null` de merge-patch RFC 7386 no
  snapshot do kernel (ADR-002, decisão 1).
- Branch por intenção do chamador (`ClosingDayProvided`), não por reconhecimento — preserva RF-09.
- `IsBankRecognized` é aditivo: `DaysBeforeDue` mantém assinatura e fallback de 7 dias intactos.
- Normalização unificada via `NewBankCode` é a fonte única — elimina o risco de divergência do ADR-002.

## Critérios de Sucesso

- `Validate()` retorna erro com campo nomeado apenas quando `ClosingDayProvided` e `ClosingDay` fora de [1,31]; nenhum erro para `ClosingDayProvided=false`.
- Provided-path constrói `BillingCycle` direto; derive-path e onboarding produzem exatamente o mesmo resultado de antes (regressão verde).
- Banco reconhecido com `closingDay` informado não altera o cálculo (defesa via branch por intenção).
- `IsBankRecognized` normaliza "Nubank", "banco XP" e entrada acentuada com o mesmo `code` da derivação.
- `gofmt -l` limpo; zero comentários em arquivos `.go` de produção (R-ADAPTER-001.1); build, vet, test race e lint do módulo verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — modelagem do valor opcional (sentinela ClosingDay+ClosingDayProvided) e branch de estado no usecase (ADR-002).
- `postgresql-production-standards` — novo read IsBankRecognized (SELECT EXISTS) no adapter postgres do módulo card.
- `design-patterns-mandatory` — gate de desenho do trio Go obrigatório para alteração de domínio/usecase.

## Testes da Tarefa

- [ ] Testes unitários — table-test testify/suite whitebox com `fake.NewProvider()` (R-TESTING-001) do usecase: provided-path (cycle direto), derive-path inalterado, regressão de onboarding, banco reconhecido ignora closing informado (defesa), erros de validação nomeados; teste unitário de `IsBankRecognized` reusando normalização `NewBankCode` ("Nubank", "banco XP", entrada acentuada).
- [ ] Testes de integração — opcional para esta tarefa; ciclo completo e replay ficam nas tarefas do consumidor (ver techspec, "Abordagem de Testes").

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/card/application/dtos/input/create_card.go` — struct `CreateCard` + `Validate()` (par sentinela).
- `internal/card/application/usecases/create_card.go` — branch por `ClosingDayProvided` (~87-101).
- `internal/card/domain/valueobjects/nickname.go`, `bank_code.go`, `billing_cycle.go` — smart constructors reusados (`NewNickname`, `NewBankCode`, `NewBillingCycle`).
- `internal/card/domain/services/purchase_day.go` — `PurchaseDayService.Decide` (caminho de derivação inalterado).
- `internal/card/application/interfaces/*` — interface do read repo/reader de reconhecimento.
- `internal/card/infrastructure/postgres/*` — leitura `IsBankRecognized` (SELECT EXISTS, normalização `NewBankCode`).

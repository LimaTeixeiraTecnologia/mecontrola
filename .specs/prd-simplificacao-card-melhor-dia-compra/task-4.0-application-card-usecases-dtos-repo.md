# Tarefa 4.0: Application card — DTOs, use cases, mapper, repositório

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reescrever a camada de aplicação de `internal/card` para o novo escopo: DTOs de `CreateCard`/`UpdateCard`
sem `name`/`closing_day`/`limit_cents` e com `bank`; novo use case `BestPurchaseDay` (consulta pura);
`CreateCard`/`UpdateCard` derivam e persistem `closing_day` via `BankDaysReader` + `PurchaseDayService`;
remover `UpdateCardLimit` (dto+usecase); ajustar mapper, `CardRepository` (remover `UpdateLimitByIDForUser`)
e todas as queries SQL do `card_repository` (bank no lugar de name/limit).

<requirements>
- RF-01: `CreateCard` exige apenas `nickname`, `bank`, `due_day`; resposta traz `closing_day`/`best_purchase_day` derivados.
- RF-04: `closing_day` derivado (banco+due_day) e persistido; deixa de ser entrada.
- RF-05: remover `UpdateCardLimit` (input DTO + use case), `CardRepository.UpdateLimitByIDForUser` e a query UPDATE_LIMIT.
- RF-07: `UpdateCard` recomputa e re-persiste `closing_day` ao alterar `bank`/`due_day`; mantém versão otimista.
- RF-12: `BestPurchaseDay.Execute` puro do ponto de vista de negócio (usa reader), sem persistência, sem exigir cartão.
- RF-13: DTOs de saída expõem `closing_day` + `best_purchase_day`.
- R-DTO-VALIDATE-001: `Validate()` com `errors.Join`, campo nomeado, chamado após `defer span.End()`.
</requirements>

## Subtarefas

- [ ] 4.1 DTOs input: reescrever `create_card.go` (`Nickname, Bank, DueDay`) e `update_card.go` (`Nickname?, Bank?, DueDay?`); criar `best_purchase_day.go` (`Bank, DueDay` + `Validate`); remover `update_card_limit.go`; ajustar `errors.go` de input.
- [ ] 4.2 DTOs output: `output/card.go` (remover `name`/`limit_cents`, add `bank`+`best_purchase_day`, manter `closing_day`); criar `output/best_purchase_day.go`.
- [ ] 4.3 Use cases: `create_card.go` (resolve `days` via reader → `PurchaseDayService.Decide` → grava cycle); `update_card.go` (recompute ao mudar bank/due_day); criar `best_purchase_day.go`; remover `update_card_limit.go`.
- [ ] 4.4 `mappers/card_mapper.go`: `ToCardOutput` sem name/limit, com bank + best_purchase_day.
- [ ] 4.5 `application/interfaces/repository.go`: remover `UpdateLimitByIDForUser`; ajustar assinaturas que carregam name/limit.
- [ ] 4.6 `infrastructure/repositories/postgres/card_repository.go`: INSERT/GET/UPDATE/LIST/FIND_DUE/`scanCard` com `bank` (sem `name`/`limit_cents`); remover query UPDATE_LIMIT; regenerar mock `CardRepository`.

## Detalhes de Implementação

Ver `techspec.md` §"Fluxo de Dados", §"Contrato do repositório" (linhas SQL exatas), ADR-002/003.
`best_purchase_day` = `closing_day + 1` derivado (não persistir coluna). `now` inline no shell
(`time.Now().UTC()`); tz de `timezone.go`. Padrão de teste testify/suite whitebox (R-TESTING-001) com
mock de `BankDaysReader` e `CardRepository` via IIFE.

## Critérios de Sucesso

- `CreateCard` grava `closing_day` = derivado; resposta traz `bank`, `closing_day`, `best_purchase_day`.
- `UpdateCard` recomputa `closing_day` ao mudar `bank`/`due_day`; erro do reader propaga.
- `BestPurchaseDay.Execute` retorna `{closing_day, best_purchase_day}` sem persistir.
- Compila sem `UpdateCardLimit`/`limit_cents`/`name` na camada application; mocks atualizados.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `create_card_test.go`, `update_card_test.go`, `best_purchase_day_test.go`, `card_mapper_test.go` (testify/suite, `fake.NewProvider()`, IIFE por mock).
- [ ] Testes de integração: `card_repository_integration_test.go` (insert/get/list/update com `bank`, sem `limit_cents`).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/card/application/dtos/input/{create_card,update_card,best_purchase_day}.go`, `dtos/output/{card,best_purchase_day}.go`, `dtos/input/errors.go`
- `internal/card/application/usecases/{create_card,update_card,best_purchase_day}.go`, `mappers/card_mapper.go`, `interfaces/repository.go`
- `internal/card/infrastructure/repositories/postgres/card_repository.go`
- Remover: `dtos/input/update_card_limit.go`, `usecases/update_card_limit.go` (+tests)

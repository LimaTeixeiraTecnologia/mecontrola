# Tarefa 4.0: Application layer — DTOs, interfaces, use cases (UoW + Storage), mocks

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Entregar a camada `internal/card/application/`: DTOs `input/`/`output/`, interfaces (`CardRepository`, `RepositoryFactory`), seis use cases (`CreateCard`, `GetCard`, `ListCards`, `UpdateCard`, `SoftDeleteCard`, `InvoiceFor`) e mocks via mockery. Use cases mutadores usam `uow.UnitOfWork[entities.Card]` do devkit-go e gravam `idempotency.Storage.Put` DENTRO do UoW (ADR-006) para 2xx exactly-once.

<requirements>
- `application` sem IO concreto: nada de `*sql.DB`, `pgx`, HTTP, brokers, SDK externo.
- Use cases mutadores recebem `uow.UnitOfWork[T]`, `RepositoryFactory`, `manager.Manager`, `idempotency.Storage`, `observability.Observability` via construtor explícito (R6).
- `CreateCard.Execute`: lê `idempotency.FromContext`, abre `uow.Execute`, chama `repo.Insert`, marshalla output via `json.Marshal(toOutput(card))`, chama `Storage.Put` no mesmo UoW. Falha em qualquer passo → rollback.
- `UpdateCard.Execute` + `SoftDeleteCard.Execute`: mesmo pattern. `SoftDelete` usa `time.Now().UTC()` inline (R6.7) — sem `clock.Clock`.
- `ListCards.Execute`: read-only; recebe `cursor string`, decodifica `base64.URLEncoding(json{created_at, id})`, valida; passa para repository keyset.
- `InvoiceFor.Execute`: busca cartão via `repo.GetByIDForUser`, chama `domain.services.BillingCycle.InvoiceFor(purchase, card.Cycle(), SaoPauloLocation())`. Retorna `output.Invoice{ClosingDate, DueDate}`.
- `RepositoryFactory.CardRepository(database.DBTX) CardRepository`.
- Mocks gerados via `mockery.yml`.
- Tests unit cobrem caminho feliz + erros sentinels + rollback do UoW em falha do `Storage.Put` (R-INT-05).
- Sem `init()`, sem `panic`, sem `var _ Interface`, sem comentários em `.go`.
</requirements>

## Subtarefas

- [ ] 4.1 `application/dtos/input/{create_card,get_card,list_cards,update_card,soft_delete_card,invoice_for}.go`.
- [ ] 4.2 `application/dtos/output/{card,card_list,invoice}.go`.
- [ ] 4.3 `application/interfaces/repository.go` — `CardRepository` + `RepositoryFactory`.
- [ ] 4.4 `application/usecases/create_card.go` — UoW + Storage atomic.
- [ ] 4.5 `application/usecases/update_card.go` — UoW + Storage atomic.
- [ ] 4.6 `application/usecases/soft_delete_card.go` — UoW + Storage atomic + `time.Now().UTC()` inline.
- [ ] 4.7 `application/usecases/get_card.go` — read-only.
- [ ] 4.8 `application/usecases/list_cards.go` — read-only + cursor decode/validation.
- [ ] 4.9 `application/usecases/invoice_for.go` — read-only, delega para domain service. Exportado em `module.go` como `CardLookup` (porta interna para futuro módulo de transações).
- [ ] 4.10 Estender `mockery.yml` para gerar mocks de `CardRepository`, `RepositoryFactory`.
- [ ] 4.11 Tests unit por use case (mockery-based).
- [ ] 4.12 Test crítico R-INT-05: mock `Storage.Put` que retorna erro após `repo.Insert` → assert que `UoW.Execute` rolou back e `repo.Insert` foi revertido (via mock counter).

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/techspec.md` §"Interfaces Chave" e §"Use cases mutadores", `adr-006-idempotency-atomicity-via-uow.md`. Pattern do UoW espelha `internal/identity/.../usecases/upsert_user_by_whatsapp.go` e `establish_principal.go`.

## Critérios de Sucesso

- `go test -race -count=1 -cover ./internal/card/application/...` ≥ 90% line coverage.
- Test R-INT-05 verde: mock `Storage.Put` retornando erro produz `UoW.Execute` rollback observável.
- Cursor inválido (base64 quebrado, JSON malformado, datas fora de range) retorna `ErrInvalidCursor`.
- `InvoiceFor` chamada com cartão soft-deleted retorna `ErrCardNotFound`.
- `go vet ./internal/card/application/...` + `golangci-lint run ./internal/card/application/...` limpos.

### Definition of Done

- [ ] Pacote `internal/card/application/` criado e commitado.
- [ ] Mocks gerados e commitados em `internal/card/application/usecases/mocks/`.
- [ ] `task generate-mocks` (ou equivalente) idempotente.
- [ ] Gate de zero comentários verde.
- [ ] RF-39, RF-40, RF-48 explicitamente apontados no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: por use case, com mocks de repository, factory e idempotency.Storage.
- [ ] Testes de integração: N/A (sem IO direto; cobertos por 5.0 e 7.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/card/application/dtos/input/*.go` (novo)
- `internal/card/application/dtos/output/*.go` (novo)
- `internal/card/application/interfaces/repository.go` (novo)
- `internal/card/application/usecases/*.go` (novo)
- `internal/card/application/usecases/*_test.go` (novo)
- `internal/card/application/usecases/mocks/` (gerado)
- `mockery.yml` (modificar)

# Tarefa 6.0: Use cases finos + mocks via mockery + unit tests table-driven

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os 5 use cases que orquestram identity em `internal/identity/application/usecases/`: `UpsertUserByWhatsAppNumberUseCase`, `FindUserByIDUseCase`, `FindUserByWhatsAppNumberUseCase`, `SoftDeleteUserUseCase`, `LinkNewNumberUseCase`. Cada use case valida input (constrói VOs), delega ao port `UserRepository` e wrappa erros com contexto em PT-BR. Mocks de `UserRepository` e `IDGenerator` são gerados via `mockery --config mockery.yml` em `application/interfaces/mocks/`. Unit tests seguem `testify/suite` table-driven com mocks (R4).

<requirements>
- RF-07 (orquestração): `SoftDeleteUserUseCase` orquestra a chamada ao `UserRepository.SoftDelete` (cascata real no adapter — task 7.0 + ADR-009).
- Use cases recebem `UserRepository`, `IDGenerator` e `clock.Clock` via construtor (DI explícita — R6.6, zero estado global).
- Receiver Go single-letter idiomático (`u` para use case); fields com nomes completos (`userRepository`, `idGenerator`, `clock`).
- Use cases wrappam erros: `fmt.Errorf("<verbo> <objeto>: %w", err)` em PT-BR.
- Mocks gerados via mockery com `with-expecter: true` (R3).
- Cobertura mínima por use case: happy path, erro de validação de VO, erro de repositório.
- Convenção: `Execute(ctx context.Context, ...) (..., error)` como método único exportado.
</requirements>

## Subtarefas

- [ ] 6.1 Adicionar `UserRepository` e `IDGenerator` em `mockery.yml` (raiz) sob `packages: github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces`.
- [ ] 6.2 Rodar `mockery --config mockery.yml` gerando `internal/identity/application/interfaces/mocks/user_repository.go` e `id_generator.go`.
- [ ] 6.3 Criar `internal/identity/application/usecases/upsert_user_by_whatsapp_number.go` recebendo `UserRepository`, `IDGenerator`, `clock.Clock`; `Execute(ctx, rawNumber string) (*entities.User, error)` constrói VO e delega ao repo.
- [ ] 6.4 Criar `internal/identity/application/usecases/find_user_by_id.go` recebendo `UserRepository`; `Execute(ctx, rawID string) (*entities.User, error)` constrói `UserID` e delega.
- [ ] 6.5 Criar `internal/identity/application/usecases/find_user_by_whatsapp_number.go` análogo a 6.4 mas com `WhatsAppNumber`.
- [ ] 6.6 Criar `internal/identity/application/usecases/soft_delete_user.go` recebendo `UserRepository`; `Execute(ctx, rawID string) error`.
- [ ] 6.7 Criar `internal/identity/application/usecases/link_new_number.go` recebendo `UserRepository`; `Execute(ctx, rawID, rawNumber, reason string) error`. Rejeitar `reason == "user_soft_deleted"` (string reservada — ADR-009).
- [ ] 6.8 Criar `_test.go` para cada use case com `Suite` table-driven cobrindo happy path, erro de VO, erro de repo. Usar `.EXPECT()` fluente do mockery.
- [ ] 6.9 Re-rodar `mockery --config mockery.yml --dry-run` para confirmar sincronia.

## Detalhes de Implementação

Ver techspec §"Design de Implementação" subseção `application/usecases/upsert_user_by_whatsapp_number.go` (exemplo canônico). Receiver pattern: `func (u *<UseCase>) Execute(...)`. ADR-009 introduz a string reservada `"user_soft_deleted"` que `LinkNewNumber` deve rejeitar.

## Critérios de Sucesso

- 5 use cases compilam e testes unitários passam.
- Cobertura `go test -cover ./internal/identity/application/usecases/...` ≥ 90%.
- Mocks regenerados são byte-idênticos ao que mockery produz a partir das interfaces atuais.
- `LinkNewNumberUseCase.Execute` rejeita `reason == "user_soft_deleted"` com erro tipado.
- `application/usecases` não importa `infrastructure` (depguard).

## Definition of Done (DoD)

- [ ] `mockery --config mockery.yml` executa sem warning.
- [ ] `mockery --config mockery.yml --dry-run` reporta zero diff.
- [ ] `go test -race -count=1 ./internal/identity/application/usecases/...` passa.
- [ ] `go test -cover ./internal/identity/application/usecases/...` ≥ 90%.
- [ ] Cada `_test.go` segue padrão suite (`SetupTest` reseta mocks, `TestXxx` registra suite, tabela de scenarios).
- [ ] `LinkNewNumberUseCase` tem teste explícito `"rejeita reason reservada user_soft_deleted"`.
- [ ] `golangci-lint run ./internal/identity/application/...` passa.
- [ ] Receivers são single-letter (`u`) em todos os 5 use cases (`grep -E 'func \([a-z]\w+ \*' internal/identity/application/usecases/*.go | grep -v _test` deve retornar vazio).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit suite table-driven para cada um dos 5 use cases (3 cenários mínimos).
- [ ] Verificação de regeneração de mocks (`--dry-run`).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `mockery.yml` (alterado — declarar `UserRepository` e `IDGenerator`)
- `internal/identity/application/usecases/upsert_user_by_whatsapp_number.go` (novo)
- `internal/identity/application/usecases/upsert_user_by_whatsapp_number_test.go` (novo)
- `internal/identity/application/usecases/find_user_by_id.go` (novo)
- `internal/identity/application/usecases/find_user_by_id_test.go` (novo)
- `internal/identity/application/usecases/find_user_by_whatsapp_number.go` (novo)
- `internal/identity/application/usecases/find_user_by_whatsapp_number_test.go` (novo)
- `internal/identity/application/usecases/soft_delete_user.go` (novo)
- `internal/identity/application/usecases/soft_delete_user_test.go` (novo)
- `internal/identity/application/usecases/link_new_number.go` (novo)
- `internal/identity/application/usecases/link_new_number_test.go` (novo)
- `internal/identity/application/interfaces/mocks/user_repository.go` (gerado)
- `internal/identity/application/interfaces/mocks/id_generator.go` (gerado)

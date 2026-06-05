# Tarefa 5.0: Use cases com uow.UnitOfWork[T] da devkit + RepositoryFactory

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar os 4 use cases de identity consumindo `uow.UnitOfWork[T]` da devkit-go (proibido reimplementar — R6.9) e `interfaces.RepositoryFactory` (ADR-008). Cada UC tipa `T` pelo retorno do callback: `UpsertUserByWhatsApp` (T = `entities.User`), `FindUserByID` (T = `entities.User`), `FindUserByWhatsApp` (T = `entities.User`), `MarkUserDeleted` (T = `struct{}` via `uow.NewVoid`). `time.Now().UTC()` é chamado **inline no call-site** dentro do callback (R6.7) — proibido `now := time.Now().UTC()`. Testes unitários mockam `UnitOfWork[T]` + `RepositoryFactory` com callback executado inline.

<requirements>
- RF-08-ter: `UpsertUserByWhatsApp` orquestra Find → decisão (criar | reanimar | preservar) → Upsert dentro de um único `uow.Do` (atomicidade).
- RF-10: semântica "touch garantido" — `updated_at` sempre atualiza, mesmo sem mudança nos demais campos (regra aplicada no UC + reforçada pelo SQL em 6.0).
- R6.7: `time.Now().UTC()` inline no call-site; sem captura em variável.
- R6.9: consumir `github.com/JailtonJunior94/devkit-go/pkg/database/uow` direto.
- ADR-008: UC recebe `uow.UnitOfWork[T]` + `interfaces.RepositoryFactory` via DI; cria repos amarrados ao `tx` dentro do callback via `u.factory.UserRepository(tx)`.
- First-write-wins (RF-08-bis): UC chama `existing.SetDisplayNameIfEmpty(in.DisplayName)` antes do upsert.
- Erros wrappeados com prefixo `identity.usecase.<op>:` (R5.10).
- Logs estruturados via `o11y.Logger().Error(...)` no nível externo do UC (não dentro do callback) — log uma vez no final.
</requirements>

## Subtarefas

- [ ] 5.1 `internal/identity/application/usecases/upsert_user_by_whatsapp.go`:
  - Campos: `uow uow.UnitOfWork[entities.User]`, `factory interfaces.RepositoryFactory`, `o11y observability.Observability`.
  - Construtor sem `IDGenerator`.
  - `Execute(ctx, in input.UpsertUserByWhatsApp) (output.UpsertUserByWhatsApp, error)`: tracer span + `uow.Do` callback fazendo `FindByWhatsAppNumber` → branch (criar via `entities.New(...)` ou reusar `existing` + `SetDisplayNameIfEmpty`) → `UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())` → retorna `(entities.User, error)`.
- [ ] 5.2 `internal/identity/application/usecases/find_user_by_id.go` (T = `entities.User`).
- [ ] 5.3 `internal/identity/application/usecases/find_user_by_whatsapp.go` (T = `entities.User`).
- [ ] 5.4 `internal/identity/application/usecases/mark_user_deleted.go` (T = `struct{}` via `uow.NewVoid` — UC carrega `uow.UnitOfWork[struct{}]`).
- [ ] 5.5 Mocks via mockery genéricos (`MockUnitOfWork[entities.User]`, `MockRepositoryFactory`, `MockUserRepository`) — gerar com `mockery v2.30+`.
- [ ] 5.6 Testes unitários para cada UC: callback executado inline (`return fn(ctx, nil)`), factory mock devolve repo mock, asserções de invariante (UUID v4 não vazio, sentinels propagados).

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §Application — usecases/upsert_user_by_whatsapp.go](./techspec.md) — esqueleto canônico.
- [Runbook §5 + §11](../../docs/runbooks/handler-usecase-uow-repository.md) — UC end-to-end + orquestração multi-repo.
- [ADR-008](./adr-008-repository-factory-per-module.md) — padrão UoW + Factory.

**Shape do callback (inegociável):**

```go
result, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.User, error) {
    userRepo := u.factory.UserRepository(tx)
    // … operações com userRepo, time.Now().UTC() inline em cada call-site
    return persisted, nil
})
```

**Find use cases (ler fora de TX longa):** usar `uow.New[entities.User](mgr, uow.WithReadOnly(true))` quando útil para hint de read-only — opcional, defaults bastam no MVP.

**MarkUserDeleted (callback retorna struct{}):**

```go
_, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
    userRepo := u.factory.UserRepository(tx)
    if err := userRepo.MarkDeleted(ctx, id, time.Now().UTC()); err != nil {
        return struct{}{}, fmt.Errorf("%s mark deleted: %w", prefix, err)
    }
    return struct{}{}, nil
})
```

## Critérios de Sucesso

- `go test -race -cover ./internal/identity/application/usecases/...` ≥85% (CA-01).
- Todos os UCs consomem `uow.UnitOfWork[T]` da devkit; **zero** menções a `manager.Manager.BeginTx` direto.
- Construtores **não** recebem `IDGenerator` nem `manager.Manager`.
- `grep "now := time.Now" internal/identity/application/usecases/` retorna 0 matches (R6.7).
- `grep "u.idGen\|idGen:" internal/identity/application/usecases/` retorna 0 matches (R6.8).
- Mocks gerados em `internal/identity/application/usecases/mocks/` (ou `internal/identity/mocks/` conforme convenção do projeto).
- `go build ./...` verde.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `upsert_user_by_whatsapp_test.go` — caminhos: criar (Find retorna `ErrUserNotFound`), atualizar com FWW (Find retorna existing com displayName vazio), preservar FWW (Find retorna existing com displayName populado), erro propagado de Find, erro propagado de Upsert.
- [ ] `find_user_by_id_test.go`, `find_user_by_whatsapp_test.go` — caminhos: encontrado, `ErrUserNotFound`, erro de IO.
- [ ] `mark_user_deleted_test.go` — caminhos: ok, `ErrUserNotFound` propagado.
- [ ] Assertion comum: `mockFactory.EXPECT().UserRepository(mock.Anything).Return(mockUserRepo)` para confirmar que UC sempre cria repo via factory (proxy de "amarra ao tx").

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/application/usecases/upsert_user_by_whatsapp.go` (criar)
- `internal/identity/application/usecases/upsert_user_by_whatsapp_test.go` (criar)
- `internal/identity/application/usecases/find_user_by_id.go` (criar)
- `internal/identity/application/usecases/find_user_by_id_test.go` (criar)
- `internal/identity/application/usecases/find_user_by_whatsapp.go` (criar)
- `internal/identity/application/usecases/find_user_by_whatsapp_test.go` (criar)
- `internal/identity/application/usecases/mark_user_deleted.go` (criar)
- `internal/identity/application/usecases/mark_user_deleted_test.go` (criar)
- `internal/identity/mocks/` ou similar (mocks gerados)
- `.mockery.yaml` (criar ou estender — configurar geração para interfaces de identity)
- Dependências (já criadas em 4.0): `application/interfaces/*`, `application/errors.go`, DTOs.

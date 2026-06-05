# Tarefa 4.0: Ports application/interfaces (UserRepository, RepositoryFactory) + erros tipados

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Declarar os ports da camada application em `internal/identity/application/interfaces/` (R6 — interface no consumidor): `UserRepository` (operações de persistência sem `tx` no argumento — a instância concreta carrega `database.DBTX`) e `RepositoryFactory` (port introduzido por ADR-008 para amarrar repos a uma `database.DBTX` recebida). Criar também `application/errors.go` com sentinels para mapeamento `pgerrcode → erro tipado` que será consumido pelo Postgres em 6.0 e pelo handler HTTP em 7.0.

<requirements>
- RF-10: port `UserRepository` cobre `UpsertByWhatsAppNumber`, `FindByID`, `FindByWhatsAppNumber`, `MarkDeleted`, `AppendWhatsAppHistory`. Erros tipados para "não encontrado" e violações de unicidade. Semântica "touch garantido" (RF-10 do PRD).
- RF-13: `Subscription` declarada em `domain` (já em 2.0); o port `UserRepository` não depende dela, mas use cases (5.0) consomem.
- ADR-008: `RepositoryFactory.UserRepository(db database.DBTX) UserRepository` permite orquestrar 2+ repos na mesma TX.
- ADR-004: sentinels via `errors.New` em `application/errors.go` (`ErrUserNotFound`, `ErrWhatsAppNumberInUse`, `ErrEmailInUse`).
- DTOs in/out por use case (esqueleto + `doc.go` placeholders nos subpacotes).
- Métodos do port **não** recebem `tx database.DBTX` na assinatura (ADR-008).
</requirements>

## Subtarefas

- [ ] 4.1 `internal/identity/application/errors.go` — 3 sentinels: `ErrUserNotFound`, `ErrWhatsAppNumberInUse`, `ErrEmailInUse`.
- [ ] 4.2 `internal/identity/application/interfaces/user_repository.go` — port `UserRepository` + struct `WhatsAppHistoryEntry` (campos públicos usados pelo repo/UC).
- [ ] 4.3 `internal/identity/application/interfaces/repository_factory.go` — port `RepositoryFactory` com método `UserRepository(db database.DBTX) UserRepository`.
- [ ] 4.4 `internal/identity/application/dtos/input/doc.go` + `internal/identity/application/dtos/output/doc.go` — placeholders com pacote declarado (`package input` / `package output`) + comentário de reserva.
- [ ] 4.5 DTOs concretos por UC: `input/upsert_user_by_whatsapp.go`, `output/upsert_user_by_whatsapp.go` (esqueleto mínimo — completados conforme 5.0 e 7.0 consumirem).

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §Application](./techspec.md) — snippet canônico do port `UserRepository`.
- [ADR-004](./adr-004-typed-errors-application-package.md) — sentinels.
- [ADR-008](./adr-008-repository-factory-per-module.md) — `RepositoryFactory`.
- [Runbook §4.1](../../docs/runbooks/handler-usecase-uow-repository.md) — padrão de port + factory.

**Assinaturas inegociáveis:**

```go
// internal/identity/application/interfaces/user_repository.go
type UserRepository interface {
    UpsertByWhatsAppNumber(ctx context.Context, u entities.User, now time.Time) (entities.User, error)
    FindByID(ctx context.Context, id string) (entities.User, error)
    FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (entities.User, error)
    MarkDeleted(ctx context.Context, id string, now time.Time) error
    AppendWhatsAppHistory(ctx context.Context, userID string, entry WhatsAppHistoryEntry) error
}

// internal/identity/application/interfaces/repository_factory.go
type RepositoryFactory interface {
    UserRepository(db database.DBTX) UserRepository
}

// internal/identity/application/errors.go
var (
    ErrUserNotFound        = errors.New("identity: user not found")
    ErrWhatsAppNumberInUse = errors.New("identity: whatsapp number already in use")
    ErrEmailInUse          = errors.New("identity: email already in use")
)
```

## Critérios de Sucesso

- `go build ./...` verde (todos os imports resolvem).
- Nenhum import de `infrastructure/**` em `application/**` (validado por `depguard` em 9.0).
- Ports são interfaces puras — sem struct concreto, sem implementação inline.
- Sentinels usáveis com `errors.Is` (são `error` produzidos por `errors.New`).
- `WhatsAppHistoryEntry` (struct de transporte) **não** é a entidade de domínio — é DTO de port. A entidade `entities.WhatsAppHistoryEntry` fica em domain; o struct daqui é projeção para a interface do repo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Compilação: `go build ./internal/identity/application/...` verde.
- [ ] Vet: `go vet ./internal/identity/application/...` sem warnings.
- [ ] Inspeção visual: ports são apenas interfaces; nenhum método tem implementação concreta neste pacote.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/application/errors.go` (criar)
- `internal/identity/application/interfaces/user_repository.go` (criar)
- `internal/identity/application/interfaces/repository_factory.go` (criar)
- `internal/identity/application/dtos/input/doc.go` (criar)
- `internal/identity/application/dtos/input/upsert_user_by_whatsapp.go` (criar)
- `internal/identity/application/dtos/output/doc.go` (criar)
- `internal/identity/application/dtos/output/upsert_user_by_whatsapp.go` (criar)

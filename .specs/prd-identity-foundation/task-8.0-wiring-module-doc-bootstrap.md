# Tarefa 8.0: Wiring — NewIdentityModule + doc.go + bootstrap em cmd/server

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar o `IdentityModule` seguindo o Padrão Obrigatório de Módulo (`AGENTS.md`) e ADR-005. O construtor `NewIdentityModule(cfg, o11y, mgr)` recebe 3 parâmetros — sem `IDGenerator`, sem `UnitOfWork` solto — e instancia 1 `uow.New[T](mgr, uow.WithObservability(o11y))` tipado por use case. Criar `doc.go` documentando contratos exportados **sem mencionar RBAC/JWT/sessions/is_admin** (F-12 do PRD). Editar `cmd/server/server.go` para instanciar o módulo após inicialização do `httpserver` e registrar `UserRouter` se não-nil. `cmd/worker/worker.go` permanece sem alteração (refactor outbox é tarefa 10.0).

<requirements>
- RF-17: `internal/identity/doc.go` documenta contratos exportados sem nenhuma menção a `JWT|RBAC|role|is_admin|session`. F-12 satisfeita por inexistência ativa.
- RF-18: `NewIdentityModule(cfg, o11y, mgr) IdentityModule` segue o Padrão (sem `opts ...Option`, sem `With...`).
- ADR-005: campos `RepositoryFactory interfaces.RepositoryFactory` e `UserRouter *server.UserRouter` (este último nil no MVP — bootstrap só registra se != nil).
- Slots vazios materializados com `doc.go` placeholder (cada subpasta declarada como reservada).
- `cmd/server/server.go` ganha: `identityModule := identity.NewIdentityModule(cfg, o11y, dbManager)` + `if identityModule.UserRouter != nil { srv.RegisterRouters(identityModule.UserRouter) }`.
- O módulo internamente cria `upsertUoW := uow.New[entities.User](mgr, uow.WithObservability(o11y))`, `markDeletedUoW := uow.NewVoid(mgr, uow.WithObservability(o11y))`, instancia UCs e handler, embrulha no `UserRouter`.
</requirements>

## Subtarefas

- [ ] 8.1 `internal/identity/module.go`:
  - Struct `IdentityModule{ RepositoryFactory: interfaces.RepositoryFactory, UserRouter: *server.UserRouter }`.
  - `NewIdentityModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) IdentityModule` que instancia factory, UoWs tipados, UCs, handler, `UserRouter`.
- [ ] 8.2 `internal/identity/doc.go` — comentário de pacote descrevendo: agregado `User`, port `RepositoryFactory`, `IsEntitled` puro, `entities.NewID` autossuficiente. **Zero menções** a RBAC/JWT/sessions/is_admin (verificado por `grep` posterior em 9.0).
- [ ] 8.3 `internal/identity/infrastructure/messaging/database/consumers/doc.go` (placeholder).
- [ ] 8.4 `internal/identity/infrastructure/messaging/database/producers/doc.go` (placeholder).
- [ ] 8.5 Editar `cmd/server/server.go`:
  - Adicionar imports: `internal/identity`.
  - Após `srv, err := httpserver.New(...)`, inserir: `identityModule := identity.NewIdentityModule(cfg, o11y, dbManager)` e bloco `if identityModule.UserRouter != nil { srv.RegisterRouters(identityModule.UserRouter) }`.
  - Linha de log informativa antes de `srv.Start(ctx)`: `o11y.Logger().Info(ctx, "identity module wired", observability.Bool("router_registered", identityModule.UserRouter != nil))`.
- [ ] 8.6 Validar visualmente que `cmd/worker/worker.go` **não é alterado** nesta tarefa.

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §13](./techspec.md) — bootstrap delta canônico.
- [Runbook §9 + §10](../../docs/runbooks/handler-usecase-uow-repository.md) — module.go + bootstrap.
- [ADR-005](./adr-005-identity-module-shape-mvp.md) — shape final aprovado.

**Imports inegociáveis em `module.go`:**

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/database/manager"
    "github.com/JailtonJunior94/devkit-go/pkg/database/uow"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"

    "github.com/LimaTeixeiraTecnologia/mecontrola/configs"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)
```

**Construtor canônico:**

```go
func NewIdentityModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) IdentityModule {
    factory := repositories.NewRepositoryFactory(o11y)

    upsertUoW := uow.New[entities.User](mgr, uow.WithObservability(o11y))
    upsertUC := usecases.NewUpsertUserByWhatsApp(upsertUoW, factory, o11y)

    markDeletedUoW := uow.NewVoid(mgr, uow.WithObservability(o11y))
    markDeletedUC := usecases.NewMarkUserDeleted(markDeletedUoW, factory, o11y)
    _ = markDeletedUC // composto em handlers futuros

    upsertHandler := handlers.NewUpsertUserByWhatsAppHandler(upsertUC, o11y)

    return IdentityModule{
        RepositoryFactory: factory,
        UserRouter:        server.NewUserRouter(upsertHandler),
    }
}
```

## Critérios de Sucesso

- `go build ./...` verde.
- `go vet ./...` verde.
- `cmd/server` sobe com `go run ./cmd server` sem erro (smoke local manual).
- `grep -RInE "JWT|RBAC|\\brole\\b|is_admin|session" internal/identity/**/*.go` retorna 0 matches (CA-03).
- `grep -RInE "IDGenerator|idGen[^a-zA-Z_]|internal/platform/uow" internal/identity/` retorna 0 (gate de aderência ao runbook).
- Construtor `NewIdentityModule` tem exatamente 3 parâmetros (cfg, o11y, mgr).
- `module.go` não usa `_ = identityModule.RepositoryFactory` no escopo interno (factory já é exposto via campo).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Compilação completa: `go build ./...` verde.
- [ ] Smoke do `cmd server`: subir com Postgres local; observar log "identity module wired" + `router_registered=true` (ou `false` se handler não for instanciado por algum motivo).
- [ ] `module_test.go` (opcional, sem build tag) — `NewIdentityModule` com `noop.NewProvider()` + `manager.Manager` stub não panica e devolve struct com `RepositoryFactory != nil`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/module.go` (criar)
- `internal/identity/module_test.go` (criar — opcional)
- `internal/identity/doc.go` (criar)
- `internal/identity/infrastructure/messaging/database/consumers/doc.go` (criar — placeholder)
- `internal/identity/infrastructure/messaging/database/producers/doc.go` (criar — placeholder)
- `cmd/server/server.go` (editar — inserir wiring)
- Dependências (já criadas em 4.0–7.0): factory, UCs, handler, router, ports.

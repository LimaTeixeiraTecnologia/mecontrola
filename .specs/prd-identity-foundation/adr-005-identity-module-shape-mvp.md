# ADR-005 — `IdentityModule` shape no MVP de E1

## Metadados

- **Título:** Forma do construtor e da struct retornada por `NewIdentityModule`
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** Time MeControla (owner: Jailton Junior)
- **Relacionados:**
  - PRD: [`prd.md`](./prd.md) — RF-18, Q-05, F-06, F-07
  - Tech Spec: [`techspec.md`](./techspec.md)
  - PRD Q em aberto fechada: **Q-05**
  - `AGENTS.md`: seção "Padrão Obrigatório de Módulo" (itens 1–7).

## Contexto

O PRD exige (RF-18) que `internal/identity/module.go` exponha `NewIdentityModule(...)` seguindo o Padrão Obrigatório de Módulo de `AGENTS.md`. O MVP de E1 entrega:

- agregado `User` + VOs;
- `UserRepository` (port em `application`) + implementação postgres;
- helper de mascaramento de PII;
- `IsEntitled` puro no domínio;
- `Subscription` contrato mínimo.

**Não entrega rotas HTTP, jobs, consumers nem producers** — esses são consumidos por E2/E3.

Restrições de `AGENTS.md`:

- Item 1: construtor nomeado `NewIdentityModule(...) IdentityModule`.
- Item 2: struct expõe apenas dependências reais para bootstrap ou outros módulos.
- Item 3: ordem `repository/client → usecase → handler → router/job/consumer/producer`.
- Item 4: routers implementam `Register(router chi.Router)` e só são registrados quando houver rotas reais.
- Item 5: jobs/consumers via adapters de `internal/platform/worker`.
- Item 6: proibido `NewModule(opts...)`, `WithDatabase(...)`, `Routers()` ou `Runners()`.
- Item 7: verificar existência antes de criar wiring.

`cmd/server/server.go` hoje **não chama** `srv.RegisterRouters(...)` (zero módulos de negócio). O método existe na API de `httpserver` do devkit-go v0.4.0, mas seu uso real só aparecerá quando o primeiro módulo for plugado.

E2 e E3 vão importar o port `UserRepository` para fazer upsert/lookup; precisam de uma forma estável de obter a implementação real.

## Decisão

`internal/identity/module.go` exporta:

```go
package identity

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

type IdentityModule struct {
    // RepositoryFactory devolve instâncias de repositório amarradas à database.DBTX
    // recebida (pool, ou tx vindo do callback de uow.UnitOfWork[T].Do).
    // Consumido por use cases deste módulo e por E2/E3 quando precisarem ler/escrever
    // identidade dentro de transações próprias.
    //
    // Regra R6 — interface no consumidor de identity: E2/E3 declaram a sua interface
    // mínima localmente e a satisfazem com este campo.
    RepositoryFactory interfaces.RepositoryFactory

    // UserRouter é nil no MVP de E1 (sem rotas). Bootstrap em cmd/server só registra
    // se != nil (item 4 do Padrão). Placeholder *server.UserRouter existe para
    // garantir compilação (ver §3 do techspec e ADR).
    UserRouter *server.UserRouter
}

// NewIdentityModule segue o Padrão Obrigatório de Módulo (AGENTS.md itens 1–7).
//
// Recebe manager.Manager — não recebe UnitOfWork solto. Cada use case do módulo
// recebe um uow.UnitOfWork[T] tipado pelo retorno do seu callback (T = entities.User,
// []events.Envelope, struct{} via NewVoid, etc.). Sem IDGenerator na assinatura:
// o domínio gera o próprio ID via entities.NewID() dentro dos construtores
// (ver §5.1 do runbook).
func NewIdentityModule(
    cfg *configs.Config,
    o11y observability.Observability,
    mgr manager.Manager,
) IdentityModule {
    factory := repositories.NewRepositoryFactory(o11y)

    upsertUoW := uow.New[entities.User](mgr, uow.WithObservability(o11y))
    upsertUC := usecases.NewUpsertUserByWhatsApp(upsertUoW, factory, o11y)

    markDeletedUoW := uow.NewVoid(mgr, uow.WithObservability(o11y))
    markDeletedUC := usecases.NewMarkUserDeleted(markDeletedUoW, factory, o11y)

    upsertHandler := handlers.NewUpsertUserByWhatsAppHandler(upsertUC, o11y)
    // … demais handlers consomem markDeletedUC etc.

    return IdentityModule{
        RepositoryFactory: factory,
        UserRouter:        server.NewUserRouter(upsertHandler),
    }
}
```

**Wiring em `cmd/server/server.go`:**

```go
identityModule := identity.NewIdentityModule(cfg, o11y, dbManager)
if identityModule.UserRouter != nil {
    srv.RegisterRouters(identityModule.UserRouter) // só registra quando houver rotas reais
}
_ = identityModule.RepositoryFactory // exposto para E2/E3 quando consumirem identity
```

**Wiring em `cmd/worker/worker.go`:** **nenhum** — identity não tem job, consumer ou producer no MVP. A lista `jobs` continua somente com outbox (que neste épico também é refatorado para o padrão UoW + factory — ver §17 do techspec).

## Alternativas Consideradas

### A) `IdentityModule` retorna ponteiro (`*IdentityModule`)

- **Vantagens:** convenção comum em Go quando há mutabilidade.
- **Desvantagens:**
  - A struct é imutável após construção; ponteiro só serve para alias.
  - Diverge do shape descrito em `AGENTS.md` ("estrutura concreta").
- **Motivo de não escolher:** struct por valor é mais idiomática para módulo composto na inicialização.

### B) Não expor `UserRepository` na struct

- **Vantagens:** mantém superfície mínima.
- **Desvantagens:**
  - E2 e E3 precisariam reinstanciar (duplicação de wiring) ou criar um getter.
  - Item 2 do Padrão: "expõe apenas dependências reais necessárias **ao bootstrap ou a outros módulos**" — outros módulos precisam.
- **Motivo de não escolher:** acoplamento desnecessário no bootstrap; E2/E3 ficariam com wiring duplicado.

### C) Adiar `NewIdentityModule(...)` até E2/E3 chegarem

- **Vantagens:** evita módulo vazio.
- **Desvantagens:**
  - Viola RF-18: "deve expor `NewIdentityModule(...)` seguindo o Padrão".
  - Adia decisão e força E2 a abrir o caminho.
- **Motivo de não escolher:** RF-18 é explícito.

### D) Expor `Repository()` getter em vez de campo

- **Vantagens:** permite versionar a forma de exposição.
- **Desvantagens:**
  - Diverge do `InvoiceModule` (campos nomeados) descrito em `AGENTS.md`.
  - Encoraja getters extras que viram débito.
- **Motivo de não escolher:** inconsistente com o padrão.

## Consequências

### Benefícios Esperados

- **Bootstrap minimal e correto** para o MVP.
- **E2 e E3 plugam sem reescrever wiring.**
- **Alinhamento total com o Padrão Obrigatório de Módulo.**
- **Crescimento incremental:** novos campos (`UserRouter`, `OnboardingConsumer`, etc.) viram extensões aditivas sem quebrar consumidores.

### Trade-offs e Custos

- `UserRouter *server.UserRouter` permanece `nil` no MVP — alguns lints podem reclamar de campo nunca preenchido (mitigável com tag/comentário ou `_ = identityModule.UserRouter` no bootstrap).
- `IdentityModule` exporta `RepositoryFactory interfaces.RepositoryFactory` (interface declarada em `application/interfaces`) — o teste de compatibilidade fica responsabilidade de E2/E3, que declaram suas próprias interfaces mínimas e as satisfazem com este campo.
- Cada use case carrega seu próprio `uow.UnitOfWork[T]` tipado — o module instancia um `uow.New[T](mgr, uow.WithObservability(o11y))` por UC, multiplicando linhas de bootstrap interno em troca de type-safety no retorno do callback. Trade-off explicitamente aceito (ver runbook §9).

### Riscos e Mitigações

- **Risco:** E2 ou E3 acoplarem-se a `*postgres.userRepository` concreto.
  - **Mitigação:** `RepositoryFactory` é exposto como `interfaces.RepositoryFactory` (interface no consumidor de identity) e devolve `interfaces.UserRepository` (também interface). E2/E3 declaram **sua própria** interface mínima e a satisfazem chamando `identityModule.RepositoryFactory.UserRepository(tx)` dentro de seus UoW callbacks. Nenhum consumidor enxerga o tipo concreto Postgres.
- **Risco:** wiring em `cmd/server` esquece de checar `if module.UserRouter != nil` quando rotas chegarem.
  - **Mitigação:** convenção documentada em comentário no campo e em ADR-005 (este).
- **Risco:** `cmd/worker/worker.go` precisar de jobs/consumers identity em E3 e o bootstrap não estar preparado.
  - **Mitigação:** quando E3 adicionar consumer/job, `IdentityModule` ganha campos novos (`Consumers []consumer.Consumer`, `Jobs []worker.Job`) e o worker passa a appendá-los — mudança aditiva.

## Plano de Implementação

1. Criar `internal/identity/application/interfaces/user_repository.go` com o port `UserRepository`.
2. Criar `internal/identity/application/interfaces/repository_factory.go` com o port `RepositoryFactory` (ver ADR-008).
3. Criar `internal/identity/infrastructure/repositories/postgres/user_repository.go` consumindo `internal/platform/sqlnull` (ver §7.1 do runbook).
4. Criar `internal/identity/infrastructure/repositories/factory.go` com `NewRepositoryFactory(o11y) interfaces.RepositoryFactory`.
5. Criar `internal/identity/infrastructure/http/server/router.go` com o tipo `UserRouter` placeholder + `Register(chi.Router)` vazio (slot do Padrão, item 4).
6. Criar `internal/identity/module.go` com `IdentityModule` e `NewIdentityModule(cfg, o11y, mgr)` conforme decidido.
7. Atualizar `cmd/server/server.go` para instanciar o módulo via `identity.NewIdentityModule(cfg, o11y, dbManager)` e (futuramente) registrar router quando existir.
8. Não alterar `cmd/worker/worker.go` neste MVP (refactor outbox em §17 do techspec entra como sub-épico paralelo).
9. Teste de compilação: `go build ./...` deve passar; testes unitários verdes com mocks de `UnitOfWork[T]` + `RepositoryFactory`.

## Monitoramento e Validação

- **Validação imediata:** `go build ./...` e `go vet ./...`.
- **Validação cross-module:** quando E2 chegar, validar que `identityModule.RepositoryFactory.UserRepository(tx)` satisfaz a interface declarada em `internal/billing/application/interfaces/...` sem cast adicional, e que o repo devolvido respeita a TX recebida (sem `BeginTx` próprio).
- **Sinal de drift:** se `module.go` começar a aceitar `opts ...Option` ou `With...(...)`, voltar ao Padrão (item 6 é explícito).

## Impacto em Documentação e Operação

- `internal/identity/doc.go` documenta o construtor e o contrato exportado.
- `AGENTS.md` permanece como fonte canônica do padrão — este ADR só fixa as escolhas do MVP de E1.
- Quando router/jobs/consumers entrarem, este ADR não precisa ser revisado — extensão é aditiva.

## Revisão Futura

- Revisitar quando o primeiro router HTTP de identity for adicionado (E3 provavelmente).
- Revisitar se `IdentityModule` precisar expor mais de 4–5 campos (sinal de que o módulo está absorvendo responsabilidade demais — abrir sub-módulo).
- Revisitar quando E2 plugar `Subscription` real e a integração cross-module amadurecer.

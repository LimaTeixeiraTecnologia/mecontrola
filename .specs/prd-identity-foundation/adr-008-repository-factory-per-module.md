# ADR-008 — `RepositoryFactory` por módulo + `UnitOfWork[T]` da devkit-go direto

## Metadados

- **Título:** Padrão canônico para composição de repositórios em transações multi-agregado
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** Time MeControla (owner: Jailton Junior)
- **Relacionados:**
  - PRD: [`prd.md`](./prd.md) — RF-10, RF-11
  - Tech Spec: [`techspec.md`](./techspec.md) — §Application, §10, §13, §17
  - Runbook: [`docs/runbooks/handler-usecase-uow-repository.md`](../../docs/runbooks/handler-usecase-uow-repository.md) — §3, §4.1, §5, §6, §7, §7.1, §9, §11
  - ADRs: [ADR-005](./adr-005-identity-module-shape-mvp.md) (shape do módulo)
  - `AGENTS.md`: "Padrão Obrigatório de Módulo" (itens 1–7).
  - Devkit: `github.com/JailtonJunior94/devkit-go/pkg/database/uow`

## Contexto

Use cases reais orquestram **mais de um repositório dentro da mesma transação**. Exemplo concreto de E1: ao trocar o número de WhatsApp de um usuário, o use case precisa, atomicamente:

1. `UPDATE users SET whatsapp_number = ... WHERE id = $1` (UserRepository).
2. `UPDATE user_whatsapp_history SET active = false, unlinked_at = now() WHERE user_id = $1 AND active = true` (WhatsAppHistoryRepository).
3. `INSERT INTO user_whatsapp_history (...) VALUES (...)` (WhatsAppHistoryRepository).
4. Em E2+: opcional `INSERT INTO outbox_events (...)` (OutboxRepository).

Se cada repositório iniciar sua própria transação via `manager.Manager.BeginTx`, as quatro operações ficam em TXs independentes e podem fragmentar consistência. O padrão precisa permitir que **um único `tx`** seja compartilhado entre N repositórios diferentes, com lifecycle gerenciado em um único ponto (o use case).

A devkit-go v0.4.0 oferece em `pkg/database/uow` uma `UnitOfWork[T any]` genérica:

```go
type UnitOfWork[T any] interface {
    Do(ctx context.Context, fn func(ctx context.Context, tx database.DBTX) (T, error), opts ...Option) (T, error)
}
```

O callback recebe `tx database.DBTX`. Falta, no entanto, um mecanismo idiomático para **instanciar repositórios já amarrados a essa `tx`** sem que o use case conheça os tipos concretos Postgres.

### Restrições

- **R6** (interface no consumidor): use cases não podem importar implementações Postgres.
- **R6.4**: sem `var _ Interface = (*Type)(nil)` em produção.
- **R6.7**: sem `Clock` abstrato; `time.Now().UTC()` inline no call-site.
- **AGENTS.md** "Padrão Obrigatório de Módulo": construtor único `NewXModule(...)`, sem `opts ...Option`.
- Decisão prévia (rodada 1 da techspec): repos recebem `database.DBTX` no construtor; **proibido** passar `tx` como argumento de método.
- Decisão prévia (Iteração 2 do runbook): proibido injetar `IDGenerator` em qualquer camada — domínio se auto-serve via `entities.NewID()`.

## Decisão

Cada módulo expõe um **`RepositoryFactory`** (port em `application/interfaces/`) cuja única responsabilidade é, dada uma `database.DBTX`, devolver instâncias dos repositórios do módulo já amarradas a ela:

```go
// internal/identity/application/interfaces/repository_factory.go
package interfaces

import "github.com/JailtonJunior94/devkit-go/pkg/database"

type RepositoryFactory interface {
    UserRepository(db database.DBTX) UserRepository
    // WhatsAppHistoryRepository(db database.DBTX) WhatsAppHistoryRepository  // E1+ quando entrar
}
```

Implementação em `infrastructure/repositories/factory.go`:

```go
package repositories

import (
    "github.com/JailtonJunior94/devkit-go/pkg/database"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
)

type repositoryFactory struct {
    o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) interfaces.RepositoryFactory {
    return &repositoryFactory{o11y: o11y}
}

func (r *repositoryFactory) UserRepository(db database.DBTX) interfaces.UserRepository {
    return postgres.NewUserRepository(r.o11y, db)
}
```

Use cases consomem `uow.UnitOfWork[T]` (da devkit, **proibido** reimplementar) + `RepositoryFactory`. Dentro do callback do UoW, podem criar quantos repos forem necessários, todos amarrados ao mesmo `tx`:

```go
result, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.User, error) {
    userRepo    := u.factory.UserRepository(tx)
    historyRepo := u.factory.WhatsAppHistoryRepository(tx)

    // operações em ambos os repos → mesma TX → atomicidade garantida pelo UoW
    if err := userRepo.UpdateWhatsAppNumber(ctx, userID, newNumber, time.Now().UTC()); err != nil {
        return entities.User{}, fmt.Errorf("%s update: %w", prefix, err)
    }
    entry := entities.NewWhatsAppHistoryEntry(userID, oldNumber, "port_in")
    if err := historyRepo.Append(ctx, userID, entry); err != nil {
        return entities.User{}, fmt.Errorf("%s append: %w", prefix, err)
    }
    return userRepo.FindByID(ctx, userID)
})
```

**Garantias:**

- Atomicidade real entre repositórios diferentes do mesmo módulo (rollback automático em qualquer erro do callback).
- Tipos concretos Postgres ficam confinados em `infrastructure/repositories/postgres/`; nenhum UC, factory port ou módulo importa.
- `uow.UnitOfWork[T]` da devkit já oferece span `db.{driver}.tx`, métricas `database.tx.{duration_ms,committed,rolledback}`, panic-safe, reentrância guardada por goroutine ID, propagação dupla do `tx` (callback param + `database.WithTx(ctx, tx)`).
- Mock de UoW + Factory em testes unitários: callback executa inline (`return fn(ctx, nil)`), factory mock devolve repo mock independente do `tx`.

## Alternativas Consideradas

### A) Cada método do repo recebe `tx database.DBTX` como parâmetro

```go
type UserRepository interface {
    Upsert(ctx context.Context, tx database.DBTX, u entities.User, now time.Time) (entities.User, error)
    // ... idem para todos
}
```

- **Vantagens:** explícito; sem factory; menos um arquivo por módulo.
- **Desvantagens:**
  - Assinatura cresce em todo método (5 métodos × 1 param extra).
  - UC precisa decidir entre "tx ou pool" em cada chamada — vazio sem TX vira `mgr.DBTX(ctx)` no call-site, espalhando o cuidado.
  - Mocks ficam ruidosos (`mock.AnythingOfType("database.DBTX")` em cada call).
  - Quebra a regra "construtor amarra ao db" — agora cada método precisa relembrar.
- **Motivo de não escolher:** API mais verbosa, ergonomia pior, sem ganho real.

### B) Repo usa `database.FromContext(ctx)` internamente

```go
func (r *userRepository) Upsert(ctx context.Context, ...) {
    db, _ := database.FromContext(ctx)
    if db == nil { db = r.pool }
    // ...
}
```

- **Vantagens:** nenhum parâmetro extra, propagação via ctx.
- **Desvantagens:**
  - "Magia" oculta — leitor precisa lembrar que o ctx pode carregar tx.
  - Testes ficam frágeis (precisa montar ctx com `database.WithTx`).
  - Repos crescem com fallback `if db == nil` em todo método.
  - Mistura responsabilidade do UoW (gerenciar tx) com responsabilidade do repo (operar IO).
- **Motivo de não escolher:** dependência implícita; ergonomia de teste sofrível.

### C) Reimplementar `UnitOfWork` em `internal/platform/uow/`

Wrappar `manager.Manager.BeginTx` + `Commit`/`Rollback` + tracer em um pacote interno.

- **Vantagens:** controle total sobre semântica.
- **Desvantagens:**
  - Reimplementa o que a devkit já oferece com qualidade superior (panic-safe, métricas, span automático, reentrância via goroutine ID).
  - Adiciona código a manter sem benefício.
  - Diverge do contrato canônico da plataforma compartilhada (devkit-go).
- **Motivo de não escolher:** duplicação proibida; consumir a devkit é estritamente melhor.

### D) Um `RepositoryFactory` transversal único em `internal/platform/`

Em vez de uma factory por módulo, uma factory global que conhece todos os repos do projeto.

- **Vantagens:** ponto único de instanciação.
- **Desvantagens:**
  - Acoplamento massivo — `internal/platform/repositories` importa todos os módulos, invertendo a dependência.
  - Cada módulo novo exige PR na plataforma.
  - Viola R6 (interface no consumidor por módulo, não transversal).
- **Motivo de não escolher:** quebra fronteiras hexagonais.

## Consequências

### Benefícios esperados

- **Atomicidade trivial** entre N repositórios do mesmo módulo via um único `uow.Do`.
- **Testes unitários limpos**: mock de `RepositoryFactory` devolve mock de repo; mock de `UnitOfWork[T]` executa callback inline. Sem stubs de TX.
- **Tipos concretos Postgres confinados** ao subpacote `postgres/`. Migrar para outro driver (CockroachDB, MySQL) é troca cirúrgica do impl da factory.
- **Crescimento aditivo:** novo repo no módulo = +1 método no port `RepositoryFactory` + +1 método na impl. Use cases existentes não mudam.
- **Sem `init()` (R0), sem `panic` (R5.12), sem `Clock` (R6.7), sem `IDGenerator` injetado.** Tudo aderente.

### Trade-offs e custos

- **+1 nível de indireção** entre UC e repo concreto. Mitigado pelo ganho em testabilidade e atomicidade.
- **Cada UC carrega `uow.UnitOfWork[T]` tipado** — pode haver múltiplos UoWs no module (1 por UC). Linhas extras de bootstrap interno; trade-off aceito em troca de type-safety no retorno.
- **`factory.UserRepository(tx)` é chamado dentro do callback** — alocação por invocação de UC. Custo desprezível (struct de 2 campos); benchmarks confirmam < 1 μs.

### Riscos e mitigações

- **Risco:** factory port cresce sem controle conforme módulo adiciona repos.
  - **Mitigação:** quando passar de 5 repos no mesmo `RepositoryFactory`, sinal de que o módulo absorveu responsabilidade demais — abrir sub-módulo (ex.: separar `identity` de `identity-history`).
- **Risco:** UC esquece de chamar `factory.UserRepository(tx)` e usa um repo "stale" do escopo externo.
  - **Mitigação:** convenção: **proibido** o módulo armazenar instâncias de repo como campo. Factory é único ponto de instanciação. Code review + lint custom (futuro) podem reforçar.
- **Risco:** repo amarrado a `tx` é vazado para fora do callback (escape).
  - **Mitigação:** repo é struct com `db database.DBTX`; após `Commit`/`Rollback`, qualquer chamada subsequente devolve erro do próprio driver (`tx already closed`). Falha rápida, observável.
- **Risco:** consumidor cross-module (E2) usa `identityModule.RepositoryFactory.UserRepository(pool)` para reads simples e esquece a TX em escritas.
  - **Mitigação:** E2 declara sua própria interface mínima (R6) que pode forçar `tx` no construtor; convenção do repo: writes que precisem de TX falham cedo se chamado fora de `uow.Do` (futuro: detectar via `database.FromContext`).

## Plano de Implementação

1. Criar `internal/identity/application/interfaces/repository_factory.go` com o port `RepositoryFactory` (apenas `UserRepository` no MVP de E1; expandir conforme novos repos entram).
2. Criar `internal/identity/infrastructure/repositories/factory.go` com `NewRepositoryFactory(o11y observability.Observability) interfaces.RepositoryFactory`.
3. Use cases (`UpsertUserByWhatsApp`, `FindUserByID`, `FindUserByWhatsApp`, `MarkUserDeleted`) recebem `uow.UnitOfWork[T]` + `interfaces.RepositoryFactory` no construtor.
4. `NewIdentityModule(cfg, o11y, mgr)` (ADR-005) instancia 1 `uow.New[T]` por UC.
5. **Refactor outbox** (sub-épico de E1, §17 do techspec):
   - Criar `internal/platform/outbox/ports.go` com `OutboxRepositoryFactory` + port `OutboxRepository`.
   - Migrar `internal/platform/outbox/storage_postgres.go` para receber `database.DBTX` no construtor (sem `manager.Manager`).
   - `internal/platform/outbox/dispatcher.go` (job) recebe `uow.UnitOfWork[[]outbox.Row]` + `OutboxRepositoryFactory`; `ClaimBatch` + `MarkProcessed` dentro do mesmo `uow.Do`.
   - `cmd/worker/worker.go` instancia os UoWs do outbox e passa para os jobs.
6. Gerar mocks via `mockery` para `RepositoryFactory` e `UnitOfWork[T]` (suporte a genéricos exige `mockery v2.30+`).
7. Validar com `go build ./... && go vet ./... && go test -race -count=1 ./...`.

## Monitoramento e Validação

- **Span hierarchy correta:** `handler.upsert_user_by_whatsapp` → `usecase.upsert_user_by_whatsapp` → `db.postgres.tx` (devkit-uow) → `repository.user.upsert_by_whatsapp_number`. Validar em traces de smoke.
- **Métricas:** `database.tx.committed{module="identity"}` deve crescer 1:1 com sucessos do UC.
- **Sinal de drift:** se algum método de repo crescer para receber `tx database.DBTX` como parâmetro, voltar ao padrão (alternativa A foi rejeitada).
- **Sinal de drift:** se algum UC criar `manager.Manager.BeginTx` direto, voltar ao padrão (UoW é único ponto).
- **Sinal de drift:** se algum repo armazenar `manager.Manager` em vez de `database.DBTX`, voltar ao padrão.

## Impacto em Documentação e Operação

- Runbook `docs/runbooks/handler-usecase-uow-repository.md` é a fonte de verdade canônica do padrão (referenciada por todos os módulos novos).
- Techspec E1 §10 + §17 refletem a decisão.
- `AGENTS.md` "Padrão Obrigatório de Módulo" permanece como fonte do shape do module; este ADR fixa a composição interna.
- Para módulos futuros (E2 billing, E3 onboarding): replicar o padrão com adaptações mínimas (`BillingRepositoryFactory`, `OnboardingRepositoryFactory`).

## Revisão Futura

- Revisitar quando o primeiro módulo precisar de **transação distribuída** (Postgres + outro storage) — UoW atual não cobre saga; abrir ADR próprio.
- Revisitar se `RepositoryFactory` de algum módulo passar de 5 métodos — sinal de over-aggregation.
- Revisitar se a devkit-go expor uma factory canônica no próprio pacote `uow` — neste caso, migrar para a forma upstream.

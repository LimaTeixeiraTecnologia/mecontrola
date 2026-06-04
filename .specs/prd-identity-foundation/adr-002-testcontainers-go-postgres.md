# ADR-002 — `testcontainers-go/modules/postgres` para testes de integração de identity

## Metadados

- **Título:** Adoção de `testcontainers-go` para testes de integração do `PgxUserRepository`
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia de plataforma + autor do PRD identity
- **Relacionados:** PRD (SQ-02, RF-18, MS-04), techspec §Abordagem de Testes, código existente `internal/platform/outbox/storage_pgx_integration_test.go`

## Contexto

RF-18 exige validação do repositório com Postgres real cobrindo (a) upsert idempotente, (b) soft delete + filtragem, (c) histórico em `LinkNewNumber`. O projeto já adota `testcontainers-go v0.42.0` (dependência direta em `go.mod`) com módulo `postgres` no pacote outbox (`storage_pgx_integration_test.go`), incluindo padrão de `SetupSuite` que provisiona container Alpine e roda migrations via `database.RunMigrations`. Existe paridade com produção (mesma versão de Postgres do `docker-compose`).

## Decisão

Usar `testcontainers-go/modules/postgres` com imagem `postgres:16-alpine` para todos os testes de integração do módulo identity. Build tag `//go:build integration` separa do `go test ./...` padrão. `SetupSuite` provisiona container e aplica `database.RunMigrations`; `SetupTest` faz `TRUNCATE users, user_whatsapp_history CASCADE`. `TearDownSuite` termina o container.

## Alternativas Consideradas

- **`docker-compose` com Postgres compartilhado** — Vantagens: container persistente, iteração local rápida. Desvantagens: depende de operador subir `docker-compose up` antes; flakiness por estado residual entre testes; CI precisa de Docker-in-Docker configurado igual. Rejeitada — testcontainers oferece o mesmo benefício sem requerer setup externo.
- **SQLite in-memory com schema compatível** — Vantagens: testes em segundos sem container. Desvantagens: divergência de SQL dialect (JSONB, índice parcial, `current_setting`), invalida cobertura de constraints Postgres-específicas. Rejeitada — RF-18 exige Postgres real.
- **Mock do repository com `pgxmock`** — Vantagens: rápido. Desvantagens: não valida SQL real, não detecta erro em constraint, não cobre `UNIQUE WHERE deleted_at IS NULL`. Rejeitada — RF-18 inegociável.
- **Mix: testcontainers no CI + docker-compose local** — Rejeitada por duplicar caminhos de boot e divergir comportamento dev/CI.

## Consequências

### Benefícios Esperados

- Paridade total com produção; constraints validadas em SQL real.
- Sem dependência externa de operador; `go test -tags=integration ./...` self-contained.
- Reuso do pattern já consolidado em outbox — onboarding zero.

### Trade-offs e Custos

- Suite ~2s mais lenta que unit (container boot).
- CI exige runner com Docker socket (já o caso no projeto).

### Riscos e Mitigações

- **Risco:** Falha de pull da imagem em ambiente offline.
- **Mitigação:** Cache local Docker; CI usa runner com imagem pré-puxada.
- **Risco:** Vazamento de containers em panic.
- **Mitigação:** `t.Cleanup` + `defer container.Terminate(ctx)` no `SetupSuite` (pattern já validado em outbox).

## Plano de Implementação

1. Criar `internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go` com build tag.
2. `SetupSuite`: `postgres.Run`, capturar `dsn`, criar `database.Manager`, chamar `database.RunMigrations`.
3. `SetupTest`: `TRUNCATE users, user_whatsapp_history CASCADE; ALTER SEQUENCE IF EXISTS ...`.
4. Implementar 6 cenários listados na techspec §Testes de Integração.
5. Adicionar `task test:integration` ou equivalente no `Taskfile.yml` (se ainda não existir filtragem por tag).

## Monitoramento e Validação

- CI: job dedicado `go test -tags=integration -race ./internal/identity/...`.
- Falha rápida (<60s suite total) é critério de aceitação.

## Impacto em Documentação e Operação

- `internal/identity/README.md` ganha seção "Como rodar testes de integração".

## Revisão Futura

Se suite de integração crescer >5 min ao longo do roadmap, reavaliar paralelização ou compartilhamento de container entre suites com isolamento por schema.

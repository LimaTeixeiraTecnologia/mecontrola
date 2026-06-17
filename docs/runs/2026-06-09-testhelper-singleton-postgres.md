# Plano: Helper Singleton Testcontainer PostgreSQL Reutilizável

## Contexto

Hoje existem **13 arquivos de teste de integração** que duplicam código para criar containers PostgreSQL via testcontainers-go. Cada suite (ou pior, cada cenário dentro de uma suite) cria seu próprio container do zero, o que torna a execução lenta e consome recursos desnecessariamente.

O objetivo é criar um helper **singleton** em `internal/platform/testcontainer` que inicie **um único container PostgreSQL** por execução de `go test`, e para cada chamada crie um **novo banco de dados isolado** dentro desse container. Isso mantém o isolamento entre suites/testes mas elimina o custo de criar/destrir múltiplos containers.

## Abordagem

### 1. Criar `internal/platform/testcontainer/postgres.go`

Helper package com API mínima:

```go
package testcontainer

func Postgres(t *testing.T) (manager.Manager, string)
```

Comportamento:
- Usa `sync.Once` para iniciar **um único container** `postgres:16` por execução de teste.
- Usa `atomic.Int64` para gerar nomes de banco únicos (`testdb_1`, `testdb_2`, ...).
- Para cada chamada:
  1. Conecta ao Postgres como superuser no container.
  2. Cria novo banco com nome único.
  3. Instancia `manager.New` apontando para o novo banco.
  4. Roda todas as migrations via `migration.New` + `migrator.Up`.
  5. Registra `t.Cleanup` para:
     - `mgr.Shutdown`
     - `DROP DATABASE` do banco criado.
- Retorna `manager.Manager` e DSN no formato `pgx5://`.
- Em caso de falha no container, chama `t.Fatalf`.

Restrições do projeto aplicadas:
- Zero comentários (R-ADAPTER-001.1).
- Sem `init()` (R0).
- Toda função pública é método de struct ou construtor (R1). Aqui, `Postgres` é a única função package-level permitida por ser um construtor/factory de infraestrutura de teste.
- Sem `panic` (R5.12).
- `context.Context` em toda fronteira de IO (R6).
- Usar `any` em vez de `interface{}` (R7.1).
- `errors.Join` para agregar erros (R7.6).

### 2. Refatorar todos os arquivos de teste de integração

Os 13 arquivos listados abaixo possuem código duplicado de `setupTestDB`/`setupOutboxDB`/`setupIntegrationDB` etc. Cada um deve ser alterado para:

1. Remover imports de `testcontainers-go`, `testcontainers-go/wait`, `strconv` e constantes de imagem (quando não usados em outro lugar).
2. Remover a função local de setup do container.
3. Importar `github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer`.
4. Substituir a chamada local por `testcontainer.Postgres(t)`.

#### Arquivos a modificar

| # | Arquivo | Função local a remover | Retorno |
|---|---------|------------------------|---------|
| 1 | `internal/identity/infrastructure/repositories/postgres/testutil_test.go` | `setupTestDB` | `(manager.Manager, string)` |
| 2 | `internal/categories/infrastructure/repositories/postgres/testutil_test.go` | `setupTestDB` | `manager.Manager` |
| 3 | `internal/billing/infrastructure/repositories/postgres/testutil_test.go` | `setupTestDB` | `manager.Manager` |
| 4 | `internal/platform/outbox/storage_postgres_integration_test.go` | `setupOutboxDB` | `manager.Manager` |
| 5 | `migrations/migrations_integration_test.go` | setup inline em `SetupSuite` | `manager.Manager` + DSN em struct |
| 6 | `internal/identity/application/usecases/establish_principal_integration_test.go` | `setupEstablishTestDB` | `manager.Manager` |
| 7 | `internal/identity/application/usecases/mark_user_deleted_integration_test.go` | setup local | `manager.Manager` |
| 8 | `internal/identity/infrastructure/jobs/handlers/auth_events_housekeeping_integration_test.go` | `setupJobIntegrationDB` | `manager.Manager` |
| 9 | `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer_integration_test.go` | setup local | `manager.Manager` |
| 10 | `internal/billing/infrastructure/jobs/handlers/reconciliation_integration_test.go` | `setupIntegrationDB` | `manager.Manager` |
| 11 | `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_integration_test.go` | setup local | `manager.Manager` |
| 12 | `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher_integration_test.go` | setup local | `manager.Manager` |
| 13 | `internal/platform/whatsapp/dispatcher/dispatcher_integration_test.go` | setup local | `manager.Manager` |

### 3. Variações a preservar

- `categories` usa `postgres:16-alpine` hoje. O helper usará `postgres:16`. Se os testes de `categories` dependerem de diferenças do Alpine (extensões), ajustamos a imagem no helper para `postgres:16-alpine` ou usamos `postgres:16` e verificamos. Dado que o Alpine é apenas uma versão lighter e os testes de categories já passam com ele, `postgres:16` deve ser compatível (a extensão `unaccent` faz parte do `postgresql-contrib` que está presente na imagem oficial).
- `identity/repositories/testutil_test.go` retorna `(manager.Manager, string)` enquanto os outros retornam apenas `manager.Manager`. O helper sempre retornará `(manager.Manager, string)`; callers que não precisam do DSN ignoram o segundo valor.
- `migrations/migrations_integration_test.go` armazena `mgr` e `dsn` na struct da suite. O helper cobre isso.
- `platform/outbox` cria um banco fresco a cada `s.Run()`. Com o helper, cada `s.Run()` ainda criará um novo banco (muito mais rápido que container), mantendo o isolamento.

### 4. Verificação

Após implementação:
1. `go build ./internal/platform/testcontainer/...`
2. `go vet ./internal/platform/testcontainer/...`
3. Rodar **todos** os testes de integração com a build tag:
   ```
   go test -tags=integration -race -count=1 ./migrations/...
   go test -tags=integration -race -count=1 ./internal/identity/...
   go test -tags=integration -race -count=1 ./internal/categories/...
   go test -tags=integration -race -count=1 ./internal/billing/...
   go test -tags=integration -race -count=1 ./internal/platform/...
   ```
4. Gate R-ADAPTER-001.1 (zero comentários):
   ```bash
   grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
     "^[[:space:]]*//" internal/platform/testcontainer/ \
     | grep -Ev "(//go:|//nolint:|// Code generated)" \
     && echo "FAIL" && exit 1 || true
   ```
5. Gate R0 (sem init):
   ```bash
   grep -rn "^func init()" --include="*.go" internal/platform/testcontainer/
   ```

# Tarefa 9.0: Testes de integração com `testcontainers-go/postgres`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar suite de testes de integração para `PgxUserRepository` em `internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go` com build tag `//go:build integration`. `SetupSuite` provisiona `postgres:16-alpine` via `testcontainers-go/modules/postgres`, aplica todas as migrations (`database.RunMigrations`), captura DSN. `SetupTest` faz `TRUNCATE users, user_whatsapp_history CASCADE` para isolar cenários. `TearDownSuite` termina o container. Cobre os cenários de RF-18 e MS-04.

<requirements>
- RF-18: validar com Postgres real (a) upsert idempotente por `whatsapp_number`, (b) soft delete + invisibilidade em leitura, (c) registro em `user_whatsapp_history` ao `LinkNewNumber`.
- MS-04: smoke E2E mínimo coberto.
- ADR-002: testcontainers-go com `postgres:16-alpine`; build tag `//go:build integration`.
- ADR-006: validar que UNIQUE parcial permite reuso de número após soft delete.
- ADR-009: validar cascata em `user_whatsapp_history` (rows ativas viram `active=false, reason='user_soft_deleted'`).
- ADR-010: validar rollback transacional em `LinkNewNumber` (injeção de erro no meio da tx).
- Test do admin seed `0004`: setar `app.admin_whatsapp_numbers` via `ALTER DATABASE`, inserir 2 users, aplicar migration, verificar `is_admin=true`.
- Suite total < 60s.
</requirements>

## Subtarefas

- [ ] 9.1 Criar `internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go` com build tag + `UserRepositoryIntegrationSuite` (testify/suite).
- [ ] 9.2 Implementar `SetupSuite` provisionando container Postgres 16-alpine, construindo `database.Manager`, chamando `database.RunMigrations`. Pattern: copiar de `internal/platform/outbox/storage_pgx_integration_test.go`.
- [ ] 9.3 Implementar `SetupTest` com `TRUNCATE users, user_whatsapp_history CASCADE`.
- [ ] 9.4 Implementar `TearDownSuite` terminando container com defensive nil check.
- [ ] 9.5 Cenário `TestUpsertIdempotenteByWhatsAppNumber`: duas chamadas com mesmo número retornam mesmo `UserID`.
- [ ] 9.6 Cenário `TestSoftDeleteFiltraEmFindByID`: soft delete + `FindByID` retorna `ErrUserNotFound`.
- [ ] 9.7 Cenário `TestSoftDeleteFiltraEmFindByWhatsApp`: idem para `FindByWhatsAppNumber`.
- [ ] 9.8 Cenário `TestSoftDeleteCascataDesativaHistorico`: após soft delete, `COUNT(*) FROM user_whatsapp_history WHERE user_id=$1 AND active=true` = 0; `reason='user_soft_deleted'` presente.
- [ ] 9.9 Cenário `TestLinkNewNumberRegistraHistorico`: após link, 2 rows em history (1 inactive, 1 active); `users.whatsapp_number` atualizado.
- [ ] 9.10 Cenário `TestUniqueIndexParcialPermiteReuso`: soft delete user A com número X → upsert número X → cria user B com novo ID, sem violação.
- [ ] 9.11 Cenário `TestDuplicateWhatsAppNumberConcorrente`: dois INSERT diretos simultâneos com mesmo número ativo — segundo recebe `ErrDuplicateWhatsAppNumber`.
- [ ] 9.12 Cenário `TestMapperRejeitaUUIDCorrompido`: INSERT direto com `id='not-a-uuid'` (bypass de constraint via desabilitar temporariamente) — `FindByID` falha com erro tipado de mapper. (Alternativa: usar UUID v3 que passa em PK mas falha no `NewUserID`.)
- [ ] 9.13 Cenário `TestAdminSeedPromocao`: setar `app.admin_whatsapp_numbers`, inserir 2 users com esses números, rodar `0004` migration (re-aplicar), `is_admin=true`.
- [ ] 9.14 Adicionar entrada no `Taskfile.yml` (se houver namespace `test:`) para `test:integration` rodando com build tag.

## Detalhes de Implementação

Ver techspec §"Testes de Integração" e §"Plano de Rollout". Pattern de SetupSuite em `internal/platform/outbox/storage_pgx_integration_test.go` é referência canônica.

## Critérios de Sucesso

- 9 cenários listados passam consistentemente.
- Suite roda em < 60s em runner típico (container boot ~2s + queries).
- Build padrão (`go test ./...`) NÃO executa integration tests (build tag isola).
- `go test -tags=integration ./internal/identity/...` executa e passa.

## Definition of Done (DoD)

- [ ] `go test -tags=integration -race -count=1 ./internal/identity/infrastructure/repositories/postgres/...` passa.
- [ ] Duração total da suite < 60s (medir via `time go test -tags=integration ...`).
- [ ] `go test ./...` (sem tag) NÃO inclui o arquivo integration.
- [ ] Todos os 9 cenários `TestXxx` aparecem no relatório (`-v`).
- [ ] `golangci-lint run ./...` passa (sem regressão em outras suites).
- [ ] Container é terminado em `TearDownSuite` mesmo em caso de falha (verificável: rodar com `--fail-fast` e checar `docker ps` após).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Suite de 9 cenários (esta tarefa É a suite).
- [ ] Regressão: `go test ./...` (sem tag) continua passando.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go` (novo)
- `Taskfile.yml` (alterado, opcional — receita `test:integration`)

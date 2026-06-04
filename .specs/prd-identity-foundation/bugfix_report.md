# Relatorio de Bugfix

- Total de bugs no escopo: 3
- Corrigidos: 3
- Testes de regressao adicionados: 3
- Pendentes: nenhum
- Estado final: done

## Bugs
- ID: BUG-001
- Severidade: critical
- Origem: finding de review; RF-11, RF-12, RF-18; task-7.0
- Estado: fixed
- Causa raiz: o adapter Postgres usa pgx via devkit-go, mas `isNoRows` reconhecia apenas `sql.ErrNoRows`. Com isso, `pgx.ErrNoRows` era tratado como erro inesperado no caminho de ausencia de registro.
- Arquivos alterados: `internal/identity/infrastructure/repositories/postgres/mapper.go`, `internal/identity/infrastructure/repositories/postgres/mapper_test.go`
- Teste de regressao: `TestMapperSuite/TestIsNoRowsRecognizesSQLAndPgxSentinels`
- Validacao: `go test ./internal/identity/infrastructure/repositories/postgres -count=1` passou; `go test ./...` passou; `go vet ./...` passou; `golangci-lint run` passou

## Bugs
- ID: BUG-002
- Severidade: major
- Origem: finding de review; task-7.0
- Estado: fixed
- Causa raiz: `LinkNewNumber` inseria em `user_whatsapp_history` antes de verificar se `users` tinha uma row ativa para atualizar. Para usuario inexistente, a FK falhava antes do contrato `ErrUserNotFound`.
- Arquivos alterados: `internal/identity/infrastructure/repositories/postgres/user_repository.go`, `internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go`
- Teste de regressao: `TestUserRepositoryIntegration/TestLinkNewNumberUsuarioInexistente`
- Validacao: `go test ./internal/identity/infrastructure/repositories/postgres -count=1` passou; `go test -tags=integration ./internal/identity/infrastructure/repositories/postgres -run 'TestUserRepositoryIntegration/(TestUpsertIdempotenteByWhatsAppNumber|TestLinkNewNumberUsuarioInexistente|TestAdminSeedPromocao)' -count=1` passou; `go test ./...` passou; `go vet ./...` passou; `golangci-lint run` passou

## Bugs
- ID: BUG-003
- Severidade: major
- Origem: finding de review; ADR-005; task-9.0
- Estado: fixed
- Causa raiz: `SetAdminWhatsAppNumbers` executava `ALTER DATABASE current_database() SET ...`, sintaxe invalida em Postgres porque `ALTER DATABASE` exige identificador concreto do database. O valor tambem precisava ficar disponivel na sessao corrente para o teste que executa o bloco de seed diretamente.
- Arquivos alterados: `internal/platform/database/admin_seed.go`, `internal/platform/database/admin_seed_internal_test.go`
- Teste de regressao: `TestAdminSeedInternal/TestBuildAlterDatabaseSettingSQL`, `TestAdminSeedInternal/TestValidateCSVReturnsTrimmedNumbers`
- Validacao: `go test ./internal/platform/database -count=1` passou; `go test ./...` passou; `go vet ./...` passou; `golangci-lint run` passou

## Comandos Executados
- `go test ./internal/identity/infrastructure/repositories/postgres -count=1` -> passou
- `go test ./internal/platform/database -count=1` -> passou
- `go test ./cmd/migrate -count=1` -> passou, pacote sem testes
- `go test ./...` -> passou
- `go vet ./...` -> passou sem output
- `golangci-lint run` -> passou, `0 issues`
- `go build ./...` -> passou sem output
- `go test -race ./internal/identity/infrastructure/repositories/postgres ./internal/platform/database -count=1` -> passou
- `go test -tags=integration ./internal/identity/infrastructure/repositories/postgres -run 'TestUserRepositoryIntegration/(TestUpsertIdempotenteByWhatsAppNumber|TestLinkNewNumberUsuarioInexistente|TestAdminSeedPromocao)' -count=1` -> passou

## Riscos Residuais
- `mockery --config mockery.yml --dry-run` nao foi executado porque a versao instalada de `mockery` nao suporta a flag `--dry-run`.

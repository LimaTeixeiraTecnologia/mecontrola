# Generated: 2026-06-18T00:00:00Z

## Tarefa: Integration Tests — SupportSignalRepository e OnboardingCleanupRepository

### Arquivos Criados

1. `internal/onboarding/infrastructure/repositories/postgres/support_signal_repository_integration_test.go`
2. `internal/onboarding/infrastructure/repositories/postgres/onboarding_cleanup_repository_integration_test.go`

### Cenários por Arquivo

**support_signal_repository_integration_test.go** — 3 cenários:
- `TestInsert_PersistsSignal`: INSERT + SELECT COUNT WHERE id + SELECT kind/occurred_at/resolved_at
- `TestInsert_DifferentKinds`: 3 kinds distintos, COUNT total=3, COUNT por kind=1
- `TestInsert_PayloadRoundtrip`: payload JSON roundtrip via `payload->>'external_sale_id'`

**onboarding_cleanup_repository_integration_test.go** — 5 cenários:
- `TestDeleteMetaProcessedOlderThan`: 3 antigos deletados, recente preservado
- `TestDeleteMetaProcessedOlderThan_Limit`: 5 antigos, limit=2, retorna 2, COUNT=3
- `TestDeleteMetaProcessedOlderThan_Empty`: tabela vazia, retorna 0
- `TestDeleteConsumerLookupAttemptsOlderThan`: 2 antigos deletados, 1 recente preservado
- `TestDeleteConsumerLookupAttemptsOlderThan_Empty`: tabela vazia, retorna 0

### Gates de Validação

```
go build ./internal/onboarding/infrastructure/repositories/postgres/...
-> PASS (sem erros)

go test -tags=integration -race -count=1 -timeout=120s \
  -run "TestSupportSignalRepositoryIntegrationSuite|TestOnboardingCleanupRepositoryIntegrationSuite" \
  ./internal/onboarding/infrastructure/repositories/postgres/...
-> ok (16.109s) — 8 cenários, todos PASS

golangci-lint run --build-tags=integration (arquivos novos)
-> 0 issues

Zero-comentários gate (R-ADAPTER-001.1)
-> PASS: zero comentarios
```

### Regras Respeitadas

- `//go:build integration` linha 1 de cada arquivo
- Package `postgres_test`
- Zero comentários Go
- Sem `var _ Interface = (*Type)(nil)`
- Sem `init()`
- Seeding via SQL direto
- Asserção via SQL direto após cada operação
- Testcontainer isolado por caso de teste via `testcontainer.Postgres(s.T())`

### Notas

Falhas pré-existentes em `TestMagicTokenRepositorySuite` (invalid uuid syntax em subscription_id) não são relacionadas a esta tarefa e estavam presentes antes da implementação.

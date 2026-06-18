# Generated: 2026-06-18T00:00:00Z

## Tarefa
Criar `magic_token_repository_integration_test.go` para o repositório `internal/onboarding/infrastructure/repositories/postgres`.

## Arquivo Criado
`/Users/jailtonjunior/Git/mecontrola/internal/onboarding/infrastructure/repositories/postgres/magic_token_repository_integration_test.go`

## Cenários Implementados (8)
1. `TestInsertAndFindByHash` — Insert PENDING, count SQL, FindByHash correto, FindByHash inexistente -> ErrTokenNotFound
2. `TestUpdateMarkPaid` — MarkPaid com UUID válido, SELECT direto verifica status/paid_at/campos, segunda chamada idempotente
3. `TestUpdateMarkConsumed` — PENDING -> PAID -> CONSUMED, SELECT verifica status/consumed_at/activation_path=direct
4. `TestBulkExpire` — 2 expirados (PENDING+PAID) + 1 não expirado, BulkExpire retorna >=2, SELECT verifica EXPIRED/PENDING
5. `TestOutreachFlow` — FindPaidForOutreach encontra token, UpdateMarkOutreachSent, verifica outreach_sent_at, UpdateMarkOutreachReset
6. `TestFindPaidByMobileForFallback` — PAID com outreach_sent_at preenchido, FindByMobile correto, mobile errado -> ErrTokenNotFound
7. `TestCountPaidUnconsumed` — 0 inicial, 2 PAID + 1 CONSUMED -> retorna 2
8. `TestUpdateTelegramExternalID` — UpdateTelegramExternalID, SELECT verifica telegram_external_id

## Correção Aplicada
O schema da tabela `onboarding_tokens` define `subscription_id UUID NULL` e `consumed_by_user_id UUID NULL`.
Os testes originais usavam strings literais não-UUID (`"sub-001"`, `"sub-002"`), causando erro `invalid input syntax for type uuid`.
Todos os campos UUID foram substituídos por `uuid.NewString()`.

## Critérios de Aceite

- build: `go build ./internal/onboarding/infrastructure/repositories/postgres/...` -> comprovado: sem output de erro
- vet: `go vet ./internal/onboarding/infrastructure/repositories/postgres/...` -> comprovado: sem output de erro
- testes: `go test -tags=integration -race -count=1 -timeout=120s ./internal/onboarding/infrastructure/repositories/postgres/...` -> comprovado: `ok ... 22.086s`
- lint: `golangci-lint run ./internal/onboarding/infrastructure/repositories/postgres/...` -> comprovado: `0 issues.`
- zero comentários (arquivos de produção): gate grep -> comprovado: sem output
- build tag linha 1: `//go:build integration` -> comprovado: presente
- package: `postgres_test` -> comprovado: correto
- SQL direto após writes: `s.queryRow(ctx, ...)` após cada operação de escrita -> comprovado: todos os cenários

## Riscos Residuais
Nenhum.

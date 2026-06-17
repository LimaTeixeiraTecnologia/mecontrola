# BDD + E2E com Godog — mecontrola 2026

## Contexto

O projeto tinha apenas testes unitários e de integração por módulo isolado. O gap era:
nenhum cenário E2E cross-módulo com Gherkin legível, requests HTTP reais e validação no banco.

## Decisão Tecnológica

**Framework:** Godog v0.15.0 (github.com/cucumber/godog)
**Infra:** testcontainers-go (já usado) — PostgreSQL real, migrations aplicadas automaticamente
**Requests:** `net/http` contra `httptest.NewServer` com todos os módulos wired
**Validação:** queries diretas via `manager.DBTX(ctx).QueryRowContext`

## Skill Obrigatória

`go-implementation`

## Estrutura Implementada

```
internal/e2e/
  suite_test.go       — TestE2E, buildServer, loadConfig, e2eAuthMiddleware
  ctx_test.go         — e2eCtx struct, makeRequest, parseResponseBody
  steps_test.go       — registerSteps + todos os step definitions
  features/
    f01_transaction_flow.feature   — Gherkin PT-BR
```

- Build tag: `//go:build e2e`
- Package: `e2e_test`
- Zero comentários em todos os arquivos Go (R-ADAPTER-001.1)

## Cenário F01 (Gherkin PT-BR)

```gherkin
# language: pt
Funcionalidade: Criação de transação via HTTP

  Cenário: Criar transação de despesa com categoria válida e validar no banco
    Dado que a categoria "prazeres" está disponível no sistema
    Quando o usuário cria uma transação de 5800 centavos no método "pix"
    Então a resposta HTTP deve ter status 201
    E a transação deve estar salva no banco com valor 5800
    E o corpo da resposta deve conter o campo "id"
```

## Wiring

Módulos wired em `buildServer`:
- `categories.NewCategoriesModule` — lookup de categoria
- `card.NewCardModule` — dependência do transactions
- `transactions.NewTransactionsModule` — CRUD de transações

Gateway auth: passthrough (`func(next) { return next }`)
Principal: fixo via `e2eAuthMiddleware` — UUID `11111111-1111-1111-1111-111111111111`

Config: `configs.LoadConfig("../../")` + `cfg.TransactionsConfig.Enabled = true`

## CI

Novo job `e2e` em `.github/workflows/ci.yml`:
- `needs: [setup]` — paralelo com `unit` e `integration`
- Cria `.env` (não `.env.test`) para satisfazer `requiresLocalEnvFile()`
- `task test:e2e` com timeout 5m
- `build-image.needs` atualizado para incluir `e2e`

## Taskfile

Nova task `e2e` em `taskfiles/test.yml`:
```
task test:e2e
```

## Validação

```bash
# Compilação
go build -tags=e2e ./internal/e2e/...
go vet -tags=e2e ./internal/e2e/...

# Gate zero comentários
grep -rn --include="*.go" --exclude-dir=mocks "^[[:space:]]*//" internal/e2e/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)"

# E2E local (requer Docker + .env)
task test:e2e
```

## Arquivos Alterados

| Arquivo | Ação |
|---------|------|
| `go.mod` / `go.sum` | adicionado `github.com/cucumber/godog v0.15.0` |
| `internal/e2e/suite_test.go` | novo |
| `internal/e2e/ctx_test.go` | novo |
| `internal/e2e/steps_test.go` | novo |
| `internal/e2e/features/f01_transaction_flow.feature` | novo |
| `taskfiles/test.yml` | task `e2e` adicionada |
| `.github/workflows/ci.yml` | job `e2e` adicionado, `build-image.needs` atualizado |

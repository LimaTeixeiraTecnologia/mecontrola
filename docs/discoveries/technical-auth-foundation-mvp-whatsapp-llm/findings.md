# Findings: Pre-build Discovery — Auth Foundation MVP (WhatsApp LLM)

> Gerado em: 2026-06-08T00:00:00Z
> Tarefa: 1.0 — Pre-build discovery
> Referência: `.specs/prd-auth-foundation/task-1.0-pre-build-discovery.md`

---

## PRE-01 — Framework de Integration Test

**Comando executado:**
```bash
cat migrations/migrations_integration_test.go | head -40
```

**Resultado:**
O projeto usa **`testcontainers-go`** como framework de integration test. Confirmado pelo import:
```go
tc "github.com/testcontainers/testcontainers-go"
"github.com/testcontainers/testcontainers-go/wait"
```
O helper de banco de dados é provido por `github.com/JailtonJunior94/devkit-go/pkg/database/...` (manager, migration, postgres).
O padrão de suíte usa `testify/suite` com `suite.Run(t, new(MigrationSuite))`.

**Decisão para tarefas 3.0, 4.0, 5.0, 7.0, 8.0:**
- Usar `testcontainers-go` + `testify/suite` em todos os integration tests.
- Usar o helper `devkit-go/pkg/database/postgres` para setup de PG.
- Build tag: `//go:build integration` em todos os arquivos de integration test.
- `dockertest` **não está em uso** — não introduzir.

---

## PRE-02 — Headers HTTP Lidos por Handlers (Allowlist Expandida)

**Comando executado:**
```bash
grep -rEn 'r\.Header\.(Get|Values)' internal/ 2>/dev/null | grep -v "_test.go"
```

**Headers encontrados:**

| Header | Arquivo | Justificativa |
|--------|---------|---------------|
| `Content-Type` | `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_handler.go:44` | Validação do mime-type do payload do webhook Kiwify. Legítimo. |
| `X-Kiwify-Signature` | `internal/billing/infrastructure/http/server/middleware/hmac_signature.go:30` | HMAC SHA-1 do webhook Kiwify; obrigatório para autenticação do webhook. |
| `Origin` | `internal/onboarding/infrastructure/http/server/router.go:87` | CORS — verificação da origem da requisição. |
| `X-Real-IP` | `internal/onboarding/infrastructure/http/server/middleware/rate_limit.go:93` | IP real do cliente via proxy reverso (nginx); usado no rate limiter. |
| `X-Forwarded-For` | `internal/onboarding/infrastructure/http/server/middleware/rate_limit.go:96` | IP do cliente via cadeia de proxies; fallback para `X-Real-IP`. |
| `X-Hub-Signature-256` | `internal/onboarding/infrastructure/http/server/middleware/meta_signature.go:33` | HMAC SHA-256 do webhook Meta/WhatsApp; obrigatório para autenticação. |

**Allowlist expandida para `.golangci.yml` (tarefa 2.0):**
```
{X-Request-ID, Content-Type, Idempotency-Key, X-Kiwify-Signature, Origin, X-Real-IP, X-Forwarded-For, X-Hub-Signature-256}
```

Nenhum handler em `internal/identity/` lê headers diretamente. Todos os usos são em `billing` e `onboarding`.

---

## PRE-03 — Pacote Canônico de Configuração

**Comandos executados:**
```bash
ls internal/platform/config 2>/dev/null  # NOT FOUND
ls configs/ 2>/dev/null
grep -rl "package config|package configs" --include="*.go" .
```

**Resultado:**
- `internal/platform/config/` **não existe**.
- O pacote canônico é **`configs/`** (`package configs`), com os arquivos:
  - `configs/config.go` — struct `Config` central com todos os grupos de config.
  - `configs/config_test.go` — testes unitários.
  - `configs/insecure.go` — placeholders inseguros para validação em produção.

**Implicação para tarefa 2.0 (forbidigo/depguard):**
- A regra `forbidigo` deve referenciar `configs.Config` (import path `github.com/LimaTeixeiraTecnologia/mecontrola/configs`).
- Proibir importação de `internal/platform/config` (inexistente — garantia defensiva).
- Qualquer novo campo de configuração do módulo `auth` vai em `configs/config.go` como novo grupo `AuthConfig`.

---

## PRE-04 — Estado de `user.deleted` em `MarkUserDeleted`

**Arquivo lido:** `internal/identity/application/usecases/mark_user_deleted.go`

**Resultado:**
`MarkUserDeleted.Execute` **não publica** evento `user.deleted` via outbox.

O usecase atual apenas:
1. Abre UoW com `u.uow.Do(...)`.
2. Chama `userRepo.MarkDeleted(ctx, in.ID, time.Now().UTC())`.
3. Retorna erro ou `nil`.

Não há referência a `outbox.Publisher`, `outbox.Event`, `outbox.EventType` ou qualquer mecanismo de publicação de evento.

**Decisão para tarefa 4.0:**
> Tarefa 4.0 DEVE adicionar publicação de `outbox.Event{Type: "user.deleted"}` dentro da transação UoW em `MarkUserDeleted.Execute`, seguindo o contrato de outbox transacional definido em `AGENTS.md` (seção "Outbox") e a regra de idempotência por `event_id`.

---

## Resumo das Decisões

| # | Descoberta | Decisão |
|---|-----------|---------|
| PRE-01 | Framework de test: `testcontainers-go` + `testify/suite` | Usar em 3.0, 4.0, 5.0, 7.0, 8.0; nunca `dockertest` |
| PRE-02 | 6 headers lidos (além da allowlist base de 3) | Allowlist expandida para 8 headers; 2.0 usa lista completa no forbidigo |
| PRE-03 | Pacote canônico: `configs/` (`package configs`) | 2.0 aponta forbidigo para `configs.Config`; novo `AuthConfig` vai em `configs/config.go` |
| PRE-04 | `MarkUserDeleted` não publica `user.deleted` | 4.0 adiciona publicação via outbox transacional |

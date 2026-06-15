# Auditoria de Production-Readiness — MeControla

**Data:** 2026-06-15
**Veredicto geral:** MVP robusto? **Sim.** Production-ready sem falso positivo? **Não ainda — 2 bloqueadores críticos.**

---

## Metodologia

Três agentes de análise executados em paralelo sobre o codebase completo:

1. **Segurança & autenticação** — middleware, HMAC, webhooks, SQL injection, secrets em logs
2. **Resiliência & shutdown** — goroutines, contexto, panic recovery, health checks, jobs
3. **Testes, CI/CD & observabilidade** — cobertura, pipeline, lint, migrações, SBOM

---

## Veredicto por dimensão

| Dimensão | Status | Severidade máxima |
|----------|--------|--------------------|
| Testes (374 arquivos) | OK | — |
| CI/CD pipeline | OK | — |
| Linting (depguard + forbidigo) | OK | — |
| Observabilidade (OTel HTTP+DB+outbox) | OK | — |
| Segurança: HMAC gateway auth | OK | — |
| Segurança: Rate limiting | OK | — |
| Segurança: CORS | OK | — |
| Segurança: WhatsApp/Telegram webhooks | OK | — |
| Segurança: SQL injection | OK | — |
| Segurança: Secrets em logs | OK | — |
| Migrações DB | OK | — |
| Build reproduzível | OK | — |
| Autenticação (X-User-ID) | RISK | Médio |
| Kiwify webhook (camada errada) | RISK | Médio |
| Health check (sem /readiness) | PARTIAL | Baixo |
| Circuit breaker externo | RISK | Baixo |
| **Panic recovery** | **CRITICAL** | **Bloqueador** |
| **Timeouts por job** | **CRITICAL** | **Bloqueador** |

---

## Bloqueadores críticos (impedem produção)

### CRÍTICO 1 — Zero panic recovery em handlers HTTP e scheduler de jobs

**Impacto:** Um único panic não tratado em qualquer handler HTTP ou callback de job derruba o processo inteiro.

**Onde ocorre:**
- HTTP handlers: sem `defer recover()` em nenhum handler (`internal/platform/whatsapp/handlers/inbound_handler.go:27`)
- Scheduler de jobs: sem recovery em callbacks do cron (`internal/platform/worker/job/scheduler.go:58-74`)
- Se o dispatcher do outbox panicar no meio de um batch, o scheduler inteiro cai → eventos ficam presos em `PROCESSING` até o reaper agir

**Diagnóstico:**
```bash
grep -rn "recover()" internal/ --include="*.go"  # retorna zero resultados
grep -rn "WithRecover\|PanicRecovery\|Recoverer" . --include="*.go"  # zero resultados
```

**Fix:** Adicionar método `runSafe()` no scheduler que envolve cada execução de job com `defer func() { if r := recover(); r != nil { logger.Error(...) } }()`. Para HTTP, confirmar se devkit-go já tem recovery — se não, adicionar middleware de recovery no chi server.

---

### CRÍTICO 2 — Jobs recebem contexto de shutdown sem timeout próprio

**Impacto:** Um job lento (`reconciliation`, `materializer`, `housekeeping`) bloqueia o drain do worker. Com shutdown timeout de 15s, o job é force-killed no meio da operação.

**Onde ocorre:**
- `internal/platform/worker/manager.go:40` — `runCtx = context.WithCancel(ctx)` sem timeout
- `internal/platform/worker/job/scheduler.go:65,71` — `rj.run(ctx)` sem timeout por job
- Exceção: `internal/platform/outbox/dispatcher.go:100` — único job com timeout por handler (`DispatcherHandlerTimeout`), mas não cobre o tick inteiro

**Jobs afetados (sem timeout):**
```
outbox-dispatcher, outbox-reaper, outbox-housekeeping
identity-auth-events-housekeeping
billing-reconciliation, billing-kiwify-events-housekeeping, billing-grace-expiration
budgets-pending-reaper, budgets-abandoned-draft, budgets-retention-purge
onboarding-outreach, onboarding-token-expiration, onboarding-meta-cleanup
transactions-recurring-materializer, transactions-monthly-summary-reconciler
```

**Fix:** Adicionar `Timeout() time.Duration` à interface `Job` pública (`internal/platform/worker/types.go`). Cada job declara seu limite. O scheduler usa `context.WithTimeout(ctx, rj.timeout)` quando `> 0`.

---

## Riscos (não bloqueadores — mitigar antes do GA)

### RISK — Autenticação header-only (X-User-ID sem JWT)

**Impacto potencial:** Médio. `X-User-ID` é um header sem garantia criptográfica. Confia que o gateway HMAC protege todas as rotas antes de qualquer processamento.

**Como está mitigado:**
- Card module exige gateway HMAC + X-User-ID (`internal/card/infrastructure/http/server/router.go:59`)
- HMAC usa SHA-256 com dual-key rotation e replay protection (janela 60s) — `internal/identity/domain/services/verify_gateway_request.go`
- Constant-time compare via `hmac.Equal()`

**Risco residual:** Se alguma rota não aplicar o middleware de gateway auth, X-User-ID pode ser forjado por qualquer cliente.

**Ação:** Auditar todas as rotas que usam `RequireUserWithO11y` sem `gatewayAuth` predecessor.

---

### RISK — Kiwify webhook: rejeição na camada errada

**Impacto:** Baixo. Assinaturas inválidas são rejeitadas, mas no use-case e não no middleware.

**Fluxo atual:**
```
Request → RawBody → HMACSignature (seta ctx mas não rejeita) → Handler → Usecase → ErrInvalidSignature → 401
```

**Fluxo correto (defense-in-depth):**
```
Request → RawBody → HMACSignature (rejeita 401 se invalid) → Handler → Usecase (check redundante)
```

**Arquivo:** `internal/billing/infrastructure/http/server/middleware/hmac_signature.go:34-35`

```go
// Atual: sempre chama next
ctx := context.WithValue(r.Context(), ctxKeySignatureStatus{}, status)
next.ServeHTTP(w, r.WithContext(ctx))

// Correto: rejeitar antes
if status == SignatureStatusInvalid {
    http.Error(w, `{"message":"invalid signature"}`, http.StatusUnauthorized)
    return
}
ctx := context.WithValue(r.Context(), ctxKeySignatureStatus{}, status)
next.ServeHTTP(w, r.WithContext(ctx))
```

---

### PARTIAL — Health check sem /readiness

**Impacto:** Baixo. Load balancer não consegue distinguir "serviço saudável" de "serviço em processo de shutdown".

**O que existe:** `/health` verifica DB (`cmd/server/server.go:109-111`). Não há `/readiness`.

**Problema:** Durante o shutdown de 15s, o load balancer continua enviando requisições ao pod porque `/health` ainda retorna 200.

**Fix:** Adicionar `/readiness` que retorna 503 quando o contexto de shutdown foi cancelado:
```go
// cmd/server/readiness.go
func (rt *readinessRouter) Register(r chi.Router) {
    r.Get("/readiness", func(w http.ResponseWriter, _ *http.Request) {
        select {
        case <-rt.ctx.Done():
            w.WriteHeader(http.StatusServiceUnavailable)
        default:
            w.WriteHeader(http.StatusOK)
        }
    })
}
```

---

### RISK — Sem circuit breaker para APIs externas

**Impacto:** Baixo. Falha da Kiwify API ou Meta/WhatsApp API se propaga via retry até esgotar tentativas, sem cooldown entre tentativas sucessivas.

**O que existe:**
- Retry para safe methods (GET/HEAD): `internal/platform/httpclient/client.go:130`
- Rate limiting client-side: `internal/billing/infrastructure/http/client/kiwify/ratelimit.go`
- Meta/WhatsApp **desabilita retry**: `internal/onboarding/infrastructure/http/client/meta/client.go:123`

**Risco:** Flood de erros 5xx da Kiwify API sem circuit open.

---

## O que está sólido (sem ressalvas)

### Testes: 374 arquivos, todas as dimensões cobertas

| Módulo | Arquivos de teste | Tipos |
|--------|-------------------|-------|
| Transactions | 65 | unit + integration + table-driven |
| Identity | 53 | unit + integration + smoke |
| Card | 38 | unit + contract + integration |
| Billing | 32 | unit + integration |
| Onboarding | 29 | unit + integration + smoke |
| Budgets | n/a (parte do card) | unit |

Separação: `//go:build integration` para testes que precisam de DB real. Smoke tests em `taskfiles/test.yml`.

---

### CI/CD: Gates completos antes de deploy

```
lint → unit tests → integration tests → govulncheck → Trivy FS → card:audit → build image
```

- Qualquer falha bloqueia o build
- Imagem assinada via cosign OIDC keyless
- SBOM + provenance attestation (SLSA)
- Trivy image scan: EXIT 1 em HIGH/CRITICAL
- CD só dispara após CI completo

---

### Linting: Fronteiras arquiteturais hard-encoded

**golangci-lint (`.golangci.yml`):**
- `depguard`: bloqueia imports cross-module (domain ← sem infra, billing ← sem identity/infra)
- `forbidigo`: bloqueia DSN em texto (exposição de senha), `os.Getenv` fora de configs/, leitura direta de headers `X-User-*` (deve usar `auth.FromContext()`), PAN/CVV/PIN no módulo card
- `revive`: cyclomatic complexity ≤ 15, function-length ≤ 40 linhas
- Pre-commit: gofmt + goimports + golangci-lint fast-only

---

### Segurança: HMAC Gateway Auth (design correto)

- HMAC-SHA256 com chave hex de 32+ bytes
- Dual-key rotation (CURRENT + NEXT) — `internal/identity/domain/services/verify_gateway_request.go:26-55`
- Replay protection: janela de 60s configurável via `IDENTITY_GATEWAY_AUTH_WINDOW`
- Constant-time compare: `hmac.Equal()` (sem timing attack)
- Canonicalização lowercase para evitar confusão de case

---

### Segurança: Webhooks WhatsApp e Telegram

- **WhatsApp**: HMAC-SHA256 com dual-key, rejeição 401 no middleware — `internal/platform/whatsapp/signature/hmac.go:38-44`
- **Telegram**: Token match com dual-token rotation, rejeição 401 no middleware — `internal/platform/telegram/signature/secret_token.go:48-53`
- Ambos rejeitam na camada correta (middleware, não use-case)

---

### Segurança: Secrets não vazam em logs

- DB DSN: `cfg.DBConfig.SafeDSN()` mascara senha com `***` — `configs/config.go:254-259`
- Token.String(): retorna `[REDACTED]` — `internal/onboarding/domain/valueobjects/token.go:62-64`
- User ID em logs: truncado para 8 chars — `internal/identity/infrastructure/http/server/middleware/require_gateway_auth.go:98-103`

---

### Observabilidade: OTel completo em HTTP + DB + Outbox

- HTTP: `httpserver.WithMetrics()`, `WithTracing()`, `WithOTelMetrics()` — auto-instrumentação em todas as rotas
- DB: `manager.WithObservability(o11y)` em todos os UoW (30+ instâncias)
- Outbox: dispatcher, reaper e housekeeping com `WithObservability(o11y)`
- Structured logging: `slog.WarnContext/InfoContext` com campos tipados

---

### Migrações: Baseline V0 imutável, embedded no binário

- `//go:embed` — schema completo no binário, sem dependência de filesystem em runtime
- Rollback `.down.sql` testado em `migrations_integration_test.go:68-75` (testcontainers)
- `TestBaselineUpDownUp` verifica idempotência

---

## Plano de ação priorizado

### Prioridade 1 — Bloqueadores (implementar antes do primeiro deploy em produção)

1. **Panic recovery no scheduler** (`internal/platform/worker/job/scheduler.go`)
   - Adicionar `runSafe()` com `defer recover()`
   - Substituir chamadas `rj.run(ctx)` por `s.runSafe(ctx, rj)`

2. **Timeout por job**
   - Adicionar `Timeout() time.Duration` à interface `Job` (`internal/platform/worker/types.go`)
   - Thread no adapter e scheduler
   - 15 jobs implementam `Timeout()` com valores fixos (vide tabela no plano de implementação)

### Prioridade 2 — Riscos (antes do GA / aumento de tráfego)

3. **Kiwify middleware: rejeitar antes do handler** (`billing/.../middleware/hmac_signature.go`)
4. **Endpoint /readiness** (`cmd/server/readiness.go` novo + `server.go`)
5. **Auditoria de rotas**: confirmar que todas as rotas protegidas têm gateway HMAC antes de `RequireUserWithO11y`

### Prioridade 3 — Melhorias operacionais (pós-GA)

6. Circuit breaker para Kiwify API e Meta/WhatsApp (via `internal/platform/httpclient/`)
7. Prometheus `/metrics` endpoint explícito
8. Alertas em `docs/alerts/` para SLA de billing e latência de autenticação

---

## Referências de implementação

- Plano de implementação técnico: `.claude/plans/analise-criteriosamente-configs-se-mossy-snowglobe.md`
- Runbook de env files: `docs/runbooks/2026-06-15-equalize-env-files.md`
- Regras de adapter: `.claude/rules/go-adapters.md`
- ADR de gateway auth: referenciado em `internal/identity/module.go`

# Auditoria de Segurança Pré Go-Live — mecontrola

**Data**: 2026-06-11
**Autor**: Claude (sessão de planning)
**Escopo**: monólito modular Go (chi v5), WhatsApp + LLM intermediária + per-user, Postgres, Caddy, VPS Hostinger
**Skill obrigatória para execução**: `.agents/skills/go-implementation/SKILL.md` — TODA alteração ou criação de código Go neste plano DEVE seguir as Etapas 1 a 5 da skill `go-implementation`, incluindo as Regras Estritas R0–R7 e a R-ADAPTER-001 (zero comentários em `.go`, adapters finos `handler → usecase`).

---

## 1. Contexto

Auditoria de production-readiness solicitada antes do go-live.

Stack real verificada no repositório:
- **Servidor HTTP**: go-chi/chi v5 via `devkit-go/pkg/http_server/chi_server`, entrypoint em `cmd/server/server.go`.
- **Canais de entrada**: **apenas WhatsApp** (Meta Cloud API). Telegram NÃO existe no código.
- **LLM intermediária**: chama API com header `X-User-ID` em rede interna; trust boundary delegado ao Caddy.
- **Modelo de dados**: per-user (não multi-tenant SaaS). Isolamento via FK `user_id` + `WHERE user_id = $1`.
- **Autenticação**: HMAC-SHA256 WhatsApp (`X-Hub-Signature-256`), HMAC-SHA1 Kiwify (`X-Kiwify-Signature`), `X-User-ID` header simples para LLM. SEM JWT.
- **Isolamento DB**: SEM RLS Postgres. Filtros explícitos em SQL.
- **Cache/dedup**: idempotência e dedup em Postgres; rate-limit in-memory (`golang.org/x/time/rate`). SEM Redis.
- **TLS**: terminado em Caddy reverse proxy; app Go escuta HTTP plain dentro da rede `backend`.
- **Containers**: distroless `nonroot` UID 65532, multi-stage; Postgres isolado em rede `backend`.
- **Observabilidade**: `log/slog` estruturado, OTel tracing + Prometheus, Promtail → Loki/Grafana.
- **Backup**: `pg_dump` + criptografia `age` + upload offsite via `rclone` (`deployment/scripts/pg-dump.sh`).

Decisões do usuário tomadas durante o planning:
1. **Escopo**: auditar o que existe (modelo per-user, single-channel). Telegram/JWT/RLS são pós go-live se justificarem.
2. **Trust boundary do `X-User-ID`**: vem da LLM em rede interna. Caddy deve bloquear o header de origem externa; a verificação disso é parte do hardening.

---

## 2. Resumo Executivo

A base de segurança do mecontrola é **acima da média** para projeto solo em VPS:
- HMAC com rotação para webhooks Meta e Kiwify.
- Distroless nonroot.
- Outbox transacional com dispatcher, reaper e handlers por tipo.
- Idempotência HTTP por hash de body com detecção de conflito.
- Auditoria parcial em `auth_events` indexada por `user_id + occurred_at`.
- Validação de secrets `CHANGE_ME_*` rejeitada no boot em `ENVIRONMENT=production`.
- Arquitetura bem decomposta (módulos, ports/adapters, Principal no contexto).

Três maiores riscos antes do go-live:
1. **`X-User-ID` sem prova de origem** — middleware `InjectPrincipalFromHeader` aceita qualquer UUID. A premissa "vem da LLM" precisa ser **enforced pelo Caddy** + shared-secret HMAC entre LLM e API. Sem isso, qualquer rota com esse middleware exposta publicamente permite impersonação trivial.
2. **CORS, security headers e admin endpoints não auditados em produção** — depende do `Caddyfile` versionado; precisa de verificação.
3. **Sem RLS no Postgres** — defesa em profundidade ausente. Bug futuro de query sem `WHERE user_id` vaza cross-user. Mitigável em segunda onda.

---

## 3. ✅ Itens implementados corretamente

| Item | Evidência (caminho:linha) |
|---|---|
| HMAC-SHA256 WhatsApp com rotação `secretCurrent`/`secretNext` (status `valid`/`rotated`/`invalid`) | `internal/platform/whatsapp/signature/hmac.go:33-81` |
| Verify token WhatsApp em constant-time | `internal/platform/whatsapp/handlers/verify_handler.go:26` |
| Dedup WhatsApp por `wamid` em tabela `meta_processed_messages` | `internal/platform/whatsapp/dedup/postgres/` + migration `000001` |
| Raw body buffering antes do parse JSON (preserva bytes para HMAC) | `internal/billing/.../middleware/raw_body_buffer.go` |
| HMAC-SHA1 Kiwify com rotação `secretCurrent`/`secretNext` | `internal/billing/.../middleware/hmac_signature.go:19-64` |
| Idempotência HTTP por `userID + scope + key` com SHA256 do body e 409 em conflict | `internal/platform/idempotency/middleware.go:35-157` + tabela `platform_idempotency_keys` (migration `000007`) |
| Rate limit por IP com cleanup automático e CIDR whitelist | `internal/onboarding/.../middleware/rate_limit.go:30-142` |
| Outbox transacional completo (dispatcher, reaper, registry, retry) | `internal/platform/outbox/` |
| Auditoria de auth (`principal_established`/`failed`/`unknown_user`) indexada | tabela `auth_events`, migration `000001` |
| Distroless `nonroot` (UID 65532), multi-stage Go 1.26.4 → distroless | `deployment/docker/Dockerfile` |
| Validação `CHANGE_ME_*` rejeitada em `production` no boot | `configs/config.go` (`Config.Validate`) |
| Logging estruturado `slog` + OTel (tracer + Prometheus) | `cmd/server/server.go` + devkit-go |
| Mascaramento de número WhatsApp em logs estruturados | handlers card e whatsapp |
| Postgres isolado em rede `backend` (sem porta exposta no host) | `deployment/compose/compose.yml` |
| Backup `pg_dump` + criptografia age + offsite via rclone | `deployment/scripts/pg-dump.sh` |
| `.gitignore` cobre `.env`, `.env.local`, `.env.test`, worktrees, vendor | `.gitignore` |
| Graceful shutdown 15s com cancelamento de context | `cmd/server/server.go:112` |
| Principal context API com getter retornando zero-value seguro | `internal/identity/application/auth/principal.go:16-37` |

---

## 4. ⚠️ Itens implementados com problemas

### A1 — `InjectPrincipalFromHeader` confia cegamente em `X-User-ID` (CRÍTICO)
- Arquivo: `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go:19-66`.
- Aceita qualquer UUID válido e cria Principal `source=header` sem nenhuma prova de origem.
- Mitigação imediata coberta em B1.

### A2 — CORS pode estar aberto por default (ALTO)
- `cmd/server/server.go:200-205` resolve `resolveCORSOrigins()` via `CORS_ALLOWED_ORIGINS`.
- Não há validação que rejeite `*` ou vazio em `production`.
- Mitigação coberta em B6.

### A3 — Rate limit é in-memory e só em onboarding (MÉDIO)
- `golang.org/x/time/rate` em processo único; OK para VPS single-node.
- **Sem rate limit no `/api/v1/whatsapp`**: atacante pode forçar burst de validações HMAC (CPU DoS). Mitigação em B7.
- **Sem rate limit por `user_id`** nas rotas autenticadas: abuso de conta legítima sem teto. Mitigação em A10.

### A4 — CSP/HSTS/security headers não validados no app (MÉDIO)
- Esperado no Caddy. Mitigação coberta em B3.

### A5 — `auth_events` sem `request_id`/`client_ip` (BAIXO)
- Forense limitada. Adicionar colunas e popular via header `X-Forwarded-For` sanitizado pelo Caddy.

### A6 — Kiwify HMAC-SHA1 (BAIXO, aceito por dependência externa)
- `internal/billing/.../middleware/hmac_signature.go:60`. SHA1 é fraco mas é o que Kiwify assina.
- Mitigação: limitar tamanho de payload aceito, alertar `signature_status=invalid` > N/min.

### A7 — `ONBOARDING_TOKEN_ENCRYPTION_KEY` em env var (BAIXO em VPS solo)
- Aceitável; documentar procedimento de rotação (gerar nova chave, re-encriptar tokens ativos, descartar antiga).

### A8 — `.env.example` mistura secrets de tooling e de produto (BAIXO)
- Considerar separar `OTEL_EXPORTER_OTLP_HEADERS`, `LOKI_API_KEY` em `.env.observability` carregado pelo mesmo Viper.

### A9 — Sem teste explícito de "query sem `WHERE user_id` falha" (MÉDIO)
- Como não há RLS, o único enforcement é o code review. Adicionar teste de integração que prove isolamento em pelo menos `cards`, `budgets`, `budgets_expenses`.

### A10 — Rate limit por `user_id` ausente em rotas autenticadas (MÉDIO)
- Adicionar limiter por `principal.UserID` (não só IP) nas rotas autenticadas. Reusar implementação existente parametrizada por chave.

---

## 5. ❌ Bloqueantes para produção

### B1 — Enforcement do trust boundary LLM ↔ API

**Por quê crítico**: modelo inteiro de auth assume "X-User-ID vem da LLM". Sem prova criptográfica, é apenas uma string.

**Implementação obrigatória**:
1. **Caddy** (em `deployment/compose/Caddyfile`):
   - Strip `X-User-ID` de qualquer request externo (`header_up -X-User-ID` antes de validar o gateway).
   - Validar `X-Gateway-Auth` (HMAC) ou whitelist de IP da LLM.
   - Bloquear rotas `/api/v1/cards*`, `/api/v1/budgets*`, etc. quando não houver auth de gateway.
2. **App Go** (defesa em profundidade): novo middleware `RequireGatewayAuth` em `internal/identity/infrastructure/http/server/middleware/require_gateway_auth.go`.
   - Carrega `cfg.Identity.GatewaySharedSecret` (novo campo em `configs/config.go`).
   - Header esperado: `X-Gateway-Auth: <hex hmac-sha256>` + `X-Gateway-Timestamp: <unix>`.
   - HMAC = `hmac_sha256(secret, userID + "." + timestamp)`.
   - Janela ±60s. Rejeita com 401 sem detalhes; loga + insere `auth_events` reason `invalid_gateway_signature`.
3. **Chain**: aplicar `RequireGatewayAuth` ANTES de `InjectPrincipalFromHeader` em TODOS os routers que hoje usam `InjectPrincipalFromHeader` (cards, budgets, categories, etc.).

**Skill**: `go-implementation` obrigatória. Refs a carregar: `architecture.md`, `api.md`, `http-handler.md`, `security.md`, `testing-unit.md`.

### B2 — Validação de timestamp do webhook WhatsApp

**Por quê crítico**: hoje só HMAC é validado. Replay com `wamid` "esquecido" após GC do dedup é possível.

**Implementação**:
- Em `internal/platform/whatsapp/handlers/inbound_handler.go`, após dedup e antes de `EstablishPrincipal`:
  - Extrair `entry[].changes[].value.messages[].timestamp` (string unix segundos).
  - Comparar com `time.Now().UTC()` (regra da memory: tempo inline, sem Clock interface).
  - Se `|delta| > 5min`, descartar com **200 OK silencioso** (Meta retransmite em 4xx/5xx).
  - Registrar `auth_events` reason `stale_webhook`.

**Skill**: `go-implementation` obrigatória. Refs: `architecture.md`, `http-handler.md`, `testing-unit.md`.

### B3 — Versionar e auditar `Caddyfile` de produção

**Por quê crítico**: TLS, security headers e bloqueio de admin/pprof ficam no Caddy.

**Implementação** (`deployment/compose/Caddyfile`):
- TLS 1.2+ (Caddy default).
- Global headers: `Strict-Transport-Security: max-age=31536000; includeSubDomains`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Permissions-Policy: ()`, `X-Frame-Options: DENY`.
- Bloquear `/admin`, `/debug/pprof`, `/metrics` (Prometheus deve ser interno só).
- ACME email via env `CADDY_EMAIL`.
- Strip `X-User-ID` e `X-Gateway-Auth` de origem externa.
- Whitelist de IP da LLM ou validação de `X-Gateway-Auth` no próprio Caddy (via `@matcher`).

### B4 — Teste de restore de backup documentado

**Por quê crítico**: backup que nunca foi restaurado não é backup.

**Implementação**:
- Criar `deployment/scripts/pg-restore-smoke.sh`: baixa último dump cifrado via rclone, descriptografa com age, restaura em container Postgres efêmero, roda `SELECT count(*) FROM mecontrola.users` e `SELECT count(*) FROM mecontrola.cards`.
- Agendar cron mensal em staging.
- Documentar em `docs/runbooks/backup-restore.md`.

### B5 — Firewall `ufw` no VPS

**Por quê crítico**: parte do go-live. Postgres/Redis/admin não devem ser alcançáveis de fora.

**Implementação**:
- Documentar em `deployment/runbooks/vps-bootstrap.md`:
  - `ufw default deny incoming` / `ufw default allow outgoing`.
  - `ufw allow 22/tcp` (com chave SSH, sem senha — `/etc/ssh/sshd_config: PasswordAuthentication no`).
  - `ufw allow 80/tcp`, `ufw allow 443/tcp`.
  - `ufw enable`.
- Script idempotente `deployment/scripts/vps-firewall.sh` que aplica e valida.

### B6 — Fechar CORS para domínio fixo em produção

**Implementação** em `configs/config.go` (`Config.Validate`):

```go
if cfg.Environment == "production" {
    origins := cfg.HTTP.CORSAllowedOrigins
    if len(origins) == 0 || slices.Contains(origins, "*") {
        return fmt.Errorf("CORS_ALLOWED_ORIGINS deve ser lista fechada em production")
    }
}
```

- Definir `CORS_ALLOWED_ORIGINS=https://app.mecontrola.com.br,https://checkout.mecontrola.com.br` no `.env` de prod (não commitado).

**Skill**: `go-implementation` obrigatória. Refs: `architecture.md`, `api.md`.

### B7 — Rate limit no webhook WhatsApp

**Por quê crítico**: validação HMAC é CPU-bound; sem limiter, atacante força DoS.

**Implementação**:
- Reusar `internal/onboarding/.../middleware/rate_limit.go` (já é genérico por IP).
- Plugar na chain de `composeWhatsAppWebhookRouter()` em `cmd/server/server.go:190-192`.
- Configurar via novos envs: `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN=600`, `WHATSAPP_WEBHOOK_RATE_LIMIT_BURST=100`.
- Opcional: whitelist dos IPs públicos da Meta Cloud API (publicados pela Meta) para limiter mais frouxo neles.

**Skill**: `go-implementation` obrigatória. Refs: `architecture.md`, `http-handler.md`, `testing-unit.md`.

---

## 6. 🔜 Pós go-live

| Item | Por quê |
|---|---|
| RLS Postgres com `SET LOCAL app.current_user` por request | Defesa em profundidade contra bug futuro de query sem filtro |
| Migrar `X-User-ID` para JWT RS256 com `jti` + JWKS | Quando > 1 gateway/integrador, ou quando houver app móvel direto |
| Audit log imutável (`audit_events` append-only com `CREATE RULE`) | Compliance / forense de operações financeiras |
| Canal Telegram com abstração `channel` | Apenas se demandado; refactor `users.whatsapp_number` → `user_identities (channel, external_id) UNIQUE` |
| Redis para idempotência/dedup/rate-limit distribuído | Quando passar de single-node |
| Docker Secrets ou Vault | Quando houver > 1 host ou equipe > 1 pessoa |
| Rotação automática de `ONBOARDING_TOKEN_ENCRYPTION_KEY` com `kid` | Requer `key_id` por token |
| Alertas Grafana: falhas HMAC/min, 401 burst, signature_status invalid Kiwify, rate limit excedido | Stack OTel/Loki/Grafana já existe — só configurar regras |
| Auditar logs do worker e outbox para mascaramento de PII | Já feito em handlers HTTP; estender |
| Pin de versão exata do Postgres + plano de upgrade documentado | Operacional |

---

## 7. Plano de Ação — Sprint de Hardening (passo a passo)

Sequência ordenada por dependência e risco. Cada item indica skill obrigatória e refs a carregar.

### Passo 1 — B1: Trust boundary LLM↔API (1 dia, BLOQUEANTE)
1. **Carregar skill `go-implementation`**. Refs: `architecture.md`, `api.md`, `http-handler.md`, `security.md`, `testing-unit.md`.
2. Adicionar campo `GatewaySharedSecret` em `configs/config.go` (struct `IdentityConfig`), bind env `IDENTITY_GATEWAY_SHARED_SECRET`, validar não-vazio em `production`.
3. Criar `internal/identity/infrastructure/http/server/middleware/require_gateway_auth.go` com middleware HMAC + janela 60s + zero comentários (R-ADAPTER-001.1).
4. Criar use case `EstablishGatewayAuth` em `internal/identity/application/auth/` se houver lógica que ultrapasse parsing/validação trivial. Se for puro middleware sem regra de negócio, manter inline (adapter fino R-ADAPTER-001.2).
5. Adicionar testes unitários em `_test.go`: assinatura válida, expirada (>60s), inválida, timestamp ausente, secret rotation (current vs next).
6. Plugar `RequireGatewayAuth` ANTES de `InjectPrincipalFromHeader` nos routers: cards, budgets, categories, e qualquer outro que use o injetor.
7. Atualizar `deployment/compose/Caddyfile`: strip de `X-User-ID`/`X-Gateway-Auth` externos + injeção controlada pelo Caddy quando vier do gateway/LLM.
8. Validação R0–R7 via `references/build.md`.

### Passo 2 — B6: CORS fechado em produção (1h, BLOQUEANTE)
1. **Carregar skill `go-implementation`**. Refs: `architecture.md`, `api.md`.
2. Em `Config.Validate()` adicionar guard listado em B6.
3. Teste unitário cobrindo: lista vazia + production → erro; `*` + production → erro; lista válida + production → ok; qualquer config + dev → ok.
4. Atualizar `.env.example` documentando domínios esperados.

### Passo 3 — B3: Caddyfile hardening (0.5 dia, BLOQUEANTE)
1. Adicionar headers de segurança globais.
2. Bloquear `/admin`, `/debug/pprof`, `/metrics`.
3. Configurar matcher `@llm` com IP allow-list ou validação de header de gateway.
4. Commitar em `deployment/compose/Caddyfile` com comentários no formato Caddy (não Go — política Go não se aplica a configs).
5. Smoke test: `curl -I https://api...` validar `Strict-Transport-Security`, `X-Content-Type-Options`.

### Passo 4 — B5: VPS firewall (0.5 dia, BLOQUEANTE)
1. Criar `deployment/scripts/vps-firewall.sh` idempotente.
2. Criar `docs/runbooks/vps-bootstrap.md`.
3. Aplicar em staging primeiro; validar com `nmap` externo.

### Passo 5 — B7: Rate limit no webhook WhatsApp (0.5 dia, BLOQUEANTE)
1. **Carregar skill `go-implementation`**. Refs: `architecture.md`, `http-handler.md`, `testing-unit.md`.
2. Adicionar `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN` e `_BURST` em `configs/config.go`.
3. Plugar middleware existente `rate_limit.go` na chain do router WhatsApp em `cmd/server/server.go`.
4. Teste de integração: 429 acima do burst.

### Passo 6 — B2: Validação de timestamp WhatsApp (2h, BLOQUEANTE)
1. **Carregar skill `go-implementation`**. Refs: `architecture.md`, `http-handler.md`, `testing-unit.md`.
2. Em `inbound_handler.go`, extrair timestamp do payload e comparar com `time.Now().UTC()` (inline, sem Clock — regra de memória).
3. Janela 5 min. Fora dela: 200 OK silencioso + `auth_events` reason `stale_webhook`.
4. Teste unitário: dentro da janela, > 5min no passado, > 5min no futuro, timestamp ausente/inválido.

### Passo 7 — A2/A4: Hardening de headers globais (2h)
1. Revisar fallback CORS no Go (deve ser sempre lista, nunca `*` quando produção).
2. Confirmar que devkit-go não está injetando `Server:` revelador.

### Passo 8 — A5: `request_id`/`client_ip` em `auth_events` (3h)
1. **Carregar skill `go-implementation`**. Refs: `architecture.md`, `persistence.md`, `observability.md`.
2. Migration: `ALTER TABLE mecontrola.auth_events ADD COLUMN request_id TEXT, ADD COLUMN client_ip INET;`
3. Atualizar use case `EstablishPrincipal` para receber `request_id` (extraído de header `X-Request-Id` ou span context) e `client_ip` (de `X-Forwarded-For` sanitizado).
4. Atualizar repositório.

### Passo 9 — B4: Teste de restore de backup (0.5 dia, BLOQUEANTE)
1. Criar `deployment/scripts/pg-restore-smoke.sh`.
2. Criar `docs/runbooks/backup-restore.md`.
3. Agendar cron mensal em staging.

### Passo 10 — A10: Rate limit por user_id (0.5 dia)
1. **Carregar skill `go-implementation`**. Refs: `architecture.md`, `http-handler.md`, `testing-unit.md`.
2. Generalizar `rate_limit.go` para aceitar key extractor (`func(*http.Request) string`); hoje extrai IP, novo modo extrai `principal.UserID`.
3. Plugar nas rotas autenticadas (cards, budgets).

**Total estimado**: ~5 dias úteis para fechar TODOS os bloqueantes + warnings principais.

Segunda onda (pós go-live, 2-3 dias): RLS + audit imutável + alertas Grafana.

---

## 8. Esqueletos de código (referência para execução)

> Os esqueletos abaixo são guias. Durante a implementação, **carregar skill `go-implementation`** e adaptar ao contexto real do código existente — não copiar literalmente. Zero comentários em arquivos `.go` de produção.

### 8.1 — Middleware `RequireGatewayAuth` (B1)

Localização: `internal/identity/infrastructure/http/server/middleware/require_gateway_auth.go`.

Estrutura esperada:
- Construtor `NewRequireGatewayAuth(secret []byte, now func() time.Time)` — **violação de memória** (não abstrair tempo); usar `time.Now().UTC()` inline e parametrizar apenas o secret.
- Função middleware retorna `func(http.Handler) http.Handler`.
- Extrai `X-Gateway-Auth` (hex) e `X-Gateway-Timestamp` (unix string).
- Parseia timestamp; rejeita se `|now - ts| > 60s`.
- Calcula `expected = hmac.SHA256(secret, userID + "." + timestamp)` onde `userID` vem de `X-User-ID` (lido aqui apenas para HMAC; injeção real continua no próximo middleware).
- `hmac.Equal(expected, received)` — constant-time obrigatório.
- Falha → 401 sem body detalhado + `auth_events` reason `invalid_gateway_signature`.

Teste obrigatório cobrindo: HMAC válido, HMAC inválido, timestamp expirado, timestamp futuro fora da janela, header ausente, secret rotation (current vs next).

### 8.2 — Validação de timestamp WhatsApp (B2)

Em `inbound_handler.go`, após o dedup e antes do `EstablishPrincipal`:

```go
ts, err := strconv.ParseInt(message.Timestamp, 10, 64)
if err != nil {
    return // 200 OK silencioso, registrar auth_events reason=invalid_webhook_timestamp
}
delta := time.Now().UTC().Sub(time.Unix(ts, 0).UTC())
if delta > 5*time.Minute || delta < -5*time.Minute {
    return // 200 OK silencioso, registrar auth_events reason=stale_webhook
}
```

Sem comentários no arquivo final. Os comentários acima são apenas para o plano.

### 8.3 — Boot fail CORS em produção (B6)

Em `configs/config.go` (`Config.Validate`):

```go
if cfg.Environment == EnvProduction {
    origins := cfg.HTTP.CORSAllowedOrigins
    if len(origins) == 0 {
        return fmt.Errorf("CORS_ALLOWED_ORIGINS obrigatorio em production")
    }
    for _, o := range origins {
        if o == "*" {
            return fmt.Errorf("CORS_ALLOWED_ORIGINS=* proibido em production")
        }
    }
}
```

### 8.4 — Esboço da migration RLS (pós go-live)

```sql
-- pós go-live, fora deste sprint
ALTER TABLE mecontrola.cards ENABLE ROW LEVEL SECURITY;
ALTER TABLE mecontrola.cards FORCE ROW LEVEL SECURITY;
CREATE POLICY cards_user_isolation ON mecontrola.cards
    USING (user_id::text = current_setting('app.current_user', true));
-- repetir para: budgets, budgets_expenses, budgets_alerts,
-- identity_entitlements, billing_subscriptions, billing_kiwify_events,
-- onboarding_tokens, platform_idempotency_keys, meta_processed_messages,
-- user_whatsapp_history, auth_events, outbox_events (se contiver user_id).
```

Requer wrapper `WithUser(ctx, userID, fn)` que faz `SET LOCAL app.current_user = $1` na conexão antes de cada query. Refactor não-trivial.

---

## 9. Verificação End-to-End

Após implementar o sprint, executar:

1. **Trust boundary** — `curl -H "X-User-ID: <uuid alheio>" https://api.mecontrola.com.br/api/v1/cards` retorna **401** (sem `X-Gateway-Auth` válido).
2. **CORS** — `curl -H "Origin: https://evil.com" -I https://api.../api/v1/cards` **não** retorna `Access-Control-Allow-Origin: https://evil.com`.
3. **HMAC WhatsApp** — `curl -X POST -d '{"x":1}' https://api.../api/v1/whatsapp` (sem header de assinatura) retorna **401** e gera `auth_events.failed` reason `invalid_signature`.
4. **Replay WhatsApp** — repostar payload válido com timestamp > 5 min é descartado (200 silencioso) e gera `auth_events.failed` reason `stale_webhook`.
5. **Rate limit WhatsApp** — `hey -n 1000 -c 50 https://api.../api/v1/whatsapp` atinge 429 antes de saturar CPU.
6. **Idempotência** — POST `/cards` com mesmo `Idempotency-Key` e body diferente → **409**.
7. **Backup restore** — `pg-restore-smoke.sh` em staging restaura dump cifrado e roda smoke queries.
8. **Firewall** — `nmap` externo só responde 22, 80, 443.
9. **Distroless** — `docker exec server sh` falha (sem shell).
10. **CSP/HSTS** — `curl -I https://api.../healthz` mostra `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`.
11. **Admin endpoints** — `curl -I https://api.../debug/pprof` retorna **404** ou **403**.
12. **Métricas Grafana** — dashboards mostram `auth_events.failed` por reason e `kiwify.signature_status` por estado.
13. **Validação Go** — `task lint && task test && task vulncheck` passam; `grep` da R-ADAPTER-001.1 retorna vazio para novos arquivos.

---

## 10. Riscos residuais aceitos

- **Sem RLS em DB**: aceito até pós go-live. Mitigado por A9 (teste de integração de isolamento).
- **`.env` em filesystem**: aceito para VPS solo; revisar quando equipe > 1.
- **Rate limit in-memory**: aceito para single-node; revisar com Redis se escalar horizontalmente.
- **HMAC-SHA1 Kiwify**: aceito por dependência externa; mitigado por TLS + alerta de invalid bursts.
- **`source=header` em Principal**: aceito após B1; o gateway HMAC é a prova de origem.

---

## 11. Skills e Refs obrigatórias por passo

| Passo | Skill | Refs go-implementation |
|---|---|---|
| 1 (B1) | go-implementation | architecture, api, http-handler, security, testing-unit |
| 2 (B6) | go-implementation | architecture, api |
| 3 (B3) | — (config Caddy) | n/a |
| 4 (B5) | — (shell/ufw) | n/a |
| 5 (B7) | go-implementation | architecture, http-handler, testing-unit |
| 6 (B2) | go-implementation | architecture, http-handler, testing-unit |
| 7 (A2/A4) | go-implementation | architecture, api |
| 8 (A5) | go-implementation | architecture, persistence, observability |
| 9 (B4) | — (shell/cron) | n/a |
| 10 (A10) | go-implementation | architecture, http-handler, testing-unit |

Regra de economia: **máximo 4 refs simultâneas**. Carregar somente o necessário por passo.

Regra inegociável: **R-ADAPTER-001.1 (zero comentários em `.go`)** e **R-ADAPTER-001.2 (adapters finos handler→usecase)** em todo arquivo novo ou modificado nos quatro caminhos de adapter.

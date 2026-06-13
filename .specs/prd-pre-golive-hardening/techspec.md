<!-- spec-hash-prd: 46b7d3b20fe8bb169b87b9034e80bb8ce7df9cf79b94745d9a14b4b57d90183b -->
<!-- spec-hash-techspec: 00048a96b0780bcb4c4b6bb9741d3e07b5d6f4cd2206b03d6d00e5cdcf361513 -->
# Especificação Técnica — Pre Go-Live Hardening

## Resumo Executivo

Implementação dos 8 itens diretos do plano-fonte de auditoria pré go-live, agrupados em entregas atômicas de baixa complexidade. Sem decisão de design pendente (todas cravadas no plano-fonte). Sem ADRs novas. Sem techspec adicional além desta — cada item ≤ 1 arquivo modificado ou criado. Skill `go-implementation` aplicada em toda alteração Go.

A divisão por natureza:
- **Go MVP**: B2, B6, B7, A2/A4, A10 (5 alterações cirúrgicas em código existente).
- **Infra/runtime**: B3 (Caddyfile), B5 (ufw VPS).
- **Operação**: B4 (script + runbook + cron).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos**

| Componente | Caminho | Responsabilidade |
|---|---|---|
| `deployment/compose/Caddyfile` | (versionar se ausente) | Reverse proxy, TLS, headers, strip, blocks |
| `deployment/scripts/pg-restore-smoke.sh` | novo | Restore + smoke automatizado de backup |
| `deployment/scripts/vps-firewall.sh` | novo | ufw idempotente |
| `docs/runbooks/backup-restore.md` | novo | Runbook restore |
| `docs/runbooks/vps-bootstrap.md` | novo | Runbook firewall + SSH hardening |
| `internal/platform/whatsapp/handlers/inbound_handler.go` (mod) | — | Validação timestamp (B2) |
| `configs/config.go` (mod) | — | CORS guard + envs novas WhatsApp/user rate-limit (B6, B7, A10) |
| `cmd/server/server.go` (mod) | — | Plug rate-limit no router WhatsApp (B7) |
| `internal/onboarding/.../middleware/rate_limit.go` (mod) | — | Generalizar key extractor (A10) |
| Rotas autenticadas (mod) | `internal/card/.../router.go` (futuras) | Plug rate-limit por user (A10) |

### Fluxo de Dados Relevante

**B2** — após HMAC válido e dedup por wamid, antes de `EstablishPrincipal`:
```
inbound POST /api/v1/whatsapp/inbound
  -> HMAC OK -> dedup wamid OK
  -> extrair messages[].timestamp
  -> if |now - ts| > 5min -> 200 OK + auth_events reason="stale_webhook" -> stop
  -> else -> EstablishPrincipal -> dispatcher
```

**B7** — chain do webhook WhatsApp:
```
POST /api/v1/whatsapp/inbound
  -> rate-limit (IP)  [NOVO]
  -> raw body buffer  [existente]
  -> HMAC validation  [existente]
  -> dedup            [existente]
  -> handler          [existente]
```

**A10** — chain de rotas autenticadas:
```
sub.Use(RequireGatewayAuth)       [PRD gateway-auth-forensics]
sub.Use(InjectPrincipalFromHeader)
sub.Use(RequireUser)
sub.Use(RateLimitByUserID)         [NOVO]
sub.Use(idempotencyMiddleware)
```

## Design de Implementação

### Interfaces Chave

**A10 — Rate-limit generalizado**:

```go
type KeyExtractor func(r *http.Request) string

func ByIP(r *http.Request) string                { /* retorna IP */ }
func ByUserID(r *http.Request) string            { /* retorna principal.UserID.String() ou "" */ }
func ByUserIDFallbackIP(r *http.Request) string  { /* combo */ }

type RateLimitConfig struct {
    PerMinute int
    Burst     int
    Extractor KeyExtractor
    Scope     string // "ip" | "user" — usado em métrica
}

func NewRateLimitMiddleware(cfg RateLimitConfig, o11y observability.Observability) func(http.Handler) http.Handler
```

A função existente em `internal/onboarding/...` é refatorada para esse contrato preservando comportamento atual (`ByIP` no extractor default).

### Modelos de Dados

Sem alteração de schema (B2 reusa coluna `reason` em `auth_events` ampliada no PRD gateway-auth-forensics — adicionar valor `stale_webhook` e `invalid_webhook_timestamp` ao CHECK na migration **000016** se este PRD for entregue depois, ou no PR de B2 senão).

### Config (novos envs)

```
# B6
CORS_ALLOWED_ORIGINS=https://app.mecontrola.com.br,https://checkout.mecontrola.com.br

# B7
WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN=600
WHATSAPP_WEBHOOK_RATE_LIMIT_BURST=100

# A10
AUTH_RATE_LIMIT_PER_USER_PER_MIN=120
AUTH_RATE_LIMIT_PER_USER_BURST=60
```

## Abordagem de Testes

### Testes Unitários

- B2: tabela cobrindo dentro/fora da janela, ausência, formato inválido. Mock do `auth_events` publisher.
- B6: tabela `(env, origins) -> err` 4 casos.
- A10: tabela `(extractor, request) -> key` cobrindo ByIP, ByUserID, ByUserIDFallbackIP com/sem Principal.

### Testes de Integração

**Necessário? SIM** — para B7 (chain real do webhook) e A10 (chain com Principal).
- B7: `httptest` mais Postgres testcontainer; provar 429 após burst.
- A10: testar com Principal injetado no context (mock); 429 quando exceder.

### Testes E2E

- B3: smoke `curl -I` listado em `docs/runbooks/caddyfile-smoke.md`.
- B4: cron mensal em staging.
- B5: `nmap` externo manual após deploy.

## Sequenciamento de Desenvolvimento

Ordem por independência e risco:

1. **B6** (CORS guard) — 30 min, zero dependência. Bloqueia boot em production se inseguro.
2. **B2** (timestamp WhatsApp) — 2h, depende apenas de `inbound_handler.go` existente.
3. **B7** (rate limit WhatsApp) — 3h, depende de envs novas + reuso de middleware.
4. **A10** (rate limit por user) — 4h, depende de generalização do middleware existente + plug no router cards.
5. **A2/A4** (fallback CORS + Server header) — 1h, validação de comportamento existente.
6. **B5** (ufw VPS) — 4h, runbook + script + validação em staging.
7. **B3** (Caddyfile) — 4h, hardening + smoke test.
8. **B4** (restore backup) — 4h, script + runbook + cron staging.

Itens 1–5 podem rodar em **paralelo** em PRs separados (não tocam mesmos arquivos).
Itens 6, 7, 8 são infra/ops; paralelizáveis entre si (4 horas cada).

### Dependências Técnicas

- Caddy 2.x já em uso (compose).
- `rclone` + `age` instalados no host (verificar antes de B4).
- `ufw` instalado no VPS (Ubuntu padrão).
- `golang.org/x/time/rate` já em uso (B7, A10).

## Monitoramento e Observabilidade

**Métricas Prometheus novas**

- `whatsapp_webhook_rate_limit_exceeded_total{}` (B7).
- `auth_rate_limit_exceeded_total{scope}` com `scope` ∈ {`ip`, `user`} (A10).
- `whatsapp_stale_webhook_total{reason}` com `reason` ∈ {`stale_webhook`, `invalid_webhook_timestamp`} (B2).

**Alertas**

- B7: `rate(whatsapp_webhook_rate_limit_exceeded_total[5m]) > 1` por 10 min → page.
- B2: `rate(whatsapp_stale_webhook_total[5m]) / rate(whatsapp_webhook_total[5m]) > 0.01` → warn (NTP drift?).

## Mapeamento Requisito → Decisão → Teste

| RF | Item | Implementação | Teste |
|---|---|---|---|
| RF-01–RF-05 | B2 | `inbound_handler.go` + `auth_events` event | unit |
| RF-06–RF-10 | B3 | `Caddyfile` | smoke curl manual |
| RF-11–RF-14 | B4 | `pg-restore-smoke.sh` + runbook + cron | manual + cron staging |
| RF-15–RF-17 | B5 | `vps-firewall.sh` + runbook | `nmap` externo |
| RF-18–RF-20 | B6 | `Config.Validate` | unit |
| RF-21–RF-24 | B7 | `cmd/server/server.go` + middleware | unit + integration |
| RF-25–RF-26 | A2/A4 | `cmd/server/server.go` | manual inspect |
| RF-27–RF-31 | A10 | `rate_limit.go` + router cards | unit + integration |
| RF-32–RF-34 | Cross | `go-implementation` + CI gates | CI |

## ADRs Vinculadas

Nenhuma. Todas as decisões já estavam cravadas no plano-fonte e foram validadas na análise crítica `~/.claude/plans/analise-de-forma-criteriosa-shiny-book.md` seção 3.

## Itens em Aberto

Nenhum.

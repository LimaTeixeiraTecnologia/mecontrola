<!-- spec-hash-prd: 5ee72819335fdd7e6416099724992d9d16d7d75fc87ec5bb9d67f02b72b5fb14 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Gateway Authentication + Auth Event Forensics

## Resumo Executivo

Implementa o middleware `RequireGatewayAuth` (módulo `internal/identity`) que enforça criptograficamente a fronteira de confiança LLM ↔ API via HMAC-SHA256 com rotação `current`/`next`, posicionado **antes** de `InjectPrincipalFromHeader` em todos os routers que consomem o injetor (hoje apenas `internal/card`). A verificação é modelada via DMMF: smart constructors em `domain/valueobjects/` para `GatewaySignature` e `GatewayTimestamp`; discriminated union `GatewayAuthResult` com 5 variantes em `domain/services/`; workflow puro `VerifyGatewayRequest` testável sem mock. O adapter middleware é fino (R-ADAPTER-001.2): parse de header → workflow → match → 401/next.

Adiciona forense em `mecontrola.auth_events` via migration `000015`: colunas `request_id TEXT NULL` e `client_ip INET NULL`, populadas pelo use case `EstablishPrincipal` a partir do header `X-Request-Id` (fallback `trace_id`) e do **último IP** de `X-Forwarded-For` (sob a premissa de que o Caddy é o único proxy confiável). Toda implementação respeita Regras Estritas R0–R7, R-ADAPTER-001 (zero comentários `.go` produção), modelagem DMMF inegociável e regra de memória de não abstrair tempo (`time.Now().UTC()` inline). Sem dependência externa nova.

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos**

| Componente | Caminho | Responsabilidade |
|---|---|---|
| `GatewaySignature` (VO) | `internal/identity/domain/valueobjects/gateway_signature.go` | Smart constructor que valida hex lowercase de 64 caracteres (SHA-256). |
| `GatewayTimestamp` (VO) | `internal/identity/domain/valueobjects/gateway_timestamp.go` | Smart constructor que parseia unix seconds, valida janela ±60s contra `now` passado por argumento. |
| `GatewayAuthResult` (DU) | `internal/identity/domain/services/gateway_auth_result.go` | Discriminated union (sealed via tipo enumerado) com 5 variantes: `Valid`, `Rotated`, `InvalidSignature`, `StaleTimestamp`, `MissingHeader`. |
| `VerifyGatewayRequest` (workflow) | `internal/identity/domain/services/verify_gateway_request.go` | Função pura: `(VerifyRequest, SecretPair, time.Time) GatewayAuthResult`. Sem IO, sem context. |
| `RequireGatewayAuth` (middleware) | `internal/identity/infrastructure/http/server/middleware/require_gateway_auth.go` | Adapter fino que invoca o workflow puro e mapeia o resultado para 200/next ou 401 + outbox. |
| `RecordGatewayAuthFailure` (use case) | `internal/identity/application/usecases/record_gateway_auth_failure.go` | Publica evento `auth.failed` com `reason="gateway_*"` no outbox. Reusa contrato do `prd-auth-foundation`. |
| Migration `000015` | `migrations/000015_auth_events_forensics.up.sql` / `.down.sql` | `ALTER TABLE auth_events ADD COLUMN request_id TEXT NULL, ADD COLUMN client_ip INET NULL`. |

**Modificados**

| Componente | Caminho | Mudança |
|---|---|---|
| `IdentityConfig` | `configs/config.go` | Adiciona `GatewaySharedSecretCurrent` e `GatewaySharedSecretNext` com validação não-vazio em `production`. |
| `EstablishPrincipal` (use case) | `internal/identity/application/usecases/establish_principal.go` | Aceita `RequestID` e `ClientIP` como input e persiste. |
| `auth_events` entity | `internal/identity/domain/entities/auth_event.go` | Adiciona campos `RequestID` (string) e `ClientIP` (*net.IP ou string-validada). |
| Repositório auth_events | (consumer do outbox) | Mapeia novos campos para colunas. |
| `internal/card` router | `internal/card/infrastructure/http/server/router.go` | Insere `RequireGatewayAuth` no chain antes de `InjectPrincipalFromHeader`. |
| `internal/identity/module.go` | Cabeamento do middleware com `SecretPair` lido de config. |

**Inalterados (premissa explícita)**

- Webhooks WhatsApp (`/api/v1/whatsapp/*`) e Kiwify (`/api/v1/kiwify/*`): HMAC próprio, sem gateway.
- `/healthz`, `/readyz`, `/metrics`: sem gateway.
- Onboarding: sem gateway (rate-limit + token mágico).

### Fluxo de Dados

```
HTTP request
   |
   v
Caddy reverse proxy  [strip externo de X-User-ID, X-Gateway-Auth, X-Gateway-Timestamp; inserção de X-Forwarded-For real]
   |
   v
chi router /api/v1/cards
   |
   v
RequireGatewayAuth  --(GatewayAuthResult ≠ Valid|Rotated)--> 401 + publish outbox auth.failed (RecordGatewayAuthFailure)
   |  Valid|Rotated
   v
InjectPrincipalFromHeaderWithO11y  --> ctx com auth.Principal
   |
   v
RequireUserWithO11y  --(Principal ausente)--> 401
   |
   v
Idempotency middleware (POST/PUT/DELETE)
   |
   v
Handler  --> use case  --> repository / outbox
```

`auth.principal_established` agora carrega `request_id` (header `X-Request-Id` ou trace_id) + `client_ip` (último de X-Forwarded-For), publicado via outbox e consumido por `ProjectAuthEvent` que escreve em `auth_events`.

## Design de Implementação

### Interfaces Chave

**Smart constructors (DMMF — invariante no construtor)**

```go
package valueobjects

type GatewaySignature struct{ raw []byte }

func NewGatewaySignature(hex string) (GatewaySignature, error)
func (s GatewaySignature) Bytes() []byte
func (s GatewaySignature) IsZero() bool
```

```go
package valueobjects

type GatewayTimestamp struct{ at time.Time }

func NewGatewayTimestamp(raw string, now time.Time, window time.Duration) (GatewayTimestamp, error)
func (t GatewayTimestamp) Time() time.Time
func (t GatewayTimestamp) Raw() string
```

**Discriminated union (5 variantes)**

```go
package services

type GatewayAuthResultKind uint8

const (
	GatewayAuthValid GatewayAuthResultKind = iota + 1
	GatewayAuthRotated
	GatewayAuthInvalidSignature
	GatewayAuthStaleTimestamp
	GatewayAuthMissingHeader
)

type GatewayAuthResult struct {
	Kind GatewayAuthResultKind
}

func (r GatewayAuthResult) IsAuthorized() bool
```

**Workflow puro**

```go
package services

type VerifyRequest struct {
	UserIDRaw       string
	SignatureRaw    string
	TimestampRaw    string
}

type SecretPair struct {
	Current []byte
	Next    []byte
}

func VerifyGatewayRequest(req VerifyRequest, secrets SecretPair, now time.Time, window time.Duration) GatewayAuthResult
```

Comportamento:
1. Se `UserIDRaw`, `SignatureRaw` ou `TimestampRaw` vazios → `MissingHeader`.
2. `NewGatewayTimestamp(req.TimestampRaw, now, window)` → erro → `StaleTimestamp`.
3. `NewGatewaySignature(req.SignatureRaw)` → erro → `InvalidSignature`.
4. Canonical = `strings.ToLower(req.UserIDRaw) + "." + req.TimestampRaw` (ADR-001).
5. Calcula `hmac_sha256(secrets.Current, canonical)`; `hmac.Equal` → `Valid`.
6. Senão, se `secrets.Next != nil`, calcula com `Next`; `hmac.Equal` → `Rotated`.
7. Caso contrário → `InvalidSignature`.

**Adapter middleware (R-ADAPTER-001.2 fino)**

```go
package middleware

type RequireGatewayAuthDeps struct {
	Secrets       services.SecretPair
	Window        time.Duration
	FailureLogger usecases.RecordGatewayAuthFailure
	O11y          observability.Observability
}

func RequireGatewayAuth(deps RequireGatewayAuthDeps) func(http.Handler) http.Handler
```

A função retornada:
- Inicia span `auth.require_gateway_auth`.
- Lê headers `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp`.
- Invoca `services.VerifyGatewayRequest(req, deps.Secrets, time.Now().UTC(), deps.Window)`.
- `switch result.Kind` — **listagem explícita das 5 variantes**, sem `default`.
  - `Valid` / `Rotated`: incrementa métrica, próxima.
  - Demais: incrementa métrica, chama `deps.FailureLogger.Handle(ctx, input)`, responde 401 com body fixo `{"error":"unauthorized"}`.
- `defer span.End()` com atributo `result`.

### Modelos de Dados

**Tabela `mecontrola.auth_events` — alterações**

```sql
ALTER TABLE mecontrola.auth_events
    ADD COLUMN request_id TEXT NULL,
    ADD COLUMN client_ip  INET NULL;

CREATE INDEX IF NOT EXISTS auth_events_request_id_idx
    ON mecontrola.auth_events (request_id)
    WHERE request_id IS NOT NULL;
```

`down`:

```sql
DROP INDEX IF EXISTS mecontrola.auth_events_request_id_idx;
ALTER TABLE mecontrola.auth_events DROP COLUMN client_ip;
ALTER TABLE mecontrola.auth_events DROP COLUMN request_id;
```

**Enum `reason` em `auth_events`** — adicionar quatro valores ao CHECK existente:

- `gateway_missing_header`
- `gateway_invalid_timestamp`
- `gateway_stale_timestamp`
- `gateway_invalid_signature`

A migration ajusta o CHECK em `up`/`down` preservando os valores legados do `prd-auth-foundation` (`invalid_signature`, `stale_webhook`, etc.).

**Config (`configs/config.go`)**

```go
type IdentityConfig struct {
	// campos existentes...
	GatewaySharedSecretCurrent []byte
	GatewaySharedSecretNext    []byte
	GatewayAuthWindow          time.Duration  // default 60s
}
```

Validate em `production`: `GatewaySharedSecretCurrent` não-vazio, len ≥ 32 bytes.

### Endpoints de API

Nenhum endpoint novo. Middleware aplicado em rotas existentes do router `/api/v1/cards` (única rota atual com `InjectPrincipalFromHeader`).

**Headers de protocolo gateway** (entrada do app, populados pela LLM, strippados pelo Caddy quando externos):

- `X-User-ID: <uuid lowercase>` (já existe; mantido)
- `X-Gateway-Auth: <hex 64 chars hmac-sha256>`
- `X-Gateway-Timestamp: <unix seconds decimal>`

Resposta de falha: HTTP 401, body `{"error":"unauthorized"}`, header `Cache-Control: no-store`. Sem `WWW-Authenticate` (gateway é interno, não user-facing).

## Pontos de Integração

- **Caddy reverse proxy**: pré-requisito B3 do plano-fonte (fora deste PRD). Strip de `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp` em request externa; encaminhamento dos mesmos quando vierem do upstream LLM autorizado.
- **LLM intermediária**: implementa cliente HTTP que calcula HMAC conforme ADR-001 com secret compartilhado lido de seu próprio config. Suposição DEP-02 do PRD.
- **Outbox + Auth Events**: contrato já estabelecido em `prd-auth-foundation`. `RecordGatewayAuthFailure` reusa `ProjectAuthEvent` consumer existente.

## Abordagem de Testes

### Testes Unitários

**`domain/services/verify_gateway_request_test.go`** — cobertura ≥ 95%, **sem mock**:

| Caso | Input | Esperado |
|---|---|---|
| Happy current | sig válido com current | `GatewayAuthValid` |
| Happy rotated | sig válido com next | `GatewayAuthRotated` |
| Missing user_id | `UserIDRaw=""` | `GatewayAuthMissingHeader` |
| Missing signature | `SignatureRaw=""` | `GatewayAuthMissingHeader` |
| Missing timestamp | `TimestampRaw=""` | `GatewayAuthMissingHeader` |
| Timestamp não numérico | `TimestampRaw="abc"` | `GatewayAuthStaleTimestamp` |
| Timestamp +61s | timestamp futuro | `GatewayAuthStaleTimestamp` |
| Timestamp -61s | timestamp antigo | `GatewayAuthStaleTimestamp` |
| Sig com wrong length | hex de 30 chars | `GatewayAuthInvalidSignature` |
| Sig com charset errado | "ZZZ..." | `GatewayAuthInvalidSignature` |
| Sig hex válido mas wrong key | HMAC com secret diferente | `GatewayAuthInvalidSignature` |
| Canonical UPPER ≡ lower | UUID maiúsculo + sig lower | `GatewayAuthValid` (lowercase no canon) |
| Vetor fixo conhecido | input fixo → hex fixo (referência cross-lang) | `GatewayAuthValid` |

**`domain/valueobjects/gateway_signature_test.go`** — cobertura ≥ 95%:
- Tabela `(input, ok)` cobrindo: 64 chars hex válido, 64 chars com upper, 63 chars, 65 chars, charset inválido, vazio.

**`domain/valueobjects/gateway_timestamp_test.go`** — cobertura ≥ 95%:
- Tabela com `now`, `raw`, `window`, `expectErr`. Casos: dentro, na borda, fora ±, formato inválido, negativo.

**`infrastructure/http/server/middleware/require_gateway_auth_test.go`** — cobertura ≥ 85%:
- `httptest.NewRecorder` + `http.NewRequest`. Mock apenas de `RecordGatewayAuthFailure` (gerado por mockery, padrão do repo).
- Casos: 200 com Valid, 200 com Rotated, 401 para cada uma das 3 falhas, header de span/metric, body fixo, `Cache-Control: no-store`.

### Testes de Integração

**Decisão:** o projeto SIM precisa de integration tests para este escopo. Critérios:
- [x] Fronteira IO crítica: `auth_events` é tabela auditável; mocks não garantem que a coluna `request_id`/`client_ip` é persistida corretamente.
- [x] Risco de regressão: o gate de auth tocou em chain de middlewares — unit test isolado não cobre ordem real.
- [x] Custo proporcional: `testcontainers-go` já é usado no repo (`migrations_integration_test.go`).

**Suite `integration_test.go` em `internal/identity/infrastructure/http/server/middleware/`** com build tag `//go:build integration`:
- Provisiona Postgres via testcontainers, aplica migrations.
- Levanta servidor HTTP mínimo com chain `RequireGatewayAuth → InjectPrincipal → RequireUser → handler-stub`.
- Cenários: request com gateway válido persiste `auth_events` row com `request_id` + `client_ip` corretos; request inválido persiste row com `reason="gateway_invalid_signature"` e 401.

### Testes E2E

Cobertos pelo smoke test do plano-fonte seção 9, item 1 (`curl` externo retorna 401). Sem suíte adicional.

### Microbenchmark de overhead

`require_gateway_auth_bench_test.go` (build tag `//go:build bench` ou padrão `testing.B`):
- `BenchmarkRequireGatewayAuth_Valid` — mede ns/op + alloc/op do happy path.
- Target: p99 ≤ 2 ms em ambiente de CI; alvo de bench < 50µs por request.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Config + validate** (`configs/config.go`): adiciona campos + validate. Bloqueia se ausente em production. Teste unitário do Validate.
2. **Smart constructors** (`domain/valueobjects/gateway_signature.go`, `gateway_timestamp.go`) + testes. Sem dependência de outro componente novo.
3. **Discriminated union + workflow puro** (`domain/services/gateway_auth_result.go`, `verify_gateway_request.go`) + testes. Depende dos VOs.
4. **Use case `RecordGatewayAuthFailure`** + DTO de input + teste. Reusa contrato outbox existente.
5. **Migration `000015`** + entity update (`entities/auth_event.go` com `RequestID` + `ClientIP`) + repositório mapping. Teste de integração de migration (padrão do repo).
6. **Modificação de `EstablishPrincipal`** (use case) para aceitar `request_id`/`client_ip` + teste.
7. **Middleware `RequireGatewayAuth`** (adapter fino) + teste unitário.
8. **Cabeamento em `internal/identity/module.go`** expondo factory do middleware com `SecretPair` da config.
9. **Plug no `internal/card` router** com ordem cravada em ADR-004.
10. **Integration test** cobrindo chain real + persistência de `auth_events`.
11. **Microbenchmark** + valida M-03.
12. **Gate de revisão** (`deployment/scripts/lint-auth-bypass.sh`) chamado por `task lint:auth-bypass`. Implementa M-09.
13. **Runbook** (`docs/runbooks/gateway-auth.md`) + procedimento de rotação.

### Dependências Técnicas

- Tabela `auth_events` (já existe).
- Outbox e `ProjectAuthEvent` consumer (já existem).
- `chi/v5` router (já em uso).
- `crypto/hmac` + `crypto/sha256` (stdlib).
- Nenhuma nova dependência em `go.mod`.

## Monitoramento e Observabilidade

**Métricas Prometheus**

- `identity_gateway_auth_total{result}` — counter. `result` ∈ {`valid`, `rotated`, `missing_header`, `invalid_timestamp`, `stale_timestamp`, `invalid_signature`}. Sem `user_id` como label.
- `identity_gateway_auth_duration_seconds` — histogram. Buckets: 0.0001, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.05.

**Spans OTel**

- `auth.require_gateway_auth` com atributos: `result`, `rotated` (bool), `has_user_id` (bool). **Sem** `user_id`, sem `signature`, sem `timestamp` raw.

**Logs estruturados (slog)**

- Em falha: `slog.Warn("gateway auth failed", "result", ..., "request_id", ..., "client_ip", ...)`. Nunca logar `signature` nem secrets. Mascarar `user_id` parcial (primeiros 8 chars).

**Alertas (configurados pós-PRD na segunda onda do plano-fonte)**

- `rate(identity_gateway_auth_total{result=~"invalid_.*|stale_.*|missing_.*"}[5m]) > 1` por mais de 10 min → page operador.
- `increase(identity_gateway_auth_total{result="rotated"}[1h]) > 0` durante janela de rotação esperada → info.

## Mapeamento Requisito → Decisão → Teste

| RF | Decisão | Implementação | Teste |
|---|---|---|---|
| RF-01, RF-13 | ADR-004 | `internal/card/.../router.go` chain order | integration |
| RF-02, RF-03 | ADR-006 | `middleware/require_gateway_auth.go` | unit + integration |
| RF-04 | ADR-001 | `services/verify_gateway_request.go` canonical | unit + vetor fixo |
| RF-05, RF-06 | ADR-002 | `configs/config.go` + `SecretPair` | unit Validate |
| RF-07 | ADR-006 | `hmac.Equal` no workflow | unit |
| RF-08 | reuso outbox | `usecases/record_gateway_auth_failure.go` | unit + integration |
| RF-09, RF-10 | ADR-006 | métricas no middleware | observação manual |
| RF-11 | DMMF | VOs + DU + workflow puro | unit (sem mock) |
| RF-12 | ADR-007 | tabela no PRD + router cards | revisão |
| RF-14, RF-15 | A5 | migration `000015` | integration |
| RF-16 | A5 | `establish_principal.go` update | unit + integration |
| RF-17 | ADR-008 | extractor `lastXForwardedFor` em adapter | unit |
| RF-18 | governance | revisão + grep | CI |
| RF-19, RF-20 | governance | task lint/test/vulncheck | CI |
| RF-21 | M-09 | `lint-auth-bypass.sh` | CI |
| RF-22 | ADR-005 | runbook + plano de deploy | manual |
| RF-23 | constraint | revisão `go.mod` diff | CI |

## ADRs Vinculadas

1. [ADR-001 — Canonicalização HMAC](adr-001-hmac-canonicalization.md)
2. [ADR-002 — Rotação de secret current/next](adr-002-secret-rotation.md)
3. [ADR-003 — Janela de replay 60s sem cache de nonce](adr-003-replay-window.md)
4. [ADR-004 — Ordem na chain de middlewares](adr-004-middleware-chain-order.md)
5. [ADR-005 — Rollout cutover atômico](adr-005-rollout-cutover.md)
6. [ADR-006 — Política de erro 401 + métrica](adr-006-error-policy.md)
7. [ADR-007 — Tabela de rotas que pulam o gateway](adr-007-routes-skipping-gateway.md)
8. [ADR-008 — Sanitização do X-Forwarded-For](adr-008-xff-sanitization.md)

## Riscos e Desvios

**Desvio R-ADAPTER-001.1 (zero comentários `.go`)**: nenhum desvio. Toda doc explicativa fica em techspec/ADR/runbook.

**Desvio "não abstrair tempo" (regra de memória)**: `time.Now().UTC()` é capturado **no middleware** e passado por argumento ao workflow puro. O workflow não chama `time.Now`. Sem `Clock` interface, sem `now func() time.Time` global. Em teste, a chamada é feita com `time.Time` literal.

**Desvio R6 (context.Context em fronteira IO)**: o workflow puro **não** recebe `ctx` (R6 não exige ctx em função pura sem IO). O middleware (adapter) propaga `ctx` para `RecordGatewayAuthFailure`.

## Itens em Aberto

Nenhum. Todas as 8 decisões materiais cravadas em ADR-001 a ADR-008 nesta entrega.

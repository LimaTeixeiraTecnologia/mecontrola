# Tarefa 7.0: Adapter Kiwify — HTTP client + OAuth + signature + payload mapper + PII redactor

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o adapter concreto `BillingProvider` em `internal/billing/infrastructure/http/client/kiwify/`: cliente HTTP base com OTel; `OAuthClient` com cache de token in-memory e re-auth (ADR-008); `TokenSignatureVerifier` com `subtle.ConstantTimeCompare` e interface plugável para futura migração HMAC (ADR-006); `PayloadMapper` com cascata de extração de `tracking.*` (RF-30) e mapeamento canônico Kiwify → `CanonicalEvent` (RF-29). Implementar `PIIRedactor` manual em-process (ADR-013) consumido pelo `AnonymizeWebhookEventsUseCase`.

<requirements>
- `SignatureVerifier` interface declarada localmente em `kiwify/` permitindo evolução para HMAC sem mudança de RF (ADR-006)
- `TokenSignatureVerifier` usa `crypto/subtle.ConstantTimeCompare` (timing-safe)
- Header name configurável (`X-Kiwify-Webhook-Token` por convenção); lookup case-insensitive
- `OAuthClient` com lock RWMutex + double-check; TTL = `expires_in − safetyMargin` (default 5min); re-auth em 401 (ADR-008)
- `OAuthClient.Token(ctx)` retorna token cacheado válido sem refresh
- `KiwifyAdapter.FetchSubscription` faz `GET /v1/sales/{order_id}` com Bearer + retry único em 401 (ADR-008)
- `PayloadMapper.Parse` mapeia 6 event_types Kiwify para `CanonicalEvent.Type` (RF-29)
- Cascata de extração de signup token: `tracking.src` → `tracking.utm_content` → `tracking.s1` → `tracking.s2` → `tracking.s3` (RF-30, primeira não-vazia vence)
- Normalização `customer.mobile` via `identity.NewWhatsAppNumber` (E.164 BR)
- Sanity check de `period_end` divergente do esperado local com tolerância ±14 dias → métrica `billing_period_divergence_total` (ADR-011)
- Rate limiter local 100 req/min para Kiwify API (RF-31b)
- `PIIRedactor` em `application/services/` com `Strip(json.RawMessage) (json.RawMessage, error)` redactando caminhos canônicos (ADR-013)
- Sem `init()`; OAuth token NUNCA em log; secret/client_secret/client_id redactados via `SafeKiwifyConfig` (R-SEC-001)
- Cobertura ≥ 90% nos arquivos do adapter (CA-04)
</requirements>

## Subtarefas

- [ ] 7.1 `kiwify/client.go` com `Client struct` (httpClient com OTel transport, baseURL, rateLimiter `golang.org/x/time/rate` configurado a 100/60s burst 10).
- [ ] 7.2 `kiwify/oauth.go` com `OAuthClient` conforme techspec (RWMutex + cache + double-check refresh, retorno de `(string, error)`, métodos `Token` e `ForceRefresh`).
- [ ] 7.3 `kiwify/signature_verifier.go` com interface `SignatureVerifier` local + `TokenSignatureVerifier` (constant-time compare + lookup header case-insensitive). Sentinelas `ErrMissingSignature`, `ErrInvalidSignature`.
- [ ] 7.4 `kiwify/payload_mapper.go` com `PayloadMapper struct{}` + método `Parse(raw []byte) (services.CanonicalEvent, error)` extraindo `tracking.*` em cascata, mapeando event_types, validando period_end com tolerância ±14 dias, registrando métrica de divergência. Tipos privados `kiwifyPayload` + `kiwifyTracking`. Sentinelas `ErrPayloadDecode`, `ErrUnknownKiwifyEventType`.
- [ ] 7.5 `kiwify/adapter.go` com `KiwifyAdapter` implementando `interfaces.BillingProvider` (delega para `SignatureVerifier`, `PayloadMapper`, `OAuthClient`, `Client`). Sentinelas `ErrFetchSubscriptionFailed`. Retry único em 401.
- [ ] 7.6 `kiwify/billing_plans_registry.go` com cache em memória de `plan_code → kiwify_product_id` populado na inicialização via `SELECT plan_code, kiwify_product_id FROM billing_plans WHERE active = true AND kiwify_product_id IS NOT NULL`. Método `ParsePlanCodeFromKiwifyProductID(id string) (PlanCode, error)`.
- [ ] 7.7 `application/services/pii_redactor.go` com `PIIRedactor struct{...}` + métodos `Strip`, `redactScalar`, `redactWildcard`, `redactStarMap`. Lista de paths canônicos hardcoded conforme ADR-013.
- [ ] 7.8 Suites unit tests cobrindo OAuth cache/refresh, signature verifier table-driven (header presente/ausente/case-insensitive/valor wrong), payload mapper table-driven (cada event_type + cascata tracking + plan code lookup + period divergence), adapter (FetchSubscription happy + 401 retry + 5xx).
- [ ] 7.9 Fuzz test `FuzzPayloadMapperParse` com seed de payloads válidos/inválidos; nunca panica.

## Detalhes de Implementação

Ver techspec §Adapter Kiwify (oauth.go, signature_verifier.go, payload_mapper.go), ADR-006 (signature pluggable), ADR-008 (OAuth cache), ADR-011 (period_end trust + divergence), ADR-013 (PII redactor manual).

## Critérios de Sucesso

- `go test ./internal/billing/infrastructure/http/client/kiwify/... -cover` retorna ≥ 90% por arquivo.
- `go test ./internal/billing/application/services/... -cover` retorna ≥ 95% para `pii_redactor.go`.
- Test "OAuth cache: 2 chamadas concorrentes geram 1 request a `/oauth/token`" passa (mock httpClient conta chamadas).
- Test "Bearer 401 → ForceRefresh → retry sucesso" passa.
- Test "signature: header em case `x-kiwify-webhook-token` minúsculo é aceito" passa.
- Test "payload tracking.src vence sobre utm_content quando ambos presentes" passa.
- Test "payload period_end divergente +20 dias → métrica incrementada, mapeamento prossegue" passa.
- Test "PIIRedactor.Strip remove customer.cpf/email/mobile, preserva product.id e tracking.src; idempotente" passa.
- Lint verde; `golangci-lint run ./internal/billing/infrastructure/http/client/kiwify/...`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Suite `OAuthClientSuite`: cache hit, cache miss → refresh, concurrent refresh → 1 request, expired → refresh, 4xx error.
- [ ] Suite `TokenSignatureVerifierSuite` table-driven: header presente correto, ausente, wrong-value, case-insensitive variants.
- [ ] Suite `PayloadMapperSuite` table-driven: 6 event_types Kiwify, cascata tracking, plan_code mapping, normalização WhatsApp, period divergence (within tolerance, outside).
- [ ] Suite `KiwifyAdapterSuite`: VerifySignature delega, ParseEvent delega, FetchSubscription happy + 401 retry + 5xx.
- [ ] Suite `PIIRedactorSuite` table-driven: cada path canônico, wildcard, payment.*.card.*, idempotência (2x Strip = mesmo resultado), payload malformado → erro.
- [ ] Fuzz test `FuzzPayloadMapperParse` com corpus seed; nunca panica.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/infrastructure/http/client/kiwify/client.go` (novo)
- `internal/billing/infrastructure/http/client/kiwify/oauth.go` (novo)
- `internal/billing/infrastructure/http/client/kiwify/signature_verifier.go` (novo)
- `internal/billing/infrastructure/http/client/kiwify/payload_mapper.go` (novo)
- `internal/billing/infrastructure/http/client/kiwify/adapter.go` (novo)
- `internal/billing/infrastructure/http/client/kiwify/billing_plans_registry.go` (novo)
- `internal/billing/infrastructure/http/client/kiwify/*_test.go` (novos)
- `internal/billing/application/services/pii_redactor.go` (novo)
- `internal/billing/application/services/pii_redactor_test.go` (novo)
- `go.mod` (alterado — adicionar `golang.org/x/time` se ainda não direta)
- Depende de: task 5.0 (interfaces.BillingProvider, PIIRedactor port), task 3.0 (CanonicalEvent), task 2.0 (PlanCode, MoneyBRL)
- Importa: `internal/identity/domain/valueobjects` (WhatsAppNumber)

# Run — Validação empírica do webhook Kiwify (concluída) + remediação completa

> **Data:** 2026-06-08
> **Executado por:** PO (jailton) + AI assistant
> **Custo do teste:** R$ 5,00 (compra real) + ~R$ 2,94 de taxa Kiwify; R$ 5,00 estornável em 7–15 dias úteis
> **Status:** **REMEDIADO + RECUPERADO** — 9 bugs estruturais detectados e corrigidos; reversão involuntária ocorreu apos remediação inicial e foi recuperada em 5 commits incrementais (ver §Recuperação)
> **Evidência crua (não versionada, PII presente):** `/tmp/kiwify-evidence/*.json` (descartar após uso)

## Resumo executivo

A validação empírica do webhook Kiwify revelou que o módulo `internal/billing` foi escrito contra um envelope especulativo que não bate com o payload real da Kiwify. Em produção pré-remediação, **0% dos webhooks reais seriam aceitos**: middleware HMAC com algoritmo errado e parser de envelope desalinhado. Após a remediação, **100% dos testes unitários + integration test (build tag) compilam e passam**, com o vetor real capturado em sandbox ancorado em `TestHMACSignature_RealKiwifyVector` como teste de regressão permanente.

## Vetores empíricos capturados em sandbox

### Vetor 1 — `order_approved`
- Webhook em `https://webhook.site/...?signature=e8c9bfc3080b49d11d026058171c9061bc5cde95`
- Body 2685 bytes, `webhook_event_type: "order_approved"`
- `TrackingParameters.sck: "01hmtkn-fake-uuid-aaa"` (token propagado intacto da URL)
- Headers presentes: `host`, `traceparent`, `accept-encoding`, `content-length`, `user-agent: axios/1.8.4`, `content-type: application/json`, `accept`
- **Nenhum** header `X-Kiwify-*` enviado

### Vetor 2 — `subscription_canceled`
- Webhook em `https://webhook.site/...?signature=258c8c07d578dbeafbc19c6012fc2a2155e304dc`
- Body 2741 bytes, `webhook_event_type: "subscription_canceled"`
- Mesmo `Subscription.status: "canceled"`

### Protocolo HMAC confirmado
- **Algoritmo:** HMAC-SHA1 (40 chars hex)
- **Encoding:** hexadecimal lowercase
- **Veículo:** query string `?signature=<sig>`
- **Secret:** Token do recurso webhook no painel Kiwify (`9ch0bpzogu9` no sandbox)
- **Payload assinado:** raw body, sem timestamp/prefixo

## Mudanças aplicadas

| # | Camada | Antes | Depois | Arquivo |
|---|---|---|---|---|
| 1 | Middleware HMAC algoritmo | SHA-256 | **SHA-1** | `internal/billing/infrastructure/http/server/middleware/hmac_signature.go` |
| 2 | Middleware HMAC encoding | base64 | **hex** | idem |
| 3 | Middleware HMAC veículo | header `X-Kiwify-Signature` (primário) | **query `?signature=`** (primária), header como fallback | idem |
| 4 | Envelope parser | wrapper `{id, trigger, data: {...}}` | **flat top-level** | `internal/billing/application/usecases/process_kiwify_webhook.go` |
| 5 | Trigger field | `trigger` | **`webhook_event_type`** | idem |
| 6 | Trigger value (venda) | `compra_aprovada` (PT) | **`order_approved`** (EN) | idem + propagado em todos os `event_key` e mocks |
| 7 | Trigger value (reembolso) | `compra_reembolsada` (PT) | **`order_refunded`** (EN) | idem |
| 8 | Struct tags case | `tracking`, `customer`, `subscription` (lowercase) | **`TrackingParameters`, `Customer`, `Subscription`** (PascalCase) | idem |
| 9 | Campo `product_id` | top-level | **`Product.product_id`** (aninhado) | idem |

Adições materiais:
- Tipo `kiwifyTime` que decodifica ambos os formatos da Kiwify (ISO 8601 RFC3339Nano e `YYYY-MM-DD HH:MM` em BRT → UTC).
- Métodos `(p kiwifyWebhookPayload).{approvedAtUTC, renewalAtUTC, cancellationAtUTC, refundAtUTC, funnelToken}` substituem helpers standalone (conformidade R1 da skill go-implementation).
- Test `TestHMACSignature_RealKiwifyVector` ancora o vetor real como regressão permanente.

## Confirmações empíricas registradas

- ✅ **ADR-002b** — protocolo HMAC-SHA1 hex via query string confirmado. ADR-002 antiga (SHA-256/base64/header) substituída.
- ✅ **ADR-004** — `TrackingParameters.sck` chega no payload do webhook produto. Suposição material da ADR refutada (sck *também* chega no webhook, não apenas na Public API).
- ✅ **ADR-005** — `IsRegression(status, trigger, occurredAt, lastEventAt)` chamado em `process_subscription_{late,renewed,canceled}.go` antes de mutação, com `MarkSuperseded` no caminho de regressão.

## Trigger nomenclatura — status por valor

Atualizado em 2026-06-08 após captura empírica de 9 webhooks de produto real via "Testar Webhook" no painel Kiwify (mesmo secret `9ch0bpzogu9`).

| Trigger | Status | Tratamento | Evidência |
|---|---|---|---|
| `order_approved` | ✅ confirmado | dispatch para `ProcessSaleApproved` | vetor 1 (compra real) |
| `subscription_canceled` | ✅ confirmado | dispatch para `ProcessSubscriptionCanceled` | vetor 2 (compra real) + Testar Webhook |
| `subscription_renewed` | ✅ confirmado | dispatch para `ProcessSubscriptionRenewed` | Testar Webhook |
| `subscription_late` | ✅ confirmado | dispatch para `ProcessSubscriptionLate` | Testar Webhook |
| `order_refunded` | ✅ confirmado | dispatch para `ProcessRefundOrChargeback` | Testar Webhook |
| `chargeback` | ✅ confirmado | dispatch para `ProcessRefundOrChargeback` | Testar Webhook |
| `billet_created` | ✅ confirmado | **no-op** (HTTP 202, persistido em `billing_kiwify_events`) | sig `06f61a6a...` validada |
| `pix_created` | ✅ confirmado | **no-op** | sig `c917b9ac...` validada |
| `order_rejected` | ✅ confirmado | **no-op** | Testar Webhook |
| `abandoned_cart` | ✅ confirmado | **no-op** (payload sem `webhook_event_type` — detectado via `status: "abandoned"` ou presença de `id` no top-level) | Testar Webhook |

**Decisão de design (production-ready inegociável):** triggers válidos da Kiwify fora do escopo do MVP (`billet_created`, `pix_created`, `order_rejected`, `abandoned_cart`) são **aceitos com HTTP 202 + persistência completa em `billing_kiwify_events`** — não retornam 422. Razão: separa "evento legítimo fora do MVP" (ignorável) de "trigger desconhecido = drift de contrato com Kiwify" (alertável). Triggers desconhecidos continuam retornando `ErrUnknownTrigger` (HTTP 422) e ficam auditados para investigação manual.

## Tests de regressão ancorados

- `internal/billing/infrastructure/http/server/middleware/hmac_signature_test.go::TestHMACSignature_RealKiwifyVectors` — table-driven com 3 vetores reais byte-exact (`order_approved`, `billet_created`, `pix_created`). Cada vetor recalcula HMAC-SHA1 hex, confronta com sig real e executa o pipeline middleware → 202. Se o protocolo Kiwify mudar, este teste falha primeiro.
- `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_handler_test.go` — cobre todos os 10 triggers + carrinho abandonado + invalid signature + 415/413/422/idempotência/rotação.

## Operações administrativas pendentes do PO

## Gates da skill go-implementation (R0–R7)

- R0 (sem `init()`): ✅ grep vazio
- R5.12 (sem `panic` em produção): ✅ panics apenas em mocks auto-gerados (teste)
- R5.16 (sem `os.Exit`/`log.Fatal` fora de `main`): ✅
- R7.1 (sem `interface{}`): ✅ grep vazio
- R6 (DI via construtor, context na fronteira): ✅
- R7.6 (errors.Join): ✅ aplicado em `parsePayload`
- `go build ./...`: ✅
- `go build -tags integration ./...`: ✅
- `go vet ./internal/billing/...`: ✅
- `go test ./internal/billing/... -count=1`: ✅ verde em todas as packages

## Gaps remanescentes (médio, não bloqueiam go-live)

- Métrica Prom `billing_webhooks_received_total{signature_status}` ainda não exposta (signature_status já é persistido em DB para forensics).
- Métrica `billing_kiwify_tracking_carrier_total{carrier}` ausente.
- Log estruturado `kiwify.tracking.legacy_carrier_seen` ausente.
- Log dedicado de rejeição de assinatura com `request_id` ausente.

Recomendação: agrupar como `task-obs-001-webhook-telemetria` em sprint próxima.

## Operações administrativas pendentes do PO

- [x] Webhook sandbox configurado (`9ch0bpzogu9`).
- [ ] **Reembolsar a venda** `order_id: aac95806-d613-4cc6-80f9-f74882dbbce2`.
- [ ] **Cancelar a assinatura** `subscription_id: 9584c28e-8c7b-44bc-9282-2fa03c45b7db` (já foi cancelada na captura do vetor 2 — confirmar no painel).
- [ ] **Rotacionar o secret** `9ch0bpzogu9` no painel Kiwify após validação.
- [ ] **Apagar evidência local:** `rm -rf /tmp/kiwify-evidence`.

## Referências

- ADR antiga: `.specs/prd-billing-pipeline/adr-002-hmac-sha256-webhook-auth.md` (Substituída)
- ADR vigente: `.specs/prd-billing-pipeline/adr-002b-hmac-sha1-hex-webhook-query-signature.md` (Implementada)
- ADR-004: `.specs/prd-onboarding-magic-token/adr-004-adopt-tracking-sck-as-magic-token-carrier.md` (Confirmada empiricamente)
- Runbook: `docs/runbooks/validacao-webhook-kiwify-sandbox.md`
- Test de regressão: `internal/billing/infrastructure/http/server/middleware/hmac_signature_test.go::TestHMACSignature_RealKiwifyVector`

<!-- spec-hash-prd: 6950ed0b2bba313d9e949e49bf711646b7ea04e015d1d7ab44f1aaac960f06f7 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Análise inicial

## Status do PRD de entrada

- **PRD:** `.specs/prd-onboarding-magic-token/prd.md`
- **Estado:** aprovado em 2026-06-06; 19 requisitos funcionais (RF-01 → RF-19), 10 funcionalidades core (F-01 → F-10), 5 objetivos, 17 suposições/questões abertas (S-01 → S-17). Dependências: bloqueado por E1 (`identity-foundation`, já entregue), co-dependente em runtime de E2 (`billing-pipeline`, já entregue), precede E4 (fora de escopo).
- **Pode prosseguir?:** sim. Decisões abertas tratadas em 2 rodadas multipla-escolha (8 perguntas) com o PO. Material confirmado: carrier `tracking.sck`, módulo dedicado `internal/onboarding`, thank-you como Astro Pages Function na landing, WhatsApp Cloud API direto (Meta), consumer outbox enriquecido para `subscription.paid`, schema dedicado `onboarding`, tabela única `support_signals`.

## Mapeamento do codebase atual

| Caminho | Estado atual | Impacto na techspec |
| --- | --- | --- |
| `cmd/server/server.go` | Wiring de `identity` + `billing` (rota webhook). Usa chi via `devkit-go/pkg/http_server/chi_server`. | Adicionar `onboarding.NewOnboardingModule(...)` e registrar `onboardingModule.PublicRouter` (`POST /v1/onboarding/checkout`, `GET /v1/onboarding/tokens/{token}/state`) e `onboardingModule.WhatsAppRouter` (`POST /webhooks/whatsapp` + `GET /webhooks/whatsapp` para verify token). |
| `cmd/worker/worker.go` | Sobe outbox dispatcher/reaper/housekeeping + jobs de billing (reconciliação, housekeeping kiwify_events). Registra handlers identity no `events.Dispatcher`. | Acrescentar `onboarding.NewOnboardingModule`; registrar consumer `SubscriptionPaidConsumer` no `events.Dispatcher` para tipo `billing.subscription.activated`; registrar 2 jobs no `WorkerManager`: `OutreachJob` (`@every 1h`) e `TokenExpirationJob` (`@every 24h`). |
| `internal/billing/application/usecases/process_kiwify_webhook.go` | Lê `tracking.s1` com fallback para `tracking.src` (ver `funnel_token.go`). `s1` não é campo documentado oficialmente. | **Drift material — D-01.** Adaptar `trackingData` para ler `sck` (campo nativo, confirmado em techspec de E2 §6.3 e em `GET /v1/sales/{id}`). Mantém `s1` apenas como fallback durante transição (toggle por config). ADR-004. |
| `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go` | Publica `subscription.activated`, `subscription.renewed`, `subscription.canceled`, `subscription.refunded`, `subscription.late` para a outbox. Payload contém `subscription_id`, `funnel_token`, `status`, `plan_id`, `period_end`. | Enriquecer payload de `subscription.activated` (renomeado conceitualmente para `subscription.activated` na fronteira, mas mantendo type literal) com `customer_mobile_e164` (digitado no checkout), `customer_email`, `external_sale_id`, `paid_at`. Quando `funnel_token` vazio, publicar `subscription.activated_without_token` (novo type). |
| `internal/identity/application/usecases/upsert_user_by_whatsapp.go` | Recebe (whatsapp normalizado em E.164, email opcional), retorna user com reanimação automática se soft-deleted dentro de janela. | Consumido pelo use case `ConsumeMagicToken` (cria/atualiza user pelo número real do canal). |
| `internal/identity/application/usecases/find_user_by_whatsapp.go` | Lookup por número E.164. | Consumido por `ConsumeMagicToken` (deduplica caso já exista). |
| `internal/identity/domain/valueobjects/whatsapp_number.go` | Normaliza E.164 com regras BR (já implementada). Reutilizar. | Usado para validar `customer_mobile` extraído do webhook E2 antes de gravar e para validar `from` do inbound WhatsApp. |
| `internal/identity/infrastructure/messaging/database/consumers/entitlement_projector.go` | Projeta `identity.entitlements` a partir de `billing.subscription.*`. | Após `ConsumeMagicToken` linkar `user_id` na subscription via novo evento `onboarding.subscription_bound`, identity reprojeta o entitlement do user. |
| `internal/platform/outbox/{publisher,dispatcher}.go` | `Publisher.Publish(ctx, evt)` valida UUID/type/aggregate/payload JSON. Consumer registra-se em `events.Dispatcher` por type. | Onboarding registra handler em `events.Dispatcher` para `billing.subscription.activated` / `billing.subscription.activated_without_token`. Onboarding também publica `onboarding.subscription_bound` para reprojeção de entitlement. |
| `internal/platform/httpclient/client.go` | Wrapper com timeout, base URL, retry opcional, observabilidade. | Cliente Meta Graph API (`https://graph.facebook.com/v18.0/{phone_number_id}/messages`) e WhatsApp Business Cloud envia outreach por esse client. |
| `internal/platform/worker/{manager,consumer/registration}.go` | `WorkerManager` orquestra `worker.Job` (`Name`/`Schedule`/`Run`). | Jobs novos: `OutreachJob` (`@every 1h`) e `TokenExpirationJob` (`@every 24h`). |
| `internal/billing/infrastructure/http/server/middleware/hmac_signature.go` | Middleware HMAC-SHA256 reutilizável. | Não reutilizado: Meta usa HMAC-SHA256 sobre raw body com header `X-Hub-Signature-256` (algoritmo idêntico, header diferente, prefixo `sha256=` no valor). Novo middleware `internal/onboarding/infrastructure/http/server/middleware/meta_signature.go` deriva do existente. |
| `configs/config.go` | Já carrega `KiwifyConfig`, `BillingConfig`, `IdentityConfig`. | Adicionar `OnboardingConfig` (TTL token, janela outreach, hosts de checkout/landing, cap rate-limit) e `WhatsAppConfig` (Meta phone_number_id, access_token, app_secret, verify_token, template_name). |
| `migrations/` | 3 migrations outbox + 3 billing + 1 identity + 1 entitlements (já aplicadas). | Adicionar 3 migrations: `0010_create_onboarding_schema_and_tokens.up.sql`, `0011_create_support_signals.up.sql`, `0012_extend_billing_subscriptions_for_onboarding.up.sql` (idempotente; adiciona `customer_mobile_e164`, `customer_email`, `external_sale_id` somente se não existirem). |
| `LimaTeixeiraTecnologia/mecontrola-landingpage` (externo, Astro 5 + Cloudflare Pages) | Hospeda `https://www.mecontrola.app.br/`. Tem rota `/` (landing). Sem rota de obrigado. CSP atual: `connect-src 'self' https://www.google-analytics.com ...`. | Adicionar rota Astro `/obrigado/[token]/index.astro` rodando em **Cloudflare Pages Function** (SSR no edge, fetch server-side ao backend Go); atualizar CSP para permitir `connect-src` do backend; trabalho coordenado em PR no repo da landing. Esta techspec define o **contrato JSON** consumido. |

## Evidências oficiais Kiwify e Meta usadas

| Fonte oficial | Fato confirmado | Impacto técnico |
| --- | --- | --- |
| `https://kiwify.notion.site/Webhooks-pt-br-c77eb84be10c42e6bb97cd391bca9dce` (Webhook produto) | (a) Retry até 5x, timeout 40s, espera 2xx. (b) Signature `?signature=` query string com `hmac_sha1(JSON.stringify(body), secret)` segundo doc. (c) Eventos com `webhook_event_type`: `pix_created`, `billet_created`, `order_rejected`, `order_approved`, `order_refunded`, `chargeback`, `subscription_canceled`, `subscription_late`, `subscription_renewed`. (d) Payload contém `TrackingParameters{src, sck, utm_source, utm_medium, utm_campaign, utm_content, utm_term}`. (e) `Customer.mobile` é o telefone digitado no checkout. (f) Subscription aninhada com `subscription_id`, `plan.id`, `next_payment`. | **Drift D-02 com E2:** doc menciona `signature` via query e SHA1. E2 implementa `X-Kiwify-Signature` header com SHA256 (ADR-002 do E2 marcou como suposição material a validar em sandbox). Este PRD **não toca** no middleware de webhook Kiwify — drift permanece sob escopo de E2 (rastreado em §11). |
| `https://docs.kiwify.com.br/api-reference/sales/single` (Public API consumida em E2) | `sale.tracking` expõe `utm_*`, `sck`, `src`, `s1..s3` simultaneamente. Documento oficial da Public API confirma `sck` como campo nativo. | Carrier oficial do magic token: `sck`. ADR-004. |
| `https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples` (Meta Cloud API) | Webhook envia payload JSON com `entry[].changes[].value.messages[]` contendo `from` (E.164 sem `+`), `id`, `timestamp`, `type`, `text.body`. Assinatura via `X-Hub-Signature-256: sha256={hex}` sobre raw body com `App Secret`. `GET` ao endpoint para verificação inicial: query params `hub.mode=subscribe`, `hub.challenge`, `hub.verify_token` → responder com `hub.challenge` em texto puro se `verify_token` confere. | Middleware `MetaSignatureMiddleware`; handler `WhatsAppInboundHandler` com dois métodos (GET verify + POST inbound). |
| `https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages` | `POST /{phone_number_id}/messages` envia template com `type=template`, `template.name`, `template.language.code`, `template.components[]` (header/body/buttons). Rate limit por número (telefone + tier). Resposta inclui `messages[0].id` (WAMID). | Cliente outbound `WhatsAppCloudClient.SendTemplate(ctx, to_e164, template_name, params)` retorna `wamid`. |
| `https://developers.facebook.com/docs/whatsapp/cloud-api/guides/send-messages` | Templates requerem aprovação prévia da Meta. Sem aprovação, erro `131051` (template not found). | `OutreachJob` checa toggle `WhatsAppConfig.OutreachEnabled` em runtime; se desligado, job loga e retorna sem efeito (S-04). |
| **Lacuna oficial Kiwify** | Não documenta propagação para URL de redirect pós-pagamento (a doc menciona `access_url` que é o link de cadastro de senha na área de membros Kiwify). | Redirect pós-pagamento da Kiwify deve ser configurado no painel para `https://www.mecontrola.app.br/obrigado/{sck}` usando substituição de parâmetro pelo próprio painel (operação fora desta entrega — registrado em §11 como pré-requisito operacional). Caso o painel não suporte placeholders, fallback é a Pages Function ler `?s=` da query string passada pelo cliente. |

## Bloqueios e dúvidas residuais

1. **L-01 (E2 carrier `s1` → `sck`):** trocar a leitura em `internal/billing/application/usecases/funnel_token.go` para priorizar `sck`. **Não bloqueia esta techspec** — encapsulado em ADR-004 com plano de migração: aceitar `sck` OU `s1` durante 30 dias, registrar log `kiwify.tracking.legacy_carrier_seen` quando vier `s1`; remover suporte a `s1` após 0 ocorrências em 7 dias.
2. **L-02 (placeholder de redirect Kiwify):** confirmar capacidade de substituição de `?s={tracking_sck}` na URL de redirect do painel Kiwify. Não bloqueia — fallback descrito em §6.5 (Pages Function aceita token tanto via path `/obrigado/{token}` quanto query `?s={token}` para resiliência).
3. **L-03 (template Meta `activation_reminder`):** aprovação Meta é dependência externa (S-04). Não bloqueia — job sobe com toggle desligado em produção até aprovação.

Nenhum bloqueio material para escrever a techspec.

---

# Techspec — Onboarding via Magic Token MVP

## 1. Objetivo técnico e recorte do MVP

Entregar a ponte confiável entre o pagamento aprovado (E2) e o número real de WhatsApp do cliente (canal Meta), por meio de um **magic token opaco** que viaja no checkout Kiwify (campo `tracking.sck`), é marcado como pago via evento outbox de E2 (`billing.subscription.activated` enriquecido), é apresentado ao cliente em página de agradecimento sob domínio MeControla (Astro Pages Function), e é resgatado pelo comando `ATIVAR <token>` no WhatsApp via webhook Meta Cloud API.

Recorte estrito **dentro do MVP**:
- 1 provedor de pagamento: Kiwify (E2 já implementado).
- 1 canal de mensageria: WhatsApp Business Cloud API (Meta direta, sem provedor terceiro).
- Estados do token estritamente: `PENDING`, `PAID`, `CONSUMED`, `EXPIRED`.
- TTL token: 7 dias corridos (RF-01).
- Outreach: 1 envio único por token, janela `> 2h` em PAID sem CONSUMED (RF-09).
- Fallback E.164: gated por outreach já enviado (RF-10, divergência deliberada de discovery — S-09).
- Cleanup expiração: cadência diária (RF-11).
- Thank-you page: Astro Pages Function na landing + endpoint state JSON no Go (RF-04, RF-05, RF-17).
- Acessibilidade: WCAG 2.1 AA nos elementos críticos do caminho feliz (RF-19).
- Suporte geográfico: somente E.164 BR (RF-16).
- Sinais para suporte: tabela única `support_signals` (RF-12, RF-15, RF-18).
- Telemetria: contadores + gauge + histograma Prometheus (RF-13).

**Fora do MVP** (apenas registrado, alinhado ao PRD §"Fora de Escopo"): segundo envio de outreach, canal dedicado de alerta de fraude (Slack/email), antifraude estatística, idempotência por `Idempotency-Key`, cancelamento automático de assinatura órfã, painel admin de suporte, multi-país no normalizador, oracle público de estado na thank-you page.

## 2. Requisitos do PRD mapeados para decisões técnicas

| Requisito | Decisão técnica | Mecanismo |
| --- | --- | --- |
| RF-01 (token opaco 7d, embutido em `?s=`) | Token = 32 bytes `crypto/rand` → `base64url` sem padding (43 chars). Persistido apenas como `token_hash = sha256(token)` hex. Carrier Kiwify = `tracking.sck` (ADR-004). | `POST /v1/onboarding/checkout` cria registro com `status=PENDING`, `expires_at=now+7d`, retorna URL Kiwify com `?sck={token}` apensado. |
| RF-02 (rate limit 10/min/IP) | Middleware `internal/onboarding/infrastructure/http/server/middleware/rate_limit.go` baseado em `golang.org/x/time/rate` (já em go.mod) com map[ip]*rate.Limiter sob `sync.Mutex` + janela limpeza. **IP real (deploy VPS Hostinger, ADR-009):** ordem de prioridade `X-Real-IP` (nginx/caddy reverse proxy local) → primeiro IP em `X-Forwarded-For` → `RemoteAddr`. Trust de proxy controlado por `OnboardingConfig.TrustedProxies` (whitelist de redes/IP que podem injetar esses headers; default: `127.0.0.1/32, ::1/128`). Requisições não vindas de proxy confiável caem para `RemoteAddr` direto. | 429 sem criar token; métrica `onboarding_checkout_rate_limited_total`. |
| RF-03 (marcar token como pago) | Consumer `SubscriptionPaidConsumer` registrado em `events.Dispatcher` para `billing.subscription.activated`. Use case `MarkTokenPaid`. Idempotente por `event_id` (já garantido pelo outbox dispatcher do platform). | Transition `PENDING → PAID` no token correspondente ao `funnel_token`. Persiste `customer_mobile_e164`, `customer_email_masked`, `external_sale_id`, `paid_at`. |
| RF-04 / RF-05 (thank-you sob domínio + auto-redirect mobile + fallback copy-paste) | Rota Astro `/obrigado/[token]/index.astro` na landing (Cloudflare Pages Function) consome `GET /v1/onboarding/tokens/{token}/state` → retorna booleano `ready_to_activate` + `wa_me_url` + `bot_number_display`. Renderiza HTML server-side com botão `wa.me/{bot}?text=ATIVAR+{token}` + `<noscript>` fallback de copy-paste + `<meta refresh>` 800ms mobile-only via UA hint. | Página estática semanticamente correta (`<button>`, `<a>`, `aria-label`); `<noscript>` mantém fallback visível. |
| RF-06 (estados do bot ATIVAR) | Use case `ConsumeMagicToken` aplica máquina de estados explícita (tabela §5.4). | Mensagens distintas por estado (texto fixo configurável via `WhatsAppConfig.ActivationMessages`). |
| RF-07 (ativação atômica) | `ConsumeMagicToken` executa em transação única (UoW) via `devkit-go/pkg/database/uow`: (a) `UpsertUserByWhatsApp` (E1), (b) update `subscription.user_id`, (c) update `onboarding_tokens.status=CONSUMED + consumed_at + consumed_by_user_id + consumed_by_mobile_e164`, (d) publica `onboarding.subscription_bound` na outbox. | Falha em qualquer etapa → rollback completo. Cliente recebe mensagem `system_unavailable_retry` (retornável). |
| RF-08 (idempotência ATIVAR mesmo número) | Em `CONSUMED` e `consumed_by_mobile_e164 == from_e164` → no-op feliz, mensagem `already_active`. | Sem novo INSERT, sem evento duplicado. |
| RF-09 (outreach 1x, janela >2h, cadência horária) | Job `OutreachJob` cron fixo `5 * * * *` (5 min após hora cheia, UTC): select `WHERE status='PAID' AND outreach_sent_at IS NULL AND paid_at < now()-2h AND customer_mobile_e164 IS NOT NULL` `LIMIT 100` `FOR UPDATE SKIP LOCKED`. Marca `outreach_sent_at=now()` antes do envio (locking otimista por linha). | Envia template `WhatsAppConfig.OutreachTemplateName` via cliente Meta. Falha de envio: log warn + reset `outreach_sent_at=NULL` apenas se erro 5xx (retry no próximo tick). Erro 4xx: persiste `outreach_sent_at` para nunca reenviar (S-11). |
| RF-10 (fallback E.164 gated por outreach) | Use case `TryFallbackActivation` chamado pelo `WhatsAppInboundHandler` quando `from_e164` não casa com `ATIVAR <token>` e o resolver acha PENDING/PAID com `customer_mobile_e164 == from_e164 AND outreach_sent_at IS NOT NULL`. | Executa mesma transação de `ConsumeMagicToken` com `activation_path='fallback_e164'`. Sem outreach prévio: responde orientação para usar `ATIVAR <token>`. |
| RF-11 (expiração diária) | Job `TokenExpirationJob` cron fixo `0 3 * * *` (03:00 UTC, ~00:00 BRT). Update `status='EXPIRED'` `WHERE status IN ('PENDING','PAID') AND expires_at < now()`. Emite linha em `support_signals` por token PAID expirado (RF-12). | Operação em batches `LIMIT 1000` com loop até zero linhas. |
| RF-12 (sinal subscription órfã expirada) | Insere em `support_signals` `(kind='orphan_expired_subscription', payload={external_sale_id, token_hash_prefix, expired_at, has_paid_state:true})`. Subscription E2 **não é cancelada**. | Métrica `onboarding_orphan_expired_total`. |
| RF-13 (métricas) | Prometheus expostas via `/metrics` existente: contadores, gauge, histograma (lista §9.2). | Labels: `activation_path={direct,fallback_e164,outreach}`, `reason`. |
| RF-14 (mascarar PII em logs) | VOs `MaskedMobile`/`MaskedEmail` em `internal/identity/domain/pii/mask.go` (já existem). Reaproveitar. Nenhum log emite mobile/email sem mascarar. | Lint check + revisão code review. |
| RF-15 (sinalização reuso por outro número) | `ConsumeMagicToken` em `CONSUMED` com `consumed_by_mobile_e164 != from_e164` → insere `support_signals(kind='token_reuse_attempt', payload={...})` + métrica `onboarding_token_reuse_attempt_total{reason="different_number"}` + log estruturado. | Cliente recebe mensagem `code_already_used_other_account`. |
| RF-16 (BR only) | Validação via `valueobjects.NewWhatsAppNumber` (E1). Erro de normalização → mensagem `invalid_country` + log `onboarding.identity.unsupported_country`. | Em pontos: extração de `Customer.mobile` em E2→E3, `from` no inbound. |
| RF-17 (thank-you mensagem genérica única) | Endpoint `GET /v1/onboarding/tokens/{token}/state` retorna **apenas booleano** `ready_to_activate`. Estados internos não vazam no HTTP. Métrica `ty_page_invalid_access_total{reason=...}` emitida server-side com motivo real. | Pages Function renderiza mensagem fixa "Link inválido ou expirado. Fale com nosso suporte." quando `ready_to_activate=false`. |
| RF-18 (paid sem token) | E2 publica `billing.subscription.activated_without_token` quando `funnel_token` vazio. Consumer onboarding insere `support_signals(kind='paid_without_token', payload={external_sale_id, customer_email_masked, customer_mobile_masked, paid_at})`. | Métrica `billing_paid_without_token_total` (label `provider=kiwify`). Pagamento **não** descartado. |
| RF-19 (WCAG 2.1 AA) | Pages Function Astro com semântica HTML estrita (button real, aria-label, contraste validado via tokens Tailwind, viewport meta, `<noscript>` visível). Testes axe-core no pipeline da landing (fora deste repo, contrato declarado em ADR-007). | Critério de aceite: axe-core sem violações críticas no `/obrigado/{token}` de teste. |

## 3. Estado atual do codebase e gaps reais

Estado verificado no working tree:

- `internal/onboarding/` **não existe**. Gap principal.
- `internal/billing/application/usecases/funnel_token.go` lê `s1`/`src` (drift com Kiwify oficial).
- `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go` publica payload sem `customer_mobile_e164` / `customer_email` / `external_sale_id` (necessários a E3). Gap a fechar (alteração mínima de E2).
- `configs/config.go` não tem `OnboardingConfig` nem `WhatsAppConfig`.
- `cmd/server` não conhece `onboarding`; `cmd/worker` idem.
- Nenhuma migration relacionada a onboarding existe.
- Não existe cliente WhatsApp/Meta no repositório.
- Não existe middleware de assinatura Meta (X-Hub-Signature-256).
- Não existe tabela `support_signals`.

Gaps fechados por esta techspec:
1. Módulo `internal/onboarding` completo (domain/application/infrastructure) com DI manual em `module.go`.
2. 3 migrations: schema + tabelas `onboarding_tokens`, `support_signals`, e extensão de payload em E2 (3 colunas idempotentes em `billing_subscriptions` — opcional, ver §6.7).
3. Wiring em `cmd/server` (1 router público + 1 router webhook WhatsApp) e `cmd/worker` (1 consumer + 2 jobs).
4. Ajuste pontual em E2: enriquecer payload do evento `billing.subscription.activated` + emissão de `billing.subscription.activated_without_token`.
5. Ajuste pontual em E2: aceitar `tracking.sck` (ADR-004).
6. Cliente Meta WhatsApp Cloud em `internal/onboarding/infrastructure/http/client/meta`.
7. Middleware `MetaSignatureMiddleware`.

## 4. Arquitetura proposta

Cumpre estritamente o Padrão Obrigatório de Módulo de `AGENTS.md` (camadas `infrastructure → application → domain`, DI manual em `module.go`).

```text
internal/onboarding/
├── doc.go
├── module.go                                  # NewOnboardingModule(...) com DI manual
├── domain/
│   ├── entities/
│   │   ├── magic_token.go                     # Aggregate root: id, token_hash, status, ttl, captured_*, outreach_*, consumed_*
│   │   └── support_signal.go                  # Entity: id, kind, payload, occurred_at, resolved_at
│   ├── valueobjects/
│   │   ├── token.go                           # NewToken() gera 32B base64url; Hash() SHA-256 hex; nunca expõe claro
│   │   ├── token_status.go                    # PENDING/PAID/CONSUMED/EXPIRED + transições
│   │   ├── activation_path.go                 # direct/fallback_e164/outreach
│   │   └── support_signal_kind.go             # orphan_expired_subscription/paid_without_token/token_reuse_attempt
│   ├── services/
│   │   └── transitions.go                     # Função pura: tabela de transições permitidas
│   └── errors.go                              # ErrTokenNotFound, ErrTokenExpired, ErrTokenAlreadyConsumed,
│                                              # ErrTokenNotYetPaid, ErrTokenReuseDifferentNumber,
│                                              # ErrUnsupportedCountry, ErrRateLimited
├── application/
│   ├── interfaces/
│   │   ├── repository_factory.go              # MagicTokenRepository, SupportSignalRepository factories
│   │   ├── magic_token_repository.go
│   │   ├── support_signal_repository.go
│   │   ├── whatsapp_gateway.go                # SendActivationTemplate(ctx, to, token) → wamid
│   │   └── checkout_url_builder.go            # BuildCheckoutURL(ctx, plan_id, token) → url
│   ├── usecases/
│   │   ├── create_checkout_session.go         # RF-01, RF-02
│   │   ├── mark_token_paid.go                 # RF-03 (consumer)
│   │   ├── consume_magic_token.go             # RF-06, RF-07, RF-08, RF-15
│   │   ├── try_fallback_activation.go         # RF-10
│   │   ├── send_outreach.go                   # RF-09 (chamado pelo job)
│   │   ├── expire_tokens.go                   # RF-11, RF-12 (chamado pelo job)
│   │   ├── get_token_state.go                 # RF-17 (boolean único + métrica)
│   │   └── handle_paid_without_token.go       # RF-18 (consumer)
│   ├── dtos/
│   │   ├── input/                             # CreateCheckoutSessionInput, ConsumeMagicTokenInput, ...
│   │   └── output/
│   └── errors.go                              # ErrFunnelTokenMissing, ErrCheckoutUnavailable, ...
└── infrastructure/
    ├── repositories/
    │   ├── factory.go
    │   └── postgres/
    │       ├── magic_token_repository.go
    │       ├── support_signal_repository.go
    │       └── testutil_test.go
    ├── http/
    │   ├── server/
    │   │   ├── router.go                      # PublicRouter, WhatsAppRouter
    │   │   ├── handlers/
    │   │   │   ├── create_checkout_handler.go # POST /v1/onboarding/checkout
    │   │   │   ├── token_state_handler.go     # GET  /v1/onboarding/tokens/{token}/state
    │   │   │   ├── whatsapp_verify_handler.go # GET  /webhooks/whatsapp (Meta verify)
    │   │   │   └── whatsapp_inbound_handler.go# POST /webhooks/whatsapp
    │   │   └── middleware/
    │   │       ├── rate_limit.go              # IP-based 10/min
    │   │       └── meta_signature.go          # X-Hub-Signature-256 HMAC-SHA256
    │   └── client/
    │       └── meta/
    │           ├── client.go                  # Cloud API outbound (templates)
    │           ├── models.go
    │           └── errors.go
    ├── jobs/
    │   └── handlers/
    │       ├── outreach_job.go                # @every 1h
    │       └── token_expiration_job.go        # @every 24h
    ├── messaging/
    │   └── database/
    │       ├── consumers/
    │       │   ├── subscription_paid_consumer.go             # billing.subscription.activated
    │       │   └── subscription_paid_without_token_consumer.go # billing.subscription.activated_without_token
    │       └── producers/
    │           └── onboarding_event_publisher.go             # onboarding.subscription_bound
    └── checkout/
        └── kiwify_url_builder.go              # CheckoutURLBuilder concreto (configura por plan_id)
```

Dependências externas do módulo:
- Entrada (consome): `internal/identity.UpsertUserUseCase`, `internal/identity.FindUserByWhatsApp` (via interfaces na borda — wiring no `module.go`).
- Saída (publica): outbox events `onboarding.subscription_bound` (consumido pelo `EntitlementProjector` de E1).
- Entrada (consome): outbox events `billing.subscription.activated` e `billing.subscription.activated_without_token` (publicados por E2).

## 5. Fluxos ponta a ponta

### 5.1 Sequência canônica (caminho feliz mobile)

```
Landing (Astro)
  └── POST https://api.mecontrola.app.br/v1/onboarding/checkout {plan_id}
        ↓ (rate-limited 10/min/IP)
        ↓ CreateCheckoutSession:
        ↓   token = base64url(rand(32))
        ↓   hash  = sha256_hex(token)
        ↓   INSERT onboarding.onboarding_tokens(token_hash, status=PENDING, expires_at=now+7d, plan_id)
        ↓   url   = CheckoutURLBuilder.Build(plan_id, token)  // appended ?sck={token}
        ↓ HTTP 201 {checkout_url}
Landing → window.location = checkout_url
Cliente paga na Kiwify
Kiwify → POST /api/v1/billing/webhooks/kiwify  (E2 — fora desta techspec)
  └── E2 ProcessKiwifyWebhook (já existente, ajustado em §6.6):
        ↓ extractFunnelToken → preferir tracking.sck, fallback tracking.s1 (legado)
        ↓ persiste Subscription (com customer_mobile_e164, customer_email, external_sale_id)
        ↓ publica evento outbox billing.subscription.activated  com payload enriquecido
Outbox dispatcher → events.Dispatcher
  └── SubscriptionPaidConsumer (onboarding) handle:
        ↓ MarkTokenPaid(token_hash_from_funnel_token):
        ↓   UPDATE onboarding.onboarding_tokens SET status=PAID, paid_at=now,
        ↓                customer_mobile_e164=?, customer_email_masked=?, external_sale_id=?
        ↓   WHERE token_hash=? AND status='PENDING'  ;; idempotente (no-op se já PAID)
        ↓ COMMIT
Kiwify redirect → https://www.mecontrola.app.br/obrigado/{token}   (configurado no painel Kiwify)
Pages Function Astro `/obrigado/[token]` (SSR no edge):
  └── GET https://api.mecontrola.app.br/v1/onboarding/tokens/{token}/state
        ↓ GetTokenState (RF-17 boolean único):
        ↓   token_hash = sha256_hex(token)
        ↓   row = SELECT status, ... WHERE token_hash=?
        ↓   if row.status == 'PAID' && expires_at > now: ready=true
        ↓   else: ready=false + métrica ty_page_invalid_access_total{reason=<internal>}
        ↓ HTTP 200 {ready_to_activate: true, wa_me_url: "https://wa.me/<bot>?text=ATIVAR+<token>",
        ↓           bot_number_display: "+55 11 9XXXX-XXXX"}
  └── Renderiza HTML:
        <button id="cta" data-href="<wa_me_url>">Abrir WhatsApp</button>
        <noscript>Envie no WhatsApp +55 11 9XXXX-XXXX: ATIVAR {token}</noscript>
        <meta http-equiv="refresh" content="0.8;url=<wa_me_url>"> (apenas mobile via UA)
Cliente → WhatsApp envia "ATIVAR {token}"
Meta Cloud → POST https://api.mecontrola.app.br/webhooks/whatsapp
  └── MetaSignatureMiddleware valida X-Hub-Signature-256
  └── WhatsAppInboundHandler:
        ↓ parseia entry[0].changes[0].value.messages[0]
        ↓ from_e164 = "+" + msg.from  // Meta envia sem o "+"
        ↓ text      = msg.text.body
        ↓ if matches /^ATIVAR\s+([A-Za-z0-9_\-]{40,45})$/:
        ↓     ConsumeMagicToken(ctx, ConsumeInput{token, from_e164}, activation_path='direct')
        ↓ else:
        ↓     TryFallbackActivation(ctx, FallbackInput{from_e164})
ConsumeMagicToken (transação UoW única):
  ↓ token_hash = sha256_hex(token)
  ↓ row = SELECT ... WHERE token_hash=? FOR UPDATE
  ↓ apply transitions (tabela §5.4)
  ↓ if PAID + valido:
  ↓     user_id = identity.UpsertUserByWhatsApp(from_e164, captured_email)
  ↓     UPDATE billing_subscriptions SET user_id=? WHERE funnel_token=token
  ↓     UPDATE onboarding_tokens SET status=CONSUMED, consumed_at=now,
  ↓                consumed_by_user_id=user_id, consumed_by_mobile_e164=from_e164,
  ↓                activation_path='direct'
  ↓     outbox.Publish onboarding.subscription_bound{user_id, subscription_id, funnel_token}
  ↓ COMMIT
EntitlementProjector (identity, já existente) consome onboarding.subscription_bound
  e atualiza entitlement do user.
WhatsAppGateway envia mensagem de confirmação ao cliente.
```

### 5.2 Variações por evento

| Evento | Use case | Estado fonte | Estado destino | Mensagem ao cliente |
| --- | --- | --- | --- | --- |
| `ATIVAR <token>` válido + token PAID | `ConsumeMagicToken` | `PAID` | `CONSUMED` | `welcome_activated` |
| `ATIVAR <token>` válido + token CONSUMED, mesmo mobile | `ConsumeMagicToken` | `CONSUMED` (mesmo número) | `CONSUMED` (no-op) | `already_active` |
| `ATIVAR <token>` válido + token CONSUMED, mobile distinto | `ConsumeMagicToken` | `CONSUMED` (outro número) | `CONSUMED` (no-op) + signal + métrica | `code_already_used_other_account` |
| `ATIVAR <token>` válido + token PENDING | `ConsumeMagicToken` | `PENDING` | `PENDING` (no-op) | `payment_still_processing_retry` |
| `ATIVAR <token>` válido + token EXPIRED | `ConsumeMagicToken` | `EXPIRED` | `EXPIRED` (no-op) | `code_expired_contact_support` |
| `ATIVAR <token>` válido + token inexistente | `ConsumeMagicToken` | (n/a) | (n/a) | `code_invalid_check_again` |
| Mensagem qualquer + match E.164 c/ token PAID outreached | `TryFallbackActivation` | `PAID` | `CONSUMED` (path=fallback_e164) | `welcome_activated` |
| Mensagem qualquer + match E.164 c/ token PAID sem outreach | `TryFallbackActivation` | `PAID` | `PAID` (no-op) | `please_use_ativar_command` |
| Mensagem qualquer + sem match | (não dispara onboarding) | (n/a) | (n/a) | (bot principal trata fora deste escopo) |
| Job outreach detecta `PAID > 2h sem outreach` | `SendOutreach` | `PAID` | `PAID` + `outreach_sent_at=now` | template `activation_reminder` |
| Job expiração detecta `expires_at < now` | `ExpireTokens` | `PENDING` ou `PAID` | `EXPIRED` (+ signal se PAID) | (sem cliente) |

### 5.3 Detecção de evento staled (consumer outbox)

O consumer recebe evento `billing.subscription.activated` que pode vir de retry da Kiwify (E2 já deduplica via `billing_processed_events`). O outbox dispatcher do platform garante 1x por `event_id` no consumer. Onboarding adicionalmente faz a transição `PENDING → PAID` via SQL com guarda `WHERE status='PENDING'` — chegar com `status` já `PAID` ou `CONSUMED` retorna 0 linhas alteradas, considerado no-op feliz (log debug).

### 5.4 Máquina de estados do token (pura, em `domain/services/transitions.go`)

```
                   PENDING   PAID    CONSUMED   EXPIRED
mark_paid           → PAID    no-op   no-op       no-op
consume_direct       fail*    → CONS  no-op/sig   fail
consume_fallback     fail*    → CONS  no-op/sig   fail
mark_outreach        fail     → PAID  fail        fail   (mantém PAID, set timestamp)
expire               → EXP    → EXP+  no-op       no-op
```

`fail*` = retorna erro tipado mapeado para mensagem específica. Para `PENDING`, o erro é `ErrTokenNotYetPaid`.

## 6. Contratos, handlers, routers e pontos de DI

### 6.1 `internal/onboarding/module.go`

```go
type OnboardingModule struct {
    PublicRouter           PublicRouter
    WhatsAppRouter         WhatsAppRouter
    SubscriptionConsumer   events.Handler
    PaidWithoutTokenConsumer events.Handler
    OutreachJob            worker.Job
    ExpirationJob          worker.Job
}

func NewOnboardingModule(
    manager uow.Manager,
    cfg OnboardingConfig,
    waCfg WhatsAppConfig,
    publisher outbox.Publisher,
    identityModule identity.IdentityModule,
    o11y observability.Provider,
) (OnboardingModule, error)
```

DI manual estrito: nenhum `init()`, nenhum singleton global, todas as dependências por construtor.

### 6.2 Rotas HTTP

| Verbo | Path | Auth | Handler | Notas |
| --- | --- | --- | --- | --- |
| POST | `/v1/onboarding/checkout` | rate-limit IP 10/min, CORS allow-list (`https://www.mecontrola.app.br`, `https://mecontrola.app.br`) | `CreateCheckoutHandler` | Body `{plan_id}` validado contra `OnboardingConfig.KiwifyCheckoutURLs` (mapa env, ADR-010) — `plan_id` deve existir como chave; sem fallback. |
| GET | `/v1/onboarding/tokens/{token}/state` | público, rate-limit IP 30/min | `TokenStateHandler` | Resposta boolean única (RF-17). Métrica server-side com motivo real. |
| GET | `/webhooks/whatsapp` | Meta verify (query `hub.verify_token` confere com config) | `WhatsAppVerifyHandler` | Retorna `hub.challenge` em texto puro. |
| POST | `/webhooks/whatsapp` | `MetaSignatureMiddleware` HMAC-SHA256 | `WhatsAppInboundHandler` | Sempre HTTP 200 após validação (Meta exige). Falhas internas viram log + métrica, não 5xx. |

Inserção em `cmd/server/server.go`: registrar `onboardingModule.PublicRouter` e `onboardingModule.WhatsAppRouter` via `httpserver.RegisterRouters`. Wiring de CORS via middleware `chi` existente (ou novo) na rota `/v1/onboarding/checkout`.

**Deployment (VPS Hostinger — ADR-009):** binário Go atrás de `nginx` (ou `caddy`) como reverse proxy + TLS terminator. Backend escuta em `127.0.0.1:8080` (localhost only). Reverse proxy injeta `X-Real-IP` e `X-Forwarded-For`. `OnboardingConfig.TrustedProxies` contém `127.0.0.1/32` para validar origem dos headers.

### 6.3 Wiring em `cmd/server/server.go`

```go
onboardingModule, err := onboarding.NewOnboardingModule(
    uowManager,
    cfg.OnboardingConfig,
    cfg.WhatsAppConfig,
    outboxPublisher,
    identityModule,
    o11y,
)
if err != nil { return fmt.Errorf("server: onboarding module: %w", err) }
httpserver.RegisterRouters(router, onboardingModule.PublicRouter, onboardingModule.WhatsAppRouter)
```

### 6.4 Wiring em `cmd/worker/worker.go`

```go
onboardingModule, err := onboarding.NewOnboardingModule(...)
if err != nil { return err }

// Consumers via events.Dispatcher (cross-module billing → onboarding)
dispatcher.Register("billing.subscription.activated",                onboardingModule.SubscriptionConsumer)
dispatcher.Register("billing.subscription.activated_without_token",  onboardingModule.PaidWithoutTokenConsumer)

// Jobs cron (UTC, schedule fixo — ADR-009)
workerManager.RegisterJob(onboardingModule.OutreachJob)                  // "5 * * * *"
workerManager.RegisterJob(onboardingModule.ExpirationJob)                // "0 3 * * *"
workerManager.RegisterJob(onboardingModule.MetaProcessedMessagesCleanup) // "30 3 * * *" — purge WAMID > 30d
```

### 6.5 `CheckoutURLBuilder` — mapeamento `plan_id` → URL Kiwify

`OnboardingConfig.KiwifyCheckoutURLs` é `map[uuid.UUID]string` carregado de env. Cada entrada é a URL pública do produto na Kiwify (gerada no painel Kiwify por produto):

```yaml
ONBOARDING_KIWIFY_CHECKOUT_URLS: |
  11111111-1111-1111-1111-111111111111=https://pay.kiwify.com.br/abc123
  22222222-2222-2222-2222-222222222222=https://pay.kiwify.com.br/def456
  33333333-3333-3333-3333-333333333333=https://pay.kiwify.com.br/ghi789
```

UUIDs correspondem a `billing.plans.id` (seeded por E2). `CheckoutURLBuilder.Build(ctx, plan_id, token)` faz lookup no mapa, valida que o host é `pay.kiwify.com.br` (defesa contra config envenenada), e apensa `?sck={token}` preservando query existente:

```go
u, err := url.Parse(cfg.KiwifyCheckoutURLs[planID])
if err != nil { return "", ErrCheckoutUnavailable }
if !cfg.AllowedCheckoutHosts.Contains(u.Host) { return "", ErrCheckoutUnavailable }
q := u.Query(); q.Set("sck", token); u.RawQuery = q.Encode()
return u.String(), nil
```

`POST /v1/onboarding/checkout` valida `plan_id` (UUID válido + chave presente no mapa); ausência retorna 400 `unknown_plan` sem criar token.

No MVP a landing envia apenas o plano Mensal; os 3 são suportados pelo backend desde o dia 1 sem mudança de contrato.

### 6.6 Contrato com a landing (Astro Pages Function)

Endpoint que a Pages Function chama no server-side:
```http
GET /v1/onboarding/tokens/{token}/state HTTP/1.1
Host: api.mecontrola.app.br
Accept: application/json
```
```http
HTTP/1.1 200 OK
Content-Type: application/json
Cache-Control: no-store

{
  "ready_to_activate": true,
  "wa_me_url": "https://wa.me/5511XXXXXXXXX?text=ATIVAR%20<token>",
  "bot_number_display": "+55 11 9XXXX-XXXX"
}
```
Quando `ready_to_activate=false`, a resposta omite `wa_me_url` e `bot_number_display`. A Pages Function renderiza **uma única mensagem genérica** independentemente do motivo interno (RF-17).

Fallback de roteamento: a Pages Function aceita `?s={token}` na query string como retaguarda (caso o redirect Kiwify não consiga embutir `{token}` no path). Implementação na landing (fora deste repo).

### 6.7 Eventos emitidos e consumidos

| Evento | Direção | Produtor | Consumidor | Payload mínimo (JSON) |
| --- | --- | --- | --- | --- |
| `billing.subscription.activated` | E2 → E3 | `SubscriptionEventPublisher` (E2 — enriquecer) | `SubscriptionPaidConsumer` (E3) | `subscription_id, funnel_token, plan_id, external_sale_id, customer_mobile_e164, customer_email, paid_at, period_end` |
| `billing.subscription.activated_without_token` | E2 → E3 | `SubscriptionEventPublisher` (E2 — novo type) | `PaidWithoutTokenConsumer` (E3) | `external_sale_id, customer_mobile_e164, customer_email, paid_at` |
| `onboarding.subscription_bound` | E3 → E1 | `OnboardingEventPublisher` (E3) | **Novo handler dedicado** `SubscriptionBoundProjector` (E1, ver §6.11 e ADR-011) | `user_id, subscription_id, funnel_token_hash_prefix, bound_at, activation_path` |

Compromisso de não vazamento: payloads transportam **e-mail em claro** apenas para uso operacional (suporte); logs do dispatcher mascaram via `MaskedEmail` ao registrar. Mobile sempre em E.164 (sem espaços/máscara).

### 6.8 Confirmação pós-ATIVAR — texto livre dentro da janela 24h (ADR-010)

Após `ConsumeMagicToken` / `TryFallbackActivation` bem-sucedidos, o use case envia mensagem de confirmação ao cliente via:

```http
POST https://graph.facebook.com/v18.0/{phone_number_id}/messages
{ "messaging_product":"whatsapp", "to":"<E.164 sem +>", "type":"text",
  "text":{"body":"<copy fixo configurável>"} }
```

Justificativa: a mensagem `ATIVAR <token>` do cliente abre a janela de 24h da Meta Cloud API, permitindo texto livre sem template. Sem custo extra de aprovação de template, sem dependência adicional para o caminho feliz.

Falha de envio é tratada como **best-effort não bloqueante**: a transação principal (UoW) **já comitou** antes do envio; falha aqui vira `slog.Warn` + métrica `onboarding_confirmation_failed_total`. Cliente está ativo no DB mesmo se a confirmação não chegar — próxima mensagem dele reabre a conversa normalmente.

Copy configurável em `WhatsAppConfig.ActivationMessages` (mapa estado → texto), permitindo ajuste sem deploy.

### 6.9 Wiring em E1 para reprojeção de entitlement após bind (ADR-011)

E1 ganha **novo handler dedicado** em `internal/identity/infrastructure/messaging/database/consumers/subscription_bound_projector.go`:

```go
func (p *SubscriptionBoundProjector) Handle(ctx context.Context, event events.Event) error {
    payload, ok := event.GetPayload().(SubscriptionBoundPayload)
    if !ok {
        return fmt.Errorf("identity.bound_projector: unexpected payload type %T", event.GetPayload())
    }
    sub, err := p.subscriptionReader.GetByID(ctx, payload.SubscriptionID)
    if err != nil {
        return fmt.Errorf("identity.bound_projector: fetch sub: %w", err)
    }
    return p.projectEntitlement(ctx, payload.UserID, sub)
}
```

`SubscriptionReader` é interface **no consumidor (E1)** com método `GetByID(ctx, sub_id) (Subscription, error)`. Implementação concreta vive em E2 (`internal/billing/application/usecases/get_subscription_read_view.go`) e é injetada no `EntitlementModule` no wiring de `cmd/worker`. Respeita R6 (interface declarada no consumidor).

Registro em `internal/identity/module.go`:
```go
{EventType: "onboarding.subscription_bound", Handler: subscriptionBoundProjector},
```

Idempotente por design: `projectEntitlement` usa `INSERT ... ON CONFLICT (user_id) DO UPDATE` em `identity.entitlements`.

### 6.10 Política de retry + degradação no consumer onboarding (ADR-011)

Quando `SubscriptionPaidConsumer` (E3) recebe `billing.subscription.activated` mas o `funnel_token` não casa com nenhuma linha em `onboarding_tokens` (lookup por `token_hash` vazio), a política é:

1. **Tentativas 1–4:** retornar `ErrTokenNotFoundTransient` — `outbox.DispatcherJob` aplica retry com backoff exponencial (mecanismo já existente em platform).
2. **A partir da 5ª tentativa** (cap `OnboardingConfig.MaxTokenLookupAttempts`, default `5`): consumer detecta cap via `attempt_count` do envelope outbox **ou** via tabela auxiliar `onboarding.consumer_lookup_attempts(event_id PK, attempts, last_attempt_at)` (criada na migration 0010 se platform não expuser o campo) e **degrada graciosamente**:
   - Insere `support_signals(kind='paid_without_token', payload={external_sale_id, customer_mobile_masked, customer_email_masked, paid_at, note:"token_lookup_exhausted"})`.
   - Métrica `onboarding_token_not_found_after_retries_total{result="degraded"}`.
   - Log warn `onboarding.consumer.token_lookup_exhausted`.
   - **Acknowledge o evento** (retorna `nil`) → outbox marca processado, sem loop infinito.

Mesma política para `PaidWithoutTokenConsumer` quando insert em `support_signals` falha por motivo transiente: retry com backoff até cap; após cap, log error + ack + métrica `onboarding_support_signal_write_failed_total`.

A primeira tarefa de implementação valida se `internal/platform/outbox` expõe `attempt_count` no envelope; caso negativo, migration 0010 inclui `consumer_lookup_attempts`. Housekeeping desta tabela entra no job `MetaProcessedMessagesCleanup` (DELETE WHERE `last_attempt_at < now() - 30d`).

### 6.11 Operação production-ready na VPS Hostinger (ADR-011)

- **Secrets:** `/etc/mecontrola/server.env` e `/etc/mecontrola/worker.env`, `chmod 0640 root:mecontrola`, carregados via `systemd EnvironmentFile=`. Rotação manual; `META_APP_SECRET_NEXT` / `KIWIFY_WEBHOOK_SECRET_NEXT` permitem zero-downtime. Evolução para cofre (Vault/Doppler/Infisical) fica para hardening pós-MVP.
- **Logs:** binários logam JSON em stdout via `slog` (LogFormat=`json`). Systemd captura em `journald`. **Grafana Alloy** roda como systemd unit, lê do `journald` (`loki.source.journal`) e envia para **Grafana Cloud Loki** (tier gratuito ~50GB/mês). Labels: `service=<server|worker>`, `env`, `module`.
- **Métricas:** `/metrics` exposto pelo server; Alloy faz scrape e remote-write para Grafana Cloud Mimir.
- **Traces:** OTLP exporter (`devkit-go/observability`) aponta para Grafana Cloud Tempo.
- **Runbook:** `docs/runbooks/deployment-vps-hostinger.md` (criado em tarefa operacional separada) cobre systemd units, EnvironmentFile, Alloy config, certbot/Let's Encrypt no proxy.

### 6.12 Schemas (esboço SQL)

**Migration 0010 — schema onboarding + tabela tokens:**

```sql
CREATE SCHEMA IF NOT EXISTS onboarding;

CREATE TABLE onboarding.onboarding_tokens (
  id                       UUID         PRIMARY KEY,
  token_hash               BYTEA        NOT NULL UNIQUE,   -- sha256 raw (32 bytes)
  status                   TEXT         NOT NULL CHECK (status IN ('PENDING','PAID','CONSUMED','EXPIRED')),
  plan_id                  UUID         NOT NULL,
  expires_at               TIMESTAMPTZ  NOT NULL,
  created_at               TIMESTAMPTZ  NOT NULL DEFAULT now(),
  paid_at                  TIMESTAMPTZ  NULL,
  consumed_at              TIMESTAMPTZ  NULL,
  outreach_sent_at         TIMESTAMPTZ  NULL,
  -- Capturados do webhook E2 (não usados no caminho feliz, só em fallback e suporte):
  customer_mobile_e164     TEXT         NULL,
  customer_email           TEXT         NULL,
  external_sale_id         TEXT         NULL,
  -- Capturados no consumo:
  consumed_by_user_id      UUID         NULL,
  consumed_by_mobile_e164  TEXT         NULL,
  activation_path          TEXT         NULL CHECK (activation_path IN ('direct','fallback_e164','outreach','admin')),
  -- Auditoria:
  metadata                 JSONB        NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_onboarding_tokens_status_expires  ON onboarding.onboarding_tokens (status, expires_at)
  WHERE status IN ('PENDING','PAID');

CREATE INDEX idx_onboarding_tokens_outreach_pick   ON onboarding.onboarding_tokens (status, paid_at)
  WHERE status='PAID' AND outreach_sent_at IS NULL;

CREATE INDEX idx_onboarding_tokens_by_mobile_paid  ON onboarding.onboarding_tokens (customer_mobile_e164)
  WHERE status='PAID' AND outreach_sent_at IS NOT NULL;
```

**Migration 0011 — support_signals:**

```sql
CREATE TABLE onboarding.support_signals (
  id            UUID         PRIMARY KEY,
  kind          TEXT         NOT NULL CHECK (kind IN ('orphan_expired_subscription','paid_without_token','token_reuse_attempt')),
  payload       JSONB        NOT NULL,
  occurred_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
  resolved_at   TIMESTAMPTZ  NULL,
  resolved_by   TEXT         NULL,
  notes         TEXT         NULL
);

CREATE INDEX idx_support_signals_kind_open ON onboarding.support_signals (kind, occurred_at)
  WHERE resolved_at IS NULL;
```

**Migration 0012 — extensão E2 (compatibilidade com payload enriquecido):**

```sql
-- Estas colunas alimentam o payload do evento publicado por E2,
-- não são consumidas diretamente por queries de billing.
ALTER TABLE billing.subscriptions
  ADD COLUMN IF NOT EXISTS customer_mobile_e164 TEXT NULL,
  ADD COLUMN IF NOT EXISTS customer_email       TEXT NULL,
  ADD COLUMN IF NOT EXISTS external_sale_id     TEXT NULL;

CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_external_sale_id
  ON billing.subscriptions (external_sale_id) WHERE external_sale_id IS NOT NULL;
```

**Migration 0013 — tabela de idempotência de mensagens Meta:**

```sql
CREATE TABLE onboarding.meta_processed_messages (
  wamid         TEXT         PRIMARY KEY,            -- WhatsApp Message ID
  processed_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_meta_processed_messages_processed_at
  ON onboarding.meta_processed_messages (processed_at);
```

Job `MetaProcessedMessagesCleanup` (cron `30 3 * * *`) deleta linhas com `processed_at < now() - interval '30 days'`.

Persistência do token: o **token em claro nunca é gravado**. A coluna `token_hash` é `BYTEA` de 32 bytes (SHA-256 raw). Comparação por reconstrução do hash na borda (use case calcula `sha256.Sum256([]byte(token))`).

## 7. Idempotência, segurança, expiração e race conditions

### 7.1 Idempotência

| Operação | Mecanismo |
| --- | --- |
| `MarkTokenPaid` (consumer) | UPDATE guardado por `WHERE status='PENDING'` + outbox dispatcher already-once. |
| `ConsumeMagicToken` mesmo número | UPDATE guardado por `WHERE status='PAID'`; se já `CONSUMED` com mesmo `consumed_by_mobile_e164` → no-op. |
| `SendOutreach` | UPDATE com `SET outreach_sent_at=now() WHERE outreach_sent_at IS NULL` antes de enviar (lock otimista). Erro 5xx Meta → revert (`UPDATE ... SET outreach_sent_at=NULL WHERE id=? AND outreach_sent_at=?`). Erro 4xx → mantém (cap de 1). |
| `ExpireTokens` | UPDATE em batch idempotente. |
| `CreateCheckoutSession` | **Não idempotente** por design do PRD (RF-01); cada chamada cria token novo. |

### 7.2 Race conditions

| Race | Cobertura |
| --- | --- |
| `ATIVAR <token>` antes do webhook E2 (Pix lento — S-07) | Token em `PENDING` → mensagem `payment_still_processing_retry`. Cliente reenvia depois. |
| Dois `ATIVAR` simultâneos do mesmo número | `SELECT ... FOR UPDATE` na linha do token serializa; segundo vê `CONSUMED` mesmo número → `already_active`. |
| `ATIVAR` no número A enquanto job de outreach faz `SendOutreach` para número B (mesma sub) | Não aplicável: outreach lê `customer_mobile_e164` capturado no webhook (do checkout). `ATIVAR` é sempre do número que envia (canal Meta). Cada operação atua em coluna distinta. |
| Fallback E.164 e `ATIVAR` chegam no mesmo segundo | `ConsumeMagicToken` e `TryFallbackActivation` competem pelo mesmo `FOR UPDATE`; segundo vê `CONSUMED` e cai em `already_active`. |
| `OutreachJob` paralelo (futuro multi-worker) | `FOR UPDATE SKIP LOCKED LIMIT 100` permite paralelismo sem duplicidade. MVP roda 1 worker — defensivo. |
| Cliente clica "Assinar" 2x em <100ms | 2 tokens PENDING criados (S-08). Rate limit absorve burst maior. Apenas o token usado vira PAID; o outro expira em 7d via job. |

### 7.3 Expiração e ciclo de vida

- TTL padrão 7d configurável via `OnboardingConfig.TokenTTL` (no MVP, sem override por chamada).
- Job de expiração roda 1x/24h às 03:00 UTC (defensivo: chegada de mensagem em token expirado é detectada via comparação `expires_at < now()` mesmo se o status ainda for `PAID`).
- Sinais de suporte em `support_signals` **não** expiram automaticamente; ficam disponíveis até `resolved_at`. Limpeza fica para E4 (housekeeping > 90d).

## 8. Segurança

### 8.1 Token magic
- 32 bytes de `crypto/rand`. Encoding `base64.RawURLEncoding` (43 chars URL-safe sem padding).
- Persistência somente como `sha256.Sum256` raw em `BYTEA`. Comparação reconstruindo hash.
- Token **nunca** é logado em claro. Logs registram `token_hash_prefix=<8 chars hex>` para diagnóstico.
- Token vazado em log do bot? Impossível: mensagens recebidas são logadas com `text` mascarado (substituição de match `ATIVAR <token>` por `ATIVAR ****`).

### 8.2 Validação de webhook Meta
- Middleware `MetaSignatureMiddleware` lê `X-Hub-Signature-256: sha256=<hex>`.
- Calcula `hmac.New(sha256.New, []byte(WhatsAppConfig.AppSecret))` sobre `raw_body` (buffer já existente — `raw_body_buffer.go`).
- Comparação `hmac.Equal` (constante).
- Header ausente ou inválido → 401 + métrica `meta_signature_invalid_total`.
- Suporte a rotação: `META_APP_SECRET` + `META_APP_SECRET_NEXT` (mesma lógica da rotação Kiwify de E2).

### 8.3 Verify token (Meta GET handshake)
- Endpoint `GET /webhooks/whatsapp` compara `hub.verify_token` (query) com `WhatsAppConfig.VerifyToken` (env).
- Match → responde `hub.challenge` em `text/plain`.
- Mismatch → 403.

### 8.4 CORS e rate limit
- `POST /v1/onboarding/checkout`: `Access-Control-Allow-Origin` apenas `https://www.mecontrola.app.br` e `https://mecontrola.app.br` (allow-list estrita, sem regex; previews `*.pages.dev` apontam para env de staging com allow-list dedicada). Métodos `POST, OPTIONS`. Headers `Content-Type`.
- `GET /v1/onboarding/tokens/{token}/state`: chamada server-side da Pages Function (Cloudflare edge IP), sem CORS necessário. Rate limit defensivo 30/min por IP.
- Rate limit baseado em `golang.org/x/time/rate.Limiter` por IP real (ADR-009): `X-Real-IP` (de proxy confiável) → primeiro IP em `X-Forwarded-For` → `RemoteAddr`. Headers só são respeitados se a request veio de IP listado em `OnboardingConfig.TrustedProxies`. `map[string]*rate.Limiter` com mutex + garbage collection a cada 5min para IPs sem hits.
- TLS terminado no reverse proxy (nginx/caddy) da VPS; backend escuta em loopback. Cert Let's Encrypt automático no proxy.

### 8.5 LGPD
- Mascaramento de PII em logs via `MaskedMobile`/`MaskedEmail` (E1 — RF-14).
- Payload de `support_signals` armazena versão **mascarada** dos contatos (`customer_mobile_masked` / `customer_email_masked`); número/email em claro persistem **apenas** em `onboarding_tokens` para uso operacional do próprio fluxo.
- Direito de exclusão: token e signals indexados por `user_id` (quando bound) permitem purga. Procedimento operacional fica fora deste PRD.

### 8.6 Defesa contra enumeração (RF-17)
- Endpoint `GET /v1/onboarding/tokens/{token}/state` retorna **mesma estrutura** para qualquer estado inválido. Sem hint de "expirado" vs "inexistente" vs "ainda processando".
- Tempo de resposta normalizado: sleep aleatório curto (0–3ms) quando `ready_to_activate=false` para reduzir oracle por timing.

## 9. Observabilidade

### 9.1 Logs (slog estruturado)

Todos os use cases emitem log com `module=onboarding`, `usecase=<nome>`, `correlation_id`, `token_hash_prefix`, e campos específicos. PII sempre mascarada via VOs.

Exemplos de eventos de log:
- `onboarding.checkout.created` info: `{plan_id, token_hash_prefix, expires_at}`
- `onboarding.token.marked_paid` info: `{token_hash_prefix, external_sale_id, paid_at}`
- `onboarding.token.consumed` info: `{token_hash_prefix, user_id, activation_path}`
- `onboarding.token.reuse_attempt` warn: `{token_hash_prefix, from_mobile_masked, consumed_by_mobile_masked}`
- `onboarding.outreach.sent` info: `{token_hash_prefix, to_mobile_masked, wamid}`
- `onboarding.outreach.failed` warn: `{token_hash_prefix, error_class, retry_planned}`
- `onboarding.token.expired_with_paid_state` warn: `{token_hash_prefix, external_sale_id}`
- `kiwify.tracking.legacy_carrier_seen` info (em E2 ajustado): `{carrier='s1'}` — telemetria para ADR-004.

### 9.2 Métricas (Prometheus)

| Métrica | Tipo | Labels | Origem |
| --- | --- | --- | --- |
| `onboarding_checkout_sessions_created_total` | Counter | `plan_id` | `CreateCheckoutSession` |
| `onboarding_checkout_rate_limited_total` | Counter | — | middleware |
| `onboarding_tokens_paid_total` | Counter | — | `MarkTokenPaid` |
| `onboarding_tokens_consumed_total` | Counter | `activation_path` | `ConsumeMagicToken` / `TryFallbackActivation` |
| `onboarding_tokens_paid_unconsumed` | Gauge | — | refresh a cada 60s via job leve (ou view materializada) |
| `onboarding_consume_latency_seconds` | Histogram | `result` | `ConsumeMagicToken` |
| `onboarding_paid_to_consumed_seconds` | Histogram | `activation_path` | computado no `ConsumeMagicToken` (now - paid_at) |
| `onboarding_outreach_sent_total` | Counter | `result` (`sent`/`failed_4xx`/`failed_5xx`) | `SendOutreach` |
| `onboarding_orphan_expired_total` | Counter | — | `ExpireTokens` (token PAID expirado) |
| `onboarding_token_reuse_attempt_total` | Counter | `reason="different_number"` | `ConsumeMagicToken` (RF-15) |
| `billing_paid_without_token_total` | Counter | `provider="kiwify"` | `PaidWithoutTokenConsumer` (RF-18) |
| `ty_page_invalid_access_total` | Counter | `reason` (`not_found`/`pending`/`expired`/`consumed`) | `GetTokenState` (RF-17) |
| `meta_signature_invalid_total` | Counter | — | `MetaSignatureMiddleware` |
| `meta_inbound_messages_total` | Counter | `kind` (`activation_cmd`/`fallback_candidate`/`unknown`) | `WhatsAppInboundHandler` |
| `meta_duplicate_messages_total` | Counter | — | inbound idempotency (WAMID) |
| `onboarding_confirmation_failed_total` | Counter | `reason` | confirmação pós-ATIVAR (ADR-010) |

### 9.3 Tracing
Spans nomeados por use case (`onboarding.create_checkout_session`, `onboarding.consume_magic_token`, etc.) com atributos: `token_hash_prefix`, `activation_path`, `result`. Propagação via `context.Context` em todas as bordas (já garantido por `internal/platform`).

### 9.4 Erros tipados (R5.10)
`internal/onboarding/application/errors.go` exporta:
```
ErrTokenNotFound          // RF-06 inexistente
ErrTokenExpired           // RF-06 expirado
ErrTokenNotYetPaid        // RF-06 ainda PENDING
ErrTokenAlreadyConsumedSame  // RF-08 mesmo número
ErrTokenAlreadyConsumedOther // RF-06 outro número (sinal RF-15)
ErrUnsupportedCountry     // RF-16
ErrCheckoutUnavailable    // builder Kiwify falha
ErrRateLimited            // middleware (já mapeado em HTTP)
```
Handlers HTTP mapeiam via switch `errors.Is`. Use cases concatenam contexto via `fmt.Errorf("onboarding: <ctx>: %w", err)`.

### 9.5 Runbook mínimo
- `onboarding_tokens_paid_unconsumed` > 1.5x baseline → checar template Meta, latência Cloud API, taxa de erro Meta.
- `onboarding_token_reuse_attempt_total` cresce sustentado → investigar fraude/fricção em landing.
- `meta_signature_invalid_total` > 0 → checar rotação `META_APP_SECRET`.
- `billing_paid_without_token_total` > 0 → checar carrier Kiwify (ADR-004) e propagação `sck`.
- Job `OutreachJob` falhando consistentemente → desligar `WhatsAppConfig.OutreachEnabled` e investigar (template pode ter sido pausado pela Meta).

## 10. Estratégia de testes

### 10.1 Unitários (alvo `go test ./internal/onboarding/...`)
- `domain/valueobjects/token.go`: geração, encoding URL-safe, hash determinístico, `String()` redacted.
- `domain/services/transitions.go`: matriz completa de transições válidas/inválidas (table-driven 16+ casos).
- `application/usecases/consume_magic_token_test.go`: 9 casos cobrindo cada linha da tabela §5.2 + concorrência (mock UoW falha após upsert user → garantir rollback).
- `application/usecases/try_fallback_activation_test.go`: gated por outreach (4 casos: sem outreach, com outreach, número diverge, token EXPIRED).
- `application/usecases/send_outreach_test.go`: select correto, erro 4xx vs 5xx, idempotência por linha.
- `application/usecases/expire_tokens_test.go`: PAID expirado emite signal; PENDING expirado não.
- `application/usecases/get_token_state_test.go`: RF-17 retorna boolean sem vazar motivo; métrica recebe motivo real.
- `infrastructure/http/server/middleware/meta_signature_test.go`: HMAC válido/ inválido/ header ausente/ rotação.
- `infrastructure/http/server/middleware/rate_limit_test.go`: hits permitidos, throttle a partir do 11º, gc de IPs antigos.
- `infrastructure/http/server/handlers/*_test.go`: contratos JSON (RF-17 não vaza motivo), CORS, verify token Meta.

Tudo com `testify` + mocks `vektra/mockery/v2` (já em go.mod). Sem mocks de tempo (`time.Now().UTC()` inline — proibido abstrair tempo, conforme memory note).

### 10.2 Integração (alvo `go test -tags=integration ./internal/onboarding/...`)
- `magic_token_repository_integration_test.go` (testcontainers Postgres): CRUD, índices, transições idempotentes, `FOR UPDATE SKIP LOCKED`.
- `support_signal_repository_integration_test.go`: insert por kind, query open signals.
- `subscription_paid_consumer_integration_test.go`: publica evento via outbox de billing → consumer reconcilia token → estado final consistente.
- `outreach_job_integration_test.go`: cron tick → select correto → mock client Meta sucesso/4xx/5xx → estado de `outreach_sent_at` esperado.

### 10.3 E2E (cobertura mínima)
- Smoke test `make test-e2e` (se existir) ou manual: `POST /v1/onboarding/checkout` → simular evento outbox `billing.subscription.activated` → `GET /v1/onboarding/tokens/{token}/state` retorna `ready=true` → POST inbound WhatsApp simulado → token CONSUMED + user criado + entitlement projetado.

### 10.4 Validação cross-module
- Test cross-module em `internal/identity/.../entitlement_projector_test.go`: registrar `onboarding.subscription_bound` no dispatcher e validar projeção.

## 11. Riscos, trade-offs e itens fora do MVP

### 11.1 Riscos técnicos
| ID | Risco | Mitigação |
| --- | --- | --- |
| RT-01 | Kiwify não propaga `sck` no webhook produto (apenas no `GET /sales`) | Fallback `s1`/`src` mantido em E2 por 30d (ADR-004). Logs telemetria carrier real visto. |
| RT-02 | HMAC-SHA256 vs SHA1 no webhook Kiwify (drift D-02) | Fora desta techspec (escopo E2). E3 não toca middleware. Issue rastreada em `support_signals` se ratio de webhooks rejeitados subir. |
| RT-03 | Template Meta `activation_reminder` não aprovado a tempo | Toggle `WhatsAppConfig.OutreachEnabled` permite subir job desligado; outreach inerte até aprovação (S-04). |
| RT-04 | Race ATIVAR antes do webhook (Pix lento) | Mensagem `payment_still_processing_retry` orienta cliente (S-07). |
| RT-05 | Cliente digita número de terceiro no checkout | Fallback E.164 gated por outreach (S-09) impede ativação não solicitada do terceiro. |
| RT-06 | Vazamento de token em log de proxy/CDN | Token só viaja em path (não em header sensível); resposta 200 da landing não loga path completo (CSP `referrer-policy: strict-origin-when-cross-origin` já configurada). |
| RT-07 | Bot recebe `ATIVAR` em formato variante (case, espaços) | Regex case-insensitive `(?i)^\s*ATIVAR\s+([A-Za-z0-9_\-]{40,45})\s*$`. |
| RT-08 | Cliente clica "Abrir WhatsApp" mas app demora; meta-refresh dispara prematuro | Delay de 800ms na meta-refresh; `<noscript>` mantém ação manual. Trade-off documentado. |

### 11.2 Trade-offs
- **Astro Pages Function vs Go template:** ganho de uniformidade com landing (mesmo design system, mesma stack do time de marketing) e CSP simétrica; trade-off é dois deploys coordenados (techspec define contrato JSON, landing pode evoluir independente).
- **Tabela `support_signals` JSONB vs tabelas tipadas por kind:** JSONB permite evolução sem migration por kind. Trade-off: tipagem fraca, exige convenção de schema por kind documentada aqui.
- **Token armazenado como SHA-256 vs bcrypt:** SHA-256 é suficiente porque a aleatoriedade já é 256 bits; bcrypt seria overkill e quebraria comparação O(1) por hash. Equivale ao padrão da indústria para tokens opacos URL-safe.
- **WhatsApp Meta direto vs Twilio/Z-API:** Meta direto evita custo recorrente, mas exige aprovação de template e provisionamento operacional. Decisão deliberada pelo PO (Q4 rodada 1).
- **Cap 1 outreach por token:** conservador para reputação Meta (S-11). Reavaliação fora do MVP.

### 11.3 Explicitamente fora do MVP (alinhado ao PRD)
- Segundo outreach.
- Canal dedicado de alerta para reuso (Slack/email).
- Antifraude estatística.
- Idempotency-Key no checkout.
- Painel admin de suporte.
- Suporte multi-país.
- Cancelamento automático de assinatura órfã.
- Reprocessamento automático de paid sem token.

### 11.4 Itens condicionados a E4
- Endpoint `GET /v1/admin/support/signals` (consulta admin com paginação/filtros).
- Reemissão de token novo via fluxo admin.
- Reanimação de subscription órfã via API.
- Anonimização programada de tokens consumidos > 90d.

## 12. ADRs criadas (todas neste diretório)

| ID | Decisão | Status |
| --- | --- | --- |
| [ADR-001](./adr-001-onboarding-schema-dedicated-and-tokens-table.md) | Schema `onboarding` dedicado + tabela `onboarding_tokens` com hash | Aceita |
| [ADR-002](./adr-002-magic-token-format-base64url-sha256.md) | Token 32B base64url + persistência apenas como SHA-256 | Aceita |
| [ADR-003](./adr-003-contract-e2-to-e3-via-outbox-enriched.md) | Contrato E2 → E3 via outbox `billing.subscription.activated` enriquecido | Aceita |
| [ADR-004](./adr-004-adopt-tracking-sck-as-magic-token-carrier.md) | Adoção de `tracking.sck` como carrier oficial; migração de `s1` em E2 | Aceita |
| [ADR-005](./adr-005-whatsapp-meta-cloud-api-direct.md) | WhatsApp Business Cloud API direta (Meta) + X-Hub-Signature-256 | Aceita |
| [ADR-006](./adr-006-support-signals-single-table.md) | Tabela única `support_signals` para RF-12/RF-15/RF-18 | Aceita |
| [ADR-007](./adr-007-thank-you-as-astro-pages-function.md) | Thank-you como Astro Pages Function na landing + endpoint state JSON RF-17 | Aceita |
| [ADR-008](./adr-008-fallback-e164-gated-by-outreach.md) | Fallback E.164 gated por outreach (divergência deliberada da discovery — S-09) | Aceita |
| [ADR-009](./adr-009-deployment-vps-hostinger-with-reverse-proxy.md) | Deployment em VPS Hostinger + nginx/caddy reverse proxy + cron fixo UTC | Aceita |
| [ADR-010](./adr-010-confirmation-via-free-text-within-24h-window.md) | Confirmação pós-ATIVAR via texto livre dentro da janela 24h Meta + `KiwifyCheckoutURLs` em env | Aceita |

## 13. Matriz requisito → decisão → teste

| RF | Decisão técnica primária | ADR | Teste de cobertura |
| --- | --- | --- | --- |
| RF-01 | Token 32B base64url + TTL 7d + carrier `sck` | ADR-002, ADR-004 | `token_test`, `create_checkout_session_test` |
| RF-02 | Rate limit IP 10/min `x/time/rate` | — | `rate_limit_test` |
| RF-03 | Consumer outbox enriquecido | ADR-003 | `subscription_paid_consumer_integration_test` |
| RF-04, RF-05 | Astro Pages Function + endpoint state JSON | ADR-007 | `token_state_handler_test` + contract test |
| RF-06 | Máquina de estados em domain/services | — | `consume_magic_token_test` (9 casos) |
| RF-07 | UoW transacional + outbox `subscription_bound` | ADR-003 | `consume_magic_token_integration_test` |
| RF-08 | Guard `consumed_by_mobile_e164` | — | `consume_magic_token_test` (idempotência) |
| RF-09 | `OutreachJob` cron + lock SKIP LOCKED | ADR-005 | `outreach_job_integration_test` |
| RF-10 | `TryFallbackActivation` gated | ADR-008 | `try_fallback_activation_test` |
| RF-11 | `TokenExpirationJob` cron diário | — | `expire_tokens_test` |
| RF-12 | Insert `support_signals` no expirar | ADR-006 | `expire_tokens_integration_test` |
| RF-13 | Métricas Prometheus listadas §9.2 | — | `metrics_test` (smoke) |
| RF-14 | VOs `MaskedMobile`/`MaskedEmail` (E1) | — | revisão code review + lint |
| RF-15 | Insert `support_signals` + métrica + log | ADR-006 | `consume_magic_token_test` (caso outro número) |
| RF-16 | `valueobjects.NewWhatsAppNumber` (E1) | — | `consume_magic_token_test` (caso país não suportado) |
| RF-17 | Endpoint boolean único + métrica server-side | ADR-007 | `get_token_state_test` |
| RF-18 | `PaidWithoutTokenConsumer` + `support_signals` | ADR-003, ADR-006 | `paid_without_token_consumer_test` |
| RF-19 | Pages Function semântica + axe-core no pipeline landing | ADR-007 | contrato com landing (testes no repo externo) |

---

## Conformidade com Padrões

Esta techspec respeita estritamente:
- **AGENTS.md** §"Layout Obrigatorio por Modulo" e §"Padrao Obrigatorio de Modulo" (DI manual, fronteiras `infrastructure → application → domain`, sem `init()`).
- **CLAUDE.md** §"Go — Regra Mandatória" — toda implementação derivada **deve** carregar `go-implementation` antes de editar; exemplos da skill carregados apenas sob demanda.
- **`.claude/rules/governance.md`** R-GOV-001 (precedência) — esta techspec é nível 3 e cede para níveis 1 e 2.
- **R0** sem `init()`. **R5.12** sem `panic` em produção. **R6** `context.Context` em toda fronteira de IO. **R7.6** `errors.Join` para agregar; `fmt.Errorf("ctx: %w", err)` para wrapping.
- **Outbox** §"Contrato": eventos publicados com `event_id` UUID, `type`, `aggregate`, `payload` JSON, `occurred_at`. Consumer idempotente por `event_id`.
- **Proibições explícitas (memory):** sem `var _ Interface = (*Type)(nil)`, sem abstração de tempo (sem Clock), sem prefixo `_` em globais.

### Riscos conhecidos
- RT-01 a RT-08 (§11.1). Mitigações documentadas. Nenhum risco material sem plano.

### Arquivos relevantes e dependentes
- **Novos:** todos sob `internal/onboarding/` (estrutura §4) + 3 migrations `migrations/0010..0013`.
- **Modificados em E2:**
  - `internal/billing/application/usecases/funnel_token.go` — adicionar `sck` (ADR-004).
  - `internal/billing/application/usecases/process_kiwify_webhook.go` — extrair `Customer.mobile`/`Customer.email`/`order_ref` e propagar para o evento.
  - `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go` — payload enriquecido + novo type `subscription.activated_without_token`.
  - `internal/billing/application/usecases/process_sale_approved.go` — não falhar com `ErrFunnelTokenMissing`; em vez disso publicar `subscription.activated_without_token` (mudança de comportamento — registrar em ADR-003).
- **Modificados em E1:**
  - `internal/identity/infrastructure/messaging/database/consumers/entitlement_projector.go` — registrar handler para `onboarding.subscription_bound`.
- **Modificados em runtime:**
  - `cmd/server/server.go` — wiring `onboarding`.
  - `cmd/worker/worker.go` — wiring consumers e jobs `onboarding`.
  - `configs/config.go` — `OnboardingConfig`, `WhatsAppConfig`.
- **Externos (PR coordenado):**
  - Repositório `LimaTeixeiraTecnologia/mecontrola-landingpage`: nova rota `/obrigado/[token]/index.astro` (Pages Function), atualização CSP `connect-src`.
  - Painel Kiwify: configurar URL de redirect pós-pagamento para `https://www.mecontrola.app.br/obrigado/{tracking_sck}`.
  - Painel Meta: criar template `activation_reminder` em `pt_BR` e configurar webhook URL `https://api.mecontrola.app.br/webhooks/whatsapp` com `verify_token`.

## Sequenciamento de Desenvolvimento

### Ordem de build
1. **Migrations** (`0010`, `0011`, `0012`, `0013`) aplicadas em dev/staging.
2. **Configs** (`OnboardingConfig`, `WhatsAppConfig`) + ENV.
3. **Domain** (`magic_token`, `support_signal`, `token` VO, `transitions`).
4. **Application interfaces** (repository factory, gateway, builder).
5. **Use cases puros** (`create_checkout_session`, `get_token_state`, `consume_magic_token`, `try_fallback_activation`, `expire_tokens`, `send_outreach`, `mark_token_paid`, `handle_paid_without_token`).
6. **Infrastructure repositories** Postgres.
7. **Cliente Meta Cloud** (`internal/onboarding/infrastructure/http/client/meta`).
8. **Middlewares** (`MetaSignatureMiddleware`, `RateLimitMiddleware`).
9. **HTTP handlers + routers** (Public + WhatsApp).
10. **Consumers outbox** (`SubscriptionPaidConsumer`, `PaidWithoutTokenConsumer`) + producer (`onboarding.subscription_bound`).
11. **Jobs** (`OutreachJob`, `TokenExpirationJob`).
12. **Ajustes E2** (carrier `sck`, payload enriquecido, novo type sem token).
13. **Ajuste E1** (registrar handler `onboarding.subscription_bound` no projector existente).
14. **Wiring final** em `cmd/server` e `cmd/worker`.
15. **Smoke E2E + checklist R0–R7**.

### Dependências técnicas
- Bloqueia: nada (E3 é folha do roadmap antes de E4).
- Bloqueado por: E1 (entregue), E2 (entregue, requer ajustes mínimos).
- Dependências externas: Meta template aprovado, número WhatsApp provisionado (S-03), painel Kiwify configurado (L-02).

## Monitoramento e Observabilidade

Operacional via `/metrics` Prometheus existente, com SLO sugerido (fora deste PRD por falta de baseline):
- `onboarding_consume_latency_seconds` p95 < 500ms.
- `onboarding_paid_to_consumed_seconds` p50 < 5min (caminho direto).
- `onboarding_outreach_sent_total{result="failed_4xx"}` ratio < 1% (saúde do template).
- `meta_signature_invalid_total` deve ser 0 em produção (qualquer não-zero → investigar rotação).

Dashboard de funil fica para tarefa operacional separada (otel-grafana-dashboards), fora desta techspec.

## Considerações Técnicas

### Decisões chave
Listadas e justificadas nas 8 ADRs. Resumo:
1. Carrier `tracking.sck` é nativo Kiwify Public API e semanticamente correto para identificador externo.
2. Módulo dedicado `internal/onboarding` evita inchar `billing`/`identity` e respeita fronteiras.
3. Schema dedicado `onboarding` no Postgres isola dados e simplifica RLS futuro.
4. Outbox enriquecido evita acoplamento síncrono entre módulos sem perda de garantia transacional (event_id idempotente).
5. Astro Pages Function alinha thank-you com design system e CSP da landing, com endpoint Go retornando boolean único (defesa contra oracle).
6. Meta Cloud direta evita risco de bloqueio Meta por uso de provedores não oficiais.
7. Fallback E.164 gated por outreach corrige risco de ativação não solicitada de terceiros (divergência deliberada da discovery).
8. Sinais de suporte em tabela única JSONB simplifica evolução e atende literalmente o PRD ("fila ou tabela consultável").

### Diretiva obrigatória para implementação Go derivada
Toda implementação Go subsequente desta techspec **deve**:
1. Carregar **obrigatoriamente** a skill `go-implementation` (`.agents/skills/go-implementation/SKILL.md`) antes de qualquer edição.
2. Carregar exemplos (`examples-domain-flow.md`, `examples-testing.md`, `examples-infrastructure.md`) **apenas sob demanda**, respeitando o teto de 4 referências simultâneas.
3. Verificar `go.mod` (`go 1.26.4`) antes de usar qualquer recurso de linguagem.
4. Partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nunca de `internal/platform/runtime`.
5. Executar Etapas 1–5 do SKILL.md na íntegra e o Checklist R0–R7 de `references/build.md` antes de fechar a tarefa.
6. Sem comentários em arquivos `.go` (exceto `// Package <name>` em `doc.go` quando exigido).
7. Sem `init()` (R0), sem `panic` em produção (R5.12), sem abstração de tempo (memory feedback), sem `var _ Interface = (*Type)(nil)` (R6.4).

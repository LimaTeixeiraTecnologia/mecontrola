# Documento de Requisitos do Produto (PRD) — Billing Pipeline

<!-- spec-version: 3 -->
<!-- epic: docs/epics/epic-02-billing-pipeline.md -->
<!-- depends_on_prd: .specs/prd-identity-foundation/prd.md -->

## Visão Geral

O módulo `internal/billing/` é o pipeline transacional que transforma webhooks de provedor de cobrança (Kiwify no MVP) em estado canônico de assinatura (`Subscription`) e em decisão de entitlement consumível pelo gate de uso da plataforma. Sem este módulo, a landing `mecontrola.app.br` captura pagamento mas o backend não conhece o estado do cliente, não bloqueia inadimplente e não tem fonte de verdade para reconciliação.

O escopo cobre **ingresso** (webhook HTTP), **processamento** (consumer de outbox aplicando máquina de estados), **decisão** (entitlement service) e **reconciliação periódica** (pull horário contra a API Kiwify). A arquitetura segue a separação inegociável "webhook é mero recebedor, processor é único mutador" estabelecida no bundle de decisão `consolidacao-core`.

Stack confirmada no codebase: `pgx/v5` + `golang-migrate` embeded + `chi` (via devkit-go) + `slog` com redacting handler + OpenTelemetry + `outbox.Publisher`/`outbox.Dispatcher` já operacionais. **Redis não está no stack**; cache de entitlement será in-memory por instância no MVP, com Postgres como fonte de verdade (ver RF-22 e Suposição S-04).

## Objetivos

- **O-1:** Capturar 100% dos webhooks da Kiwify de forma idempotente, com latência de ack inferior a 2s p99.
- **O-2:** Tornar a `Subscription` a única fonte de verdade do estado de cobrança, com máquina de estados explícita e transições auditáveis.
- **O-3:** Disponibilizar decisão de entitlement (`granted | denied | grace`) com latência inferior a 5ms p99 em cache quente.
- **O-4:** Garantir convergência eventual entre estado interno e Kiwify via reconciliação horária, detectando webhooks perdidos em até 1h.
- **O-5:** Preparar o pipeline para múltiplos provedores (Asaas, Pagar.me, Stripe) sem refator do processor — apenas adapter novo.

### Métricas-chave

- `billing_webhook_received_total{provider, event_type, outcome}` — `outcome ∈ {accepted, duplicate, rejected_signature, rejected_payload}`.
- `billing_webhook_ack_latency_seconds{provider}` — p99 < 2s.
- `billing_event_processed_total{event_type, outcome}` — `outcome ∈ {applied, ignored_stale, ignored_unknown, dlq}`.
- `billing_subscription_state_total{state}` — gauge por estado canônico.
- `billing_reconciliation_run_total{outcome}` — `outcome ∈ {clean, divergence_fixed, error}`.
- `billing_reconciliation_divergence_total{state_local, state_remote}`.
- `entitlement_check_total{decision}` — `decision ∈ {granted, denied, grace}`.
- `entitlement_cache_hit_ratio` — alvo > 0.95 em regime estável.
- `entitlement_check_latency_seconds` — p99 < 5ms cache quente, < 30ms cache frio.

## Histórias de Usuário

### Cliente final (paga via landing)

- **HU-01:** Como cliente recém-pago, quero que minha assinatura seja ativada em segundos após o pagamento aprovado, para que eu possa começar a usar o MeControla pelo WhatsApp imediatamente.
- **HU-02:** Como cliente em atraso, quero receber uma notificação clara via WhatsApp informando que minha assinatura está pendente, para que eu possa regularizar antes de perder acesso.
- **HU-03:** Como cliente cancelado, quero receber confirmação do cancelamento via WhatsApp informando a data efetiva de fim do acesso, para que eu não fique surpreso quando o bot parar de responder.
- **HU-04:** Como cliente reembolsado, quero perder o acesso ao serviço imediatamente após o reembolso, para que o histórico do meu pagamento seja consistente.

### Operador da plataforma

- **HU-05:** Como operador, quero que o sistema reaplique automaticamente um webhook perdido detectado por reconciliação, sem intervenção manual, para que divergências entre Kiwify e nosso estado se autorresolvam dentro de 1h.
- **HU-06:** Como operador, quero inspecionar o payload original de qualquer webhook em event store imutável (`webhook_events`), para que casos de suporte e disputas sejam auditáveis com evidência crua.
- **HU-07:** Como operador, quero que webhooks com payload inválido ou assinatura quebrada caiam em DLQ com classificação explícita, para que ataques, bugs de provedor e erros de configuração se distingam.

### Bot (consumidor do entitlement)

- **HU-08:** Como bot do WhatsApp processando uma mensagem, quero consultar entitlement em < 5ms para decidir se chamo o LLM ou respondo com mensagem de bloqueio, sem onerar o p95 da resposta ao usuário.

## Funcionalidades Core

### F-1: Webhook Ingress (`POST /webhooks/kiwify`)

Handler HTTP responsável por **3 e somente 3 passos**: (1) verificar assinatura/token do provedor; (2) deduplicar por `(provider, external_event_id)`; (3) persistir payload bruto em `webhook_events` e publicar `BillingProviderEventReceived` no outbox via `outbox.Publisher.Publish(ctx, tx, evt)`. Tudo dentro da mesma transação Postgres usando `database.UnitOfWork[T]`. Retorna `200 OK` em ack feliz, `204 No Content` em duplicata, `401` em assinatura inválida, `400` em payload malformado. **Nunca parseia regra de negócio, nunca decide estado.** Ack alvo < 2s p99.

### F-2: Event Store imutável (`webhook_events`)

Tabela append-only com JSONB do payload bruto. Contém: `id` (ULID), `provider`, `external_event_id`, `event_type`, `signature` (header recebido), `payload` (JSONB cru), `received_at`, `processed_at` (preenchido pelo handler). Constraint `UNIQUE (provider, external_event_id)` garante dedup atômica via `INSERT ... ON CONFLICT DO NOTHING`. Nunca apagar; referenciado por `subscriptions.last_webhook_event_id` para auditoria. Replay pós-incidente via job manual que república eventos no outbox.

### F-3: `Subscription` agregado e máquina de estados canônica

Agregado `Subscription` em `internal/billing/domain` com PK ULID, FK `user_id` para `identity.users`. Estados canônicos: `TRIALING`, `ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED`. Transições explícitas e exaustivas. `period_length` por plano (`MONTHLY=30d`, `QUARTERLY=90d`, `ANNUAL=365d`) em VO `BillingPeriod`. Implementa o contrato mínimo `identity/domain.Subscription` exigido pela função `IsEntitled` (E1). Sentinelas para transição ilegal (`ErrIllegalTransition`).

### F-4: `BillingEventProcessor` (consumidor único do outbox)

Único ponto de mutação de `Subscription`. Implementa `outbox.Handler` registrado via `outbox.Registry` para event_type `billing.kiwify.received`. Idempotente por `evt.ID()` via tabela `billing_event_applications (event_id PK, subscription_id, applied_at)`. Aplica mapeamento provider → estado canônico (delega ao adapter) e transição de estado em transação única com `UnitOfWork[T]`. Ordem-tolerante: ignora evento cujo `occurred_at` seja anterior ao último aplicado para o mesmo `subscription_id`. Erros classificados: payload malformado → `ErrPermanent` → DLQ; falha Postgres transitória → retry com backoff exponencial pelo Dispatcher.

### F-5: `BillingProvider` (porta hexagonal)

Interface em `internal/billing/application/interfaces` que define o contrato consumido pelo processor e pela reconciliação:

```
type BillingProvider interface {
    VerifySignature(payload []byte, headers map[string]string, secret string) error
    ParseEvent(payload []byte) (CanonicalEvent, error)
    FetchSubscription(ctx context.Context, externalSubscriptionID string) (CanonicalSubscription, error)
}
```

Adapter Kiwify em `internal/billing/infrastructure/providers/kiwify`. Mapeamento de eventos `compra_aprovada`, `subscription_renewed`, `subscription_late`, `subscription_canceled`, `compra_reembolsada`, `chargeback` para `CanonicalEvent` mora **no adapter**, nunca no processor.

### F-6: `EntitlementService.Check`

API: `Check(ctx, userID) (Decision, error)` em `internal/billing/application`. Decision: `{Status: granted|denied|grace, Reason, SubscriptionID, ExpiresAt}`. Reutiliza a função pura `IsEntitled(sub, now)` de `internal/identity/domain` (E1). Cache **in-memory por instância** (LRU + TTL); TTL = `min(period_end - now, 5min)`. Negative cache de 5 min para "sem subscription". Invalidação síncrona pós-commit no `BillingEventProcessor` na **mesma instância** que processou (best-effort). Outras instâncias convergem em ≤ 5 min via TTL. Sem Redis no MVP (ver S-04).

### F-7: Reconciliação horária

Cron via `outbox.cron` ou job separado roda a cada 1h, percorrendo subscriptions em `ACTIVE`/`PAST_DUE`. Para cada uma, chama `BillingProvider.FetchSubscription` e compara com estado local. Divergência **não muta direto** — publica evento sintético `billing.reconciliation.divergence_detected` no outbox para o `BillingEventProcessor` aplicar pela mesma máquina de estados. Rate limit local de 100 req/min para respeitar limites Kiwify. Batch size configurável; default 200/ciclo.

### F-8: Notificações de mudança de estado (best-effort)

Após commit no `BillingEventProcessor`, publica evento `billing.subscription.state_changed` no `events.Bus` (volátil). Subscribers em `notifications/` montam mensagem WhatsApp para 4 transições alvo: ativação (`→ACTIVE`), inadimplência (`→PAST_DUE`), cancelamento (`→CANCELED_PENDING|EXPIRED`), reembolso (`→REFUNDED`). Falha de notificação **nunca** reverte a mutação. Subscribers fora de escopo deste PRD (entregues em E3 ou módulo `notifications`).

## Requisitos Funcionais

### Webhook Ingress

- **RF-01:** O sistema DEVE expor `POST /webhooks/kiwify` recebendo `Content-Type: application/json`.
- **RF-02:** O handler DEVE validar o token do webhook por comparação constant-time (`crypto/subtle.ConstantTimeCompare`) entre o valor recebido (header `X-Kiwify-Webhook-Token` por convenção; nome configurável via `KIWIFY_WEBHOOK_TOKEN_HEADER`) e o segredo armazenado em `KiwifyConfig.WebhookSecret`. Implementação defensiva: a interface `BillingProvider.VerifySignature` é estável e suporta evolução para HMAC-SHA256 sem mudança de contrato. Falha de validação retorna `401 Unauthorized` sem expor detalhe.
- **RF-03:** O handler DEVE extrair o `external_event_id` do payload em cascata: (a) campo `id` do objeto raiz se presente e não-vazio; (b) `order.id` se presente; (c) hash SHA-256 do payload bruto canonicalizado (`payload_hash`) como fallback determinístico. O valor resolvido é gravado em `webhook_events.external_event_id` e dedup acontece via `INSERT ... ON CONFLICT DO NOTHING` sobre `UNIQUE (provider, external_event_id)`.
- **RF-04:** Em duplicata detectada, o handler DEVE retornar `204 No Content` sem publicar no outbox.
- **RF-05:** Em payload novo, o handler DEVE persistir o payload bruto integral em `webhook_events.payload` (JSONB) e publicar `BillingProviderEventReceived` via `outbox.Publisher.Publish(ctx, tx, evt)` na MESMA transação.
- **RF-06:** O handler DEVE retornar `200 OK` em ack de evento novo, dentro de 2s p99.
- **RF-07:** O handler NÃO DEVE interpretar campos de negócio do payload (estado, valores, datas). Apenas extrai assinatura, `external_event_id` e `event_type` para chaves técnicas.
- **RF-08:** O handler DEVE registrar no `webhook_events.signature` o valor do header de assinatura recebido para auditoria; e no `headers` (JSONB) os demais headers relevantes (sem `Authorization`/`Cookie`).
- **RF-09:** Em payload malformado (JSON inválido ou `external_event_id` ausente), o handler DEVE retornar `400 Bad Request` e incrementar `billing_webhook_received_total{outcome="rejected_payload"}` sem persistir.

### Persistência e schema

- **RF-10:** O sistema DEVE criar migration `0009_billing_schema` contendo as tabelas: `webhook_events`, `subscriptions`, `billing_event_applications`, `billing_plans` (read-only seed).
- **RF-11:** `webhook_events` DEVE ter constraint `UNIQUE (provider, external_event_id)` e índice em `received_at DESC`.
- **RF-12:** `subscriptions` DEVE conter: `id` (ULID PK), `user_id` (FK `identity.users` ON DELETE RESTRICT), `provider`, `external_subscription_id`, `plan_code`, `status`, `period_start`, `period_end`, `grace_period_end`, `last_webhook_event_id` (FK), `created_at`, `updated_at`, `deleted_at`. Constraint `UNIQUE (provider, external_subscription_id) WHERE deleted_at IS NULL`.
- **RF-13:** O sistema DEVE garantir, por índice parcial único, a regra "1 user = 1 subscription ativa": `UNIQUE (user_id) WHERE status IN ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING') AND deleted_at IS NULL`.
- **RF-14:** `billing_event_applications (event_id ULID PK, subscription_id ULID FK, applied_at timestamptz)` DEVE existir para dedup idempotente do processor.
- **RF-15:** `billing_plans` DEVE ser populada via migration de seed com os 3 planos fixos do MVP:
  | `plan_code` | `display_name` | `period_length_days` | `price_brl_cents` |
  |---|---|---|---|
  | `MONTHLY` | Mensal | 30 | 2990 |
  | `QUARTERLY` | Trimestral | 90 | 8073 |
  | `ANNUAL` | Anual | 365 | 29780 |

  Coluna `kiwify_product_id` (string, nullable) é preenchida operacionalmente após criação dos produtos na Kiwify (atualizável via UPDATE sem migration). Valores são definitivos e fixados no PRD.

### Domínio e máquina de estados

- **RF-16:** O agregado `Subscription` DEVE residir em `internal/billing/domain` com campos privados e construtores `NewSubscription` (criação) e `RehydrateSubscription` (carga do repositório).
- **RF-17:** O sistema DEVE implementar a máquina de estados canônica com as transições legais: `TRIALING→ACTIVE`, `TRIALING→EXPIRED`, `ACTIVE→PAST_DUE`, `ACTIVE→CANCELED_PENDING`, `ACTIVE→REFUNDED`, `PAST_DUE→ACTIVE`, `PAST_DUE→EXPIRED`, `PAST_DUE→REFUNDED`, `CANCELED_PENDING→EXPIRED`, `CANCELED_PENDING→ACTIVE` (reativação), `CANCELED_PENDING→REFUNDED`. Transição não listada DEVE retornar `ErrIllegalTransition`.
- **RF-17a:** **Política conservadora de chargeback:** qualquer evento Kiwify `chargeback` (total OU parcial, independentemente do valor reembolsado) DEVE produzir transição para `REFUNDED`. O processor NÃO inspeciona `chargeback_amount` para tomar decisão de estado; valor parcial é apenas registrado em `subscriptions.refund_amount_cents` para auditoria. Cliente perde acesso imediatamente; casos limítrofes são tratados por suporte humano fora do pipeline automatizado.
- **RF-18:** A função `IsEntitled(sub, now)` (E1, em `identity/domain`) DEVE retornar `true` para `ACTIVE` e `TRIALING` com `period_end > now`, e para `PAST_DUE` com `grace_period_end > now`; `false` para os demais estados.
- **RF-19:** `BillingPeriod` (VO) DEVE encapsular `period_length` por plano (30/90/365 dias) e expor método `Advance(from time.Time) time.Time`.
- **RF-20:** `Subscription.ApplyEvent(canonicalEvent, now)` DEVE ser a única mutação pública; retorna a transição aplicada para o caller persistir.

### Processador de eventos

- **RF-21:** O `BillingEventProcessor` DEVE ser registrado como `outbox.Handler` para event_type `billing.kiwify.received`, subscription_name `billing-event-processor`. Política de retry e DLQ herdada do `OutboxConfig` global existente (verificado em `configs/config.go:174-184`): `RetryMaxAttempts=15`, `RetryBaseBackoff=2s`, `RetryMaxBackoff=5min`, `DispatcherHandlerTimeout=10s`, `DispatcherBatchSize=50`, `DispatcherTickInterval=500ms`. Nenhum override específico para billing no MVP; ajustes futuros via env var sem mudança de código.
- **RF-22:** O processor DEVE executar idempotência por `evt.ID()` via `INSERT INTO billing_event_applications(event_id, subscription_id) ON CONFLICT (event_id) DO NOTHING`; se zero linhas afetadas, retornar `nil` (no-op idempotente).
- **RF-23:** O processor DEVE invocar `BillingProvider.ParseEvent(payload)` para obter `CanonicalEvent`; falha de parse DEVE retornar `fmt.Errorf("parse: %w", outbox.ErrPermanent)` para mover delivery a DLQ.
- **RF-24:** O processor DEVE resolver `userID` chamando `identity.UserRepository.UpsertByWhatsAppNumber` (entregue por E1) com o `WhatsAppNumber` extraído do `CanonicalEvent.Customer`.
- **RF-25:** O processor DEVE ignorar evento cujo `CanonicalEvent.OccurredAt` seja estritamente anterior ao `Subscription.LastEventAt` corrente, retornando `nil` e incrementando `billing_event_processed_total{outcome="ignored_stale"}`.
- **RF-26:** Eventos de tipo desconhecido (não mapeados em adapter) DEVEM retornar erro não-permanente para retry; após exceder `MaxAttempts` configurado, vão a DLQ via Dispatcher.
- **RF-27:** Após mutação bem-sucedida da `Subscription`, o processor DEVE publicar `billing.subscription.state_changed` em `events.Bus` (volátil, best-effort) com payload `StateChangedEvent`:
  ```
  {
    "subscription_id": "ULID",
    "user_id": "ULID",
    "whatsapp_number": "E.164 (não mascarado — handler de notificação decide masking)",
    "plan_code": "MONTHLY|QUARTERLY|ANNUAL",
    "previous_state": "TRIALING|ACTIVE|PAST_DUE|CANCELED_PENDING|EXPIRED|REFUNDED",
    "new_state": "TRIALING|ACTIVE|PAST_DUE|CANCELED_PENDING|EXPIRED|REFUNDED",
    "transition_reason": "purchase_approved|subscription_renewed|subscription_late|subscription_canceled|refund_issued|chargeback_received|reconciliation_sync",
    "period_end": "RFC3339",
    "grace_period_end": "RFC3339 ou null",
    "occurred_at": "RFC3339"
  }
  ```
  Este é o **contrato mínimo viável** para handoff ao módulo `notifications/`. Mensagens humanas (copy, idioma, templates Meta) são responsabilidade do PRD dedicado de `notifications/`. Subscribers que disparam mensagens WhatsApp são MANDATÓRIOS apenas para 4 transições alvo: `→ACTIVE` (ativação/reativação), `→PAST_DUE` (inadimplência), `→CANCELED_PENDING|→EXPIRED` (cancelamento), `→REFUNDED` (reembolso/chargeback). Outras transições publicam o evento mas notification handler pode optar por silêncio.

### Adapter Kiwify

- **RF-28:** O adapter Kiwify DEVE implementar `BillingProvider.VerifySignature` por comparação constant-time de token em header configurável (default `X-Kiwify-Webhook-Token`). A assinatura do método aceita `headers map[string]string` e `payload []byte`, permitindo evolução para HMAC-SHA256 (assinando o `payload` com `WebhookSecret` e comparando contra header `X-Kiwify-Signature`) sem mudança de contrato — apenas troca de implementação concreta.
- **RF-29:** O adapter DEVE mapear os 6 tipos de evento Kiwify do MVP para `CanonicalEvent.Type` ∈ `{purchase_approved, subscription_renewed, subscription_late, subscription_canceled, refund_issued, chargeback_received}`.
- **RF-30:** O adapter DEVE extrair o token de signup em cascata, na ordem: `tracking.src` → `tracking.utm_content` → `tracking.s1` → `tracking.s2` → `tracking.s3`. A primeira chave não-nula e não-vazia vence. Independentemente, `customer.mobile` DEVE ser normalizado via `identity.NewWhatsAppNumber` para E.164 BR. Quando nenhum token estiver presente, a identificação cai para matching exclusivamente por `WhatsAppNumber` (fluxo coberto por E3 — `signup_attempts` com janela de 24h).
- **RF-31:** O adapter DEVE implementar `FetchSubscription` chamando `GET https://public-api.kiwify.com/v1/sales/{order_id}` com bearer token OAuth, retornando `CanonicalSubscription`.
- **RF-31a:** **Contrato OAuth Kiwify (resolvido por documentação oficial):** o adapter DEVE obter token via `POST https://public-api.kiwify.com/v1/oauth/token` com body `application/x-www-form-urlencoded` contendo `client_id` e `client_secret` (lidos de `KiwifyConfig.ClientID` e `KiwifyConfig.ClientSecret`). Response retorna `access_token` (JWT Bearer), `expires_in: 86400` (24h), `scope`. **Sem refresh_token publicado** — renovação por re-autenticação no mesmo endpoint. O adapter DEVE cachear o token em memória com TTL = `expires_in - 300` (margem de 5min) e re-autenticar proativamente OU em resposta a `401 Unauthorized` (retry único após refresh). Token compartilhado entre processor e reconciliação (não há benefício em manter cópias separadas).
- **RF-31b:** O adapter DEVE respeitar rate limit global Kiwify de **100 req/min** (limite documentado oficialmente). Resposta `429 Too Many Requests` DEVE disparar backoff exponencial (1s, 2s, 4s, máx 30s) com retry; após 3 tentativas falhadas, propagar erro transitório ao chamador. Sem headers `Retry-After` ou `X-RateLimit-*` documentados — política puramente client-side.

### Entitlement e cache

- **RF-32:** O `EntitlementService.Check(ctx, userID)` DEVE consultar primeiro o cache in-memory (`map[user_id]Decision` com TTL); cache hit retorna em < 5ms p99.
- **RF-33:** Em cache miss, o serviço DEVE consultar `SubscriptionRepository.FindActiveByUserID(ctx, userID)`, aplicar `IsEntitled` e popular cache com TTL = `min(period_end - now, 5min)`.
- **RF-34:** Para usuário sem `Subscription` ativa, o serviço DEVE retornar `Decision{Status: denied, Reason: "no_active_subscription"}` e cachear como negative entry por 5 min.
- **RF-35:** O `BillingEventProcessor` DEVE invocar `EntitlementService.Invalidate(userID)` síncrono pós-commit na mesma instância. Outras instâncias DEVEM convergir em ≤ 5 min via TTL natural (ver S-04 para evolução pós-MVP).
- **RF-36:** O cache DEVE limitar memória via LRU com capacidade configurável (default 50_000 entries); evicção LRU em alta cardinalidade.

### Reconciliação

- **RF-37:** O sistema DEVE rodar job `billing.reconciliation.hourly` em intervalo configurável (default 60min).
- **RF-38:** O job DEVE iterar `subscriptions WHERE status IN ('ACTIVE','PAST_DUE') AND deleted_at IS NULL` em batches (default 200), respeitando rate limit local de 100 req/min para Kiwify.
- **RF-39:** Para cada divergência detectada (`status_local ≠ status_remote` OU `period_end_local ≠ period_end_remote`), o job DEVE publicar evento sintético `billing.reconciliation.divergence_detected` no outbox com `Subscription` remota como payload.
- **RF-40:** O job DEVE registrar métricas `billing_reconciliation_run_total` e `billing_reconciliation_divergence_total` ao final de cada execução.
- **RF-41:** O processor DEVE tratar `billing.reconciliation.divergence_detected` aplicando o estado remoto via mesma máquina de estados (sem caminho privilegiado).

### Observabilidade e segurança

- **RF-42:** Todos os logs do módulo DEVEM aplicar `mask.WhatsApp` em campos de telefone, `mask.Email` em campos de email, e omitir CPF e cartão (campos já cobertos por `redaction.PIIFields`).
- **RF-43:** Spans OpenTelemetry DEVEM ser emitidos em: webhook ingress (`billing.webhook.ingress`), processor (`billing.event.process`), entitlement check (`billing.entitlement.check`), reconciliation tick (`billing.reconciliation.tick`).
- **RF-44:** O `KiwifyConfig` (struct viper com prefix `KIWIFY_`) DEVE conter:
  - `WebhookSecret` (string, secret) — token de validação do header de webhook
  - `WebhookTokenHeader` (string, default `X-Kiwify-Webhook-Token`) — nome do header configurável
  - `ClientID` (string, secret) — credencial OAuth
  - `ClientSecret` (string, secret) — credencial OAuth
  - `APIBaseURL` (string, default `https://public-api.kiwify.com`) — base URL da REST API
  - `OAuthTokenSafetyMargin` (time.Duration, default `5m`) — margem antes de re-autenticar
  - `RateLimitMaxRequestsPerMin` (int, default `100`) — alinhado com limite global Kiwify
  - `ReconciliationInterval` (time.Duration, default `1h`)
  - `ReconciliationBatchSize` (int, default `200`)

  Método `SafeKiwifyConfig()` análogo a `SafeDSN()` DEVE redactar `WebhookSecret`, `ClientID`, `ClientSecret` em logs.
- **RF-45:** A regra `depguard` em `.golangci.yml` DEVE ser estendida para `billing-no-identity-infra` (billing pode importar `identity/domain` e `identity/application`, mas não `identity/infrastructure`) e `domain-no-infrastructure` no `internal/billing/domain`.

### Configuração e runtime

- **RF-46:** O webhook ingress DEVE ser montado no chi router via builder em `internal/platform/http/server.go` na rota `/webhooks/kiwify`, sem middleware de autenticação JWT (autenticação é por assinatura do provedor).
- **RF-47:** O subsystem `billing` DEVE ser registrado em `cmd/api` (e `cmd/worker` se separado) seguindo o padrão `Subsystem.Start/Stop` existente, populando `outbox.Registry` antes do `Dispatcher` iniciar.

### Retenção e anonimização de `webhook_events`

- **RF-48:** A tabela `webhook_events` DEVE conter coluna `anonymized_at TIMESTAMPTZ NULL` (default NULL) e índice parcial `WHERE anonymized_at IS NULL` para acelerar a varredura do job de anonimização.
- **RF-49:** O sistema DEVE executar job `billing.webhook_events.anonymize` em schedule diário (default `@daily` via `outbox.cron` ou job dedicado). O job DEVE selecionar linhas onde `received_at < NOW() - INTERVAL '365 days' AND anonymized_at IS NULL`, em batches (default 500 por execução), e DEVE substituir o campo `payload` por uma versão anonimizada onde os caminhos JSON `customer.cpf`, `customer.cnpj`, `customer.email`, `customer.mobile`, `customer.address.*`, `card.*`, `payment.*.card.*` são substituídos pelo sentinel string `"[REDACTED]"`. Demais campos do payload são preservados. O job DEVE preencher `anonymized_at = NOW()` na mesma transação por linha.
- **RF-50:** Após anonimização, **metadados são preservados indefinidamente** (`id`, `provider`, `external_event_id`, `event_type`, `signature`, `headers`, `received_at`, `processed_at`, `anonymized_at`). Sem hard delete no MVP. Coluna `payload` permanece NOT NULL (sempre contém JSONB, anonimizado ou íntegro).
- **RF-51:** O job DEVE registrar métrica `billing_webhook_events_anonymized_total` por execução e DEVE expor `billing_webhook_events_pending_anonymization` (gauge) consultando `SELECT count(*) FROM webhook_events WHERE received_at < NOW() - INTERVAL '365 days' AND anonymized_at IS NULL`.
- **RF-52:** A anonimização é **irreversível por design** — uma vez aplicada, não há mecanismo para recuperar o payload original. Disputas que exijam acesso a payload com PII DEVEM ser tratadas dentro da janela de 365 dias.

## Experiência do Usuário

Funcionalidade primariamente backend. Os pontos com superfície ao usuário final são:

- **Notificações WhatsApp** disparadas em 4 transições (HU-01 a HU-04). Mensagens, copy, opt-out e templates são responsabilidade do módulo `notifications/` e fora do escopo deste PRD (consumidor de `events.Bus`).
- **Comportamento percebido pelo cliente:** ativação detectável em < 30s após pagamento aprovado pela Kiwify (latência: ack webhook < 2s + processamento outbox por dispatcher tick padrão).

Acessibilidade e UX da landing/checkout: responsabilidade da Kiwify (página de checkout) e do front da landing — fora do escopo.

## Restrições Técnicas de Alto Nível

### Integrações externas

- **Kiwify** — única dependência externa no MVP. URL produção `https://public-api.kiwify.com.br`. Webhooks enviados em JSON. Token OAuth para API REST (reconciliação) obtido via `POST /v1/oauth/token`.
- **Postgres** — pgx/v5, pool gerenciado por `platform/database`. Migrations via `golang-migrate` embed.
- **OpenTelemetry** — emissão obrigatória; sem dependência de coletor externo no MVP (logs estruturados são suficientes para observabilidade de produção, métricas exportadas via OTLP quando o collector estiver disponível).

### Compliance e segurança

- **LGPD:** CPF, email, telefone, dados de cartão DEVEM ser mascarados em logs. Payload bruto em `webhook_events` é armazenado com PII por **365 dias** (cobre janela de chargeback Kiwify + ciclo anual); após esse período, job diário substitui PII por sentinel `"[REDACTED]"` preservando metadados (D-08, RF-48..52). Acesso restrito por política Postgres a roles específicas (configuração de role fora do MVP, mas tabela DEVE ter `REVOKE ALL ... FROM PUBLIC` aplicado na migration).
- **Webhook security:** verificação de assinatura obrigatória antes de qualquer side-effect. Falha de verificação retorna 401 sem corpo, sem expor causa.
- **Idempotência** é requisito de correção, não de performance. Replay seguro é obrigatório.

### Performance

- Ack do webhook: p99 < 2s (alvo conservador; processor é assíncrono).
- Entitlement check: p99 < 5ms em cache quente, < 30ms em cache frio.
- Reconciliação: ≤ 1h de janela máxima de inconsistência.
- Escala alvo do MVP: 5.000 assinaturas ativas. Reconciliação respeita rate limit 100 req/min — cobre 6.000 sub/h. Revisão em > 5.000 subs.

### Privacidade de dados

- `webhook_events` é tabela de PII por design (payload bruto). Considerada zona quente; rotação ou anonimização pós-90d é tópico de E4.
- Cache in-memory NÃO persiste em disco; reinício do processo limpa estado.

### Não-negociáveis tecnológicos

- Pipeline obrigatoriamente passa por `outbox.Publisher` para mutações de estado. Sem atalho "webhook → repository direto".
- `BillingEventProcessor` é o único mutador. Reconciliação publica evento, nunca atualiza direto.
- Estado canônico interno é independente de provider; mapeamento mora no adapter.

## Fora de Escopo

- Implementação de adapters Asaas, Pagar.me, Stripe — interface preparada mas sem código.
- Sweep diário full de reconciliação (últimos 90 dias) — vai para E4.
- Dashboard MRR, churn, LTV — E4.
- Override administrativo de entitlement (`entitlement_overrides`) — pós-MVP.
- Painel administrativo web para reembolso manual — pós-MVP.
- Trial gratuito (não prometido na landing).
- Rate limit por usuário no gate LLM — escopo de E4 ou PRD próprio.
- Hard delete de `webhook_events` (apenas anonimização aos 365d; metadados preservados indefinidamente — ver D-08, RF-48..52).
- Cache distribuído (Redis) — pós-MVP se métricas justificarem; ver S-04.
- Notificações WhatsApp (implementação de templates, opt-out, retries) — módulo `notifications/`, este PRD só publica o evento de mudança de estado.
- Reativação pós-`EXPIRED` — fluxo de re-onboarding é E3.

## Critérios de Sucesso (testáveis)

- **CA-01:** Webhook ingress retorna `200` em < 2s p99 com payload Kiwify real (medido em ambiente de staging com carga sintética de 10 req/s sustentadas por 10 min).
- **CA-02:** Mesmo `external_event_id` enviado 5x ao webhook produz **uma** entrada em `webhook_events`, **uma** publicação no outbox, **uma** linha em `billing_event_applications` e mesmo estado final em `subscriptions`. Verificado por teste de integração com testcontainers-go/postgres.
- **CA-03:** Sequência `subscription_renewed` → `compra_aprovada` (fora de ordem) processa sem regressão — estado final é o esperado para o evento de maior `occurred_at`. Verificado por teste de integração.
- **CA-04:** Cobertura de testes ≥ 100% nos arquivos `subscription.go`, `state_machine.go`, `entitlement_service.go`, `kiwify_adapter.go`; ≥ 90% no `billing_event_processor.go`. Medido por `go test -cover` no Taskfile.
- **CA-05:** `EntitlementService.Check` serve decisão em < 5ms p99 com cache quente; verificado por benchmark `go test -bench` com 100k iterações.
- **CA-06:** Mock da API Kiwify simulando divergência (`status=ACTIVE` local, `status=CANCELED` remoto) é detectado pelo job de reconciliação e converge para `CANCELED_PENDING` em < 5 min após o tick. Verificado em integração.
- **CA-07:** Smoke E2E com Postgres real + mock Kiwify cobrindo: `compra_aprovada` → `Subscription ACTIVE` → `Check` retorna `granted` → `subscription_canceled` → estado final `CANCELED_PENDING` → `Check` retorna `granted` até `period_end` → após `period_end`, `Check` retorna `denied`.
- **CA-08:** `go test ./internal/billing/... -run TestLog` confirma que nenhum log contém número de WhatsApp em texto claro, email completo, CPF ou dado de cartão (verificação por grep no output capturado em teste).
- **CA-09:** `golangci-lint run ./...` passa com regras `depguard` estendidas; `internal/billing/domain` sem import de I/O, `internal/billing/infrastructure` é único permitido a importar `pgx`, `net/http`.
- **CA-10:** `ai-spec check-spec-drift .specs/prd-billing-pipeline/tasks.md` retorna verde após techspec + tasks materializados.
- **CA-11:** Em ambiente de staging, simulação de 1.000 webhooks Kiwify (mock) durante 1h resulta em zero entradas em DLQ e ≥ 99.9% de eventos processados em < 30s desde o ack.
- **CA-12:** Teste de integração com testcontainers cria `webhook_events` com `received_at = NOW() - INTERVAL '366 days'`, executa job de anonimização e verifica: (a) `payload.customer.cpf|email|mobile` retornam `"[REDACTED]"`; (b) `payload.tracking.src` e `payload.product.id` permanecem íntegros; (c) `anonymized_at` é preenchido; (d) re-execução do job não-op (idempotência).

## Suposições e Questões em Aberto

### Decisões fechadas (registradas para histórico)

As seguintes questões foram **resolvidas explicitamente** e não bloqueiam techspec/execução:

- **D-01 (ex-S-01/S-02/S-03 — gate empírico Kiwify):** PRD adota **implementação defensiva** em vez de validação prévia em sandbox. (a) Verificação de assinatura por comparação constant-time de token em header configurável (RF-02, RF-28), interface estável para evolução a HMAC sem mudança de contrato; (b) extração de `external_event_id` em cascata com hash SHA-256 do payload como fallback determinístico (RF-03); (c) extração de token de signup em cascata por 5 campos `tracking.*` com fallback final por `WhatsAppNumber` (RF-30). **Sem compra-teste obrigatória antes da execução.** Se sandbox/produção revelar contrato diferente do assumido, ajuste é localizado ao adapter Kiwify sem mudança de RF.
- **D-02 (ex-S-04 — cache de entitlement):** Cache in-memory por instância com TTL=5min e Postgres como fonte de verdade, **sem Redis no MVP**. Critério explícito de revisão pós-MVP: introduzir cache distribuído se `entitlement_check_latency_seconds_p99 > 5ms` sustentado OU número de pods > 4 OU reclamações de cliente com janela "pago mas bloqueado" > 0.5% do total de ativações.
- **D-03 (ex-S-05 — planos):** Valores e nomes definitivos (`MONTHLY R$ 29,90 / 30d`, `QUARTERLY R$ 80,73 / 90d`, `ANNUAL R$ 297,80 / 365d`). Fixados em RF-15 e em migration de seed `0009_billing_plans_seed`. `kiwify_product_id` preenchido operacionalmente por UPDATE pós-criação dos produtos na Kiwify.
- **D-04 (ex-Q-03 — chargeback parcial):** Política conservadora — qualquer chargeback (total ou parcial) move para `REFUNDED`. Valor parcial armazenado em `refund_amount_cents` apenas para auditoria. Sem ramo de decisão no processor (RF-17a).
- **D-05 (ex-S-01 — contrato OAuth Kiwify):** Resolvido via documentação oficial. `POST https://public-api.kiwify.com/v1/oauth/token`, body `application/x-www-form-urlencoded` com `client_id` + `client_secret`. Response `access_token` (JWT Bearer), `expires_in: 86400` (24h), sem `refresh_token`. Renovação por re-auth no mesmo endpoint. Sem rate limit publicado para `/oauth/token`. Estratégia: cache em memória com TTL = `expires_in − 5min`, re-auth proativa, retry único em `401`. (RF-31a)
- **D-06 (ex-S-02 — retry/DLQ do processor):** PRD reusa defaults globais de `OutboxConfig` (verificados em `configs/config.go:174-184`): `RetryMaxAttempts=15`, `RetryBaseBackoff=2s`, `RetryMaxBackoff=5min`, `DispatcherHandlerTimeout=10s`, `DispatcherBatchSize=50`, `DispatcherTickInterval=500ms`. Sem override específico para billing no MVP — ajustes via env var sem mudança de código. Erro `outbox.ErrPermanent` (payload malformado, evento desconhecido após esgotar tentativas) move a delivery a DLQ. (RF-21)
- **D-07 (ex-Q-01 — contrato de notificação WhatsApp):** PRD entrega o **contrato técnico mínimo viável** do evento `billing.subscription.state_changed` (RF-27) — payload completo, lista de 4 transições MANDATÓRIAS para notificação. Copy, idioma, templates Meta, opt-out e UX da mensagem são responsabilidade do PRD dedicado de `notifications/`, que consome este contrato. Sem dependência circular: billing publica, notifications consome.
- **D-08 (ex-Q-02 — retenção de `webhook_events`):** Política two-tier. **Primeiros 365 dias:** payload integral preservado (cobre janela de chargeback Kiwify + 1 ciclo anual + disputas). **Após 365 dias:** job diário substitui `payload` por versão anonimizada (CPF, email, telefone, cartão, endereço removidos; substituídos por sentinel `"[REDACTED]"`) e marca `anonymized_at`. Metadados (`id`, `provider`, `external_event_id`, `event_type`, `received_at`, `occurred_at`, `anonymized_at`) preservados indefinidamente para auditoria. Sem hard delete no MVP. (RF-49, RF-50)

**Todas as questões originais foram resolvidas (D-01..D-08). Nenhuma suposição aberta remanesce.** Detalhes abaixo.

## Dependências entre PRDs e Pré-requisitos

### Dependência hard de E1 (`prd-identity-foundation`)

Este PRD **assume** os seguintes artefatos de E1 disponíveis em runtime:

- Agregado `User` em `internal/identity/domain` com PK ULID/UUID.
- VO `WhatsAppNumber` com construtor `NewWhatsAppNumber(string) (WhatsAppNumber, error)`.
- Port `UserRepository.UpsertByWhatsAppNumber(ctx, WhatsAppNumber) (User, error)`.
- Função pura `IsEntitled(sub Subscription, now time.Time) bool`.
- Contrato mínimo `identity/domain.Subscription` (interface com `Status() string`, `PeriodEnd() time.Time`, `GracePeriodEnd() time.Time`).
- Pacote `internal/platform/observability/mask` com `WhatsApp(string) string` e `Email(string) string` (já existente, confirmado em codebase).

### Pré-requisitos não-técnicos (gate antes da execução)

- **Produto criado na Kiwify** para cada plano com `kiwify_product_id` capturado (operacionalmente atualizável via UPDATE em `billing_plans`, sem migration).
- **Token OAuth + Webhook Secret** provisionados em vault para `dev`, `staging`, `prod` e expostos via env vars `KIWIFY_API_TOKEN` e `KIWIFY_WEBHOOK_SECRET`.
- **URL pública dos webhooks** definida por ambiente e cadastrada no painel Kiwify apontando para `/webhooks/kiwify`.

### Bloqueia execução de tarefas (não bloqueia escrita de techspec/tasks)

E1 DEVE estar com `status: implemented` antes de `ai-spec execute-all-tasks` rodar neste PRD. Techspec e tasks deste PRD podem ser materializados em paralelo a E1.

## Riscos residuais

- **R-01 (alto):** O contrato real do webhook Kiwify (nome dos campos, mecanismo de assinatura) pode divergir do assumido pela implementação defensiva. Mitigação: (a) interface `BillingProvider` estável isola troca de implementação; (b) extração de `external_event_id` em cascata com fallback por hash do payload garante dedup mesmo sem campo canônico; (c) header de token configurável por env permite trocar sem deploy. Detecção em produção: métrica `billing_webhook_received_total{outcome="rejected_signature|rejected_payload"}` com alerta em taxa > 1%.
- **R-02 (médio):** Cache in-memory sem invalidação cross-pod pode causar janela "pago mas bloqueado" perceptível em multi-pod. Mitigação: TTL agressivo (5 min) limita o pior caso; métrica `entitlement_cache_hit_ratio` por pod identifica padrão de stale; critério de revisão pós-MVP documentado em D-02.
- **R-03 (médio):** Webhook fora de ordem ou duplicado — mitigado por idempotência (RF-22), dedup (RF-03) e verificação de `occurred_at` (RF-25).
- **R-04 (médio):** Rate limit Kiwify estoura em reconciliação com > 6.000 subs. Mitigação: batch + rate limit local (RF-38); revisão de capacidade ao atingir 5.000 subs ativas.
- **R-05 (baixo):** Mapeamento provider → canônico fica desatualizado se Kiwify introduzir novo `event_type`. Mitigação: eventos desconhecidos vão a retry e depois DLQ (RF-26), métrica `billing_event_processed_total{outcome="dlq"}` alerta.
- **R-06 (baixo):** Hotmart deixado fora do MVP — interface `BillingProvider` permite plugar depois sem refator do processor.
- **R-07 (baixo):** Chargeback parcial trata cliente legítimo em disputa antifraude como reembolsado integral (perda de acesso imediata). Mitigação: política conservadora documentada em D-04; runbook de suporte cobre reativação manual via comando admin (fora do MVP).

## Referências

- Épico: `docs/epics/epic-02-billing-pipeline.md`.
- Bundle de decisão: `.agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md` (blocos **A. Layout**, **C. Billing**, **D. Entitlement**, **G. Plataforma**).
- Discovery técnica: `docs/discoveries/discovery-billing-hotmart-kiwify.md`.
- PRD bloqueador: `.specs/prd-identity-foundation/prd.md`.
- Contratos de plataforma reutilizados:
  - `internal/platform/outbox/publisher.go:20-27` — `Publisher.Publish`.
  - `internal/platform/outbox/handler.go:5-23` — contrato `Handler` e classificação de erros.
  - `internal/platform/outbox/registry.go:20-36` — `Registry.Register` para subscriptions.
  - `internal/platform/database/uow.go:19-56` — `UnitOfWork[T]` com 5s timeout.
  - `internal/platform/observability/mask/` — mascaramento de PII (WhatsApp, Email).
  - `internal/platform/events/bus.go` — barramento volátil para notificações best-effort.
  - `internal/platform/http/server.go:30-89` — montagem do chi server.
  - `migrations/embed.go` — padrão de migrations golang-migrate.
  - `configs/config.go:1-62` — padrão viper para `KiwifyConfig`.
- Documentação Kiwify:
  - [Parâmetros de rastreamento no checkout](https://ajuda.kiwify.com.br/pt-br/article/como-passar-parametros-de-rastreamento-na-url-do-checkout-src-utm-tags-entre-outros-1spiptc/)
  - [API Consultar venda — schema `tracking`](https://docs.kiwify.com.br/api-reference/sales/single.md)
  - [Webhooks Kiwify — visão geral](https://ajuda.kiwify.com.br/pt-br/article/como-funcionam-os-webhooks-2ydtgl/)
  - [API Webhooks](https://docs.kiwify.com.br/api-reference/webhooks/single.md)
  - [API Auth OAuth](https://docs.kiwify.com.br/api-reference/auth/oauth.md)
- Governança: `AGENTS.md` (seção "Outbox vs events.Bus"), `CLAUDE.md`, `.claude/rules/governance.md`.

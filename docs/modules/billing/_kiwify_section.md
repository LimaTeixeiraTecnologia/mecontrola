## Integração com Kiwify

### Client HTTP (KiwifyClient)

O client é implementado em `internal/billing/infrastructure/http/client/kiwify/` e satisfaz a interface `interfaces.KiwifyClient`. A struct `Config` agrupa todos os parâmetros de inicialização; valores ausentes ou `<= 0` recebem defaults internos.

#### Autenticação OAuth 2.0

Implementada em `auth.go` pelo tipo interno `tokenProvider`. O grant usado é `client_credentials` via `POST /v1/oauth/token`. O token é mantido em memória com proteção por `sync.Mutex`. A validade efetiva é calculada como:

```
expiresAt = now + expiresIn(s) − safetyMargin
```

A `safetyMargin` é configurável via `KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN`. Se o valor configurado for `<= 0`, aplica o fallback interno de `600 s`. Cada chamada a `token()` verifica `cachedToken != ""` e `now < expiresAt`; se ambas as condições forem verdadeiras, retorna o valor em cache sem realizar I/O. A requisição de token **não usa retry** (`httpclient.WithoutRetry()`).

#### Rate Limiting

Rate limiter do tipo token bucket implementado com `golang.org/x/time/rate`. A taxa é calculada como `rps = maxRequestsPerMin / 60.0`. A chamada `limiter.Wait(ctx)` bloqueia até que um token esteja disponível ou o contexto seja cancelado.

| Parâmetro | Variável de ambiente | Padrão |
|---|---|---|
| Requisições por minuto | `KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN` | 100 |
| Burst máximo | `KIWIFY_RATE_LIMIT_BURST` | 10 |

Quando o contexto expira aguardando token, o erro é encapsulado em `ErrKiwifyRateLimited`. A API respondendo `HTTP 429` também produz `ErrKiwifyRateLimited`.

#### Retry e Resiliência

Requisições de API utilizam `devkithttp.IdempotentRetryPolicy`.

| Código HTTP | Erro sentinel |
|---|---|
| 429 | `ErrKiwifyRateLimited` |
| 5xx | `ErrKiwifyServer` |
| 4xx | `ErrKiwifyBadRequest` |
| 2xx | sem erro |

| Parâmetro | Variável de ambiente | Padrão |
|---|---|---|
| Tentativas máximas | `KIWIFY_HTTP_RETRY_MAX_ATTEMPTS` | 3 |
| Backoff entre tentativas | `KIWIFY_HTTP_RETRY_BACKOFF` | 1 s |
| Timeout HTTP | `KIWIFY_HTTP_TIMEOUT` | 10 s |

#### Métodos da Interface

| Método | Parâmetros | Retorno | Paginação |
|---|---|---|---|
| `ListSalesUpdatedSince` | `ctx`, `windowStart`, `windowEnd`, `page int` | `(KiwifySalePage, error)` | Cursor numérico via `page_number`; campo `HasMore` indica próxima página |
| `GetSale` | `ctx`, `saleID string` | `(KiwifySale, error)` | Não paginado |

Ambos os métodos enviam os headers `Authorization: Bearer <token>` e `x-kiwify-account-id: <accountID>` em cada requisição. O campo `FunnelToken` de `KiwifySale` é preenchido por `mapSale` com prioridade `sck > s1 > src`.

---

### Classificação de Eventos (Payload)

O pacote `kiwifypayload` (`internal/billing/application/usecases/kiwifypayload/`) é a camada de adaptação entre o corpo bruto do webhook Kiwify e os DTOs de entrada dos use cases de billing.

#### Estrutura do Payload Kiwify

| Campo Go | Tag JSON | Tipo Go | Semântica |
|---|---|---|---|
| `OrderID` | `order_id` | `string` | ID canônico da venda; usado como `EnvelopeID` principal |
| `OrderRef` | `order_ref` | `string` | Referência legível da ordem |
| `OrderStatus` | `order_status` | `string` | Status da ordem no momento do evento |
| `WebhookEventType` | `webhook_event_type` | `string` | Tipo de trigger enviado pela Kiwify; fonte primária de classificação |
| `SubscriptionID` | `subscription_id` | `string` | ID da assinatura Kiwify |
| `AbandonedID` | `id` | `string` | ID de carrinho abandonado |
| `AbandonedStatus` | `status` | `string` | Status do abandono; valor `"abandoned"` ativa `TriggerAbandonedCart` |
| `Product` | `Product` | `product` | Sub-objeto: `product_id`, `product_name` |
| `Customer` | `Customer` | `customer` | Sub-objeto: `email`, `mobile`, `CPF` |
| `Subscription` | `Subscription` | `*subscription` | Ponteiro opcional: `start_date`, `next_payment`, `status` |
| `TrackingParameters` | `TrackingParameters` | `tracking` | Sub-objeto: `sck`, `s1`, `src` |
| `RefundedAt` | `refunded_at` | `*Time` | Ponteiro; ausente em eventos que não são reembolso |
| `ApprovedDate` | `approved_date` | `Time` | Data de aprovação da venda |
| `UpdatedAt` | `updated_at` | `Time` | Data de última atualização |
| `CreatedAt` | `created_at` | `Time` | Data de criação do evento |

O método `EnvelopeID()` retorna `OrderID` se não-vazio; caso contrário, retorna `AbandonedID`.

#### Tabela de Triggers

| Trigger Kiwify (`WebhookEventType`) | Constante Go | Use Case Destino | Observação |
|---|---|---|---|
| `order_approved` | `TriggerOrderApproved` | `ProcessSaleApproved` | Inclui `FunnelToken` |
| `subscription_renewed` | `TriggerSubscriptionRenewed` | `ProcessSubscriptionRenewed` | — |
| `subscription_late` | `TriggerSubscriptionLate` | `ProcessSubscriptionLate` | — |
| `subscription_canceled` | `TriggerSubscriptionCanceled` | `ProcessSubscriptionCanceled` | — |
| `order_refunded` | `TriggerOrderRefunded` | `ProcessRefundOrChargeback` | Compartilha use case com `chargeback` |
| `chargeback` | `TriggerChargeback` | `ProcessRefundOrChargeback` | Compartilha use case com `order_refunded` |
| `billet_created` | `TriggerBilletCreated` | `noopTrigger` | Sem ação de domínio |
| `pix_created` | `TriggerPixCreated` | `noopTrigger` | Sem ação de domínio |
| `order_rejected` | `TriggerOrderRejected` | `noopTrigger` | Sem ação de domínio |
| `abandoned_cart` | `TriggerAbandonedCart` | `noopTrigger` | Inferido por `Classify` quando `WebhookEventType` é vazio |
| _(ausente no mapa)_ | `TriggerUnknown` (`""`) | `ErrUnknownTrigger` | — |

A função `Classify(p Payload) Trigger` aplica a seguinte lógica em ordem:

1. Se `WebhookEventType != ""` → retorna `Trigger(WebhookEventType)` diretamente.
2. Se `AbandonedID != ""` ou `AbandonedStatus == "abandoned"` → retorna `TriggerAbandonedCart`.
3. Caso contrário → retorna `TriggerUnknown`.

#### Mapeamento de Timestamps por Trigger

| Trigger | Campos em ordem de prioridade |
|---|---|
| `order_approved` | `ApprovedDate` → `UpdatedAt` → `Subscription.StartDate` |
| `subscription_renewed` | `UpdatedAt` → `Subscription.NextPayment` |
| `subscription_late` | `UpdatedAt` → `Subscription.NextPayment` |
| `subscription_canceled` | `UpdatedAt` → `Subscription.StartDate` |
| `order_refunded` / `chargeback` | `RefundedAt` (se não nulo e não zero) → `UpdatedAt` |

#### Parsing de Timestamps

O tipo `Time` encapsula `time.Time` com `UnmarshalJSON` customizado. Os formatos são tentados na seguinte ordem:

| Ordem | Layout | Fuso aplicado |
|---|---|---|
| 1 | `time.RFC3339Nano` | Preservado; convertido para UTC |
| 2 | `time.RFC3339` | Preservado; convertido para UTC |
| 3 | `"2006-01-02 15:04:05"` (sem fuso) | BRT (UTC-3); convertido para UTC |
| 4 | `"2006-01-02 15:04"` (sem fuso, sem segundos) | BRT (UTC-3); convertido para UTC |

Strings vazias ou `"null"` resultam em zero value sem erro.

#### Rastreamento de Funil

| Prioridade | Campo JSON | Carrier retornado | Semântica |
|---|---|---|---|
| 1 | `sck` | `"sck"` | Token de campanha principal (Kiwify SCK) |
| 2 | `s1` | `"s1"` | Parâmetro de rastreamento legado S1 |
| 3 | `src` | `"src"` | Parâmetro de origem Src |
| — | _(nenhum preenchido)_ | `"none"` | Venda sem rastreamento de funil |

Os carriers `"s1"` e `"src"` ativam log `Info` com chave `kiwify.tracking.legacy_carrier_seen`. A métrica `billing_kiwify_tracking_carrier_total{carrier}` permite observar a distribuição ao longo do tempo.

---

### Variáveis de Ambiente

| Variável | Propósito | Obrigatória |
|---|---|---|
| `KIWIFY_CLIENT_ID` | `client_id` OAuth2 | Sim |
| `KIWIFY_CLIENT_SECRET` | `client_secret` OAuth2 | Sim |
| `KIWIFY_ACCOUNT_ID` | Header `x-kiwify-account-id` em toda requisição | Sim |
| `KIWIFY_PRODUCT_ID_MONTHLY` | ID do produto Kiwify para plano mensal | Sim |
| `KIWIFY_PRODUCT_ID_QUARTERLY` | ID do produto Kiwify para plano trimestral | Sim |
| `KIWIFY_PRODUCT_ID_ANNUAL` | ID do produto Kiwify para plano anual | Sim |
| `KIWIFY_WEBHOOK_SECRET` | Segredo HMAC para validação de assinatura de webhooks | Sim |
| `KIWIFY_API_BASE_URL` | Base URL da API REST Kiwify | Não (padrão: `https://public-api.kiwify.com`) |
| `KIWIFY_WEBHOOK_SECRET_NEXT` | Segredo rotacional aceito durante rotação de chave | Não |
| `KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN` | Margem de segurança para renovação antecipada do token OAuth | Não (padrão: 600 s) |
| `KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN` | Requisições por minuto ao client REST | Não (padrão: 100) |
| `KIWIFY_RATE_LIMIT_BURST` | Burst do token bucket do client REST | Não (padrão: 10) |
| `KIWIFY_WEBHOOK_RATE_LIMIT_PER_MIN` | Requisições por minuto aceitas no endpoint de webhook | Não (padrão: 60) |
| `KIWIFY_WEBHOOK_RATE_LIMIT_BURST` | Burst do rate limiter do endpoint de webhook | Não (padrão: 30) |
| `KIWIFY_HTTP_TIMEOUT` | Timeout das requisições HTTP ao client REST | Não (padrão: 10 s) |
| `KIWIFY_HTTP_RETRY_MAX_ATTEMPTS` | Número máximo de tentativas de retry | Não (padrão: 3) |
| `KIWIFY_HTTP_RETRY_BACKOFF` | Intervalo de backoff entre tentativas | Não (padrão: 1 s) |
| `KIWIFY_RECONCILIATION_INTERVAL` | Cron expression do job de reconciliação | Não (padrão: `@hourly`) |
| `BILLING_KIWIFY_EVENTS_RETENTION_DAYS` | Retenção dos eventos brutos de webhook | Não (padrão: 90 dias) |
| `BILLING_KIWIFY_EVENTS_HOUSEKEEPING_SCHEDULE` | Cron do job de limpeza de eventos expirados | Não (padrão: `@daily`) |
| `BILLING_KIWIFY_EVENTS_HOUSEKEEPING_BATCH` | Tamanho do batch do housekeeping de eventos | Não (padrão: 500) |

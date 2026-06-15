# Plano: Postman Collection Completa — mecontrola API

## Context

O projeto `mecontrola` expõe 42 endpoints REST distribuídos em 9 módulos + 3 integrações webhook (Kiwify, WhatsApp/Meta, Telegram). Todos os endpoints, bodies, status codes e comportamentos de erro foram verificados lendo os arquivos de handler, DTO e router diretamente — sem inferências. O objetivo é gerar dois artefatos Postman prontos para importar: a collection (v2.1) e o environment file, cobrindo todos os 63 cenários com Pre-request Scripts de assinatura automática para os webhooks.

---

## Arquivos a criar

| Arquivo | Destino |
|---|---|
| `mecontrola-api.postman_collection.json` | `docs/postman/` |
| `mecontrola-api.postman_environment.json` | `docs/postman/` |
| `2026-06-15-postman-collection-mecontrola-api.md` | `docs/runbooks/` |

---

## Variáveis de ambiente

| Variável | Valor placeholder | Obrigatória |
|---|---|---|
| `base_url` | `http://localhost:8080` | Sim |
| `user_id` | `<seu-user-id-uuid>` | Sim (maioria dos endpoints) |
| `gateway_auth` | `<hmac-sha256-hex>` | Sim (endpoints protegidos) |
| `gateway_timestamp` | `<unix-timestamp>` | Sim (endpoints protegidos) |
| `card_id` | `` | Populada por POST /cards |
| `transaction_id` | `` | Populada por POST /transactions |
| `card_purchase_id` | `` | Populada por POST /card-purchases |
| `recurring_template_id` | `` | Populada por POST /recurring-templates |
| `category_id` | `` | Populada por GET /categories |
| `expense_external_id` | `` | Definida manualmente (UUID) antes de POST /budgets/expenses |
| `competence` | `2026-06` | Sim (budgets) |
| `ref_month` | `2026-06` | Sim (transactions/card invoices) |
| `kiwify_webhook_secret` | `<hmac-sha1-secret>` | Sim (webhooks Kiwify) |
| `meta_app_secret` | `<hmac-sha256-secret>` | Sim (webhooks WhatsApp) |
| `meta_verify_token` | `<verify-token>` | Sim (verificação WhatsApp) |
| `telegram_secret_token` | `<telegram-secret>` | Sim (webhooks Telegram) |
| `whatsapp_phone_number_id` | `102847398923746` | Sim (payload WhatsApp) |

---

## Estrutura de pastas (63 requests)

```
mecontrola API
├── 01 — Health
│   └── GET /health
│
├── 02 — Identity
│   └── POST /api/v1/identity/users
│
├── 03 — Categories
│   ├── GET /api/v1/categories                       (salva category_id)
│   ├── GET /api/v1/categories/:id
│   ├── GET /api/v1/category-dictionary
│   └── GET /api/v1/category-dictionary/search
│
├── 04 — Onboarding
│   ├── POST /api/v1/onboarding/checkout
│   └── GET  /api/v1/onboarding/tokens/:token/state
│
├── 05 — Cards
│   ├── POST   /api/v1/cards                         (salva card_id)
│   ├── GET    /api/v1/cards
│   ├── GET    /api/v1/cards/:id
│   ├── PUT    /api/v1/cards/:id
│   ├── DELETE /api/v1/cards/:id                     (body: {version})
│   └── GET    /api/v1/cards/:id/invoices            (query: ?for=YYYY-MM-DD)
│
├── 06 — Transactions
│   ├── POST   /api/v1/transactions                  (salva transaction_id)
│   ├── GET    /api/v1/transactions
│   ├── GET    /api/v1/transactions/:id
│   ├── PATCH  /api/v1/transactions/:id
│   └── DELETE /api/v1/transactions/:id              (body: {version})
│
├── 07 — Card Purchases
│   ├── POST   /api/v1/card-purchases                (salva card_purchase_id)
│   ├── GET    /api/v1/card-purchases
│   ├── GET    /api/v1/card-purchases/:id
│   ├── PATCH  /api/v1/card-purchases/:id
│   └── DELETE /api/v1/card-purchases/:id            (body: {version})
│
├── 08 — Card Invoice
│   └── GET /api/v1/cards/:card_id/invoices/:ref_month
│
├── 09 — Recurring Templates
│   ├── POST   /api/v1/recurring-templates           (salva recurring_template_id)
│   ├── GET    /api/v1/recurring-templates
│   ├── GET    /api/v1/recurring-templates/:id
│   ├── PATCH  /api/v1/recurring-templates/:id
│   └── DELETE /api/v1/recurring-templates/:id       (body: {version})
│
├── 10 — Monthly (Transactions)
│   ├── GET /api/v1/months/:ref_month
│   └── GET /api/v1/months/:ref_month/entries
│
├── 11 — Budgets
│   ├── POST   /api/v1/budgets                       → 201
│   ├── POST   /api/v1/budgets/recurrence            → 207
│   ├── GET    /api/v1/budgets/alerts
│   ├── POST   /api/v1/budgets/expenses              → 201 (body: {external_transaction_id, ...})
│   ├── PATCH  /api/v1/budgets/expenses/:id          → 200 (body: {expected_version, ...})
│   ├── DELETE /api/v1/budgets/expenses/:id          → 204 (body: {expected_version})
│   ├── POST   /api/v1/budgets/:competence/activate  → 200
│   ├── GET    /api/v1/budgets/:competence/summary   → 200
│   └── DELETE /api/v1/budgets/:competence           → 204
│
├── 12 — Kiwify Webhook (10 eventos)
│   ├── order_approved          → 202
│   ├── subscription_renewed    → 202
│   ├── subscription_late       → 202
│   ├── subscription_canceled   → 202
│   ├── order_refunded          → 202
│   ├── chargeback              → 202
│   ├── billet_created          → 202 (no-op)
│   ├── pix_created             → 202 (no-op)
│   ├── order_rejected          → 202 (no-op)
│   └── abandoned_cart          → 202 (no-op, sem webhook_event_type)
│
├── 13 — WhatsApp Webhook (5 cenários)
│   ├── GET  /api/v1/whatsapp/verify               → 200 (query: hub.mode/verify_token/challenge)
│   ├── POST /api/v1/whatsapp/inbound — texto simples          → 200
│   ├── POST /api/v1/whatsapp/inbound — comando ATIVAR <token> → 200
│   ├── POST /api/v1/whatsapp/inbound — imagem (non-text)      → 200
│   └── POST /api/v1/whatsapp/inbound — assinatura inválida    → 401
│
└── 14 — Telegram Webhook (6 cenários)
    ├── POST — texto simples                        → 200
    ├── POST — comando /start                       → 200
    ├── POST — gasto em linguagem natural           → 200
    ├── POST — mensagem de bot (rejeição interna)   → 200
    ├── POST — chat de grupo (rejeição interna)     → 200
    └── POST — token inválido                       → 401
```

---

## Responsabilidades verificadas por endpoint

### 01 — Health
**GET /health**
Verifica se o servidor está up. Retorna 200. Nenhuma auth.

---

### 02 — Identity
**POST /api/v1/identity/users** — `upsert_user_by_whatsapp_handler.go`
- Body: `{ "whatsapp": string, "email"?: string, "display_name"?: string }`
- Chama use case `UpsertUserByWhatsApp`
- **200 OK** → `{ id, whatsapp, email, display_name, status, created_at, updated_at }`
- 400: `invalid_payload` | `invalid_whatsapp` | `invalid_email`
- 409: `whatsapp_in_use` | `email_in_use`
- Sem auth, sem idempotência.

---

### 03 — Categories
**GET /api/v1/categories** — `list_categories_handler.go`
- Query: `kind` (income|expense), `parent_id` (UUID), `include_deprecated` (bool)
- Header: `If-None-Match: <etag>` → 304 se igual
- **200 OK** + `ETag` header → `{ categories[], version }`
- 422: `invalid_kind` | `invalid_query`
- Auth: RequireUser. Métricas: `categories_list_total`, `categories_list_duration_seconds`.

**GET /api/v1/categories/{id}** — `get_category_handler.go`
- Query: `include_deprecated` (bool)
- Header: `If-None-Match: <etag>` → 304 se igual
- **200 OK** + `ETag` → categoria com subcategorias
- 404: `not_found` | 422: `invalid_query`

**GET /api/v1/category-dictionary** — `list_dictionary_handler.go`
- Query: `category_id`, `kind`, `signal_type`, `cursor`, `page_size`
- Header: `If-None-Match` → 304
- **200 OK** → `{ entries[], next_cursor, version }`

**GET /api/v1/category-dictionary/search** — `search_dictionary_handler.go`
- Query: `q` (min 3 chars), `kind` (obrigatório)
- Header: `If-None-Match` → 304
- **200 OK** → `{ result: "candidates"|"no_match", candidates[], version }`
- 422: `invalid_kind` | `invalid_query`

---

### 04 — Onboarding
**POST /api/v1/onboarding/checkout** — `create_checkout_handler.go`
- Body: `{ "plan_id": "monthly"|"quarterly"|"annual" }`
- Sem auth. Rate limited. CORS habilitado.
- **201 Created** → `{ checkout_url }`
- 400: `unknown_plan` | 503: `checkout_unavailable`

**GET /api/v1/onboarding/tokens/{token}/state** — `token_state_handler.go`
- Path: token (string encriptado)
- Sem auth. Rate limited. `Cache-Control: no-store` na resposta.
- **200 OK** → `{ ready_to_activate: bool, wa_me_url?: string, bot_number_display?: string }`
- Sempre retorna 200 (falhas internas retornam `ready_to_activate: false` com jitter 0–3ms).

---

### 05 — Cards
**POST /api/v1/cards** — `create.go`
Headers: `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp`, `Idempotency-Key`, `Content-Type: application/json`
```json
{ "name": "Nubank", "nickname": "Nu pessoal", "closing_day": 3, "due_day": 10 }
```
- **201 Created** + `Location: /api/v1/cards/{id}` → card object
- 400: `invalid_payload` | `invalid_card_name` | `invalid_nickname` | `invalid_closing_day` | `invalid_due_day` | `nickname_in_use`
- Test script: `pm.collectionVariables.set("card_id", pm.response.json().id);`

**GET /api/v1/cards** — `list.go`
- Query: `cursor`, `limit` (default 20, max 100)
- **200 OK** → `{ items: Card[], next_cursor }`
- 400: `invalid_limit`

**GET /api/v1/cards/{id}** — `get.go`
- **200 OK** → Card | 400: `invalid_card_id` | 404: `card_not_found`

**PUT /api/v1/cards/{id}** — `update.go`
Headers: `Idempotency-Key`
```json
{ "name": "Nubank Updated", "nickname": "Nu", "closing_day": 5, "due_day": 12 }
```
- Todos os campos opcionais; ao menos um obrigatório
- **200 OK** → Card | 400: `invalid_card_id` | `empty_payload` | 404: `card_not_found` | 409: `nickname_in_use`

**DELETE /api/v1/cards/{id}** — `delete.go`
Headers: `Idempotency-Key`. **Sem body.**
- **204 No Content** | 400: `invalid_card_id` | 404: `card_not_found`

**GET /api/v1/cards/{id}/invoices** — `invoice_for.go`
- Query: `for=YYYY-MM-DD` (**obrigatório** — data de compra, não ref_month)
- **200 OK** → `{ closing_date, due_date }`
- 400: `missing_for_param` | `invalid_purchase_date` | `invalid_card_id` | 404: `card_not_found`

---

### 06 — Transactions
**POST /api/v1/transactions** — `create_transaction_handler.go`
Headers: `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp`, `Idempotency-Key`
```json
{
  "direction": "expense",
  "payment_method": "pix",
  "amount_cents": 5000,
  "description": "Almoço",
  "category_id": "{{category_id}}",
  "occurred_at": "2026-06-15"
}
```
- `direction`: `"income"` | `"expense"`
- `payment_method`: `"cash"` | `"card"` | `"pix"` | outros
- **201 Created** → Transaction object

**GET /api/v1/transactions** — `list_transactions_handler.go`
- Query: `ref_month` (YYYY-MM), `cursor`, `limit` (default 50)
- **200 OK** → `{ items: Transaction[], next_cursor }`

**GET /api/v1/transactions/{id}** — `get_transaction_handler.go`
- **200 OK** → Transaction | 400: `validation_error` | 404: `not_found`

**PATCH /api/v1/transactions/{id}** — `update_transaction_handler.go`
Headers: `Idempotency-Key`
```json
{
  "direction": "expense",
  "payment_method": "pix",
  "amount_cents": 6000,
  "description": "Almoço atualizado",
  "category_id": "{{category_id}}",
  "occurred_at": "2026-06-15",
  "version": 1
}
```
- **200 OK** → Transaction atualizada

**DELETE /api/v1/transactions/{id}** — `delete_transaction_handler.go`
Headers: `Idempotency-Key`. Body **obrigatório**:
```json
{ "version": 1 }
```
- **204 No Content** | 400: `validation_error` | 404: `not_found` | 409: conflito de versão

**mapError (transactions):**
- 401: unauthorized | 404: not_found | 409: conflict | 502: `card_lookup_failed` | 400: validation | 500: internal

---

### 07 — Card Purchases
**POST /api/v1/card-purchases** — `create_card_purchase_handler.go`
Headers: `Idempotency-Key`
```json
{
  "card_id": "{{card_id}}",
  "total_amount_cents": 120000,
  "installments_total": 3,
  "description": "TV 55 polegadas",
  "category_id": "{{category_id}}",
  "purchased_at": "2026-06-15"
}
```
- **201 Created** → CardPurchase object (inclui `items[]` e `ref_months_affected[]`)

**GET /api/v1/card-purchases** — `list_card_purchases_handler.go`
- Query: `card_id` (UUID, opcional), `ref_month`, `cursor`, `limit` (default 50)
- **200 OK** → `{ items: CardPurchase[], next_cursor }`

**GET /api/v1/card-purchases/{id}** — `get_card_purchase_handler.go`
- **200 OK** → CardPurchase | 400: `validation_error` | 404: `not_found`

**PATCH /api/v1/card-purchases/{id}** — `update_card_purchase_handler.go`
Headers: `Idempotency-Key`
```json
{
  "total_amount_cents": 120000,
  "installments_total": 3,
  "description": "TV 55\" Samsung",
  "category_id": "{{category_id}}",
  "purchased_at": "2026-06-15",
  "version": 1
}
```
- **200 OK** → CardPurchase atualizada

**DELETE /api/v1/card-purchases/{id}** — `delete_card_purchase_handler.go`
Headers: `Idempotency-Key`. Body **obrigatório**:
```json
{ "version": 1 }
```
- **204 No Content**

---

### 08 — Card Invoice (Transactions module)
**GET /api/v1/cards/{card_id}/invoices/{ref_month}** — `get_card_invoice_handler.go`
- Path: `card_id` (UUID), `ref_month` (string, ex: `2026-06`)
- **200 OK** → CardInvoice object

---

### 09 — Recurring Templates
**POST /api/v1/recurring-templates** — `create_recurring_template_handler.go`
Headers: `Idempotency-Key`
```json
{
  "direction": "expense",
  "payment_method": "pix",
  "amount_cents": 9900,
  "description": "Netflix",
  "category_id": "{{category_id}}",
  "frequency": "monthly",
  "day_of_month": 15,
  "installments_total": 0,
  "started_at": "2026-01-01"
}
```
- `card_id` opcional (para compras parceladas recorrentes)
- `ended_at` opcional
- **201 Created** → RecurringTemplate object

**GET /api/v1/recurring-templates** — `list_recurring_templates_handler.go`
- Query: `active=true` (filtra apenas ativos), `cursor`, `limit` (default 50)
- **200 OK** → `{ items: RecurringTemplate[], next_cursor }`

**GET /api/v1/recurring-templates/{id}** — `get_recurring_template_handler.go`
- **200 OK** → RecurringTemplate | 400 | 404

**PATCH /api/v1/recurring-templates/{id}** — `update_recurring_template_handler.go`
Headers: `Idempotency-Key`
```json
{
  "direction": "expense",
  "payment_method": "pix",
  "amount_cents": 10900,
  "description": "Netflix premium",
  "category_id": "{{category_id}}",
  "frequency": "monthly",
  "day_of_month": 15,
  "installments_total": 0,
  "started_at": "2026-01-01",
  "version": 1
}
```
- **200 OK** → RecurringTemplate atualizado

**DELETE /api/v1/recurring-templates/{id}** — `delete_recurring_template_handler.go`
Headers: `Idempotency-Key`. Body **obrigatório**:
```json
{ "version": 1 }
```
- **204 No Content**

---

### 10 — Monthly (Transactions)
**GET /api/v1/months/{ref_month}** — `get_monthly_summary_handler.go`
- Path: `ref_month` (string, ex: `2026-06`)
- **200 OK** → `{ user_id, ref_month, income_cents, outcome_cents, total_cents, updated_at }`

**GET /api/v1/months/{ref_month}/entries** — `list_monthly_entries_handler.go`
- Path: `ref_month`. Query: `cursor`, `limit` (default 50)
- **200 OK** → `{ items: MonthlyEntry[], next_cursor }`
- MonthlyEntry: `{ kind: "transaction"|"card_invoice_item", id, user_id, ref_month, amount_cents, direction, description, created_at }`

---

### 11 — Budgets (router: `/api/v1/budgets`)

**POST /api/v1/budgets** — `create_budget_handler.go`
```json
{
  "competence": "{{competence}}",
  "total_cents": 500000,
  "allocations": [
    { "root_slug": "alimentacao", "basis_points": 3000 },
    { "root_slug": "transporte",  "basis_points": 2000 }
  ]
}
```
- **201 Created** + `Location` → BudgetOutput | 409: `budget_conflict`

**POST /api/v1/budgets/recurrence** — `create_recurrence_handler.go`
```json
{ "source_competence": "{{competence}}", "months": 3 }
```
- **207 Multi-Status** → `{ created_budgets: int, competences: string[] }`
- 422: `recurrence_invalid`

**GET /api/v1/budgets/alerts** — `list_alerts_handler.go`
- Query: `cursor`, `limit`, `competence` (YYYY-MM), `root_slug`, `threshold` (int 0–100)
- **200 OK** → `{ items: Alert[], next_cursor }`

**POST /api/v1/budgets/expenses** — `upsert_expense_handler.go#HandleCreate`
```json
{
  "external_transaction_id": "{{expense_external_id}}",
  "subcategory_id": "{{category_id}}",
  "competence": "{{competence}}",
  "amount_cents": 4500,
  "occurred_at": "2026-06-15T12:00:00Z"
}
```
- `external_transaction_id`: UUID fornecido pelo cliente (idempotência por identidade externa)
- `occurred_at`: opcional (`*time.Time`)
- `expected_version`: **não enviado** no create (internamente nil)
- **201 Created** → ExpenseOutput | 409: `expense_version_conflict` | `expense_tombstone_conflict`

**PATCH /api/v1/budgets/expenses/{id}** — `upsert_expense_handler.go#HandleUpdate`
- Path: `id` = `external_transaction_id`
```json
{
  "subcategory_id": "{{category_id}}",
  "competence": "{{competence}}",
  "amount_cents": 5000,
  "occurred_at": "2026-06-15T12:00:00Z",
  "expected_version": 1
}
```
- `expected_version` **obrigatório**; ausência → 400 `version_required`
- **200 OK** → ExpenseOutput

**DELETE /api/v1/budgets/expenses/{id}** — `delete_expense_handler.go`
- Path: `id` = `external_transaction_id`. Body **obrigatório**:
```json
{ "expected_version": 1 }
```
- **204 No Content** | 404: `expense_not_found` | 409: `expense_version_conflict`

**POST /api/v1/budgets/{competence}/activate** — `activate_budget_handler.go`
- Path: `competence`. Sem body.
- **200 OK** → BudgetOutput | 404: `budget_not_found` | 409: `budget_already_active` | 422: `activation_invalid`

**GET /api/v1/budgets/{competence}/summary** — `get_monthly_summary_handler.go`
- **200 OK** → `{ competence, total_cents, total_spent_cents, state, allocations[] }`
- 404: `budget_not_found`

**DELETE /api/v1/budgets/{competence}** — `delete_budget_handler.go`
- Só permite deletar orçamentos em estado **draft**
- **204 No Content** | 404: `budget_not_found` | 409: `budget_active_conflict`

---

### 12 — Kiwify Webhook

**Endpoint:** `POST /api/v1/billing/webhooks/kiwify`
**Assinatura:** HMAC-SHA1 do raw body, hex sem prefixo.
**Header:** `X-Kiwify-Signature: <hex>` OU query `?signature=<hex>`
**Content-Type:** `application/json`

**Pre-request Script (todos os 10 requests desta pasta):**
```js
const secret = pm.environment.get("kiwify_webhook_secret");
const body   = pm.request.body.raw;
const sig    = CryptoJS.HmacSHA1(body, secret).toString(CryptoJS.enc.Hex);
pm.request.headers.add({ key: "X-Kiwify-Signature", value: sig });
```

**Status de retorno:**
- 202: sucesso (inclui event já processado / supersedido)
- 401: `invalid_signature`
- 422: `invalid_json` | `funnel_token_missing` | `unknown_trigger`
- 500: erro interno no dispatch

**Payloads verificados no código** (`kiwifypayload/payload.go`):

```json
// order_approved
{
  "order_id": "order-approved-001", "order_ref": "ref-001",
  "order_status": "paid", "webhook_event_type": "order_approved",
  "subscription_id": "sub-001",
  "Product": { "product_id": "prod-monthly-001", "product_name": "MeControla Mensal" },
  "Customer": { "email": "user@example.com", "mobile": "+5511999999999", "CPF": "00000000000" },
  "Subscription": { "status": "active", "start_date": "2026-06-15T10:00:00.000Z", "next_payment": "2026-07-15T10:00:00.000Z" },
  "TrackingParameters": { "sck": "funnel-token-abc", "s1": null, "src": null },
  "approved_date": "2026-06-15 10:00:00", "updated_at": "2026-06-15 10:00:00", "created_at": "2026-06-15 10:00:00"
}

// subscription_renewed
{ ..., "webhook_event_type": "subscription_renewed",
  "Subscription": { "status": "active", "start_date": "2026-06-15T10:00:00.000Z", "next_payment": "2026-08-15T10:00:00.000Z" },
  "approved_date": "2026-07-15 10:00:00", ... }

// subscription_late
{ ..., "webhook_event_type": "subscription_late", "order_status": "waiting_payment",
  "Subscription": { "status": "late", ... }, "updated_at": "2026-07-18 10:00:00", ... }

// subscription_canceled
{ ..., "webhook_event_type": "subscription_canceled",
  "Subscription": { "status": "canceled", ... }, "updated_at": "2026-07-01 10:00:00", ... }

// order_refunded
{ ..., "webhook_event_type": "order_refunded", "order_status": "refunded",
  "Subscription": { "status": "canceled", ... },
  "refunded_at": "2026-06-20 10:00:00", "updated_at": "2026-06-20 10:00:00", ... }

// chargeback
{ ..., "webhook_event_type": "chargeback", "order_status": "chargedback",
  "Subscription": { "status": "canceled", ... }, ... }

// billet_created (no-op — expects 202)
{ ..., "webhook_event_type": "billet_created", "order_status": "waiting_payment",
  "subscription_id": "", ... }

// pix_created (no-op — expects 202)
{ ..., "webhook_event_type": "pix_created", "order_status": "waiting_payment", ... }

// order_rejected (no-op — expects 202)
{ ..., "webhook_event_type": "order_rejected", "order_status": "refused", ... }

// abandoned_cart (no-op, sem webhook_event_type)
{
  "id": "c6euk9v1lfj9jqxhfs", "status": "abandoned", "checkout_link": "IDhfYNV",
  "country": "br", "cpf": "30574187242",
  "created_at": "2026-06-15T15:44:25.411Z",
  "email": "johndoe@example.com", "name": "John Doe",
  "offer_name": null, "phone": "(11) 99999-9999",
  "product_id": "prod-monthly-001", "product_name": "MeControla Mensal",
  "store_id": "Q33AnzwYbfkFFwS", "subscription_plan": "Mensal"
}
```

---

### 13 — WhatsApp Webhook

**Paths verificados em `whatsapp_router.go`:**
- `GET  /api/v1/whatsapp/verify`
- `POST /api/v1/whatsapp/inbound`

**Assinatura (inbound):** `X-Hub-Signature-256: sha256=<hmac-sha256-hex>` do raw body.

**Pre-request Script (requests inbound com assinatura válida):**
```js
const secret = pm.environment.get("meta_app_secret");
const body   = pm.request.body.raw;
const sig    = "sha256=" + CryptoJS.HmacSHA256(body, secret).toString(CryptoJS.enc.Hex);
pm.request.headers.add({ key: "X-Hub-Signature-256", value: sig });
```

**13.1 — GET /api/v1/whatsapp/verify**
- Query: `hub.mode=subscribe`, `hub.verify_token={{meta_verify_token}}`, `hub.challenge=meu-challenge-12345`
- **200 OK** → body = `meu-challenge-12345` (text/plain)
- 403: modo != subscribe ou token incorreto

**13.2 — POST inbound — texto simples**
```json
{
  "object": "whatsapp_business_account",
  "entry": [{ "id": "entry-1", "changes": [{ "field": "messages", "value": {
    "messaging_product": "whatsapp",
    "metadata": { "display_phone_number": "+5511988776655", "phone_number_id": "{{whatsapp_phone_number_id}}" },
    "messages": [{ "from": "5511999999999", "id": "wamid-text-001", "timestamp": "1718400000",
      "type": "text", "text": { "body": "Olá, quero saber meu saldo" } }]
  }}]}]
}
```
- **200 OK** (sem body)

**13.3 — POST inbound — comando ATIVAR**
```json
{
  "object": "whatsapp_business_account",
  "entry": [{ "id": "entry-2", "changes": [{ "field": "messages", "value": {
    "messaging_product": "whatsapp",
    "metadata": { "display_phone_number": "+5511988776655", "phone_number_id": "{{whatsapp_phone_number_id}}" },
    "messages": [{ "from": "5511999999999", "id": "wamid-ativar-001", "timestamp": "1718400001",
      "type": "text", "text": { "body": "ATIVAR abc123token" } }]
  }}]}]
}
```

**13.4 — POST inbound — imagem (tipo não-texto)**
```json
{
  "object": "whatsapp_business_account",
  "entry": [{ "id": "entry-3", "changes": [{ "field": "messages", "value": {
    "messaging_product": "whatsapp",
    "metadata": { "display_phone_number": "+5511988776655", "phone_number_id": "{{whatsapp_phone_number_id}}" },
    "messages": [{ "from": "5511999999999", "id": "wamid-image-001", "timestamp": "1718400002",
      "type": "image" }]
  }}]}]
}
```
- **200 OK** (não-texto é silenciosamente ignorado)

**13.5 — POST inbound — assinatura inválida**
- Mesmo body de 13.2, **sem** Pre-request Script. Header: `X-Hub-Signature-256: sha256=invalido`
- **401 Unauthorized**

---

### 14 — Telegram Webhook

**Path:** `{{base_url}}/api/v1/channels/telegram/webhook` (configurável via `TELEGRAM_WEBHOOK_PATH`)
**Header:** `X-Telegram-Bot-Api-Secret-Token: {{telegram_secret_token}}`

Regras de rejeição interna verificadas em `payload/parser.go`:
- `bot_sender` (`from.is_bot == true`) → 200 silencioso
- `non_private_chat` (`chat.type != "private"`) → 200 silencioso
- `missing_text` (texto vazio) → 200 silencioso
- Token inválido → 401

**14.1 — Texto simples**
```json
{ "update_id": 100001, "message": {
    "message_id": 1,
    "from": { "id": 987654321, "is_bot": false, "language_code": "pt-BR" },
    "chat": { "id": 987654321, "type": "private" },
    "date": 1718400000, "text": "Olá, quero ver minhas finanças"
}}
```
- **200 OK**

**14.2 — Comando /start**
```json
{ "update_id": 100002, "message": {
    "message_id": 2,
    "from": { "id": 987654321, "is_bot": false, "language_code": "pt-BR" },
    "chat": { "id": 987654321, "type": "private" },
    "date": 1718400010, "text": "/start"
}}
```

**14.3 — Gasto em linguagem natural**
```json
{ "update_id": 100003, "message": {
    "message_id": 3,
    "from": { "id": 987654321, "is_bot": false, "language_code": "pt-BR" },
    "chat": { "id": 987654321, "type": "private" },
    "date": 1718400020, "text": "Gastei 50 reais no almoço hoje"
}}
```

**14.4 — Mensagem de bot (rejeição interna)**
```json
{ "update_id": 100004, "message": {
    "message_id": 4,
    "from": { "id": 111111111, "is_bot": true, "language_code": "en" },
    "chat": { "id": 987654321, "type": "private" },
    "date": 1718400030, "text": "Sou um bot"
}}
```
- **200 OK** (rejeitado silenciosamente pelo parser)

**14.5 — Chat de grupo (rejeição interna)**
```json
{ "update_id": 100005, "message": {
    "message_id": 5,
    "from": { "id": 987654321, "is_bot": false, "language_code": "pt-BR" },
    "chat": { "id": -100123456789, "type": "group" },
    "date": 1718400040, "text": "Mensagem no grupo"
}}
```
- **200 OK** (rejeitado silenciosamente)

**14.6 — Token inválido**
- Mesmo body de 14.1. Header: `X-Telegram-Bot-Api-Secret-Token: token-invalido-xpto`
- **401 Unauthorized**

---

## Testes Postman por tipo de request

```js
// Criações (POST → 201)
pm.test("Status 201", () => pm.response.to.have.status(201));
pm.test("Resposta tem id", () => pm.expect(pm.response.json()).to.have.property("id"));
pm.collectionVariables.set("<resource>_id", pm.response.json().id);

// Leituras (GET → 200)
pm.test("Status 200", () => pm.response.to.have.status(200));

// Atualizações (PUT/PATCH → 200)
pm.test("Status 200", () => pm.response.to.have.status(200));

// Deleções (DELETE → 204)
pm.test("Status 204", () => pm.response.to.have.status(204));

// Recorrência (POST → 207)
pm.test("Status 207", () => pm.response.to.have.status(207));

// Webhooks — cenários válidos
pm.test("Status 2xx", () => pm.expect(pm.response.code).to.be.oneOf([200, 202]));

// Cenários de erro esperado
pm.test("Status 401", () => pm.response.to.have.status(401));
```

---

## Implementação passo a passo

1. Criar `docs/postman/` e `docs/runbooks/` se não existirem.
2. Gerar `mecontrola-api.postman_collection.json` (Postman Collection v2.1) com as 14 pastas e 63 requests conforme toda a especificação verificada acima.
3. Incluir Pre-request Scripts de HMAC nos requests de webhook Kiwify (SHA-1) e WhatsApp inbound (SHA-256).
4. Gerar `mecontrola-api.postman_environment.json` com todas as variáveis da tabela de variáveis.
5. Salvar réplica deste plano em `docs/runbooks/2026-06-15-postman-collection-mecontrola-api.md`.
6. Não incluir valores reais de secrets — usar placeholders descritivos.

---

## Verificação

- Importar os dois arquivos no Postman Desktop.
- Preencher as variáveis de environment marcadas como "Sim".
- Executar Collection Runner na ordem das pastas 01 → 14.
- Cada POST de criação popula automaticamente a variável correspondente via test script.
- Para webhooks: os Pre-request Scripts calculam a assinatura automaticamente a partir do `kiwify_webhook_secret` e `meta_app_secret`.
- Os cenários 13.5 e 14.6 devem retornar 401; todos os outros cenários de webhook devem retornar 200 ou 202.

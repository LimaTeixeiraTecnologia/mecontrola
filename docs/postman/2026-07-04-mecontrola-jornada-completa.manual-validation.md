# Validação manual — MeControla Jornada Completa + Todos os Endpoints

- Data-base do artefato: 2026-07-04
- Base URL default: `http://localhost:8080`
- Objetivo: cobrir 100% das rotas montadas (52) e simular a jornada real encadeada
  `landing/checkout -> pagamento Kiwify -> ativação/onboarding WhatsApp -> API autenticada`,
  para dois perfis (Jailton e Stefany), sem compra real.

## Arquivos

- Collection: `docs/postman/2026-07-04-mecontrola-jornada-completa.postman_collection.json`
- Environment: `docs/postman/2026-07-04-mecontrola-jornada-completa.postman_environment.json`

## Segredos obrigatórios (preencher no environment antes de executar)

| Variável no Postman | Origem no `.env` | Uso |
| --- | --- | --- |
| `kiwify_webhook_secret` | `KIWIFY_WEBHOOK_SECRET` | Assinatura HMAC-SHA1 do webhook Kiwify |
| `meta_app_secret` | `META_APP_SECRET` | Assinatura HMAC-SHA256 do inbound/status WhatsApp |
| `meta_verify_token` | `META_VERIFY_TOKEN` | Handshake GET de verificação Meta |
| `gateway_secret` | `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` | Assinatura gateway (hex-decoded como chave) |
| `whatsapp_phone_number_id` | `META_PHONE_NUMBER_ID` | `metadata.phone_number_id` do payload Meta |

Não versione valores reais de secrets no environment.

## Variáveis encadeadas automaticamente (não preencher)

Por perfil (`_u1` = Jailton, `_u2` = Stefany): `onboarding_token`, `funnel_token`, `wa_me_url`,
`user_id`, `category_id`, `subcategory_id`, `card_id`, `transaction_id`/`_version`,
`recurring_id`/`_version`, `expense_id`/`_version`, `ref_month`,
`competence_active`, `competence_draft`, `now_iso`, `purchase_date`.

## Contratos de assinatura (verbatim do código)

- **Kiwify** — header `X-Kiwify-Signature` (ou query `?signature=`); `HMAC-SHA1` hex do raw body;
  secret `KIWIFY_WEBHOOK_SECRET`. Resp `202 {"received":true}`.
- **WhatsApp (Meta)** — header `X-Hub-Signature-256` = `sha256=` + `HMAC-SHA256` hex do raw body;
  secret `META_APP_SECRET`. Resp `200`.
- **Gateway** — canonical `lower(user_id) + "." + timestamp` (unix segundos); `HMAC-SHA256` hex;
  **a chave é o `gateway_secret` hex-decodificado** (`CryptoJS.enc.Hex.parse`); janela 60s; headers
  `X-User-ID`, `X-Gateway-Timestamp`, `X-Gateway-Auth`. Mutáveis de transactions/cards/
  recurring-templates exigem `Idempotency-Key` (gerado via `{{$guid}}`).

## Ordem de execução (Runner do Postman, top-to-bottom)

1. `00 — Infra & Health` — probes + catálogo OpenAPI (docs exigem `ENVIRONMENT=local`; senão 404).
2. `01 — Webhooks & Verificação` — verify Meta (GET) + status (POST).
3. `02 — CORS Preflight` — OPTIONS do onboarding (204).
4. `10 — Jailton` e `20 — Stefany`:
   - `A — Funil`: checkout → beacon `page_opened` → state (PENDING) → webhook Kiwify → state (PAID) → beacon `whatsapp_opened`.
   - `B — Ativação + onboarding`: `ATIVAR <token>` + turnos de onboarding (texto livre; asserção só de 200 — a resposta do bot é assíncrona via Send API).
   - `C1 — Identidade`: `POST /identity/users` (idempotente por whatsapp) → captura `user_id`.
   - `C — App autenticado`: categorias → transações (Pix, TED, débito em conta, cartão de débito, dinheiro, boleto, income salário) → cartões → transações no cartão (crédito à vista e parcelado) → recorrências → orçamentos. O pre-request da pasta assina cada request e calcula datas/competências.
5. `99 — Regressão negativa` — webhook Kiwify com secret errado → 401.

## Enums/invariantes usados (verbatim do domínio, evitam 400/409)

- Transação: `direction` ∈ {income, outcome}; `payment_method` ∈ {pix, ted, debit_in_account, debit_card, cash, boleto, credit_card} (`doc` é read-only); `amount_cents > 0`; subcategoria obrigatória em `outcome`. Para `credit_card`, `card_id` e `installments` (1..24) são obrigatórios.
- Recorrência: `frequency` ∈ {monthly, yearly}, `day_of_month` 1..28.
- Cartão: `due_day` 1..31; `bank` string livre.
- Orçamento: `total_cents > 0`; `allocations.basis_points` 0..10000 e **soma = 10000 exigida para ativar**; `root_slug` ∈ {expense.custo_fixo, expense.conhecimento, expense.prazeres, expense.metas, expense.liberdade_financeira}; recurrence `months` 1..12. DELETE de orçamento é permitido **apenas em rascunho** — por isso a collection cria uma competência-rascunho dedicada (`competence_draft`) para exercitar o DELETE.

## Cobertura — 52/52 rotas montadas

| Grupo | Qtde | Pasta |
| --- | --- | --- |
| Health probes (`/healthz`,`/livez`,`/readiness`,`/readyz`) | 4 | 00 |
| OpenAPI docs (`/__docs/`, index.json, `{module}.yaml`) | 3 | 00 |
| WhatsApp verify (GET `/verify`, GET `/inbound`) + status (POST) | 3 | 01 |
| CORS OPTIONS (checkout, tokens/state, tokens/opened) | 3 | 02 |
| Onboarding (checkout, tokens/state, tokens/opened) | 3 | 10/20 A |
| Billing webhook (kiwify) | 1 | 10/20 A + 99 |
| WhatsApp inbound (POST) | 1 | 10/20 B |
| Identity (POST /users) | 1 | 10/20 C1 |
| Categories (list, get, dictionary, search) | 4 | 10/20 C |
| Cards (create, list, best-day, get, put, delete, invoices, invoices/{m}) | 8 | 10/20 C |
| Budgets (create, recurrence, alerts, expenses POST/PATCH/DELETE, activate, delete, summary) | 9 | 10/20 C |
| Transactions (tx CRUD 5, recurring CRUD 5, months 2) | 12 | 10/20 C |

Total de rotas distintas cobertas: **52** (auditado por `method + path`). Total de requisições na collection: **118** (rotas autenticadas repetidas nos 2 perfis, incluindo múltiplos exemplos de `POST /transactions` com diferentes formas de pagamento).

## Validação executada

- JSON válido (`python3 -m json.tool`) para collection e environment.
- `schema` = Postman Collection v2.1.
- Auditoria automática: 52 pares `method+path` distintos == inventário de rotas montadas.
- Execução real (opcional, exige API no ar + `.env`):
  `newman run docs/postman/2026-07-04-mecontrola-jornada-completa.postman_collection.json -e docs/postman/2026-07-04-mecontrola-jornada-completa.postman_environment.json`

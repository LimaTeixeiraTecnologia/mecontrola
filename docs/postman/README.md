# Postman — mecontrola

Coleção Postman completa para a API do mecontrola, alinhada ao ambiente local definido em `deployment/compose/compose.local.yml`.

## Arquivos

| Arquivo | Propósito |
|---|---|
| `mecontrola.postman_collection.json` | Coleção completa cobrindo todos os módulos: identity, onboarding, billing (Kiwify webhook), cards, budgets, categories, whatsapp webhook. |
| `mecontrola.local.postman_environment.json` | Variáveis de ambiente local (`base_url=http://localhost:8080`, secrets placeholder, IDs preenchidos por scripts). |
| `card.postman_collection.json` / `card.postman_environment.json` | Coleção legada — subset do módulo `card`. Mantida para retrocompatibilidade. |

## Subir o stack local

```bash
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml up -d
```

Serviços expostos:

- `server` → `http://localhost:8080`
- Postgres → `localhost:5432`
- Grafana (otel-lgtm) → `http://localhost:3000`

## Variáveis obrigatórias antes de executar

| Variável | Origem | Quando preencher |
|---|---|---|
| `base_url` | Já preenchida | — |
| `kiwify_webhook_secret` | `.env` → `KIWIFY_WEBHOOK_SECRET_CURRENT` | Antes de simular o webhook Kiwify |
| `whatsapp_app_secret` | `.env` → app secret WhatsApp Business | Antes de chamar `POST /api/v1/whatsapp/inbound` |
| `whatsapp_verify_token` | `.env` → verify token configurado | Antes de chamar `GET /api/v1/whatsapp/verify` |

As demais (`user_id`, `funnel_token`, `card_id`, `category_id`, `idempotency_key`, `kiwify_sig`, `whatsapp_sig`) são preenchidas automaticamente por pre-request/test scripts.

## Auth — como bate em cada módulo

| Módulo | Auth | Header/Query |
|---|---|---|
| `identity` (upsert) | Pública (backend-to-backend) | — |
| `onboarding` | Pública (rate-limit + CORS) | — |
| `billing` (webhook) | HMAC-SHA1 hex sobre body raw | `?signature=<hex>` ou `X-Kiwify-Signature` |
| `cards`, `budgets`, `categories` | Header de usuário | `X-User-ID: <uuid v4>` |
| `cards` (mutações) | + idempotência | `Idempotency-Key: <ascii 1–128>` |
| `whatsapp` (inbound) | HMAC-SHA256 hex sobre body raw | `X-Hub-Signature-256: sha256=<hex>` |

## Fluxo end-to-end (uso completo)

1. **Identity → Upsert user by WhatsApp** — captura `user_id`.
2. **Onboarding → Create checkout** (`plan_id=monthly`) — captura `funnel_token`.
3. **Onboarding → Get token state** — confirma `ready_to_activate=false` (ainda sem pagamento).
4. **Billing → order_approved** — pre-request calcula HMAC; espera `202 {"received":true}`.
5. **Onboarding → Get token state** — agora `ready_to_activate=true`.
6. **Categories → Listar categorias** — captura `category_id`.
7. **Cards → Criar cartão** — pre-request gera `idempotency_key`; captura `card_id`.
8. **Budgets → Criar orçamento → Ativar → Criar despesa → Resumo mensal → Listar alertas**.

## Validar a coleção

```bash
jq empty docs/postman/mecontrola.postman_collection.json
jq empty docs/postman/mecontrola.local.postman_environment.json
```

## Smoke com Newman

```bash
newman run docs/postman/mecontrola.postman_collection.json \
  -e docs/postman/mecontrola.local.postman_environment.json \
  --folder "1. Identity"
```

## Notas

- Os scripts HMAC usam a lib `CryptoJS` embutida no Postman/Newman.
- Nenhum secret real foi commitado — todos são placeholders descritivos.
- O envelope do webhook Kiwify usa estrutura **flat** com chaves PascalCase para `Customer`/`Subscription`/`TrackingParameters` e triggers em inglês (`order_approved`, `subscription_renewed`, etc.) conforme o payload real observado em produção.

# Setup: Kiwify (Checkout + Webhook)

**Última revisão:** 2026-06-15
**Status:** ✅ produtos configurados, ⚠️ webhook em produção pendente.

## Produtos já configurados (Kiwify)

| Plano | URL de checkout | Preço | Uso |
|-------|-----------------|-------|-----|
| Mensal | https://pay.kiwify.com.br/ocPt7sv | R$ 29,90 | Produção |
| Trimestral | https://pay.kiwify.com.br/Sh2upAU | R$ 74,90 | Produção |
| Anual | https://pay.kiwify.com.br/HquleKA | R$ 239,90 | Produção |
| Teste (interno) | https://pay.kiwify.com.br/ix6YIk3 | R$ 5,00 | QA E2E sem custo alto |

URLs aplicadas em:
- Landing page: `mecontrola-landingpage/src/lib/content.ts:127-134`
- API local `.env`: `ONBOARDING_KIWIFY_CHECKOUT_URLS`
- API VPS `.env`: idem (a configurar)

## Etapas (ordem obrigatória)

### 1. Painel Kiwify

1. Login em https://dashboard.kiwify.com.br
2. **Produtos → Novo Produto** para cada plano (já feito).
3. Anotar o `product_id` de cada produto (visível na URL `dashboard.kiwify.com.br/products/<id>`).

### 2. Capturar credenciais da API

1. **Dashboard → Configurações → Integrações → API**.
2. Gerar **OAuth Client**: copiar `client_id` e `client_secret`.
3. Anotar `account_id` (mesmo lugar).

**Anotar:**
- `KIWIFY_CLIENT_ID=<client-id>`
- `KIWIFY_CLIENT_SECRET=<client-secret>`
- `KIWIFY_ACCOUNT_ID=<account-id>`
- `KIWIFY_PRODUCT_ID_MONTHLY=<id-mensal>`
- `KIWIFY_PRODUCT_ID_QUARTERLY=<id-trimestral>`
- `KIWIFY_PRODUCT_ID_ANNUAL=<id-anual>`

### 3. Configurar Webhook

1. **Dashboard → Integrações → Webhooks → Novo**.
2. URL: `https://api.mecontrola.app.br/api/v1/billing/webhooks/kiwify`
3. **Secret**: gerar localmente (`openssl rand -hex 20`) e anotar como
   `KIWIFY_WEBHOOK_SECRET`. Colar o mesmo no painel Kiwify.
4. **Eventos** a marcar:
   - `order_approved` (cria MagicToken + outreach)
   - `subscription_renewed` (estende period_end)
   - `subscription_late` (entra em PAST_DUE, grace 3 dias)
   - `subscription_canceled` (status canceled)
   - `order_refunded`
   - `chargeback`
   - (opcional/no-op) `billet_created`, `pix_created`, `order_rejected`, `abandoned_cart`
5. Salvar.

### 4. Configurar tracking parameters

A API exige pelo menos um de `sck`, `s1` ou `src` não-vazio (`funnel_token`). Sem isso, o
webhook retorna 422.

- Em **Configurações → Funil** do produto, adicionar parâmetros UTM ou custom:
  - `sck` para token de funil (rastreio de campanha).
  - `s1` para origem (instagram, google_ads, organico).
  - `src` para source secundário.
- Garantir que pelo menos `sck` esteja sempre populado.

### 5. Definir no `.env` da VPS

```sh
sudo nano /opt/mecontrola/.env
# Substituir todos os CHANGE_ME_*:
#   KIWIFY_CLIENT_ID=<real>
#   KIWIFY_CLIENT_SECRET=<real>
#   KIWIFY_ACCOUNT_ID=<real>
#   KIWIFY_PRODUCT_ID_MONTHLY=<real>
#   KIWIFY_PRODUCT_ID_QUARTERLY=<real>
#   KIWIFY_PRODUCT_ID_ANNUAL=<real>
#   KIWIFY_WEBHOOK_SECRET=<real>
#   ONBOARDING_KIWIFY_CHECKOUT_URLS=monthly=https://pay.kiwify.com.br/ocPt7sv\nquarterly=https://pay.kiwify.com.br/Sh2upAU\nannual=https://pay.kiwify.com.br/HquleKA
```

Reiniciar:

```sh
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d --no-deps --force-recreate server worker
```

### 6. Validar webhook

No painel Kiwify, **botão "Testar"** para `order_approved`. Verificar nos logs:

```sh
docker compose ... logs server | grep "kiwify webhook" | tail -10
# Esperado: "received webhook order_approved" + 202 accepted
```

Conferir no DB:
```sh
docker compose ... exec postgres psql -U mecontrola -d mecontrola_db -c \
  "SELECT id, status, customer_mobile_e164 FROM mecontrola.onboarding_tokens \
   ORDER BY created_at DESC LIMIT 3;"
```

### 7. Reconciliação automática

O job `KiwifyReconciliationJob` (config `KIWIFY_RECONCILIATION_SCHEDULE=@hourly`) compara
assinaturas do painel Kiwify com `billing_subscriptions` local. Falhas de evento (perda de
webhook) são reconciliadas. Não requer ação manual.

## Custos

- **Plataforma Kiwify**: 5,99% por transação + R$ 1,00 (gateway).
- **Webhook**: gratuito.
- **API rate**: 100 req/min (`KIWIFY_RATE_LIMIT_PER_MIN`).

## Rotação do Webhook Secret

Ver `deployment/runbooks/rotate-secret.md` → "Kiwify Webhook Secret".

## Bug conhecido — descoberto em sandbox 2026-06-08

Há **9 divergências estruturais** entre payload Kiwify real e structs do código (envelope
flat vs nested, triggers em inglês, PascalCase em alguns campos). Documentado em memória:
`project_e2_webhook_envelope_drift`. Antes de produção, executar smoke E2E e ajustar
parsing conforme payload real.

## Referências externas

- Docs Kiwify: https://docs.kiwify.com.br
- Painel: https://dashboard.kiwify.com.br
- API reference: `KIWIFY_API_BASE_URL=https://public-api.kiwify.com`

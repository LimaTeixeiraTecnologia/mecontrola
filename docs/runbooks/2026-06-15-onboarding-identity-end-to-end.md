# Runbook: Onboarding Identity End-to-End

**Escopo:** Jornada completa do cliente — da landing page até a conta ativa no WhatsApp — com
instrução passo a passo para ambiente local e VPS.

**Módulos envolvidos:** `onboarding`, `billing`, `identity`

**Última revisão:** 2026-06-15

> ⚠️ **LEIA ANTES**: este runbook descreve o handshake `checkout → webhook Kiwify → MagicToken
> → ATIVAR <token>` que **já está implementado**. A jornada conversacional pós-ativação
> (renda → cartões → categorias → percentuais → uso diário → alertas) **ainda está em stub no
> agent** (`AGENT_MODE=stub` apenas responde "MeControla recebeu sua mensagem"). Ver
> `2026-06-15-mvp-gap-analysis.md` para o roadmap completo de implementação MVP.
>
> **Estratégia de canal:** validar tudo primeiro no **Telegram** (sandbox); migrar para
> WhatsApp quando o número Business for liberado pela Meta.

---

## Jornada do Cliente (visão geral)

```
[EXTERNO]
  1. Clique no anúncio → acessa mecontrola.app.br
  2. Escolhe plano → POST /api/v1/onboarding/checkout → redirect Kiwify
  3. Preenche dados e realiza o pagamento na Kiwify
  4. Kiwify dispara webhook → POST /api/v1/billing/webhooks/kiwify
     └─ Sistema gera MagicToken + envia e-mail com link de ativação
  5. Cliente clica no link do e-mail → GET /api/v1/onboarding/tokens/{token}/state (polling)
  6. Redirect para wa.me → WhatsApp abre conversa com o bot
  7. Cliente envia "ATIVAR <código>" → POST /api/v1/whatsapp/inbound
     └─ Sistema cria usuário + vincula assinatura → CONTA ATIVA

[ONBOARDING NO WHATSAPP]
  8.  Bot pergunta renda mensal
  9.  Bot cadastra cartões de crédito
  10. Bot apresenta 5 categorias de gasto
  11. Bot sugere percentuais baseados na renda
  12. Conta ativada → uso diário via WhatsApp
```

---

## 1. Pré-requisitos

### 1.1 Ferramentas locais obrigatórias

```bash
# Verificar versões
go version          # >= 1.24
docker --version    # >= 26.x
docker compose version  # >= 2.x
task --version      # Taskfile (go install github.com/go-task/task/v3/cmd/task@latest)
psql --version      # PostgreSQL client (brew install libpq)
openssl version     # >= 3.x
jq --version        # brew install jq
gh --version        # >= 2.x (GitHub CLI)
node --version      # >= 20.x (para landing page)
pnpm --version      # >= 9.x (npm install -g pnpm)
```

### 1.2 Repos

```bash
# API principal
git clone git@github.com:LimaTeixeiraTecnologia/mecontrola.git
cd mecontrola

# Landing page (Astro 5 + Tailwind v4 — Cloudflare Pages)
gh repo clone LimaTeixeiraTecnologia/limateixeira-landingpage
```

### 1.3 Coleção Postman

Arquivos na pasta `docs/postman/`:
- `mecontrola-api.postman_collection.json` — 62 cenários, 42 endpoints REST + 21 webhooks
- `mecontrola-api.postman_environment.json` — variáveis de ambiente

Importar ambos no Postman: **File → Import → selecionar os dois arquivos**.

---

## 2. Configuração do Ambiente Local

### 2.1 Copiar e editar .env

```bash
cp .env.example .env
```

### 2.2 Gerar secrets locais

Execute cada comando abaixo e substitua os valores no `.env`:

```bash
# Gateway Auth — 32 bytes hex (64 chars)
openssl rand -hex 32
# → ex: a1b2c3d4e5f6... (64 chars)
# Colocar em: IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<valor>

# Token Encryption — exatamente 32 bytes ASCII imprimíveis
python3 -c "import secrets, string; print(secrets.token_hex(16))"
# OU usando openssl:
openssl rand -hex 16
# Colocar em: ONBOARDING_TOKEN_ENCRYPTION_KEY=<32 chars exatos>

# Kiwify Webhook Secret — qualquer string (HMAC-SHA1)
openssl rand -hex 20
# Colocar em: KIWIFY_WEBHOOK_SECRET=<valor>

# Meta/WhatsApp App Secret (para simular webhook local)
openssl rand -hex 16
# Colocar em: META_APP_SECRET=<valor>

# Meta Verify Token (string arbitrária)
echo "mecontrola-verify-$(openssl rand -hex 8)"
# Colocar em: META_VERIFY_TOKEN=<valor>
```

### 2.3 Valores mínimos para funcionar local

Editar `.env` com pelo menos estes valores (os demais podem ficar como estão no .env.example):

```dotenv
ENVIRONMENT=local
DB_PASSWORD=mecontrola_local_pass
DB_USER=mecontrola
DB_NAME=mecontrola_db
DB_HOST=localhost
DB_PORT=5432

# Gerados no passo anterior:
IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<64-char-hex>
ONBOARDING_TOKEN_ENCRYPTION_KEY=<32-chars-exatos>
KIWIFY_WEBHOOK_SECRET=<seu-secret-sha1>
META_APP_SECRET=<32-char-hex>
META_VERIFY_TOKEN=mecontrola-verify-local

# Planos Kiwify — usar IDs fictícios local (qualquer string)
KIWIFY_PRODUCT_ID_MONTHLY=prod_monthly_local
KIWIFY_PRODUCT_ID_QUARTERLY=prod_quarterly_local
KIWIFY_PRODUCT_ID_ANNUAL=prod_annual_local

# Mapa plan_id → URL de checkout (separados por newline no .env)
ONBOARDING_KIWIFY_CHECKOUT_URLS=monthly=https://pay.kiwify.com.br/monthly\nquarterly=https://pay.kiwify.com.br/quarterly\nannual=https://pay.kiwify.com.br/annual

# CORS — para landing page local
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:4321
ONBOARDING_CHECKOUT_CORS_ORIGINS=http://localhost:4321,https://www.mecontrola.app.br

# Número de bot local (qualquer E.164 brasileiro)
META_PHONE_NUMBER_ID=100000000000001
META_BOT_NUMBER_E164=+5511900000000
META_BOT_NUMBER_DISPLAY=+55 11 9XXXX-XXXX
META_ACCESS_TOKEN=local-mock-token

# Worker ativo
OUTBOX_DISPATCHER_ENABLED=true
AGENT_MODE=stub
```

### 2.4 Validar .env antes de subir

```bash
# Nenhuma variável CHANGE_ME deve estar presente para ENVIRONMENT=local
grep "CHANGE_ME" .env && echo "ATENÇÃO: ainda há valores CHANGE_ME" || echo "OK"
```

---

## 3. Stack Local (Docker Compose)

### 3.1 Subir serviços

```bash
# Primeiro build da imagem local (opcional — pode usar imagem do registry)
docker build -t ghcr.io/limateixeiratecnologia/mecontrola:latest .

# Subir stack completa com override local
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.local.yml \
  up -d
```

### 3.2 Serviços e portas

| Serviço      | Porta local    | Descrição                         |
|--------------|---------------|-----------------------------------|
| `postgres`   | `5432`        | Banco principal (exposto no local) |
| `pgbouncer`  | interno       | Connection pooler                 |
| `migrate`    | —             | Roda migrations e sai             |
| `server`     | `8080`        | HTTP API                          |
| `worker`     | —             | Outbox dispatcher + jobs          |
| `otel-lgtm`  | `3000` (Grafana), `4317` (OTLP gRPC), `4318` (OTLP HTTP) | Stack de observabilidade |

### 3.3 Verificar saúde dos serviços

```bash
# Todos os serviços devem estar "healthy" ou "running"
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml ps

# Health check da API
curl -s http://localhost:8080/health | jq .
# Esperado: {"status":"ok"} ou 200 sem corpo

# Logs ao vivo
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml logs -f server
```

### 3.4 Conectar ao banco local

```bash
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db

# Tabelas principais do onboarding/identity/billing:
\dt
# Relevantes: users, magic_tokens, subscriptions, kiwify_processed_events, outbox_messages, auth_events
```

### 3.5 Parar stack

```bash
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml down
# Para apagar volumes (limpa banco):
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml down -v
```

---

## 4. Configuração do Postman

### 4.1 Variáveis de ambiente a preencher

Após importar `mecontrola-api.postman_environment.json`, editar estas variáveis:

| Variável            | Valor                                          | Tipo    |
|---------------------|------------------------------------------------|---------|
| `base_url`          | `http://localhost:8080`                        | default |
| `user_id`           | *(vazio inicialmente — preenchido depois)*      | secret  |
| `gateway_secret`    | Valor de `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` | secret |
| `kiwify_webhook_secret` | Valor de `KIWIFY_WEBHOOK_SECRET`           | secret  |
| `meta_app_secret`   | Valor de `META_APP_SECRET`                     | secret  |
| `meta_verify_token` | Valor de `META_VERIFY_TOKEN`                   | secret  |
| `competence`        | `2026-06`                                      | default |
| `ref_month`         | `2026-06`                                      | default |

### 4.2 Como o pre-request script computa o gateway auth

A collection já tem um pre-request script global que calcula automaticamente:

```javascript
// Algoritmo executado antes de cada request autenticado:
const userId = pm.environment.get("user_id");
const secretHex = pm.environment.get("gateway_secret");
const ts = Math.floor(Date.now() / 1000).toString();

// Decodifica hex → bytes
const secretBytes = CryptoJS.enc.Hex.parse(secretHex);

// Mensagem canônica: lowercase(user_id) + "." + unix_timestamp
const message = userId.toLowerCase() + "." + ts;

// HMAC-SHA256 → hex
const sig = CryptoJS.HmacSHA256(message, secretBytes).toString(CryptoJS.enc.Hex);

pm.environment.set("gateway_auth", sig);
pm.environment.set("gateway_timestamp", ts);
```

Os headers enviados automaticamente em requests autenticados:
```
X-User-ID: {{user_id}}
X-Gateway-Auth: {{gateway_auth}}
X-Gateway-Timestamp: {{gateway_timestamp}}
```

---

## 5. Endpoints Detalhados

### 5.1 Health Check

```bash
curl -s http://localhost:8080/health
```

**Resposta esperada:** `200 OK`

---

### 5.2 Upsert User — `POST /api/v1/identity/users`

**Rota protegida por gateway auth.** Cria ou atualiza um usuário por número WhatsApp.

#### Gerar gateway auth manualmente (bash)

```bash
GATEWAY_SECRET="<seu-IDENTITY_GATEWAY_SHARED_SECRET_CURRENT>"
USER_ID=""  # vazio para criar novo usuário

TS=$(date +%s)
MESSAGE="${USER_ID}.${TS}"  # "" + "." + timestamp (lowercase do user_id)

SIG=$(echo -n "$MESSAGE" | openssl dgst -sha256 -mac HMAC \
  -macopt "hexkey:${GATEWAY_SECRET}" | awk '{print $2}')

echo "X-User-ID: $USER_ID"
echo "X-Gateway-Auth: $SIG"
echo "X-Gateway-Timestamp: $TS"
```

#### Criar usuário (sem user_id prévio — gateway auth com user_id vazio)

```bash
GATEWAY_SECRET="<seu-secret-hex-64-chars>"
TS=$(date +%s)
SIG=$(echo -n ".${TS}" | openssl dgst -sha256 -mac HMAC \
  -macopt "hexkey:${GATEWAY_SECRET}" | awk '{print $2}')

curl -s -X POST http://localhost:8080/api/v1/identity/users \
  -H "Content-Type: application/json" \
  -H "X-User-ID: " \
  -H "X-Gateway-Auth: $SIG" \
  -H "X-Gateway-Timestamp: $TS" \
  -d '{
    "whatsapp": "+5511987654321",
    "email": "cliente@example.com",
    "display_name": "João Silva"
  }' | jq .
```

**Resposta `200 OK`:**
```json
{
  "id": "01927a3b-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "whatsapp": "+5511987654321",
  "email": "cliente@example.com",
  "display_name": "João Silva",
  "status": "ACTIVE",
  "created_at": "2026-06-15T10:00:00Z",
  "updated_at": "2026-06-15T10:00:00Z"
}
```

**Salvar o `id` retornado** — é o `user_id` para usar em todos os demais requests autenticados.

No Postman: após executar, o script de teste da requisição já popula `user_id` automaticamente.

**Códigos de erro:**

| HTTP | Código          | Causa                                  |
|------|-----------------|----------------------------------------|
| 400  | `invalid_whatsapp` | Número não é E.164 brasileiro       |
| 400  | `invalid_email`    | E-mail inválido                     |
| 400  | `invalid_payload`  | JSON malformado                     |
| 409  | `whatsapp_in_use`  | Número já vinculado a outra conta   |
| 409  | `email_in_use`     | E-mail já vinculado a outra conta   |
| 401  | —               | Falha no gateway auth               |

---

### 5.3 Checkout — `POST /api/v1/onboarding/checkout`

**Rota pública** (sem auth). Retorna a URL de checkout da Kiwify para o plano solicitado.

```bash
curl -s -X POST http://localhost:8080/api/v1/onboarding/checkout \
  -H "Content-Type: application/json" \
  -H "Origin: http://localhost:4321" \
  -d '{"plan_id": "monthly"}' | jq .
```

**`plan_id` válidos:** `monthly`, `quarterly`, `annual`
(conforme mapeamento em `ONBOARDING_KIWIFY_CHECKOUT_URLS`)

**Resposta `201 Created`:**
```json
{
  "checkout_url": "https://pay.kiwify.com.br/monthly"
}
```

**Códigos de erro:**

| HTTP | Código                | Causa                                           |
|------|-----------------------|-------------------------------------------------|
| 400  | `unknown_plan`        | `plan_id` não existe no mapa de checkout        |
| 503  | `checkout_unavailable`| URL não configurada para o plano                |
| 429  | —                     | Rate limit excedido (10 req/min por IP)         |
| 415  | —                     | Content-Type não é `application/json`           |

> **CORS local:** o header `Origin` deve estar na lista `ONBOARDING_CHECKOUT_CORS_ORIGINS` do `.env`.
> Adicionar `http://localhost:4321` para testar com a landing page local.

---

### 5.4 Token State — `GET /api/v1/onboarding/tokens/{token}/state`

Verifica se o magic token já está pronto para ativação (polling após checkout aprovado).

```bash
TOKEN="<token-de-ativacao-do-email>"

curl -s "http://localhost:8080/api/v1/onboarding/tokens/${TOKEN}/state" | jq .
```

**Resposta quando pronto (`ready_to_activate: true`):**
```json
{
  "ready_to_activate": true,
  "wa_me_url": "https://wa.me/5511900000000?text=ATIVAR%20abc123",
  "bot_number_display": "+55 11 9XXXX-XXXX"
}
```

**Resposta quando ainda processando (`ready_to_activate: false`):**
```json
{
  "ready_to_activate": false
}
```

> Fazer polling a cada 2–5 segundos. O token fica pronto após o webhook `order_approved` ser
> processado pelo sistema.

**Rate limit:** 30 req/min por IP, burst 10.

---

### 5.5 Kiwify Webhook — `POST /api/v1/billing/webhooks/kiwify`

**Assinatura obrigatória:** HMAC-SHA1 do body JSON, passada via query param `?signature=<hex>`
OU header `X-Kiwify-Signature`.

#### Calcular assinatura HMAC-SHA1

```bash
KIWIFY_SECRET="<seu-KIWIFY_WEBHOOK_SECRET>"
PAYLOAD_FILE="payload.json"  # arquivo com o JSON do evento

SIG=$(openssl dgst -sha1 -mac HMAC \
  -macopt "key:${KIWIFY_SECRET}" \
  "${PAYLOAD_FILE}" | awk '{print $2}')

echo "Signature: $SIG"
```

#### Script helper — scripts/sign-kiwify-webhook.sh

Já criado no repo (commit de fix Bug C, 2026-06-15). Usa **HMAC-SHA1 sobre os bytes
exatos do arquivo** e envia via `curl --data-binary` (preserva newlines, ao contrário do
`-d` que strip-a). Exemplo de uso:

```bash
export KIWIFY_WEBHOOK_SECRET=<seu-secret>
bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-order-approved.json
# -> Assinatura: <hex-40>
# -> Enviando POST http://localhost:8080/api/v1/billing/webhooks/kiwify?signature=<hex-40>
# {"received":true}
# 202
```

> ⚠️ **Bug histórico (fixed 2026-06-15):** versões antigas do runbook usavam
> `curl -d @file` para enviar o webhook — isso strip-a newlines do body antes do envio,
> quebrando a assinatura computada sobre o arquivo original. Use **sempre** o script
> `scripts/sign-kiwify-webhook.sh` ou `curl --data-binary @file` manualmente.

#### Trigger: `order_approved` (cria MagicToken + inicia onboarding)

**Este é o evento mais importante.** Substitua os valores marcados com `<...>`.

```bash
cat > /tmp/kiwify-order-approved.json << 'EOF'
{
  "webhook_event_type": "order_approved",
  "order_id": "order_001_local_test",
  "order_ref": "K-001-TEST",
  "order_status": "paid",
  "subscription_id": "sub_001_local_test",
  "Product": {
    "product_id": "prod_monthly_local",
    "product_name": "MeControla Mensal"
  },
  "Customer": {
    "email": "cliente@example.com",
    "mobile": "+5511987654321",
    "CPF": "123.456.789-09"
  },
  "Subscription": {
    "start_date": "2026-06-15T10:00:00Z",
    "next_payment": "2026-07-15T10:00:00Z",
    "status": "active"
  },
  "TrackingParameters": {
    "sck": "token-de-funil-abc123",
    "s1": "",
    "src": "instagram"
  },
  "approved_date": "2026-06-15T10:00:00Z",
  "updated_at": "2026-06-15T10:00:00Z",
  "created_at": "2026-06-15T10:00:00Z"
}
EOF

KIWIFY_WEBHOOK_SECRET="<seu-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-order-approved.json
```

> **Atenção:** `TrackingParameters.sck` é o `funnel_token`. O sistema usa `sck`, depois `s1`,
> depois `src` (primeiro não-vazio) para rastrear de onde o cliente veio. Se todos forem vazios,
> o webhook retorna `422 funnel_token_missing`.
>
> Para testes locais, use qualquer string não-vazia em `sck`.

**Resposta `202 Accepted`:**
```json
{"received": true}
```

**Códigos de erro:**

| HTTP | Código                  | Causa                                              |
|------|-------------------------|----------------------------------------------------|
| 401  | —                       | Assinatura HMAC-SHA1 inválida                      |
| 415  | —                       | Content-Type não é `application/json`              |
| 422  | `invalid_json`          | Payload JSON malformado                            |
| 422  | `funnel_token_missing`  | `sck`, `s1` e `src` todos vazios                  |
| 422  | `unknown_trigger`       | `webhook_event_type` não reconhecido               |
| 202  | —                       | `event_already_processed` (idempotência, OK)       |
| 202  | —                       | `event_superseded` (evento mais recente já existe) |

#### Trigger: `subscription_renewed`

```bash
cat > /tmp/kiwify-renewed.json << 'EOF'
{
  "webhook_event_type": "subscription_renewed",
  "order_id": "order_002_local_test",
  "order_ref": "K-002-TEST",
  "order_status": "paid",
  "subscription_id": "sub_001_local_test",
  "Product": {
    "product_id": "prod_monthly_local",
    "product_name": "MeControla Mensal"
  },
  "Customer": {
    "email": "cliente@example.com",
    "mobile": "+5511987654321",
    "CPF": "123.456.789-09"
  },
  "Subscription": {
    "start_date": "2026-07-15T10:00:00Z",
    "next_payment": "2026-08-15T10:00:00Z",
    "status": "active"
  },
  "TrackingParameters": {
    "sck": "token-de-funil-abc123",
    "s1": "",
    "src": ""
  },
  "approved_date": "2026-07-15T10:00:00Z",
  "updated_at": "2026-07-15T10:00:00Z",
  "created_at": "2026-07-15T10:00:00Z"
}
EOF

KIWIFY_WEBHOOK_SECRET="<seu-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-renewed.json
```

#### Trigger: `subscription_late` (entra em PAST_DUE — grace period 3 dias)

```bash
cat > /tmp/kiwify-late.json << 'EOF'
{
  "webhook_event_type": "subscription_late",
  "order_id": "order_003_local_test",
  "order_ref": "K-003-TEST",
  "order_status": "waiting_payment",
  "subscription_id": "sub_001_local_test",
  "Product": {
    "product_id": "prod_monthly_local",
    "product_name": "MeControla Mensal"
  },
  "Customer": {
    "email": "cliente@example.com",
    "mobile": "+5511987654321",
    "CPF": "123.456.789-09"
  },
  "Subscription": {
    "start_date": "2026-06-15T10:00:00Z",
    "next_payment": "2026-07-15T10:00:00Z",
    "status": "late"
  },
  "TrackingParameters": {
    "sck": "token-de-funil-abc123",
    "s1": "",
    "src": ""
  },
  "approved_date": "2026-06-15T10:00:00Z",
  "updated_at": "2026-07-15T10:00:00Z",
  "created_at": "2026-06-15T10:00:00Z"
}
EOF

KIWIFY_WEBHOOK_SECRET="<seu-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-late.json
```

> A assinatura entra em `past_due`. O worker `GraceExpirationJob` expira após 3 dias
> (configurável via `BILLING_GRACE_EXPIRATION_SCHEDULE=@daily`).

#### Trigger: `subscription_canceled`

```bash
cat > /tmp/kiwify-canceled.json << 'EOF'
{
  "webhook_event_type": "subscription_canceled",
  "order_id": "order_004_local_test",
  "order_ref": "K-004-TEST",
  "order_status": "canceled",
  "subscription_id": "sub_001_local_test",
  "Product": {
    "product_id": "prod_monthly_local",
    "product_name": "MeControla Mensal"
  },
  "Customer": {
    "email": "cliente@example.com",
    "mobile": "+5511987654321",
    "CPF": "123.456.789-09"
  },
  "Subscription": {
    "start_date": "2026-06-15T10:00:00Z",
    "next_payment": "2026-07-15T10:00:00Z",
    "status": "canceled"
  },
  "TrackingParameters": {
    "sck": "token-de-funil-abc123",
    "s1": "",
    "src": ""
  },
  "approved_date": "2026-06-15T10:00:00Z",
  "updated_at": "2026-07-15T10:00:00Z",
  "created_at": "2026-06-15T10:00:00Z"
}
EOF

KIWIFY_WEBHOOK_SECRET="<seu-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-canceled.json
```

#### Trigger: `order_refunded` e `chargeback`

```bash
# Trocar "webhook_event_type" para "order_refunded" ou "chargeback"
# Adicionar campo refunded_at:
cat > /tmp/kiwify-refunded.json << 'EOF'
{
  "webhook_event_type": "order_refunded",
  "order_id": "order_001_local_test",
  "order_ref": "K-001-TEST",
  "order_status": "refunded",
  "subscription_id": "sub_001_local_test",
  "Product": {
    "product_id": "prod_monthly_local",
    "product_name": "MeControla Mensal"
  },
  "Customer": {
    "email": "cliente@example.com",
    "mobile": "+5511987654321",
    "CPF": "123.456.789-09"
  },
  "TrackingParameters": {
    "sck": "token-de-funil-abc123",
    "s1": "",
    "src": ""
  },
  "refunded_at": "2026-06-20T10:00:00Z",
  "approved_date": "2026-06-15T10:00:00Z",
  "updated_at": "2026-06-20T10:00:00Z",
  "created_at": "2026-06-15T10:00:00Z"
}
EOF

KIWIFY_WEBHOOK_SECRET="<seu-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-refunded.json
```

#### Triggers no-op (aceitos mas sem efeito)

Os triggers abaixo são aceitos com `202` mas não alteram estado:
- `billet_created`
- `pix_created`
- `order_rejected`
- `abandoned_cart`

---

### 5.6 WhatsApp Webhook — Verificação (GET)

```bash
curl -s "http://localhost:8080/api/v1/whatsapp/verify?\
hub.mode=subscribe&\
hub.verify_token=<META_VERIFY_TOKEN>&\
hub.challenge=desafio_aleatorio_123"
```

**Resposta esperada:** `200 OK` com body `desafio_aleatorio_123` (echo do challenge).

---

### 5.7 WhatsApp Inbound — `POST /api/v1/whatsapp/inbound`

**Simula o bot recebendo uma mensagem do cliente.** Requer assinatura HMAC-SHA256 do body
com `META_APP_SECRET`, no header `X-Hub-Signature-256: sha256=<hex>`.

#### Calcular assinatura WhatsApp (HMAC-SHA256)

```bash
META_APP_SECRET="<seu-META_APP_SECRET>"
PAYLOAD_FILE="/tmp/wa-inbound.json"

SIG=$(cat "$PAYLOAD_FILE" | openssl dgst -sha256 -mac HMAC \
  -macopt "key:${META_APP_SECRET}" | awk '{print $2}')

echo "X-Hub-Signature-256: sha256=$SIG"
```

#### Payload: mensagem "ATIVAR <código>"

Substituir `<PHONE_NUMBER_ID>` pelo valor de `META_PHONE_NUMBER_ID` e `<CODIGO>` pelo token
obtido do e-mail (ou do banco após o `order_approved` webhook).

```bash
cat > /tmp/wa-ativar.json << 'EOF'
{
  "object": "whatsapp_business_account",
  "entry": [
    {
      "id": "WABA_ID_LOCAL",
      "changes": [
        {
          "value": {
            "messaging_product": "whatsapp",
            "metadata": {
              "display_phone_number": "11900000000",
              "phone_number_id": "<META_PHONE_NUMBER_ID>"
            },
            "contacts": [
              {
                "profile": {"name": "João Silva"},
                "wa_id": "5511987654321"
              }
            ],
            "messages": [
              {
                "from": "5511987654321",
                "id": "wamid.local_test_001",
                "timestamp": "1750000000",
                "text": {
                  "body": "ATIVAR token-de-funil-abc123"
                },
                "type": "text"
              }
            ]
          },
          "field": "messages"
        }
      ]
    }
  ]
}
EOF

META_APP_SECRET="<seu-secret>"
SIG=$(cat /tmp/wa-ativar.json | openssl dgst -sha256 -mac HMAC \
  -macopt "key:${META_APP_SECRET}" | awk '{print $2}')

curl -s -X POST http://localhost:8080/api/v1/whatsapp/inbound \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=$SIG" \
  -d @/tmp/wa-ativar.json | jq .
```

**Resposta esperada:** `200 OK` com `{}`

> O número `from` deve estar no formato E.164 sem `+` no campo `wa_id` (ex: `5511987654321`).
> O `META_PHONE_NUMBER_ID` no payload `metadata.phone_number_id` deve bater com o configurado.

**Mensagens de texto que o bot interpreta:**
- `ATIVAR <código>` — ativa a conta com o magic token
- Qualquer outra coisa em `AGENT_MODE=stub` → resposta "MeControla recebeu sua mensagem"

**Verificar ativação no banco:**

```bash
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db \
  -c "SELECT id, status, consumed_at, consumed_by_mobile_e164 FROM magic_tokens WHERE status='CONSUMED' LIMIT 5;"

psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db \
  -c "SELECT id, user_id, status, plan, period_start, period_end FROM subscriptions LIMIT 5;"
```

---

## 6. Jornada End-to-End Local (12 passos)

Execute em sequência. Cada passo cita o endpoint e o resultado esperado no banco.

### Passo 1 — Subir a stack

```bash
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.local.yml \
  up -d

# Aguardar migrate completar
docker compose -f deployment/compose/compose.yml \
  -f deployment/compose/compose.local.yml \
  logs migrate --follow
# Aguardar: "migrations applied successfully" ou similar
```

### Passo 2 — Health check

```bash
curl -s http://localhost:8080/health
# Esperado: 200
```

### Passo 3 — Simular o cliente escolhendo o plano (checkout)

```bash
curl -s -X POST http://localhost:8080/api/v1/onboarding/checkout \
  -H "Content-Type: application/json" \
  -H "Origin: http://localhost:4321" \
  -d '{"plan_id": "monthly"}' | jq .
# Esperado: {"checkout_url": "https://pay.kiwify.com.br/monthly"}
```

> Em produção o cliente seria redirecionado para essa URL. Localmente, vamos simular o
> webhook de aprovação diretamente.

### Passo 4 — Simular aprovação do pagamento (webhook Kiwify `order_approved`)

```bash
KIWIFY_WEBHOOK_SECRET="<seu-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-order-approved.json
# Esperado: {"received": true}
```

### Passo 5 — Verificar magic token criado no banco

```bash
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db \
  -c "SELECT id, status, plan_id, expires_at, customer_mobile_e164, customer_email \
      FROM magic_tokens ORDER BY created_at DESC LIMIT 3;"
```

> O token deve estar com `status='PAID'` e `customer_mobile_e164='+5511987654321'`.

### Passo 6 — Buscar o código de ativação

O **token visível para o cliente** é o `activation_token_ciphertext` (40-45 chars
`[A-Za-z0-9_\-]`, validado pelo regex em
`internal/platform/channels/activation_command.go:3`). Ele é gerado durante o processamento
do `order_approved` e enviado por e-mail. Para teste local, ele aparece nos logs do worker
quando o outreach é disparado, e também fica salvo no banco:

```bash
# Extrair o ciphertext do magic token mais recente em estado PAID:
TOKEN=$(psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db \
  -t -c "SELECT activation_token_ciphertext FROM magic_tokens \
         WHERE status='PAID' ORDER BY created_at DESC LIMIT 1;" \
  | tr -d ' ')

echo "Activation token: $TOKEN"
# Esperado: string de 40-45 chars [A-Za-z0-9_-]
```

> ⚠️ **NÃO use o `id` do magic token** — o `id` é o identificador interno UUID. O usuário
> recebe o `activation_token_ciphertext` (também alimentado em `wa_me_url` retornado por
> `/onboarding/tokens/{token}/state`).

### Passo 7 — Verificar estado do token (polling como faria o front)

```bash
curl -s "http://localhost:8080/api/v1/onboarding/tokens/${TOKEN}/state" | jq .
# Esperado quando pronto:
# {"ready_to_activate": true, "wa_me_url": "https://wa.me/...", "bot_number_display": "+55 11 ..."}
```

### Passo 8 — Simular o cliente enviando "ATIVAR <código>" no WhatsApp

Editar `/tmp/wa-ativar.json` substituindo `token-de-funil-abc123` pelo valor real do `sck`
ou pelo ID do magic token, conforme o token enviado no e-mail.

```bash
META_APP_SECRET="<seu-secret>"
SIG=$(cat /tmp/wa-ativar.json | openssl dgst -sha256 -mac HMAC \
  -macopt "key:${META_APP_SECRET}" | awk '{print $2}')

curl -s -X POST http://localhost:8080/api/v1/whatsapp/inbound \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=$SIG" \
  -d @/tmp/wa-ativar.json | jq .
```

### Passo 9 — Confirmar usuário ativo e assinatura vinculada

```bash
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db << 'SQL'
SELECT
  u.id as user_id,
  u.whatsapp,
  u.email,
  u.status as user_status,
  s.id as subscription_id,
  s.status as sub_status,
  s.plan,
  s.period_start,
  s.period_end
FROM users u
LEFT JOIN subscriptions s ON s.user_id = u.id
WHERE u.whatsapp = '+5511987654321';
SQL
```

> Esperado: `user_status='ACTIVE'`, `sub_status='active'`.

### Passo 10 — Verificar magic token consumido

```bash
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db \
  -c "SELECT status, consumed_at, consumed_by_mobile_e164 FROM magic_tokens \
      WHERE customer_mobile_e164='+5511987654321';"
# Esperado: status='CONSUMED', consumed_at preenchido
```

### Passo 11 — Simular renovação

```bash
KIWIFY_WEBHOOK_SECRET="<seu-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-renewed.json
# Esperado: {"received": true}

# Verificar period_end atualizado:
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db \
  -c "SELECT status, period_start, period_end FROM subscriptions LIMIT 5;"
```

### Passo 12 — Simular inadimplência → grace → cancelamento

```bash
# 1. Entrar em grace (PAST_DUE — 3 dias de carência)
KIWIFY_WEBHOOK_SECRET="<seu-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-late.json

# Verificar status past_due e grace_end:
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db \
  -c "SELECT status, grace_end FROM subscriptions LIMIT 5;"

# 2. Cancelar
KIWIFY_WEBHOOK_SECRET="<seu-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-canceled.json

# Verificar status canceled:
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db \
  -c "SELECT status FROM subscriptions LIMIT 5;"
```

---

## 7. Landing Page Local

### 7.1 Repos

- **GitHub oficial (obrigatório usar via `gh` CLI):**
  `https://github.com/LimaTeixeiraTecnologia/limateixeira-landingpage`
- **Pasta local:** `/Users/jailtonjunior/Git/mecontrola-landingpage`

```bash
# Se ainda não tem o clone local:
gh repo clone LimaTeixeiraTecnologia/limateixeira-landingpage /Users/jailtonjunior/Git/mecontrola-landingpage

cd /Users/jailtonjunior/Git/mecontrola-landingpage
make install
```

### 7.2 Rodar em modo dev

```bash
make dev
# Acesso: http://localhost:4321
```

> A landing page usa Astro 5 + Tailwind CSS v4, deploy via Cloudflare Pages. Comandos via
> `make` (`make install`, `make dev`, `make build`, `make preview`, `make check`,
> `make og-image`).

### 7.3 Integrar botão de checkout com a API local

O repo atual é uma landing page institucional da Lima Teixeira Consulting.
Para testar o fluxo de checkout do MeControla localmente, adicionar um botão que chama
`POST /api/v1/onboarding/checkout` da API local.

Exemplo de snippet JavaScript para adicionar em qualquer página `.astro`:

```html
<script>
  async function handleCheckout(planId) {
    const res = await fetch('http://localhost:8080/api/v1/onboarding/checkout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ plan_id: planId }),
    });
    if (!res.ok) {
      alert('Erro ao iniciar checkout: ' + res.status);
      return;
    }
    const { checkout_url } = await res.json();
    window.location.href = checkout_url;
  }
</script>

<button onclick="handleCheckout('monthly')">Assinar Mensal</button>
<button onclick="handleCheckout('quarterly')">Assinar Trimestral</button>
<button onclick="handleCheckout('annual')">Assinar Anual</button>
```

> **CORS:** adicionar `http://localhost:4321` em `ONBOARDING_CHECKOUT_CORS_ORIGINS` no `.env`
> e reiniciar o server.

### 7.4 Build para Cloudflare Pages

```bash
make build
# Saída em: dist/
# Deploy automático via GitHub Actions (.github/workflows/deploy.yml) ao push em main.
# Secrets necessários no repo:
#   - CLOUDFLARE_API_TOKEN  ✅ configurado em 2026-05-24
#   - CLOUDFLARE_ACCOUNT_ID ✅ configurado em 2026-05-24
#   - PUBLIC_GA_ID          ⚠️ pendente — `gh secret set PUBLIC_GA_ID --body G-XXXXXXXXXX`
```

---

## 8. Observabilidade Local

### 8.1 Grafana

URL: `http://localhost:3000`
Usuário: `admin` | Senha: `admin@dev` (configurado em `OTEL_LGTM_ADMIN_USER/PASSWORD`)

**Painéis disponíveis após envio de traces/logs:**
- Explore → Loki → `{service_name="mecontrola-api"}` — logs estruturados
- Explore → Tempo — traces distribuídos (rastrear o fluxo checkout → webhook → ativação)
- Explore → Prometheus — métricas de rate limit, auth, outbox

### 8.2 Métricas relevantes

```bash
# Ver métricas expostas (se endpoint /metrics estiver habilitado)
curl -s http://localhost:8080/metrics | grep -E "identity_gateway|outbox|billing|onboarding"
```

### 8.3 Consultar auth_events no banco

```bash
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db << 'SQL'
SELECT
  kind,
  source,
  reason,
  created_at
FROM auth_events
ORDER BY created_at DESC
LIMIT 20;
SQL
```

### 8.4 Consultar outbox_messages

```bash
psql postgres://mecontrola:mecontrola_local_pass@localhost:5432/mecontrola_db << 'SQL'
SELECT
  event_type,
  status,
  attempts,
  created_at,
  last_attempted_at
FROM outbox_messages
ORDER BY created_at DESC
LIMIT 20;
SQL
```

---

## 9. VPS — Checklist de Deploy (Hostinger)

### 9.1 Pré-requisitos na VPS

```bash
# Na VPS (como root):
apt-get update && apt-get install -y docker.io docker-compose-plugin git openssl
systemctl enable docker && systemctl start docker

# Clonar o repo
git clone git@github.com:LimaTeixeiraTecnologia/mecontrola.git /opt/mecontrola
cd /opt/mecontrola
```

### 9.2 Criar .env de produção

```bash
cp .env.example .env
chmod 600 .env
chown root:root .env
```

**Substituir TODOS os valores `CHANGE_ME_*` e ajustar:**

```dotenv
ENVIRONMENT=production

# Banco — SSL obrigatório em produção
DB_PASSWORD=<senha-forte-gerada-com-openssl-rand-base64-32>
DB_SSL_MODE=require

# Caddy (HTTPS automático via Let's Encrypt)
APP_DOMAIN=api.mecontrola.app.br
CADDY_EMAIL=devops@mecontrola.app.br

# Observabilidade — Grafana Cloud
LOKI_URL=https://logs-prod-xxx.grafana.net/loki/api/v1/push
LOKI_USER_ID=<id-do-usuario-loki>
LOKI_API_KEY=<api-key-loki>
OTEL_EXPORTER_OTLP_ENDPOINT=tempo-prod-xxx.grafana.net:443
OTEL_EXPORTER_OTLP_INSECURE=false

# Kiwify (credenciais reais do painel Kiwify)
KIWIFY_CLIENT_ID=<client-id-real>
KIWIFY_CLIENT_SECRET=<client-secret-real-min-64-chars>
KIWIFY_ACCOUNT_ID=<account-id-real>
KIWIFY_PRODUCT_ID_MONTHLY=<product-id-mensal-real>
KIWIFY_PRODUCT_ID_QUARTERLY=<product-id-trimestral-real>
KIWIFY_PRODUCT_ID_ANNUAL=<product-id-anual-real>
KIWIFY_WEBHOOK_SECRET=<secret-configurado-no-painel-kiwify>
ONBOARDING_KIWIFY_CHECKOUT_URLS=monthly=https://pay.kiwify.com.br/<slug-mensal>\nquarterly=https://pay.kiwify.com.br/<slug-trimestral>\nannual=https://pay.kiwify.com.br/<slug-anual>

# Meta / WhatsApp Business API (painel developers.facebook.com)
META_PHONE_NUMBER_ID=<phone-number-id-real>
META_ACCESS_TOKEN=<access-token-permanente>
META_APP_SECRET=<app-secret-do-painel-meta>
META_VERIFY_TOKEN=<string-aleatoria-configurada-no-webhook-meta>
META_BOT_NUMBER_E164=+55<ddd><numero>
META_BOT_NUMBER_DISPLAY=+55 <ddd> <numero-formatado>

# Gateway Auth
IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=$(openssl rand -hex 32)

# Token Encryption (exatamente 32 bytes)
ONBOARDING_TOKEN_ENCRYPTION_KEY=$(openssl rand -hex 16)

# CORS — domínio real da landing page
CORS_ALLOWED_ORIGINS=https://www.mecontrola.app.br,https://mecontrola.app.br
ONBOARDING_CHECKOUT_CORS_ORIGINS=https://www.mecontrola.app.br,https://mecontrola.app.br
ONBOARDING_TRUSTED_PROXIES=127.0.0.1/32,::1/128

# Grafana admin (stack local de monitoring se não usar Cloud)
GRAFANA_ADMIN_PASSWORD=$(openssl rand -base64 24)
```

### 9.3 Subir stack de produção

```bash
cd /opt/mecontrola

# Pull da imagem mais recente
docker pull ghcr.io/limateixeiratecnologia/mecontrola:latest

# Subir com compose de produção
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d

# Verificar containers
docker compose -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml ps
```

### 9.4 Registrar webhook no painel Kiwify

1. Acessar **painel.kiwify.com → Integrações → Webhooks**
2. URL: `https://api.mecontrola.app.br/api/v1/billing/webhooks/kiwify`
3. Secret: valor de `KIWIFY_WEBHOOK_SECRET` (mesmo configurado no `.env`)
4. Eventos: marcar `order_approved`, `subscription_renewed`, `subscription_late`,
   `subscription_canceled`, `order_refunded`, `chargeback`
5. Salvar e usar o botão "Testar" para cada evento

> A Kiwify passa a assinatura via query param `?signature=<hex>` (HMAC-SHA1 do body).

### 9.5 Registrar webhook no painel Meta (WhatsApp Business)

1. Acessar **developers.facebook.com → Meu App → WhatsApp → Configuração**
2. Callback URL: `https://api.mecontrola.app.br/api/v1/whatsapp/inbound`
3. Verify Token: valor de `META_VERIFY_TOKEN`
4. Clicar em "Verificar e salvar" — o sistema responde ao desafio automaticamente
5. Assinar campo: `messages`

### 9.6 Verificar DNS e HTTPS

```bash
# Na VPS ou localmente:
curl -s https://api.mecontrola.app.br/health
# Esperado: 200 OK

# Verificar certificado:
openssl s_client -connect api.mecontrola.app.br:443 -servername api.mecontrola.app.br \
  </dev/null 2>/dev/null | openssl x509 -noout -dates
```

---

## 10. Testes na VPS

Repetir todos os passos da Seção 5 substituindo a `base_url`:

```bash
BASE_URL="https://api.mecontrola.app.br"

# Health check
curl -s ${BASE_URL}/health

# Checkout (origin deve ser o domínio real da landing page)
curl -s -X POST ${BASE_URL}/api/v1/onboarding/checkout \
  -H "Content-Type: application/json" \
  -H "Origin: https://www.mecontrola.app.br" \
  -d '{"plan_id": "monthly"}' | jq .

# Kiwify webhook (usando script)
BASE_URL="${BASE_URL}" KIWIFY_WEBHOOK_SECRET="<real-secret>" \
  bash scripts/sign-kiwify-webhook.sh /tmp/kiwify-order-approved.json
```

---

## 11. Rotação de Secrets (sem downtime)

### 11.1 Gateway Auth

```bash
# 1. Gerar novo secret
NEW_SECRET=$(openssl rand -hex 32)

# 2. Adicionar como NEXT (ambos válidos simultaneamente):
# No .env:
# IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<antigo>
# IDENTITY_GATEWAY_SHARED_SECRET_NEXT=<novo>

# 3. Reiniciar server (sem downtime com compose)
docker compose -f ... restart server

# 4. Quando todos os clientes estiverem usando o novo:
# IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<novo>
# IDENTITY_GATEWAY_SHARED_SECRET_NEXT=
# Reiniciar novamente
```

### 11.2 Kiwify Webhook Secret

```bash
# Mesmo padrão: KIWIFY_WEBHOOK_SECRET=<antigo> + KIWIFY_WEBHOOK_SECRET_NEXT=<novo>
# Atualizar o novo no painel Kiwify → salvar
# Reiniciar server → promover NEXT para CURRENT → limpar NEXT
```

### 11.3 Meta App Secret

```bash
# META_APP_SECRET=<antigo> + META_APP_SECRET_NEXT=<novo>
# Atualizar no painel Meta → promover → limpar
```

---

## 12. Troubleshooting

### 12.1 Webhook Kiwify retorna 401

- Verificar se `KIWIFY_WEBHOOK_SECRET` no `.env` bate com o configurado no painel Kiwify
- Confirmar que o HMAC é SHA-1 (não SHA-256) do body exato sem whitespace extra
- A assinatura vai via `?signature=<hex>` (query param) OU `X-Kiwify-Signature` (header)
- Testar localmente com o script `sign-kiwify-webhook.sh`

### 12.2 Gateway auth retorna 401

```bash
# Verificar: o header X-User-ID deve ser o UUID do usuário (lowercase)
# O campo "message" no canonical é: lowercase(user_id) + "." + unix_timestamp
# O timestamp deve estar dentro de 60 segundos (IDENTITY_GATEWAY_AUTH_WINDOW)

# Verificar auth_events para diagnóstico:
psql ... -c "SELECT reason, created_at FROM auth_events ORDER BY created_at DESC LIMIT 10;"
```

### 12.3 WhatsApp inbound retorna 401

- Verificar se `META_APP_SECRET` no `.env` é o mesmo do painel Meta
- A assinatura é HMAC-SHA256 do body raw (sem modificação)
- Header: `X-Hub-Signature-256: sha256=<hex>` (com prefixo `sha256=`)

### 12.4 Webhook retorna 422 `funnel_token_missing`

- Verificar se `TrackingParameters.sck`, `s1` ou `src` tem ao menos um valor não-vazio
- O sistema usa o primeiro não-vazio como funnel_token

### 12.5 Server não sobe

```bash
# Ver logs de inicialização:
docker compose ... logs server | head -100

# Causas comuns em produção:
# - ENVIRONMENT=production com variável CHANGE_ME_* → Config.Validate() falha
# - ONBOARDING_TOKEN_ENCRYPTION_KEY != 32 bytes exatos
# - DB_SSL_MODE=require mas postgres não tem SSL configurado
```

### 12.6 Magic token não é criado após `order_approved`

```bash
# Verificar worker outbox:
docker compose ... logs worker | grep "outbox\|dispatch\|billing"

# Verificar outbox_messages pendentes:
psql ... -c "SELECT event_type, status, attempts, last_error FROM outbox_messages \
             WHERE status != 'applied' ORDER BY created_at DESC LIMIT 10;"
```

---

## Referências

- Coleção Postman: `docs/postman/mecontrola-api.postman_collection.json`
- Diagrama billing: `docs/diagrams/billing/`
- Diagrama onboarding: `docs/diagrams/onboarding/`
- Diagrama identity: `docs/diagrams/identity/`
- Deploy runbook: `deployment/runbooks/deploy.md`
- Rotate secrets: `deployment/runbooks/rotate-secret.md`
- ADR-006 (DMMF transactions): `.specs/prd-billing-pipeline/`

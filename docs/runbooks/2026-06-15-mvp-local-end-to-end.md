# MVP Local End-to-End — MeControla (WhatsApp + Telegram + Kiwify + ngrok)

> Data: 2026-06-15
> Objetivo: rodar o ciclo completo (anúncio → checkout Kiwify → email → ativação → WhatsApp/Telegram → onboarding → uso diário → alertas) 100% localmente antes do primeiro deploy na Hostinger.

## Stack Local

- **Postgres 16** + pgBouncer — `docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml up -d`
- **Mailpit** (SMTP 1025 / UI 8025) — captura emails de ativação sem custo
- **Grafana LGTM** (OTLP 4317/4318) — observabilidade
- **server** (8080) + **worker** (background jobs)
- **Landing Astro** (`mecontrola-landingpage`) na porta 4321
- **ngrok** — túnel para expor `server` ao WhatsApp Business, Telegram e Kiwify

## Pré-requisitos

- Conta Meta WhatsApp Business sandbox
- Bot Telegram criado via @BotFather → `TELEGRAM_BOT_TOKEN`
- Conta Kiwify sandbox com webhook configurável
- Chave OpenRouter (Gemini 2.5 Flash) — `OPENROUTER_API_KEY`
- ngrok autenticado localmente (`ngrok config add-authtoken ...`)

## Passo a Passo

### 1. Subir infra local

```bash
cp .env.local.example .env
# Edite .env preenchendo OPENROUTER_API_KEY, KIWIFY_*, META_*, TELEGRAM_*
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml up -d postgres mailpit otel-lgtm
task migrate
task server &     # roda em foreground; ou docker-compose up server worker
task worker &
```

Verifique:
- Postgres: `psql -h localhost -U mecontrola -d mecontrola -c '\dt'`
- Mailpit: abrir http://localhost:8025
- Grafana: http://localhost:3000 (admin/admin@dev)

### 2. Subir túnel ngrok

```bash
task ngrok:server
# Anote a URL pública impressa, ex: https://abcd-1234.ngrok.io
```

### 3. Configurar webhooks externos com a URL ngrok

**Kiwify (modo teste):** Settings → Webhooks → adicionar:
- URL: `https://<ngrok>/api/v1/billing/webhooks/kiwify`
- Header: `X-Kiwify-Webhook-Token: <KIWIFY_WEBHOOK_SECRET>` (defina o mesmo valor no `.env`)

**Meta WhatsApp Business:** Configuration → Webhooks → Edit:
- Callback URL: `https://<ngrok>/api/v1/whatsapp/inbound`
- Verify Token: valor de `META_VERIFY_TOKEN`
- Subscribe: `messages`

**Telegram bot:** rodar uma vez:
```bash
curl -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/setWebhook" \
  -d "url=https://<ngrok>${TELEGRAM_WEBHOOK_PATH}" \
  -d "secret_token=${TELEGRAM_SECRET_TOKEN}"
```

### 4. Subir landing local

```bash
cd ../mecontrola-landingpage
cp .env.example .env
# Edite PUBLIC_BACKEND_URL=https://<ngrok>
pnpm install
pnpm dev   # http://localhost:4321
```

### 5. Simular fluxo end-to-end

#### 5.1 — Compra Kiwify (sandbox)
- Abrir landing local `/` e clicar em "Quero o MeControla" (link Kiwify).
- Completar checkout sandbox.
- Kiwify dispara webhook → backend chama `MarkTokenPaid` → consumer dispara `SendActivationEmail`.

#### 5.2 — Verificar email em Mailpit
- Abrir http://localhost:8025.
- Conferir email "Ative sua conta MeControla".
- Clicar no botão **Ativar MeControla** → abre `http://localhost:4321/activate?token=<token>`.

#### 5.3 — Página de ativação
- A página chama `GET <PUBLIC_BACKEND_URL>/api/v1/onboarding/tokens/<token>/state`.
- Backend responde `{ ready_to_activate: true, wa_me_url, telegram_deep_link, bot_number_display }`.
- Aparecem 2 botões: **WhatsApp** (verde #25D366) e **Telegram** (azul #229ED9).

#### 5.4 — Ativação WhatsApp
- Clicar WhatsApp → abre app com texto `ATIVAR <token>` preenchido.
- Envia mensagem → Meta posta no webhook → backend identifica token → registra `WhatsApp account` no `identity` → resposta "Bem-vindo".

#### 5.5 — Ativação Telegram (alternativa)
- Voltar ao email, clicar Telegram → abre Telegram com `/start ATIVAR_<token>`.
- Mesmo fluxo, mesmo resultado.

#### 5.6 — Onboarding conversacional *(Fase 2 — em implementação)*
- Bot pergunta renda → cartões → percentuais por categoria → resumo.
- Estado persistido em `onboarding_sessions` (sobrevive a restart).

#### 5.7 — Uso diário *(Fase 4 — em implementação)*
- "gastei 58 no iFood" → LLM parseia → cria expense em Prazeres → reply estruturado.
- "quanto gastei em Prazeres?" → consulta DB → resposta.
- "resumo do mês" → materialização do monthly summary.

#### 5.8 — Alertas proativos *(Fase 3 — em implementação)*
- Forçar gastos acumulando >80% de uma categoria.
- Aguardar próximo tick do cron `BUDGETS_THRESHOLD_ALERTS_CRON` (default `@hourly`).
- Ou disparar manualmente via task `task alerts:run-once`.

## Variáveis `.env` mínimas

Use `.env.local.example` como ponto de partida (incluído neste runbook em `infra-config`). Campos críticos para o MVP local:

| Variável | Valor local |
|----------|-------------|
| `EMAIL_PROVIDER` | `smtp` |
| `SMTP_HOST` | `mailpit` (dentro do compose) ou `localhost` (host) |
| `SMTP_PORT` | `1025` |
| `EMAIL_FROM_ADDRESS` | `noreply@mecontrola.local` |
| `EMAIL_ACTIVATE_URL` | `http://localhost:4321/activate` |
| `META_VERIFY_TOKEN` | qualquer string aleatória (mesma no Meta) |
| `META_APP_SECRET` | do Meta App |
| `META_ACCESS_TOKEN` | do Meta App |
| `META_PHONE_NUMBER_ID` | do Meta App |
| `META_BOT_NUMBER_E164` | `+5511XXXXXXXXX` |
| `TELEGRAM_ENABLED` | `true` |
| `TELEGRAM_BOT_TOKEN` | do BotFather |
| `TELEGRAM_BOT_USERNAME` | `mecontrola_bot` (sem @) |
| `TELEGRAM_SECRET_TOKEN` | qualquer string aleatória |
| `KIWIFY_WEBHOOK_SECRET` | do Kiwify |
| `AGENT_MODE` | `openrouter` |
| `OPENROUTER_API_KEY` | sua chave |
| `AGENT_LLM_PRIMARY_MODEL` | `google/gemini-2.5-flash` |

## Critérios de aceitação (sem falso positivo)

- Email chega em Mailpit com link clicável e expira em N horas conforme `ONBOARDING_TOKEN_TTL_DAYS`.
- Token inválido/expirado → handler retorna `ready_to_activate=false` (não 500).
- Onboarding sobrevive a restart do `server` no meio do fluxo (estado em DB).
- Mensagem fora do estado FSM responde mantendo estado, não trava.
- Alerta de categoria dispara em ≤1h após cruzar 80%; reenvio mesmo dia é dedupado por `(user_id, budget_id, kind, ref_day)`.
- Paridade Telegram: mesmo cenário end-to-end funciona em ambos canais.

## Troubleshooting

- **Email não chega:** `docker compose logs mailpit` + conferir `SMTP_HOST/SMTP_PORT`. No host fora do compose, use `SMTP_HOST=localhost`.
- **ngrok URL muda a cada restart:** repita configuração dos 3 webhooks (Kiwify, Meta, Telegram).
- **`ready_to_activate=false` mas token deveria ser válido:** consultar `select * from magic_tokens where token_hash = ...` para ver status real.
- **WhatsApp não responde:** validar HMAC em `META_APP_SECRET` e logs `onboarding.consumer.subscription_paid.*`.

## Próximos passos

- [ ] Fase 2: FSM onboarding conversacional persistido (DMMF)
- [ ] Fase 3: Job de alertas proativos
- [ ] Fase 4: LLM intent parser PT-BR
- [ ] Smoke task `task mvp:smoke`
- [ ] Validar paridade WhatsApp ↔ Telegram em todo onboarding
- [ ] Deploy Hostinger (compose.prod.yml + Caddy + SSL)

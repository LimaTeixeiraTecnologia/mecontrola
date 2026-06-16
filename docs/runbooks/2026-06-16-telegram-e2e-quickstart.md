# Telegram E2E Quickstart — MeControla

> Data: 2026-06-16
> Objetivo: rodar o ciclo MVP ponta-a-ponta no canal Telegram (sem Meta API) em ~10 minutos, com **3 ações humanas reais** e o resto automatizado.

## Por que este runbook existe

O MVP do MeControla foi codificado, testado e auditado (gaps G1-G5 + Opção A fechados). Mas até este momento, **nenhuma mensagem real** percorreu o ciclo Kiwify → email → ativação → onboarding → gasto → alerta proativo. Este runbook fecha essa lacuna.

Resultado esperado ao final: você recebe no Telegram uma mensagem proativa do tipo:
> "Sua fatura no cartão está em R$ 4.500,00. Você já utilizou 90% do limite."

Se isso acontecer, o produto é production-proof para o canal Telegram.

## Pré-requisitos do host

- Docker Desktop rodando
- Go 1.26.4, Node 24, pnpm 11 instalados
- `task` (go-task), `curl`, `jq`, `psql`, `openssl`, `uuidgen` no PATH
- `ngrok` autenticado: `ngrok config add-authtoken <YOUR_TOKEN>`
- Conta Telegram com app instalado

## Visão geral

| Etapa | Quem | Tempo |
|-------|------|-------|
| Criar bot via @BotFather | Você | ~2 min |
| Editar `.env` com 4 valores | Você | ~2 min |
| `task mvp:telegram:prepare` | Script | ~1-2 min |
| `task mvp:telegram:drive` | Script + 5 prompts | ~5 min |
| Confirmação visual | Você | ~10 seg |

---

## Fase 1 — Criar bot Telegram

1.1. No app Telegram, abra **@BotFather**
1.2. Envie `/newbot`
1.3. Nome (display): qualquer (ex. `MeControla Dev`)
1.4. Username (precisa terminar em `bot`): ex. `mecontrola_dev_bot`
1.5. BotFather responde com `TELEGRAM_BOT_TOKEN` no formato `1234567890:AAH...`

**Anote**:
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_BOT_USERNAME` (o que você escolheu, sem `@`)

1.6. Pegar `bot_id`:
```bash
TOKEN="<COLE_AQUI>"
curl -s "https://api.telegram.org/bot${TOKEN}/getMe" | jq
```
**Anote**: campo `id` → `TELEGRAM_BOT_ID`

1.7. Gerar secret token:
```bash
openssl rand -hex 32
```
**Anote**: → `TELEGRAM_SECRET_TOKEN`

---

## Fase 2 — Editar `.env`

Abra `/Users/jailtonjunior/Git/mecontrola/.env` e garanta as seguintes variáveis. As 4 da Fase 1 são as únicas que você precisa preencher do zero — as demais já existem com valor padrão funcional:

```dotenv
# Da Fase 1 (você cola estes 4):
TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=<da BotFather>
TELEGRAM_BOT_ID=<numérico de getMe>
TELEGRAM_BOT_USERNAME=<sem @>
TELEGRAM_SECRET_TOKEN=<openssl rand -hex 32>

# Ajuste obrigatório (default é 'legacy'):
BUDGETS_THRESHOLD_ALERTS_MODE=job

# Confirme que estes JÁ existem com valor válido (não CHANGE_ME):
KIWIFY_WEBHOOK_SECRET=<32+ chars estável>
KIWIFY_PRODUCT_ID_MONTHLY=<uuid estável>
ONBOARDING_TOKEN_ENCRYPTION_KEY=<32 bytes hex ou base64>
OPENROUTER_API_KEY=<sua chave OpenRouter>
EMAIL_PROVIDER=smtp
SMTP_HOST=mailpit
SMTP_PORT=1025
EMAIL_FROM_ADDRESS=noreply@mecontrola.local
EMAIL_ACTIVATE_URL=http://localhost:4321/activate
```

**Verificação**:
```bash
grep -E "TELEGRAM_BOT_TOKEN|TELEGRAM_SECRET_TOKEN|AGENT_MODE|OPENROUTER_API_KEY|KIWIFY_WEBHOOK_SECRET|BUDGETS_THRESHOLD_ALERTS_MODE" .env
```
Confirme: nenhum valor vazio ou contendo `CHANGE_ME`.

---

## Fase 3 — `task mvp:telegram:prepare`

```bash
cd /Users/jailtonjunior/Git/mecontrola
task mvp:telegram:prepare
```

**O que esse comando faz** (~1-2 min):
1. Valida que todas as variáveis `.env` críticas estão preenchidas.
2. `go build ./...` + `go vet ./...` (falha cedo se código quebrado).
3. `docker compose up -d postgres mailpit otel-lgtm`.
4. `go run ./cmd/migrate up` (aplica migrations 000001..000006).
5. Inicia `server` e `worker` em background (logs em `/tmp/telegram-{server,worker}.log`).
6. Sobe `ngrok http 8080` detached.
7. Lê URL pública do ngrok via `localhost:4040/api/tunnels`.
8. Chama `setWebhook` na API Telegram com a URL ngrok + secret_token.
9. Confirma via `getWebhookInfo`.

**Saída esperada** ao final:
```
==================================================================
 STACK PRONTO PARA E2E TELEGRAM
==================================================================
 ngrok URL:        https://abcd-1234.ngrok.io
 Telegram bot:     @mecontrola_dev_bot
 Server logs:      tail -f /tmp/telegram-server.log
 Worker logs:      tail -f /tmp/telegram-worker.log
 ngrok logs:       tail -f /tmp/telegram-ngrok.log
 Mailpit UI:       http://localhost:8025
 Postgres:         postgres://mecontrola:mecontrola@localhost:5432/mecontrola

 PROXIMO PASSO:
   task mvp:telegram:drive
==================================================================
```

**Se falhar**: leia o `FAIL: <razão>` e cheque o log apontado.

---

## Fase 4 — `task mvp:telegram:drive`

```bash
task mvp:telegram:drive
```

O script vai pausar 5 vezes pedindo ação humana. Cada pausa imprime `>>> <instrução>` e espera você pressionar ENTER. **Não pressione ENTER antes de completar a ação no Telegram** — o script vai pollar o DB e validar que a ação foi efetiva antes de seguir.

### Sequência

**Step 1/8** — Simula webhook Kiwify (automatizado).

**Step 2/8** — Aguarda email aterrissar no Mailpit; extrai token (automatizado).

**Step 3/8** — Consulta `/api/v1/onboarding/tokens/<token>/state` e exibe `telegram_deep_link` (automatizado).

**Step 4/8** — 🧑 **PROMPT humano #1**:
- Script imprime `https://t.me/<seu_bot>?start=ATIVAR_<token>`
- Você abre o link no celular ou no Telegram Desktop
- Clica em **INICIAR**
- (Se o regex G1 falhar por algum motivo edge case, você pode digitar manualmente `ATIVAR <token>` com espaço)
- Bot responde "Bem-vindo(a) ao MeControla via Telegram!"
- Pressione ENTER no terminal
- Script polla `user_identities` até linha com `channel='telegram'` aparecer (até 90s timeout)

**Step 5/8** — 🧑 **PROMPT humano #2, #3, #4** (onboarding FSM):
- Envie `3500` no Telegram → ENTER
- Script aguarda `onboarding_sessions.payload->>'IncomeCents'='350000'`
- Envie `nao` no Telegram → ENTER
- Envie `esta otimo` no Telegram → ENTER
- Script aguarda `state='active'`

**Step 6/8** — 🧑 **PROMPT humano #5** (LogExpense via LLM live):
- Envie `gastei 50 reais no iFood` no Telegram → ENTER
- Script aguarda `budgets_expenses` com `source='agent'` e `amount_cents=5000`
- Bot deve responder "Anotei: R$ 50,00 em iFood (Prazeres). Já está no seu orçamento do mês."

**Step 7/8** — Cria cartão + invoice 90% via SQL (automatizado):
- `INSERT mecontrola.cards (..., limit_cents=500000)`
- `INSERT mecontrola.transactions_card_invoices (..., items_total_cents=450000, ref_month=YYYY-MM)`
- Dispara `worker` efêmero por 8s com `BUDGETS_THRESHOLD_ALERTS_CRON='@every 2s'`
- Polla `budget_alerts_sent.kind='card_limit_near'`
- Polla `budget_alerts_sent.notified_at IS NOT NULL`

**Step 8/8** — Imprime dump de evidências (`users`, `user_identities`, `onboarding_sessions`, `budgets_expenses`, `budget_alerts_sent`).

**Confirmação visual final**:
Você deve receber AGORA no Telegram, **sem você ter enviado nada**, uma mensagem do tipo:
> "Sua fatura no cartão está em R$ 4.500,00. Você já utilizou 90% do limite."

Se sim → **production-ready end-to-end para Telegram.**

---

## Cleanup

```bash
# Remover webhook Telegram
TOKEN=$(grep TELEGRAM_BOT_TOKEN .env | cut -d= -f2)
curl -s -X POST "https://api.telegram.org/bot${TOKEN}/deleteWebhook?drop_pending_updates=true" | jq

# Parar server, worker, ngrok
kill $(cat /tmp/telegram-server.pid) $(cat /tmp/telegram-worker.pid) $(cat /tmp/telegram-ngrok.pid) 2>/dev/null || true

# Parar Docker
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml down
```

---

## Troubleshooting

| Sintoma | Causa provável | Onde olhar |
|---------|----------------|------------|
| `task mvp:telegram:prepare` falha em "variaveis .env faltando" | Você esqueceu de preencher algo da Fase 2 | `grep CHANGE_ME .env` |
| `setWebhook` retorna `ok:false` | TOKEN errado ou ngrok URL não-https | `tail /tmp/telegram-ngrok.log` |
| Step 4 timeout (ativação não chega) | Telegram webhook não atinge backend, ou secret_token mismatch | `tail /tmp/telegram-server.log \| grep -i telegram` |
| Step 5 timeout (FSM não avança) | LLM em modo stub ou OpenRouter sem créditos | `grep AGENT_MODE .env` (deve ser `openrouter`); `tail /tmp/telegram-server.log \| grep parse_inbound` |
| Step 6 timeout (expense não persiste) | SearchDictionary não casa "iFood" → categoria não encontrada | Tente `gastei 50 no mercado` (mais provável de bater no dicionário PT-BR) |
| Step 7 alerta criado mas `notified_at` nulo | Consumer `ThresholdAlertNotifier` quebrou ou Telegram gateway falhou | `tail /tmp/telegram-worker.log \| grep threshold_alert` |
| Step 7 `notified_at` preenchido mas mensagem não chega no Telegram | `UserChannelResolver` preferiu WhatsApp (não disponível) | `SELECT notify_channel FROM mecontrola.budget_alerts_sent ORDER BY sent_at DESC LIMIT 1;` — se vier `whatsapp`, o resolver está com bug. Esperado: `telegram` quando user só tem identidade Telegram |

---

## Próximos passos após sucesso

1. **Anotar a métrica de custo**: quantos tokens OpenRouter foram gastos. Olhe o dashboard OpenRouter ou os logs do agent module.
2. **Validar `agent_intent_parse_decode_failed_total` em 0**: significa que o schema do Gemini decodificou perfeitamente.
3. **Rodar mais 5-10 expenses** com merchants variados para mapear cobertura do dicionário.
4. **Documentar tempo de resposta** observado (p50, p99).
5. **Decidir** com PO sobre `BUDGETS_THRESHOLD_GOAL_RATIO` (default 0.50).
6. **Migrar para Hostinger** com `compose.prod.yml` + Caddy SSL.

---

## Limitações conhecidas (não bloqueiam Telegram E2E)

- **Outreach Telegram** (`SendOutreach` usecase) só funciona via WhatsApp por enquanto. Para Telegram-only, a coluna `telegram_external_id` precisa ser adicionada a `onboarding_tokens` em migration futura.
- **At-least-once delivery** do alerta: se o consumer crashar entre `SendText` e `MarkNotified`, redelivery pode mandar 2× (mitigação aceita para MVP — `SELECT FOR UPDATE` em PR futuro).
- **Endpoint `POST /cards` exige auth gateway HMAC** (não testado neste runbook — usamos `INSERT` SQL direto na Fase 7 para simplificar).
- **Provider OpenRouter** tem 2 schemas coexistindo (`mecontrola_intent` legacy + `mecontrola_parse_intent` novo). Quando o `HandleInboundMessage` legacy for deprecado, limpar.

---

## Referências

- Plano original: `~/.claude/plans/dado-essa-imagem-users-jailtonjunior-dow-misty-avalanche.md`
- Runbook E2E geral: `docs/runbooks/2026-06-15-mvp-local-end-to-end.md`
- Production readiness status: `docs/runbooks/2026-06-16-mvp-production-readiness-status.md`
- Discovery limit_cents: `docs/discovery/2026-06-16-card-limit-cents.md`
- Scripts: `scripts/smoke/telegram_{prepare,drive}.sh`
- Taskfile: `mvp:telegram:prepare`, `mvp:telegram:drive`, `alerts:run-once`

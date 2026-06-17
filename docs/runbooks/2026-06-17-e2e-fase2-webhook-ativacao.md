# E2E Fase 2 — Webhook → Email → WhatsApp → Banco

**Data:** 2026-06-17
**Ambiente:** VPS 187.77.45.48 (produção)
**Executado em:** macOS
**Pré-condição:** Fase 1 concluída — VPS healthy, secrets Kiwify configurados, SMTP ativo.
**Objetivo:** Validar o caminho do dinheiro completo: webhook `order_approved` → worker processa → email enviado → token ativado no WhatsApp → usuário criado no banco.

---

## Artefatos Postman

| Arquivo | Descrição |
|---|---|
| `docs/runbooks/mecontrola-e2e-fase2.postman_collection.json` | Collection com Passos 1–3 (health, checkout, webhook) |
| `docs/runbooks/mecontrola-e2e-fase2.postman_environment.json` | Environment com todas as variáveis de produção |

**Importar no Postman:**
1. Abra o Postman → **Import**
2. Arraste os dois arquivos acima (collection + environment)
3. Selecione o environment **"MeControla — Produção"** no seletor do canto superior direito
4. Pronto — os scripts calculam HMAC-SHA1 e passam o token entre os passos automaticamente

---

## Referência rápida

| Item | Valor |
|---|---|
| API produção | `https://api.mecontrola.app.br` |
| VPS SSH | `root@187.77.45.48` |
| Bot MeControla (WhatsApp) | `+55 11 93621-2870` |
| Número pessoal (cliente de teste) | `+55 11 98689-6322` |
| Email de teste | `jailton.junior94@outlook.com` |
| KIWIFY_WEBHOOK_SECRET | `47cyjfb3gag` |
| product_id MONTHLY | `2d7d8e25-ecfd-45f0-98ba-54a496060959` |
| Checkout URL MONTHLY | `https://pay.kiwify.com.br/ocPt7sv` |

---

## Fluxo desta fase

```
[Postman]
  ├─ GET  /health                              → API saudável
  ├─ POST /api/v1/onboarding/checkout          → magic token PENDING criado no banco
  └─ POST /api/v1/billing/webhooks/kiwify      → webhook aceito, evento publicado no outbox

[VPS — Worker]
  ├─ MarkTokenPaid   → token PENDING → PAID
  └─ SendActivationEmail → email com link de ativação enviado

[Email — jailton.junior94@outlook.com]
  └─ Copia token do link: https://www.mecontrola.app.br/ativar?token=<TOKEN>

[Celular — WhatsApp, do +55 11 98689-6322]
  └─ Envia "ATIVAR <token>" para +55 11 93621-2870

[Bot MeControla]
  └─ token PAID → CONSUMED, usuário criado, responde boas-vindas

[DBeaver — tunnel mecontrola-db]
  └─ Confirma token CONSUMED + subscription ACTIVE + usuário criado
```

---

## Checklist

- [x] Passo 0 — novo código deployado na VPS (CD auto-deploy commit b23defdc — 4 bug fixes)
- [ ] Passo 1 — health OK
- [ ] Passo 2 — checkout retornou `checkout_url` com `?sck=` e token salvo no Postman
- [ ] Passo 3 — webhook retornou `{"received": true}`
- [ ] Passo 4 — worker logou `process_sale_approved` + `mark_token_paid` + `activation_email_dispatched`
- [ ] Passo 5 — email recebido em `jailton.junior94@outlook.com`
- [ ] Passo 6 — bot respondeu "Sua conta foi ativada com sucesso! Bem-vindo ao MeControla."
- [ ] Passo 7 — DBeaver mostra token `CONSUMED` + `subscription_status=ACTIVE` + `whatsapp_number=+5511986896322`

---

## Passo 0 — Deploy do novo código na VPS

> **Status: CONCLUÍDO automaticamente via CD** — commit `b23defdc` (4 bug fixes) deployado em 2026-06-17T12:39:20Z. Healthcheck OK após 10s.
>
> O pipeline CD (`Deploy to VPS`) executa automaticamente após cada CI verde no `main`. Só execute o comando manual abaixo se o CD falhar no job `Deploy to VPS`.

**Bugs presentes neste deploy:**
- `ErrNestedTransaction` no `UpsertUserByWhatsApp` — corrigido com `database.FromContext(ctx)`
- Nil pointer em `ProcessSubscriptionGraceExpired` — `kiwifyDBTX` injetado no construtor
- False-positive warning do outbox para `billing.subscription.activated` — `noUserEventAllowlist`
- Landing page `/ativar` ausente — redirect criado preservando `?token=`

**Nota sobre o CD "failure":** o job `Auth Smoke (staging)` falha porque `STAGING_DB_URL` tenta conectar em `187.77.45.48:5432` que está bloqueado pelo firewall (correto). O deploy em si (`Deploy to VPS`) **teve sucesso**. Falha pré-existente — não indica problema no código.

### Fallback — Deploy manual (só se CD falhar)

```bash
ssh root@187.77.45.48 << 'DEPLOY'
set -e
cd /opt/mecontrola

echo "==> Atualizando código..."
git pull origin main

echo "==> Rebuild das imagens (server + worker)..."
IMAGE_TAG=local docker compose \
  --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  build server worker

echo "==> Reiniciando containers..."
IMAGE_TAG=local docker compose \
  --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  up -d --no-deps --force-recreate server worker

echo "==> Verificando saúde..."
sleep 5
curl -sf https://api.mecontrola.app.br/health | python3 -m json.tool
DEPLOY
```

**Resultado esperado:**
```json
{
    "status": "healthy",
    "service": "mecontrola-api",
    "environment": "production",
    "checks": { "database": { "status": "healthy" } }
}
```

---

## Passo 1 — Verificar pré-condições

**O que faz:** confirma que a API está online e os secrets estão corretos na VPS antes de qualquer teste.

### No Postman

Abra a request **"Passo 1 — Health Check"** e clique **Send**.

Resultado esperado:
```json
{
    "status": "healthy",
    "service": "mecontrola-api",
    "environment": "production",
    "checks": { "database": { "status": "healthy" } }
}
```

### Na VPS

```bash
ssh root@187.77.45.48 \
  "grep -E '^KIWIFY_WEBHOOK_SECRET|^KIWIFY_CLIENT_ID|^KIWIFY_ACCOUNT_ID|^SMTP_HOST|^EMAIL_FROM_ADDRESS' \
  /opt/mecontrola/.env"
```

Resultado esperado:
```
KIWIFY_WEBHOOK_SECRET=47cyjfb3gag
KIWIFY_CLIENT_ID=ee760e58-fce3-4b67-b007-d71c8adb0292
KIWIFY_ACCOUNT_ID=JhojQYNsUNcvgTV
SMTP_HOST=smtp.resend.com
EMAIL_FROM_ADDRESS=noreply@mecontrola.app.br
```

**Se algum valor estiver errado:** corrigir na VPS e reiniciar server + worker antes de continuar:
```bash
ssh root@187.77.45.48 \
  "export IMAGE_TAG=local && docker compose \
    --env-file /opt/mecontrola/.env \
    -f /opt/mecontrola/deployment/compose/compose.yml \
    -f /opt/mecontrola/deployment/compose/compose.prod.yml \
    up -d --no-deps --force-recreate server worker"
```

---

## Passo 2 — Criar checkout session (magic token)

**O que faz:** chama o endpoint de checkout como se fosse a landing page. O servidor cria um registro `magic_token` com status `PENDING` no banco e retorna a URL do Kiwify com `?sck=<token>`. O token é o elo entre o pagamento e a ativação — o script do Postman o extrai e salva em `{{checkout_token}}` automaticamente.

### No Postman

Abra a request **"Passo 2 — Checkout Session"** e clique **Send**.

Resultado esperado (status `201`):
```json
{
    "checkout_url": "https://pay.kiwify.com.br/ocPt7sv?sck=CyV1AtPl8k3m...",
    "token_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

Verifique na aba **Test Results** que todos os testes passaram:
- `status 201` ✅
- `checkout_url presente com sck` ✅
- `token extraído — tamanho válido` ✅

Verifique no **Console** do Postman (View → Show Postman Console):
```
Token salvo: CyV1AtPl8k3m...
```

**Se retornar erro:**
- `plan not found` → `ONBOARDING_KIWIFY_CHECKOUT_URLS` não carregou. Reiniciar server na VPS.
- `403` / CORS → confirmar `CORS_ALLOWED_ORIGINS` inclui `https://www.mecontrola.app.br` na VPS.

---

## Passo 3 — Simular webhook order_approved

**O que faz:** monta o payload exato que o Kiwify enviaria após um pagamento aprovado, usando o token gerado no Passo 2. O Pre-request Script do Postman calcula a assinatura HMAC-SHA1 automaticamente e injeta body + signature na request. O servidor valida a assinatura, processa a venda e publica o evento no outbox.

### No Postman

> **Importante:** execute imediatamente após o Passo 2 na mesma sessão — o `{{checkout_token}}` é de coleção e persiste até você limpar.

Abra a request **"Passo 3 — Webhook order_approved"** e clique **Send**.

Resultado esperado (status `200`):
```json
{
    "received": true
}
```

Verifique no **Console** do Postman:
```
Token usado  : CyV1AtPl8k3m...
Assinatura   : a3f7b2c1d4e5f6...
```

**Se retornar erro:**
- `{"error":"invalid signature"}` → `{{kiwify_webhook_secret}}` diverge do configurado na VPS. Verificar no Passo 1.
- `{"error":"plan not found"}` → `product_id` no body não bate com `KIWIFY_PRODUCT_ID_MONTHLY` da VPS.
- `checkout_token vazio` no console → Passo 2 não foi executado nesta sessão. Repetir Passo 2 primeiro.
- `500` sem body → checar logs do server: `ssh root@187.77.45.48 "docker logs mecontrola-server-1 --tail=30"`.

---

## Passo 4 — Verificar processamento do worker

**O que faz:** o worker roda o `DispatcherJob` que consome o outbox. Em até ~30s após o Passo 3, ele processa a venda, marca o token como PAID e dispara o email de ativação. Ruído OTEL (`traces export`, `failed to upload`) é esperado e ignorado.

### Na VPS

```bash
ssh root@187.77.45.48 \
  "timeout 90 docker logs -f mecontrola-worker-1 2>&1 \
  | grep --line-buffered -v 'traces export\|failed to upload\|Unimplemented\|rpc error' \
  | grep --line-buffered -E 'billing|onboarding|token|email|dispatch|error|plan|subscription'"
```

Pressione `Ctrl+C` após ver os logs esperados.

**Logs esperados (em ordem, dentro de ~30s):**
```
INFO billing.usecase.process_sale_approved   subscription_id=sub-test-... plan=MONTHLY
INFO onboarding.usecase.mark_token_paid      token=CyV1AtPl... status=PAID
INFO onboarding.activation_email_dispatched  to=jailton.junior94@outlook.com
```

**Se aparecer:**
- `plan not found` → `product_id` no webhook não bate com os IDs configurados.
- `smtp` / `send_failed` → credenciais SMTP inválidas. Verificar `SMTP_HOST`, `SMTP_USERNAME`, `SMTP_PASSWORD`.
- `funnel_token_missing` → `sck` chegou vazio. Repetir do Passo 2.
- Nenhum log após 90s → worker pode estar parado: `ssh root@187.77.45.48 "docker ps | grep worker"`.

---

## Passo 5 — Verificar email de ativação

**O que faz:** o worker enviou o email via Resend (`smtp.resend.com:587`). O email contém o link com o token — você vai precisar copiar esse token para o Passo 6.

### No seu cliente de email

1. Abra a caixa de **`jailton.junior94@outlook.com`**
2. Procure email de **`noreply@mecontrola.app.br`** com assunto **"Ative sua conta MeControla"**
3. O email contém um link: `https://www.mecontrola.app.br/ativar?token=<TOKEN>`
4. **Copie o valor após `token=`** — você vai usar no Passo 6

> Se não chegar em 2 minutos: verificar pasta **spam/lixo eletrônico**.
> Se ainda não aparecer: revisar os logs do Passo 4 por `activation_email_dispatched` ou `send_failed`.

---

## Passo 6 — Ativar conta no WhatsApp

**O que faz:** o bot recebe a mensagem, busca o token no banco (status PAID), valida, muda para CONSUMED, cria o usuário com o número do remetente e envia a mensagem de boas-vindas.

### No celular

**Do número `+55 11 98689-6322`** (seu número pessoal), envie para **`+55 11 93621-2870`** (bot MeControla):

```
ATIVAR <token-copiado-do-email>
```

Exemplo:
```
ATIVAR CyV1AtPl8k3mNqR2xZbfGjWoLpD4vYuHeTs9cA0
```

> Formatos aceitos: `ATIVAR <token>` (maiúscula ou minúscula) ou `/start ATIVAR_<token>` (deep link).

**Resultado esperado — bot responde em até 10s:**
```
Sua conta foi ativada com sucesso! Bem-vindo ao MeControla.
```

**Se o bot não responder:**
```bash
ssh root@187.77.45.48 \
  "docker logs mecontrola-server-1 --tail=50 2>&1 \
  | grep -E 'whatsapp|activation|HandleActivation|token|consumed|error'"
```

---

## Passo 7 — Confirmar no banco de dados

**O que faz:** valida que todos os registros foram criados corretamente — token consumido, subscription ativa, usuário criado com o número de WhatsApp correto.

### No DBeaver (tunnel mecontrola-db)

1. Abra o DBeaver e conecte-se à conexão **`mecontrola-db`** (tunnel SSH já configurado)
2. Abra um **SQL Editor** (`Ctrl+]` ou botão "Open SQL Script")
3. Execute a query abaixo:

```sql
SELECT
    mt.status                AS token_status,
    mt.activation_path,
    mt.consumed_by_mobile_e164,
    s.plan_id,
    s.status                 AS subscription_status,
    u.whatsapp_number,
    u.created_at             AS user_created_at
FROM mecontrola.magic_tokens mt
LEFT JOIN mecontrola.subscriptions s ON s.id = mt.subscription_id
LEFT JOIN mecontrola.users u ON u.id = mt.consumed_by_user_id
ORDER BY mt.created_at DESC
LIMIT 3;
```

**Resultado esperado:**

| token_status | activation_path | consumed_by_mobile_e164 | plan_id | subscription_status | whatsapp_number | user_created_at |
|---|---|---|---|---|---|---|
| CONSUMED | fallback_e164 | +5511986896322 | MONTHLY | ACTIVE | +5511986896322 | 2026-06-17 ... |

**Se `token_status` for `PAID` (não CONSUMED):** o bot recebeu mas não processou. Ver logs do server no Passo 6.
**Se `subscription_status` for `null`:** o webhook não foi processado pelo worker. Ver Passo 4.
**Se `whatsapp_number` for `null`:** o usuário não foi criado — o token pode ter sido ativado mas algo falhou. Ver logs do server.

### Alternativa — via VPS (se DBeaver não estiver disponível)

```bash
ssh root@187.77.45.48 bash -s << 'EOF'
DB_USER=$(grep "^DB_USER=" /opt/mecontrola/.env | cut -d= -f2)
DB_NAME=$(grep "^DB_NAME=" /opt/mecontrola/.env | cut -d= -f2)

docker exec mecontrola-postgres-1 psql -U "$DB_USER" -d "$DB_NAME" -x -c "
SELECT
    mt.status                AS token_status,
    mt.activation_path,
    mt.consumed_by_mobile_e164,
    s.plan_id,
    s.status                 AS subscription_status,
    u.whatsapp_number,
    u.created_at             AS user_created_at
FROM mecontrola.magic_tokens mt
LEFT JOIN mecontrola.subscriptions s ON s.id = mt.subscription_id
LEFT JOIN mecontrola.users u ON u.id = mt.consumed_by_user_id
ORDER BY mt.created_at DESC
LIMIT 3;
"
EOF
```

---

## Verificação de saúde pós-teste

### No Postman

Re-execute **"Passo 1 — Health Check"** para confirmar que o servidor continua saudável.

### Na VPS

```bash
# Status dos containers
ssh root@187.77.45.48 \
  "docker compose --env-file /opt/mecontrola/.env \
    -f /opt/mecontrola/deployment/compose/compose.yml \
    -f /opt/mecontrola/deployment/compose/compose.prod.yml \
    ps"

# Logs recentes (sem ruído OTEL)
ssh root@187.77.45.48 \
  "docker logs mecontrola-server-1 --tail=20 2>&1 \
  | grep -v 'traces export\|failed to upload\|Unimplemented\|rpc error'"

ssh root@187.77.45.48 \
  "docker logs mecontrola-worker-1 --tail=20 2>&1 \
  | grep -v 'traces export\|failed to upload\|Unimplemented\|rpc error'"
```

---

## Troubleshooting rápido

| Sintoma | Onde investigar | Ação |
|---|---|---|
| Postman: `{"error":"invalid signature"}` | VPS: `grep KIWIFY_WEBHOOK_SECRET /opt/mecontrola/.env` | Confirmar que é `47cyjfb3gag` |
| Postman: `{"error":"plan not found"}` | VPS: `grep KIWIFY_PRODUCT_ID_MONTHLY /opt/mecontrola/.env` | Confirmar que é `2d7d8e25-...` |
| Postman: `checkout_token vazio` no console | — | Executar Passo 2 antes do Passo 3 |
| VPS worker: `funnel_token_missing` | Passo 2 — verificar `sck` no payload | Repetir do Passo 2 |
| VPS worker: `smtp`/`send_failed` | VPS: `grep SMTP /opt/mecontrola/.env` | Verificar credenciais SMTP |
| Email não chega | Pasta spam | Checar logs do worker: `activation_email_dispatched` |
| Bot não responde | VPS: logs do server | `grep -E 'whatsapp\|HandleActivation'` |
| DBeaver: `token_status=PAID` | VPS: logs do server | Checar se a mensagem WhatsApp chegou ao handler |
| DBeaver: `subscription_status=null` | VPS: logs do worker | Worker não processou — ver Passo 4 |

---

## Histórico de correções (2026-06-17)

### Bug 1 — `ErrNestedTransaction` no fluxo de ativação WhatsApp

**Sintoma:** bot não respondia após `ATIVAR <token>`; logs do server mostravam `database: nested transaction not supported`.

**Causa:** `ConsumeMagicToken.Execute` → `uow.Do` (insere tx no ctx) → `handlePaidToken` → `BindAndConsume` → `identityGateway.UpsertUserByWhatsApp` → `UpsertUserByWhatsApp.Execute` → `uow.Do` aninhado → `database.FromContext` detecta tx ativa → `ErrNestedTransaction`.

**Correção:** `UpsertUserByWhatsApp.Execute` verifica `database.FromContext(ctx)` antes de chamar `uow.Do`. Se tx já estiver no contexto, usa-a diretamente via `persistUpsert(ctx, tx, ...)`.

**Arquivo:** `internal/identity/application/usecases/upsert_user_by_whatsapp.go`

### Bug 2 — Nil pointer em `ProcessSubscriptionGraceExpired`

**Sintoma:** worker panic/nil pointer ao executar o job de expiração de graça.

**Causa:** `SubscriptionRepository(nil)` — o campo `db` não era injetado no construtor, resultando em `nil` passado para o driver PostgreSQL.

**Correção:** campo `db database.DBTX` adicionado ao struct; `kiwifyDBTX` (já disponível em `module.go`) passado como segundo parâmetro do construtor — mesmo padrão de `ReconcileSubscriptions`.

**Arquivo:** `internal/billing/application/usecases/process_subscription_grace_expired.go`, `internal/billing/module.go`

### Bug 3 — False-positive warning do outbox para `billing.subscription.activated`

**Sintoma:** logs do server mostravam `WARN outbox.event.missing_aggregate_user_id` para o evento `billing.subscription.activated`, que é publicado antes do usuário ser criado (fluxo de onboarding).

**Correção:** `noUserEventAllowlist` adicionado em `internal/platform/outbox/system_event_allowlist.go`; `billing.subscription.activated` listado como evento que legitimamente não tem `aggregate_user_id`.

**Arquivo:** `internal/platform/outbox/system_event_allowlist.go`, `internal/platform/outbox/outbox.go`

### Bug 4 — Landing page `/ativar` ausente

**Sintoma:** link do email de ativação (`https://www.mecontrola.app.br/ativar?token=...`) retornava 404; `EMAIL_ACTIVATE_URL` apontava para `/ativar` mas apenas `/activate` existia.

**Correção:** `src/pages/ativar.astro` criado com redirect client-side preservando `?token=` via `window.location.search`.

**Repositório:** `LimaTeixeiraTecnologia/mecontrola-landingpage`

### Postman Collection — Pre-request Script corrigido

**Problema:** `pm.collectionVariables.set('webhook_body', body)` não resolvia o `{{webhook_body}}` no body antes do envio; `pm.collectionVariables.set('webhook_sig', sig)` não resolvia `{{webhook_sig}}` na URL.

**Correção:** substituído por `pm.request.body.raw = body` (injeta body diretamente) e `pm.environment.set('webhook_sig', sig)` (resolve a variável na URL via environment).

# E2E Produção — Compra Kiwify → Ativação WhatsApp

**Data:** 2026-06-16
**Ambiente:** VPS 187.77.45.48 (produção)
**Objetivo:** Validar o fluxo completo: compra real no Kiwify → webhook → email de ativação → bot WhatsApp respondendo
**Executado em:** macOS

---

## Fluxo implementado

```
mecontrola.app.br → POST /api/v1/onboarding/checkout
        ↓ cria magic token no banco, retorna URL Kiwify com ?sck=<token>
Kiwify checkout (usuário paga)
        ↓ Kiwify envia order_approved webhook para api.mecontrola.app.br
POST /api/v1/billing/webhooks/kiwify?signature=<hmac-sha1>
        ↓ ProcessSaleApproved: cria subscription, publica evento no outbox
Worker (DispatcherJob)
        ├─ MarkTokenPaid: muda token de PENDING → PAID
        └─ SendActivationEmail: envia email com o token
Usuário recebe email → copia token → envia "ATIVAR <token>" no WhatsApp
        ↓ HandleActivation: valida token PAID → CONSUMED, cria usuário, envia boas-vindas
Bot responde: "Sua conta foi ativada com sucesso! Bem-vindo ao MeControla."
```

**Números:**
- Bot MeControla: `+55 11 93621-2870`
- Número pessoal (cliente de teste): `+55 11 98689-6322`

**⚠️ CRÍTICO:** O fluxo correto obrigatoriamente passa pela landing page (ou pelo curl do Passo 8B). Se o usuário acessar o Kiwify diretamente, o `sck` não estará no webhook e o email de ativação **não** será enviado.

---

## Checklist E2E

- [x] Passo 1 — KIWIFY_PRODUCT_ID_* atualizados na VPS
- [x] Passo 2 — ONBOARDING_KIWIFY_CHECKOUT_URLS configurado na VPS
- [x] Passo 3 — SMTP configurado (Resend, domínio mecontrola.app.br, noreply@mecontrola.app.br)
- [x] Passo 4 — EMAIL_ACTIVATE_URL=https://www.mecontrola.app.br/ativar
- [x] Passo 5 — server e worker reiniciados, health OK
- [x] Passo 6 — logs sem erro de config (ruído OTEL é esperado, não bloqueia)
- [x] Passo 7 — checkout endpoint retorna URL com `?sck=` (testado via curl)
- [x] Landing page — botões chamam POST /api/v1/onboarding/checkout antes de redirecionar
- [x] Cloudflare Pages — PUBLIC_BACKEND_URL=https://api.mecontrola.app.br em produção
- [x] CSP corrigida — scripts movidos para arquivos estáticos, guard anti-regressão no CI
- [x] KIWIFY_WEBHOOK_SECRET=47cyjfb3gag atualizado na VPS (era CHANGE_ME)
- [x] KIWIFY_CLIENT_ID=ee760e58-fce3-4b67-b007-d71c8adb0292 atualizado na VPS
- [x] KIWIFY_ACCOUNT_ID=JhojQYNsUNcvgTV atualizado na VPS
- [x] server e worker reiniciados após secrets — health OK (2026-06-17 10:33)
- [ ] Passo 8B — webhook simulado enviado → `{"received": true}`
- [ ] Passo 9 — worker processou: process_sale_approved + mark_token_paid + activation_email_dispatched
- [ ] Passo 10 — email de ativação recebido em jailton.junior94@outlook.com
- [ ] Passo 11 — `ATIVAR <token>` enviado no WhatsApp, bot respondeu com boas-vindas
- [ ] Passo 12 — banco mostra token CONSUMED e usuário criado

---

## Passo 8B — Simular compra via curl (sem gastar dinheiro)

**O que faz:** cria um magic token via checkout, monta um payload `order_approved` idêntico ao que o Kiwify enviaria, assina com HMAC-SHA1 e envia ao webhook. Isso dispara todo o pipeline downstream sem cartão real.

Execute no **terminal local (macOS)**:

```bash
# 1. Criar checkout session — gera o magic token no banco
#    O servidor retorna a URL do Kiwify com ?sck=<token>
CHECKOUT=$(curl -sf -X POST https://api.mecontrola.app.br/api/v1/onboarding/checkout \
  -H "Content-Type: application/json" \
  -H "Origin: https://www.mecontrola.app.br" \
  -d '{"plan_id": "MONTHLY"}')

# Ver resposta completa formatada
echo "$CHECKOUT" | python3 -m json.tool

# Extrair a URL e o token do ?sck=
# (macOS não tem grep -oP — usamos python3)
CHECKOUT_URL=$(echo "$CHECKOUT" | python3 -c "import sys,json; print(json.load(sys.stdin)['checkout_url'])")
TOKEN=$(echo "$CHECKOUT_URL" | python3 -c "import sys; url=sys.stdin.read().strip(); print(url.split('sck=')[1].split('&')[0])")

echo "Checkout URL: $CHECKOUT_URL"
echo "Token gerado: $TOKEN"

# Garantia: abortar se o token estiver vazio (checkout falhou)
[ -z "$TOKEN" ] && echo "ERRO: TOKEN vazio — checkout falhou. Ver resposta acima." && exit 1
```

Resultado esperado:
```json
{
  "checkout_url": "https://pay.kiwify.com.br/ocPt7sv?sck=<token-40-45-chars>",
  "token_id": "<uuid>"
}
```

```bash
# 2. Montar payload order_approved e calcular assinatura HMAC-SHA1
#    product_id = MONTHLY real da conta Kiwify
#    mobile = seu número pessoal (será o whatsapp_number do usuário criado)
PHONE="+5511986896322"
EMAIL="jailton.junior94@outlook.com"
NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
TS=$(date +%s)

BODY=$(python3 -c "
import json, sys
payload = {
  'order_id': 'test-e2e-$TS',
  'order_ref': 'test-ref-001',
  'order_status': 'paid',
  'webhook_event_type': 'order_approved',
  'subscription_id': 'sub-test-$TS',
  'Product': {
    'product_id': '2d7d8e25-ecfd-45f0-98ba-54a496060959',
    'product_name': 'Me Controla Mensal'
  },
  'Customer': {
    'email': '$EMAIL',
    'mobile': '$PHONE',
    'CPF': ''
  },
  'Subscription': {
    'start_date': '$NOW',
    'next_payment': '2026-07-17T00:00:00Z',
    'status': 'active'
  },
  'TrackingParameters': {
    'sck': '$TOKEN',
    's1': '',
    'src': ''
  },
  'approved_date': '$NOW',
  'updated_at': '$NOW',
  'created_at': '$NOW'
}
print(json.dumps(payload, separators=(',', ':')))
")

# Calcular assinatura HMAC-SHA1 com o webhook secret real
SIG=$(echo -n "$BODY" | openssl dgst -sha1 -hmac "47cyjfb3gag" | awk '{print $2}')

echo "Assinatura: $SIG"
echo "Body: $BODY" | python3 -m json.tool

# 3. Enviar webhook ao servidor
#    ?signature= é validado pelo handler antes de processar
curl -sf -X POST "https://api.mecontrola.app.br/api/v1/billing/webhooks/kiwify?signature=${SIG}" \
  -H "Content-Type: application/json" \
  -d "$BODY" | python3 -m json.tool
```

**Resultado esperado:** `{"received": true}`

**Se retornar erro:**
- `{"error":"invalid signature"}` → o `KIWIFY_WEBHOOK_SECRET` na VPS não bate com `47cyjfb3gag`. Verificar com `grep KIWIFY_WEBHOOK_SECRET /opt/mecontrola/.env` na VPS.
- `{"error":"plan not found"}` → `product_id` no body não corresponde a nenhum `KIWIFY_PRODUCT_ID_*` da VPS.
- `500` sem body → checar logs do server: `ssh root@187.77.45.48 "docker logs mecontrola-server-1 --tail=30 2>&1"`.

---

## Passo 9 — Verificar processamento do worker (logs em tempo real)

**O que faz:** o worker roda um DispatcherJob que consome o outbox. Após o webhook ser aceito, ele deve processar a venda, marcar o token como PAID e disparar o email. Isso acontece em segundos.

```bash
# Acompanhar logs do worker por 90s filtrando o que importa
# (ruído OTEL — "traces export", "failed to upload" — é esperado e ignorado)
ssh root@187.77.45.48 \
  "timeout 90 docker logs -f mecontrola-worker-1 2>&1 \
  | grep --line-buffered -E 'billing|onboarding|token|email|dispatch|error|plan|subscription'"
```

**Logs esperados (em ordem, dentro de ~30s após o Passo 8B):**
```
INFO billing.usecase.process_sale_approved   subscription_id=sub-test-... plan=MONTHLY
INFO onboarding.usecase.mark_token_paid      token=<TOKEN> status=PAID
INFO onboarding.activation_email_dispatched  to=jailton.junior94@outlook.com
```

**Se aparecer:**
- `plan not found` → `product_id` no webhook não bate com os IDs configurados. Confirmar Passo 1.
- `smtp` / `send_failed` → credenciais SMTP com problema. Ver Passo 3 do runbook original.
- `funnel_token_missing` → o campo `sck` não chegou no body — refazer Passo 8B garantindo que `$TOKEN` não está vazio.
- `invalid signature` → secret errado no handler, mas o worker já não veria esse erro (é do server).

---

## Passo 10 — Verificar email de ativação

**O que faz:** o worker enviou o email via Resend (SMTP). Verificar caixa de entrada.

1. Abra `jailton.junior94@outlook.com`
2. Procure email de `noreply@mecontrola.app.br` com assunto **"Ative sua conta MeControla"**
3. O email contém um link: `https://www.mecontrola.app.br/ativar?token=<TOKEN>`
4. **Copie o valor após `token=`** — você vai precisar no Passo 11

> Se não chegar em 2 minutos: verificar pasta **spam/lixo eletrônico**. Se ainda não aparecer, checar os logs do worker (Passo 9) por `activation_email_dispatched` ou `send_failed`.

---

## Passo 11 — Ativar conta no WhatsApp

**O que faz:** o bot valida o token (muda de PAID → CONSUMED), cria o usuário no banco e responde com boas-vindas.

No WhatsApp, **do número `+55 11 98689-6322`**, envie para **`+55 11 93621-2870`**:

```
ATIVAR <token-copiado-do-email>
```

Exemplo:
```
ATIVAR CyV1AtPl8k3mNqR2xZbfGjWoLpD4vYuHeTs9cA0
```

> Formatos aceitos:
> - `ATIVAR <token>` (maiúscula ou minúscula)
> - `/start ATIVAR_<token>` (formato deep link WhatsApp)

**Resultado esperado — bot responde:**
```
Sua conta foi ativada com sucesso! Bem-vindo ao MeControla.
```

**Se o bot não responder em 30s:**
```bash
# Verificar se a mensagem chegou e foi processada
ssh root@187.77.45.48 \
  "docker logs mecontrola-server-1 --tail=50 2>&1 \
  | grep -E 'whatsapp|activation|HandleActivation|token|consumed'"
```

---

## Passo 12 — Confirmar no banco de dados

**O que faz:** valida que o registro está íntegro — token consumido, subscription ativa, usuário criado com o número correto.

```bash
ssh root@187.77.45.48 bash -s << 'EOF'
DB_USER=$(grep "^DB_USER=" /opt/mecontrola/.env | cut -d= -f2)
DB_NAME=$(grep "^DB_NAME=" /opt/mecontrola/.env | cut -d= -f2)

docker exec mecontrola-postgres-1 psql -U "$DB_USER" -d "$DB_NAME" -x -c "
SELECT
  mt.status            AS token_status,
  mt.activation_path,
  mt.consumed_by_mobile_e164,
  s.plan_id,
  s.status             AS subscription_status,
  u.whatsapp_number,
  u.created_at         AS user_created_at
FROM mecontrola.magic_tokens mt
LEFT JOIN mecontrola.subscriptions s ON s.id = mt.subscription_id
LEFT JOIN mecontrola.users u ON u.id = mt.consumed_by_user_id
ORDER BY mt.created_at DESC
LIMIT 3;
"
EOF
```

**Resultado esperado:**
```
-[ RECORD 1 ]------------+-------------------------
token_status             | CONSUMED
activation_path          | fallback_e164
consumed_by_mobile_e164  | +5511986896322
plan_id                  | MONTHLY
subscription_status      | ACTIVE
whatsapp_number          | +5511986896322
user_created_at          | 2026-06-17 ...
```

---

## Verificação de saúde pós-teste

```bash
# Health da API
curl -sf https://api.mecontrola.app.br/health | python3 -m json.tool

# Status dos containers
ssh root@187.77.45.48 \
  "docker compose --env-file /opt/mecontrola/.env \
    -f /opt/mecontrola/deployment/compose/compose.yml \
    -f /opt/mecontrola/deployment/compose/compose.prod.yml \
    ps"

# Logs recentes sem ruído OTEL
ssh root@187.77.45.48 \
  "docker logs mecontrola-server-1 --tail=20 2>&1 \
  | grep -v 'traces export\|failed to upload\|Unimplemented\|rpc error'"

ssh root@187.77.45.48 \
  "docker logs mecontrola-worker-1 --tail=20 2>&1 \
  | grep -v 'traces export\|failed to upload\|Unimplemented\|rpc error'"
```

---

## Referência rápida

| Variável | Valor |
|---|---|
| `KIWIFY_WEBHOOK_SECRET` | `47cyjfb3gag` |
| `KIWIFY_PRODUCT_ID_MONTHLY` | `2d7d8e25-ecfd-45f0-98ba-54a496060959` |
| `KIWIFY_PRODUCT_ID_QUARTERLY` | `c2c2ec27-18d4-4bff-a551-ab5f98a78eb5` |
| `KIWIFY_PRODUCT_ID_ANNUAL` | `abaac314-0ab6-4474-aeab-aca498cb8c4a` |
| Checkout MONTHLY | `https://pay.kiwify.com.br/ocPt7sv` |
| Checkout QUARTERLY | `https://pay.kiwify.com.br/Sh2upAU` |
| Checkout ANNUAL | `https://pay.kiwify.com.br/HquleKA` |
| Bot MeControla | `+55 11 93621-2870` |
| Número pessoal (cliente de teste) | `+55 11 98689-6322` |
| Email de teste | `jailton.junior94@outlook.com` |
| VPS | `root@187.77.45.48` |

```bash
# Alias útil na VPS
alias mc='docker compose --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml'

# Logs limpos
mc logs -f server worker 2>&1 \
  | grep -v "traces export\|failed to upload\|Unimplemented\|rpc error"

# Restart rápido
export IMAGE_TAG=local && mc up -d --no-deps --force-recreate server worker
```

---

## Riscos e observações

| Risco | Impacto | Mitigação |
|---|---|---|
| Email cai no spam | Bloqueia ativação | Verificar pasta spam; domínio mecontrola.app.br verificado no Resend |
| `sck` ausente no webhook Kiwify real | Email não enviado, bot silencioso | Confirmar que a landing page chama `/api/v1/onboarding/checkout` antes do redirect |
| Token expirado (TTL 7 dias) | `code_expired_contact_support` | Usar token recém-gerado |
| OTEL TLS errors nos logs | Ruído (não bloqueia) | Ignorar — coletor local sem TLS configurado |
| App Meta em desenvolvimento | Não bloqueia | Campo `messages` assinado, número real inscrito |

---

## Histórico de correções (2026-06-16/17)

### Secrets Kiwify (2026-06-17)
`KIWIFY_WEBHOOK_SECRET`, `KIWIFY_CLIENT_ID` e `KIWIFY_ACCOUNT_ID` estavam como `CHANGE_ME` na VPS. Atualizados com os valores reais do `.env` local e server+worker reiniciados.

### 4 bugs no fluxo de ativação WhatsApp (2026-06-17)

Descobertos durante a sessão de E2E produção pós-configuração dos secrets. Todos corrigidos antes do re-teste:

- **Bug 1 — `ErrNestedTransaction`:** `UpsertUserByWhatsApp.Execute` chamava `uow.Do` dentro de um `txCtx` que já continha uma transação ativa. Corrigido com `database.FromContext(ctx)` — se tx no ctx, usa diretamente sem novo `uow.Do`. (`internal/identity/application/usecases/upsert_user_by_whatsapp.go`)
- **Bug 2 — Nil pointer em `ProcessSubscriptionGraceExpired`:** `SubscriptionRepository(nil)` por falta de `db` no construtor. Corrigido injetando `kiwifyDBTX` — mesmo padrão de `ReconcileSubscriptions`. (`internal/billing/module.go`, `process_subscription_grace_expired.go`)
- **Bug 3 — False-positive warning outbox:** `billing.subscription.activated` é publicado antes do usuário existir; o outbox emitia `WARN missing_aggregate_user_id`. Corrigido com `noUserEventAllowlist`. (`internal/platform/outbox/`)
- **Bug 4 — Landing page `/ativar` ausente:** `EMAIL_ACTIVATE_URL` apontava para `/ativar` mas apenas `/activate` existia. Criado `src/pages/ativar.astro` com redirect preservando `?token=`. (`LimaTeixeiraTecnologia/mecontrola-landingpage`)

**Commits mecontrola:** `000d623` (checkout bugs + frente B + deploy) — inclui os 4 fixes acima.

### 3 bugs no checkout no browser (2026-06-16)
Causa: **3 bugs encadeados**, todos corrigidos:
- **Bug 1 — CORS:** `CORS_ALLOWED_ORIGINS` não incluía `www.mecontrola.app.br` → `OPTIONS 403`. Corrigido no `.env` da VPS.
- **Bug 2 — Fallback silencioso:** em qualquer falha o botão redirecionava ao Kiwify **sem `?sck=`**. Removido o fallback; agora exibe erro visível e não redireciona.
- **Bug 3 — CSP:** `connect-src` não listava `https://api.mecontrola.app.br` + handler inline bloqueado pelo browser. Scripts movidos para arquivos estáticos (`/js/checkout.js`, `/js/cookie-consent.js`, `/js/activate.js`). Guard `scripts/verify-no-inline-scripts.mjs` adicionado ao CI.

**Repositório landing:** `LimaTeixeiraTecnologia/mecontrola-landingpage` (commits `5681ef0`, `026447f`, `6b72476`, `dc04f1b`).

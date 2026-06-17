# E2E Produção — Compra Kiwify → Ativação WhatsApp

**Data:** 2026-06-16
**Ambiente:** VPS 187.77.45.48 (staging/produção)
**Objetivo:** Validar o fluxo completo: compra real no Kiwify → webhook → email de ativação → bot WhatsApp respondendo

---

## Fluxo implementado (o que o código faz)

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

**⚠️ CRÍTICO:** Se o usuário acessar o Kiwify diretamente (sem passar pelo checkout da API), o `sck` não estará no webhook e o email de ativação NÃO será enviado. O fluxo correto obrigatoriamente passa pela landing page.

---

## Estado atual da VPS (pré-condições confirmadas)

- [x] Docker, postgres, pgbouncer, server, worker, caddy — todos healthy
- [x] `curl https://api.mecontrola.app.br/health` → `{"status":"healthy"}`
- [x] Meta webhook verificado (`messages` assinado, toggle ON)
- [x] `META_PHONE_NUMBER_ID=1223374060850702` e `META_ACCESS_TOKEN` com token permanente
- [ ] `KIWIFY_PRODUCT_ID_*` — ainda `CHANGE_ME_*` → **Passo 1**
- [ ] `ONBOARDING_KIWIFY_CHECKOUT_URLS` — vazio → **Passo 2**
- [ ] `SMTP_HOST/PORT/USER/PASS` — não configurado → **Passo 3**
- [ ] `EMAIL_ACTIVATE_URL` — usa default `localhost` → **Passo 4**

---

## Passo 1 — Atualizar KIWIFY_PRODUCT_IDs

IDs das subscriptions do Kiwify (confirmados via API):

```bash
ssh root@187.77.45.48

sed -i 's|KIWIFY_PRODUCT_ID_MONTHLY=.*|KIWIFY_PRODUCT_ID_MONTHLY=2d7d8e25-ecfd-45f0-98ba-54a496060959|' /opt/mecontrola/.env
sed -i 's|KIWIFY_PRODUCT_ID_QUARTERLY=.*|KIWIFY_PRODUCT_ID_QUARTERLY=c2c2ec27-18d4-4bff-a551-ab5f98a78eb5|' /opt/mecontrola/.env
sed -i 's|KIWIFY_PRODUCT_ID_ANNUAL=.*|KIWIFY_PRODUCT_ID_ANNUAL=abaac314-0ab6-4474-aeab-aca498cb8c4a|' /opt/mecontrola/.env

# Verificar
grep "KIWIFY_PRODUCT_ID" /opt/mecontrola/.env
```

Resultado esperado:
```
KIWIFY_PRODUCT_ID_MONTHLY=2d7d8e25-ecfd-45f0-98ba-54a496060959
KIWIFY_PRODUCT_ID_QUARTERLY=c2c2ec27-18d4-4bff-a551-ab5f98a78eb5
KIWIFY_PRODUCT_ID_ANNUAL=abaac314-0ab6-4474-aeab-aca498cb8c4a
```

---

## Passo 2 — Configurar ONBOARDING_KIWIFY_CHECKOUT_URLS

Mapeia o código do plano (`MONTHLY`, `QUARTERLY`, `ANNUAL`) para a URL de checkout do Kiwify. O token será injetado como `?sck=<token>`.

URLs dos planos (link IDs do Kiwify):
- Mensal: `ocPt7sv` → `https://pay.kiwify.com.br/ocPt7sv`
- Trimestral: `Sh2upAU` → `https://pay.kiwify.com.br/Sh2upAU`
- Anual: `HquleKA` → `https://pay.kiwify.com.br/HquleKA`

```bash
ssh root@187.77.45.48

# Adicionar/atualizar no .env (formato: CHAVE=URL separado por ;)
grep -q "^ONBOARDING_KIWIFY_CHECKOUT_URLS=" /opt/mecontrola/.env \
  && sed -i 's|^ONBOARDING_KIWIFY_CHECKOUT_URLS=.*|ONBOARDING_KIWIFY_CHECKOUT_URLS=MONTHLY=https://pay.kiwify.com.br/ocPt7sv;QUARTERLY=https://pay.kiwify.com.br/Sh2upAU;ANNUAL=https://pay.kiwify.com.br/HquleKA|' /opt/mecontrola/.env \
  || echo 'ONBOARDING_KIWIFY_CHECKOUT_URLS=MONTHLY=https://pay.kiwify.com.br/ocPt7sv;QUARTERLY=https://pay.kiwify.com.br/Sh2upAU;ANNUAL=https://pay.kiwify.com.br/HquleKA' >> /opt/mecontrola/.env

# Verificar
grep "ONBOARDING_KIWIFY_CHECKOUT_URLS" /opt/mecontrola/.env
```

---

## Passo 3 — Configurar SMTP para email de ativação

### Opção A — Resend (recomendado, gratuito, 3.000 emails/mês)

1. Acesse **resend.com** → crie conta → **Add Domain** ou use o domínio de teste
2. Vá em **API Keys** → crie uma chave
3. Resend SMTP:
   - Host: `smtp.resend.com`
   - Port: `587`
   - Username: `resend`
   - Password: `<sua-api-key>`

```bash
ssh root@187.77.45.48

# Substituir pelos valores reais
SMTP_HOST="smtp.resend.com"
SMTP_PORT="587"
SMTP_USER="resend"
SMTP_PASS="<sua-api-key-resend>"
EMAIL_FROM="noreply@mecontrola.app.br"  # ou o domínio verificado no Resend
EMAIL_NAME="MeControla"
EMAIL_REPLY="suporte@mecontrola.app.br"

grep -q "^SMTP_HOST=" /opt/mecontrola/.env \
  && sed -i "s|^SMTP_HOST=.*|SMTP_HOST=${SMTP_HOST}|" /opt/mecontrola/.env \
  || echo "SMTP_HOST=${SMTP_HOST}" >> /opt/mecontrola/.env

grep -q "^SMTP_PORT=" /opt/mecontrola/.env \
  && sed -i "s|^SMTP_PORT=.*|SMTP_PORT=${SMTP_PORT}|" /opt/mecontrola/.env \
  || echo "SMTP_PORT=${SMTP_PORT}" >> /opt/mecontrola/.env

grep -q "^SMTP_USERNAME=" /opt/mecontrola/.env \
  && sed -i "s|^SMTP_USERNAME=.*|SMTP_USERNAME=${SMTP_USER}|" /opt/mecontrola/.env \
  || echo "SMTP_USERNAME=${SMTP_USER}" >> /opt/mecontrola/.env

grep -q "^SMTP_PASSWORD=" /opt/mecontrola/.env \
  && sed -i "s|^SMTP_PASSWORD=.*|SMTP_PASSWORD=${SMTP_PASS}|" /opt/mecontrola/.env \
  || echo "SMTP_PASSWORD=${SMTP_PASS}" >> /opt/mecontrola/.env

grep -q "^SMTP_STARTTLS=" /opt/mecontrola/.env \
  && sed -i "s|^SMTP_STARTTLS=.*|SMTP_STARTTLS=true|" /opt/mecontrola/.env \
  || echo "SMTP_STARTTLS=true" >> /opt/mecontrola/.env

grep -q "^EMAIL_FROM_ADDRESS=" /opt/mecontrola/.env \
  && sed -i "s|^EMAIL_FROM_ADDRESS=.*|EMAIL_FROM_ADDRESS=${EMAIL_FROM}|" /opt/mecontrola/.env \
  || echo "EMAIL_FROM_ADDRESS=${EMAIL_FROM}" >> /opt/mecontrola/.env

grep -q "^EMAIL_FROM_NAME=" /opt/mecontrola/.env \
  && sed -i "s|^EMAIL_FROM_NAME=.*|EMAIL_FROM_NAME=${EMAIL_NAME}|" /opt/mecontrola/.env \
  || echo "EMAIL_FROM_NAME=${EMAIL_NAME}" >> /opt/mecontrola/.env

grep -q "^EMAIL_REPLY_TO=" /opt/mecontrola/.env \
  && sed -i "s|^EMAIL_REPLY_TO=.*|EMAIL_REPLY_TO=${EMAIL_REPLY}|" /opt/mecontrola/.env \
  || echo "EMAIL_REPLY_TO=${EMAIL_REPLY}" >> /opt/mecontrola/.env

# Verificar
grep -E "^SMTP_|^EMAIL_FROM|^EMAIL_REPLY" /opt/mecontrola/.env
```

### Opção B — Gmail com App Password

1. Conta Google → **Segurança** → **Verificação em duas etapas** (ativar)
2. **Senhas de apps** → criar senha para "Aplicativo: Outro (MeControla)"
3. Use os valores abaixo:

```
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=seu@gmail.com
SMTP_PASSWORD=<senha-de-app-16-chars>
SMTP_STARTTLS=true
EMAIL_FROM_ADDRESS=seu@gmail.com
```

---

## Passo 4 — Configurar EMAIL_ACTIVATE_URL

Para o teste, o email conterá um link `<EMAIL_ACTIVATE_URL>?token=<token>`. O usuário extrai o token e envia `ATIVAR <token>` para o bot.

Para o teste de produção, use a landing page:

```bash
ssh root@187.77.45.48

grep -q "^EMAIL_ACTIVATE_URL=" /opt/mecontrola/.env \
  && sed -i "s|^EMAIL_ACTIVATE_URL=.*|EMAIL_ACTIVATE_URL=https://www.mecontrola.app.br/ativar|" /opt/mecontrola/.env \
  || echo "EMAIL_ACTIVATE_URL=https://www.mecontrola.app.br/ativar" >> /opt/mecontrola/.env

# Verificar
grep "EMAIL_ACTIVATE_URL" /opt/mecontrola/.env
```

> Se a landing page não tiver a rota `/ativar`, deixe em branco — o email vai conter o token diretamente, e o usuário envia `ATIVAR <token>` no WhatsApp.

---

## Passo 5 — Reiniciar server e worker

```bash
ssh root@187.77.45.48

export IMAGE_TAG=local
docker compose --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  up -d --no-deps --force-recreate server worker

# Aguardar estabilização (~15s) e verificar
sleep 15
docker compose --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  ps

curl -sf https://api.mecontrola.app.br/health | python3 -m json.tool
```

Resultado esperado:
```json
{"status": "healthy"}
```

---

## Passo 6 — Verificar configuração carregada (logs de startup)

```bash
ssh root@187.77.45.48

# Verificar que server e worker carregaram sem erro de config
docker logs mecontrola-server-1 2>&1 | grep -E "error|SMTP|smtp|kiwify|checkout|plan" | head -20
docker logs mecontrola-worker-1 2>&1 | grep -E "error|billing|webhook|plan|kiwify" | head -20
```

Não deve aparecer `plan not found`, `smtp host vazio`, `checkout unavailable` ou similar.

---

## Passo 7 — Testar endpoint de checkout (smoke check)

Verifica se a API cria magic token e retorna URL do Kiwify com `?sck=`:

```bash
curl -sf -X POST https://api.mecontrola.app.br/api/v1/onboarding/checkout \
  -H "Content-Type: application/json" \
  -H "Origin: https://www.mecontrola.app.br" \
  -d '{"plan_id": "MONTHLY"}' | python3 -m json.tool
```

Resultado esperado:
```json
{
  "checkout_url": "https://pay.kiwify.com.br/ocPt7sv?sck=<token-de-40-45-chars>",
  "token_id": "<uuid>"
}
```

**Se retornar erro:**
- `plan not found` → ONBOARDING_KIWIFY_CHECKOUT_URLS não carregou — reiniciar server
- `checkout unavailable` → URL inválida ou host não permitido em ONBOARDING_KIWIFY_ALLOWED_HOSTS
- `CORS error` → origin não está em ONBOARDING_CHECKOUT_CORS_ORIGINS (já configurado)

---

## Passo 8 — Compra real no Kiwify

### 8A — Via landing page (fluxo correto com magic token)

1. Acesse **https://www.mecontrola.app.br** no celular ou browser
2. Clique no botão do plano desejado (ex: "Assinar Mensal")
3. Confirme que a URL do Kiwify tem `?sck=` (ex: `https://pay.kiwify.com.br/ocPt7sv?sck=abc123...`)
4. Complete o pagamento com cartão real ou use o plano "Teste" (R$ 5,00 — link `ix6YIk3`)
5. Anote o email usado no cadastro

### 8B — Via curl (simula landing page, para teste sem gastar dinheiro)

```bash
# 1. Criar checkout session (gera magic token)
CHECKOUT=$(curl -sf -X POST https://api.mecontrola.app.br/api/v1/onboarding/checkout \
  -H "Content-Type: application/json" \
  -H "Origin: https://www.mecontrola.app.br" \
  -d '{"plan_id": "MONTHLY"}')

echo "$CHECKOUT" | python3 -m json.tool
CHECKOUT_URL=$(echo "$CHECKOUT" | python3 -c "import sys,json; print(json.load(sys.stdin)['checkout_url'])")
TOKEN=$(echo "$CHECKOUT_URL" | grep -oP '(?<=sck=)[^&]+')
echo "Token gerado: $TOKEN"

# 2. Simular webhook Kiwify order_approved com o token real
PHONE="+55119XXXXXXX6322"  # ← seu número real com DDI
EMAIL="jailton.junior94@outlook.com"

BODY="{\"order_id\":\"test-e2e-$(date +%s)\",\"order_ref\":\"test-ref-001\",\"order_status\":\"paid\",\"webhook_event_type\":\"order_approved\",\"subscription_id\":\"sub-test-$(date +%s)\",\"Product\":{\"product_id\":\"2d7d8e25-ecfd-45f0-98ba-54a496060959\",\"product_name\":\"Me Controla Mensal\"},\"Customer\":{\"email\":\"${EMAIL}\",\"mobile\":\"${PHONE}\",\"CPF\":\"\"},\"Subscription\":{\"start_date\":\"2026-06-16T00:00:00Z\",\"next_payment\":\"2026-07-16T00:00:00Z\",\"status\":\"active\"},\"TrackingParameters\":{\"sck\":\"${TOKEN}\",\"s1\":\"\",\"src\":\"\"},\"approved_date\":\"2026-06-16T00:00:00Z\",\"updated_at\":\"2026-06-16T00:00:00Z\",\"created_at\":\"2026-06-16T00:00:00Z\"}"

SIG=$(echo -n "$BODY" | openssl dgst -sha1 -hmac "47cyjfb3gag" | awk '{print $2}')

curl -sf -X POST "https://api.mecontrola.app.br/api/v1/billing/webhooks/kiwify?signature=${SIG}" \
  -H "Content-Type: application/json" \
  -d "$BODY" | python3 -m json.tool
```

Resultado esperado: `{"received": true}`

---

## Passo 9 — Verificar processamento do webhook (logs do worker)

```bash
ssh root@187.77.45.48

# Acompanhar logs do worker em tempo real por 60s
timeout 60 docker logs -f mecontrola-worker-1 2>&1 | grep -E "billing|onboarding|token|email|dispatch|error"
```

Logs esperados (ordem):
```
INFO billing.usecase.process_sale_approved ...
INFO onboarding.usecase.mark_token_paid ...
INFO onboarding.activation_email_dispatched ...
```

**Se aparecer `plan not found`:** KIWIFY_PRODUCT_ID_MONTHLY não corresponde ao `product_id` do webhook. Confirme que o webhook usa o subscription ID correto.

**Se aparecer `smtp` error:** Verificar credenciais SMTP no Passo 3.

**Se aparecer `funnel_token_missing`:** O token `sck` não chegou no webhook — confirmar que o curl do Passo 8B usou a variável `$TOKEN` corretamente.

---

## Passo 10 — Verificar email recebido

1. Acesse a caixa de entrada do email usado na compra
2. Procure por email de: `noreply@mecontrola.app.br` com assunto: **"Ative sua conta MeControla"**
3. O email contém o link de ativação: `https://www.mecontrola.app.br/ativar?token=<TOKEN>`
4. Copie o valor do parâmetro `token=` da URL

> Se o email não chegar em 2 minutos, verificar spam. Se ainda não aparecer, checar logs do worker (Passo 9) para `email_dispatched` ou `send_failed`.

---

## Passo 11 — Ativar conta no WhatsApp

No WhatsApp, envie para **+55 11 93621-2870**:

```
ATIVAR <token-copiado-do-email>
```

Exemplo:
```
ATIVAR abc123def456ghi789jkl012mno345pqr678stu90
```

> Formatos aceitos:
> - `ATIVAR <token>` (maiúscula ou minúscula)
> - `/start ATIVAR_<token>` (formato deep link WhatsApp)

**Resultado esperado:** Bot responde com:
```
Sua conta foi ativada com sucesso! Bem-vindo ao MeControla.
```

---

## Passo 12 — Verificar no banco de dados

```bash
ssh root@187.77.45.48

DB_USER=$(grep "^DB_USER=" /opt/mecontrola/.env | cut -d= -f2)
DB_NAME=$(grep "^DB_NAME=" /opt/mecontrola/.env | cut -d= -f2)

docker exec -it mecontrola-postgres-1 psql -U "$DB_USER" -d "$DB_NAME" -c "
SELECT
  mt.status,
  mt.activation_path,
  mt.consumed_by_mobile_e164,
  s.plan_id,
  s.status AS subscription_status,
  u.whatsapp_number
FROM mecontrola.magic_tokens mt
LEFT JOIN mecontrola.subscriptions s ON s.id = mt.subscription_id
LEFT JOIN mecontrola.users u ON u.id = mt.consumed_by_user_id
ORDER BY mt.created_at DESC
LIMIT 5;
"
```

Resultado esperado:
```
 status   | activation_path | consumed_by_mobile_e164 | plan_id  | subscription_status | whatsapp_number
----------+-----------------+-------------------------+----------+---------------------+-----------------
 CONSUMED | fallback_e164   | +5511XXXXXXX6322        | MONTHLY  | ACTIVE              | +5511XXXXXXX6322
```

---

## Verificação rápida de saúde pós-teste

```bash
ssh root@187.77.45.48

# Health da API
curl -sf https://api.mecontrola.app.br/health | python3 -m json.tool

# Containers
docker compose --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  ps

# Últimos logs server (sem ruído OTEL)
docker logs mecontrola-server-1 --tail=20 2>&1 | grep -v "traces export\|failed to upload\|Unimplemented"

# Últimos logs worker
docker logs mecontrola-worker-1 --tail=20 2>&1 | grep -v "traces export\|failed to upload\|Unimplemented"
```

---

## Checklist E2E

- [x] Passo 1 — KIWIFY_PRODUCT_ID_* atualizados
- [x] Passo 2 — ONBOARDING_KIWIFY_CHECKOUT_URLS configurado
- [x] Passo 3 — SMTP configurado (Resend, domínio mecontrola.app.br verificado, noreply@mecontrola.app.br)
- [x] Passo 4 — EMAIL_ACTIVATE_URL=https://www.mecontrola.app.br/ativar
- [x] Passo 5 — server e worker reiniciados, health OK
- [x] Passo 6 — logs sem erro de config (apenas ruído OTEL — esperado)
- [x] Passo 7 — checkout endpoint retorna URL com `?sck=` (testado via curl, OK)
- [x] Landing page corrigida — botões agora chamam POST /api/v1/onboarding/checkout antes de redirecionar (build OK)
- [x] Cloudflare Pages — PUBLIC_BACKEND_URL=https://api.mecontrola.app.br configurado em Produção
- [x] Deploy landing page — via GitHub Actions (push → main dispara `Deploy to Cloudflare Pages`)
- [x] Verificar botão no browser — confirmado `?sck=` na URL do Kiwify no navegador real (ver "Atualização 2026-06-16")
- [ ] Passo 8 — compra efetuada (real ou simulada via curl) — **bloqueado**: `KIWIFY_WEBHOOK_SECRET=CHANGE_ME` na VPS
- [ ] Passo 9 — worker processou webhook (billing + email nos logs)
- [ ] Passo 10 — email de ativação recebido na caixa de entrada
- [ ] Passo 11 — `ATIVAR <token>` enviado, bot respondeu com boas-vindas
- [ ] Passo 12 — banco mostra token CONSUMED e usuário criado

---

## Riscos e observações

| Risco | Impacto | Mitigação |
|-------|---------|-----------|
| Email cai no spam | Bloqueia ativação | Verificar pasta spam; usar domínio verificado no Resend |
| `sck` ausente no webhook Kiwify real | Email não enviado, bot silencioso | Confirmar que a landing page chama `/api/v1/onboarding/checkout` antes do redirect |
| Token expirado (TTL 7 dias) | `code_expired_contact_support` | Usar token recém-gerado; TTL configurado em `ONBOARDING_TOKEN_TTL_DAYS=7` |
| OTEL TLS errors nos logs | Ruído (não bloqueia) | Ignorar — são logs do coletor local sem TLS |
| App Meta em desenvolvimento | ✅ Não bloqueia | Campo `messages` já assinado e o número real está inscrito |

---

## Atualização 2026-06-16 — Correção do checkout no browser (3 bugs encadeados) + Frente B (CSP)

A camada **pré-pagamento** (clicar no plano → ir ao Kiwify **com `?sck=`**) estava quebrada no
navegador real, apesar de funcionar via `curl`. Causa: **3 bugs encadeados**, cada um mascarando o
próximo. Todos corrigidos, com deploy e verificação no site vivo.

### Bug 1 — CORS rejeitava `www.mecontrola.app.br`
- **Sintoma:** preflight `OPTIONS /api/v1/onboarding/checkout` retornava `403`.
- **Causa:** `CORS_ALLOWED_ORIGINS` na VPS não incluía o domínio `www`.
- **Correção (VPS `/opt/mecontrola/.env`):**
  `CORS_ALLOWED_ORIGINS=https://www.mecontrola.app.br,https://mecontrola.app.br` + restart do server.
- **Durabilidade:** `deployment/scripts/deploy.sh` reusa `--env-file /opt/mecontrola/.env`; um novo
  deploy de imagem **não** sobrescreve o `.env`. Fix persiste.
- **Evidência:** `OPTIONS → 204` + `access-control-allow-origin` correto; `POST` com `Origin → 201`.

### Bug 2 — Fallback silencioso para o Kiwify sem token
- **Sintoma:** em qualquer falha do checkout o botão redirecionava para `pay.kiwify.com.br/...`
  **sem `?sck=`** — cliente pagava e **nunca** recebia email de ativação (falha invisível).
- **Correção (landing `Pricing.astro`):** removido o fallback token-less; agora exibe erro visível
  (`role="alert"`) e não redireciona sem `checkout_url` válido.

### Bug 3 — CSP bloqueava o checkout no browser (a peça que faltava)
`curl` sempre funcionou porque **CSP só é aplicada pelo navegador**. Dois sub-bugs no
`public/_headers`:
- **3a — `connect-src`:** não listava `https://api.mecontrola.app.br` → o `fetch` do checkout era
  bloqueado. Adicionado ao `connect-src`.
- **3b — `script-src 'self'` (sem `unsafe-inline`/hash):** o handler era um `<script>` **inline**
  (gerado pelo `define:vars` do Astro). O browser **recusava executá-lo** → o clique não era
  interceptado e o `<a href>` cru ia ao Kiwify sem token. **Correção:** mover o handler para
  `public/js/checkout.js` (arquivo estático, mesmo domínio, coberto por `'self'`); backend lido via
  `data-backend-url` no DOM.
- **Evidência (browser real):** URL final `pay.kiwify.com.br/ocPt7sv?sck=CyV1AtPl...` ✅.

### Frente B — Mesma causa raiz (CSP) em outros scripts inline
- **Consent banner (LGPD/GA):** `cookie-consent` era inline → bloqueado (aceitar/recusar sem efeito,
  GA não carregava). Movido para `public/js/cookie-consent.js`.
- **Página `/activate`:** o script (aberto pelo link do email) era inline → spinner infinito. Movido
  para `public/js/activate.js`.
- **Guard anti-regressão:** `scripts/verify-no-inline-scripts.mjs` roda no `build` e em step
  dedicado no `deploy.yml` — **falha** se `dist/` tiver qualquer `<script>` executável inline.
  Converte essa classe de bug silencioso em erro de build alto.
- **Limpeza:** removidos órfãos `src/lib/api.ts` e `src/scripts/cookie-consent.ts`.

### Validação automatizada da camada pré-pagamento (sem falso positivo)
- 3 planos: `OPTIONS 204` → `POST 201` + header CORS + `?sck=` válido, batendo com os link IDs
  corretos (`ocPt7sv`/`Sh2upAU`/`HquleKA`).
- **Negativo de CORS:** origem maliciosa → `403`, sem header ecoado (não é allow-all).
- **Persistência:** token criado responde no state endpoint como `ready_to_activate:false` (PENDING
  real no banco).
- 3 estáticos (`/js/checkout.js`, `/js/cookie-consent.js`, `/js/activate.js`) servidos `200` do
  mesmo domínio; `0` scripts inline executáveis em todas as páginas; guard passou no CI.

**Repositório:** `LimaTeixeiraTecnologia/mecontrola-landingpage` (commits `5681ef0`, `026447f`,
`6b72476`, `dc04f1b`).

---

## Próximos passos

### Bloqueador da Frente A (caminho do dinheiro) — secrets do Kiwify na VPS
`/opt/mecontrola/.env` ainda tem **placeholders** que impedem o webhook real e a reconciliação:

```
KIWIFY_WEBHOOK_SECRET=CHANGE_ME_generate_secure_webhook_secret   # assinatura do webhook
KIWIFY_CLIENT_ID=CHANGE_ME_generate_secure_client_id             # OAuth API Kiwify
KIWIFY_ACCOUNT_ID=CHANGE_ME_generate_secure_account_id           # conta Kiwify
```

Sem o `KIWIFY_WEBHOOK_SECRET` real (o valor configurado no painel do Kiwify), o `order_approved`
real será rejeitado pela validação de assinatura. **Ação:** obter os valores reais no painel do
Kiwify, atualizar o `.env` e reiniciar server + worker.

### Caminho do dinheiro (Passo 8B→12) — pendente, fazer após resolver o secret
1. **Passo 8B (simulado, sem gastar):** gerar token via checkout, montar `order_approved` assinado
   com o `KIWIFY_WEBHOOK_SECRET` real, `POST` ao webhook → esperar `{"received": true}`.
2. **Passo 9:** acompanhar logs do worker (`process_sale_approved`, `mark_token_paid`,
   `activation_email_dispatched`).
3. **Passo 10:** confirmar email de ativação na caixa de entrada.
4. **Passo 11:** abrir `/activate?token=...` (validar agora que o JS externo carrega) ou enviar
   `ATIVAR <token>` no WhatsApp → bot responde boas-vindas.
5. **Passo 12:** banco mostra token `CONSUMED` + usuário criado.
6. **Por último (gasto real):** compra de verdade via landing → Kiwify, ponta a ponta.

### Validação manual recomendada da Frente B (browser)
- `/` → banner de cookies aparece; "Aceitar" carrega GA; "Recusar" some sem GA.
- `/activate?token=<token-pendente>` → não fica no spinner; mostra erro tratado (token ainda não
  pago) ou os botões WhatsApp/Telegram (token pronto).

---

## Referência — Comandos úteis na VPS

```bash
# Alias
alias mc='docker compose --env-file /opt/mecontrola/.env -f /opt/mecontrola/deployment/compose/compose.yml -f /opt/mecontrola/deployment/compose/compose.prod.yml'

# Logs sem ruído OTEL
mc logs -f server worker 2>&1 | grep -v "traces export\|failed to upload\|Unimplemented\|rpc error"

# Restart rápido
export IMAGE_TAG=local && mc up -d --no-deps --force-recreate server worker

# Ver .env atual
grep -E "^KIWIFY|^ONBOARDING_KIWIFY|^SMTP_|^EMAIL_" /opt/mecontrola/.env
```

# Setup: WhatsApp Business Cloud API (Meta)

**Última revisão:** 2026-06-15
**Status do MeControla:** ⚠️ aguardando aprovação do número Business pela Meta. Sandbox de
validação MVP é Telegram (ver `docs/integrations/telegram-setup.md` — TODO).

---

## Visão Geral

O MeControla usa a **WhatsApp Business Cloud API** (não a versão On-Premise) — endpoints
hospedados pela Meta em `https://graph.facebook.com/v21.0/`. Vantagens:
- Sem servidor SIP/Docker próprio para WhatsApp.
- Free tier para até 1000 conversas/mês.
- Webhook em HTTPS gerenciado pela Meta.

## Pré-requisitos

- Facebook Business Manager (gratuito).
- Número de telefone exclusivo (não cadastrado no app WhatsApp pessoal); pode ser fixo ou
  móvel.
- Domínio com HTTPS válido para receber webhook (`https://api.mecontrola.app.br/api/v1/whatsapp/inbound`).

## Etapas (ordem obrigatória)

### 1. Criar conta no Business Manager

1. Acessar https://business.facebook.com.
2. Criar **Business Account**: nome, e-mail, país BR.
3. Configurar perfil completo (CNPJ, razão social — `Lima Teixeira Tecnologia LTDA`,
   endereço fiscal).
4. Confirmar e-mail e verificação do CNPJ (pode levar 1-3 dias úteis).

### 2. Criar App Meta com produto WhatsApp

1. Acessar https://developers.facebook.com → My Apps → Create App.
2. Tipo: **Business**.
3. Vincular ao Business Account criado no passo 1.
4. Adicionar produto **WhatsApp** → Setup.
5. A Meta gera automaticamente: `WhatsApp Business Account ID (WABA_ID)`, `App ID`,
   `App Secret`.

### 3. Adicionar número de telefone

1. WhatsApp → Configuration → **Phone Numbers** → Add Phone Number.
2. Inserir o número, escolher método de verificação (SMS ou ligação).
3. Confirmar código.
4. Meta retorna `phone_number_id` (não confundir com o número em si).

**Anotar:**
- `META_PHONE_NUMBER_ID` = `<phone_number_id>` (ex: 102847398923746)
- `META_BOT_NUMBER_E164` = `+55<DDD><NUMERO>` (formato E.164)
- `META_BOT_NUMBER_DISPLAY` = `+55 <DDD> <NUMERO-formatado>` (mostrado ao usuário)

### 4. Gerar Access Token permanente

⚠️ O token gerado por padrão expira em 24h. Precisamos de **System User Token permanente**.

1. Business Manager → **Business Settings** → System Users → **Add**.
2. Nome: `mecontrola-system-user`. Role: `Admin`.
3. Em System Users → Add Assets → WhatsApp Accounts → marcar a WABA.
4. Em System Users → **Generate New Token** → escolher o App → marcar `whatsapp_business_messaging`
   e `whatsapp_business_management` → expiration `Never`.
5. Copiar o token (uma vez só — não há como recuperar depois).

**Anotar:** `META_ACCESS_TOKEN=<token-permanente>`

### 5. Obter App Secret

1. App → **Settings → Basic** → mostrar App Secret.
2. Anotar: `META_APP_SECRET=<app-secret>`.

### 6. Configurar Webhook

1. WhatsApp → Configuration → **Webhook** → Edit.
2. Callback URL: `https://api.mecontrola.app.br/api/v1/whatsapp/inbound`
3. Verify Token: gerar string aleatória e anotar — `META_VERIFY_TOKEN=$(openssl rand -hex 16)`.
4. Antes de salvar: a API MeControla precisa estar **online** no domínio. A Meta vai chamar
   `GET /api/v1/whatsapp/verify?hub.mode=subscribe&hub.verify_token=<token>&hub.challenge=...`
   e esperar resposta 200 com echo do challenge.
5. Após validação OK: **Subscribe to fields** → marcar `messages`.

### 7. Definir variáveis na VPS

```sh
sudo nano /opt/mecontrola/.env
# Substituir:
#   META_PHONE_NUMBER_ID=<phone_number_id>
#   META_ACCESS_TOKEN=<token-permanente>
#   META_APP_SECRET=<app-secret>
#   META_VERIFY_TOKEN=<verify-token-gerado>
#   META_BOT_NUMBER_E164=+55<DDD><NUMERO>
#   META_BOT_NUMBER_DISPLAY=+55 <DDD> 9XXXX-XXXX
```

Reiniciar:

```sh
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d --no-deps --force-recreate server worker
```

### 8. Testar envio outbound

```sh
# Do próprio servidor (ou local com WireGuard):
curl -X POST "https://graph.facebook.com/v21.0/${META_PHONE_NUMBER_ID}/messages" \
  -H "Authorization: Bearer ${META_ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "messaging_product": "whatsapp",
    "to": "5511<seu-numero-pessoal>",
    "type": "text",
    "text": {"body": "MeControla — teste de canal"}
  }'
# Esperado: { "messaging_product": "whatsapp", "contacts": [...], "messages": [{"id":"wamid..."}] }
```

⚠️ **Limite inicial:** primeiras 24h após adicionar número, só é possível mandar mensagens para
contatos que **iniciaram conversa primeiro**. Depois de aprovado para envio outbound, sobe para
1000 conversas/mês free tier.

### 9. Aprovar templates de mensagem (HSM)

Para enviar mensagens proativas (não-resposta) — caso do alerta de categoria/meta/cartão:

1. WhatsApp → **Message Templates** → Create Template.
2. Categoria: `UTILITY` (alertas de uso). NÃO usar `MARKETING` para alertas.
3. Idioma: pt_BR.
4. Body com variáveis: `{{1}}`, `{{2}}`, etc.
5. Submeter para aprovação Meta (até 24h).

**Templates necessários para MVP:**
- `mecontrola_outreach_first` — primeira mensagem após ativação Kiwify.
- `mecontrola_alert_category` — alerta de categoria > 80%.
- `mecontrola_alert_goal` — alerta de meta > 50% antes do prazo.
- `mecontrola_alert_card_limit` — alerta de cartão > 50% do limite.

## Validação E2E

```sh
# 1. Health check
curl https://api.mecontrola.app.br/health
# Esperado: 200

# 2. Webhook verify (simulando Meta)
curl "https://api.mecontrola.app.br/api/v1/whatsapp/verify?hub.mode=subscribe&hub.verify_token=${META_VERIFY_TOKEN}&hub.challenge=test_challenge"
# Esperado: 200 com body "test_challenge"

# 3. Enviar mensagem teste do app WhatsApp pessoal para o número do bot
#    Conferir nos logs do server:
ssh deploy@<vps> 'docker compose ... logs server | grep "whatsapp inbound"'
```

## Rotação do App Secret

Ver `deployment/runbooks/rotate-secret.md` → "Meta App Secret".

## Custos esperados (Brasil, 2026)

| Tipo de conversa | Preço médio |
|------------------|-------------|
| Service (resposta dentro de 24h) | Free (até 1000/mês) |
| Utility (alertas, billing) | R$ 0,15 |
| Authentication | R$ 0,15 |
| Marketing | R$ 0,40 |

Estimativa MVP (100 users × 30 alertas + 100 onboarding/mês): ~R$ 600/mês.

## Referências externas

- Cloud API quickstart: https://developers.facebook.com/docs/whatsapp/cloud-api/get-started
- Pricing: https://developers.facebook.com/docs/whatsapp/pricing
- Template guidelines: https://developers.facebook.com/docs/whatsapp/business-management-api/message-templates

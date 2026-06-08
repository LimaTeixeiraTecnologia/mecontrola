# ADR-005 — WhatsApp Business Cloud API direta (Meta) + verificação `X-Hub-Signature-256`

## Metadados

- **Título:** Canal WhatsApp inbound/outbound via Meta Cloud API sem provedor intermediário
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §6.2, §8.2, RF-06, RF-09, RF-10, S-03, S-04

## Contexto

O PRD exige um canal WhatsApp para:
1. Receber `ATIVAR <token>` do cliente (RF-06).
2. Enviar template aprovado de outreach (RF-09).
3. Enviar mensagens de erro/confirmação (várias RFs).

Não existe receiver de WhatsApp no workspace. Opções de implementação:
1. **Meta WhatsApp Business Cloud API direta** — Graph API oficial; webhook com `X-Hub-Signature-256` (HMAC-SHA256 com App Secret); templates aprovados pela Meta.
2. **Provedor oficial Business Solution Provider (BSP)** — Twilio, MessageBird, 360dialog. Custo por mensagem; abstrai operações.
3. **Provedor não oficial (Z-API, WAHA, Evolution API)** — usa WhatsApp Web não oficial; risco alto de banimento.
4. **Mock interno + adiar implementação real** — define interface, implementação fica para tarefa separada.

## Decisão

**Adotar Meta WhatsApp Business Cloud API direta.** Outbound via Graph API:
```
POST https://graph.facebook.com/v18.0/{phone_number_id}/messages
Authorization: Bearer {access_token}
Content-Type: application/json

{ "messaging_product":"whatsapp",
  "to":"5511XXXXXXXXX",
  "type":"template",
  "template":{
    "name":"activation_reminder",
    "language":{"code":"pt_BR"},
    "components":[{"type":"body","parameters":[{"type":"text","text":"<token>"}]}]
  }
}
```
Resposta inclui `messages[0].id` (WAMID) para auditoria.

Inbound webhook:
- `GET /webhooks/whatsapp` — handshake Meta. Compara `hub.verify_token` (query) com `WhatsAppConfig.VerifyToken` (env). Match → responde `hub.challenge` em `text/plain`.
- `POST /webhooks/whatsapp` — middleware `MetaSignatureMiddleware` valida `X-Hub-Signature-256: sha256=<hex>` usando `App Secret` (HMAC-SHA256 sobre raw body). Comparação `hmac.Equal`. Header ausente ou inválido → 401.

Rotação de secret: `META_APP_SECRET` + `META_APP_SECRET_NEXT` (janela aceita ambos). Mesmo padrão de E2.

Sempre responder HTTP 200 após validação bem sucedida (Meta exige; falhas internas viram log + métrica).

Configuração via `WhatsAppConfig`:
- `PhoneNumberID`, `AccessToken`, `AppSecret`, `AppSecretNext`, `VerifyToken`
- `OutreachTemplateName` (default `activation_reminder`), `OutreachLanguageCode` (default `pt_BR`)
- `OutreachEnabled` (toggle runtime — falso até template aprovado)
- `BotNumberDisplay` (para renderizar na thank-you), `BotNumberE164` (para `wa.me`)
- `ActivationMessages` (mapa estado → texto)

## Alternativas Consideradas

1. **Z-API / WAHA / Evolution API (não oficial).** Recusada — usa WhatsApp Web não oficial; Meta bane números atrelados a comportamento automatizado fora do BSP/Cloud; risco operacional catastrófico para um produto cujo canal é o produto.
2. **Twilio / MessageBird / 360dialog (BSP oficial).** Recusada — custo recorrente por mensagem; abstrai mas adiciona dependência; latência extra; sem ganho técnico relevante para MVP.
3. **Mock interno + adiar implementação real.** Recusada parcialmente — toggle `OutreachEnabled` cobre a parte de templates não aprovados, mas o inbound `ATIVAR` é o caminho feliz do PRD; mock não atende.

## Consequências

### Benefícios
- Canal oficial Meta — sem risco de banimento por uso indevido.
- Sem custo recorrente por mensagem (apenas limites de conversa Meta).
- Latência mínima (chamada Graph API direta).
- Webhook autenticado via padrão Meta amplamente documentado.

### Trade-offs
- Provisionamento operacional Meta (S-03): número, conta Business, App, template — dependência externa do time de operações.
- Templates Meta sujeitos a aprovação manual (S-04); pode atrasar outreach. Mitigação: toggle.
- Quota Meta (mensagens/dia por tier) precisa monitoramento futuro.

### Riscos e Mitigações
- **R:** Aprovação Meta atrasa. **M:** `OutreachEnabled=false` em prod até aprovação; resto do fluxo (ATIVAR, fallback E.164 sem outreach gating) continua operando.
- **R:** Mudança de versão Graph API (v18 → v19 etc.). **M:** Cliente Meta configura versão via `WhatsAppConfig.GraphAPIVersion` (default `v18.0`); upgrade documentado.
- **R:** Token Meta vazado em log. **M:** `httpclient` wrapper redacta `Authorization` header.
- **R:** Replay de webhook Meta. **M:** Idempotência por `message.id` (WAMID) — guard em `WhatsAppInboundHandler` (insert em `meta_processed_messages(wamid PK)` antes de processar).

## Plano de Implementação
1. `internal/onboarding/infrastructure/http/client/meta/client.go` usando `internal/platform/httpclient`.
2. `internal/onboarding/infrastructure/http/server/middleware/meta_signature.go` baseado no `hmac_signature.go` de E2 (algoritmo idêntico, header diferente, prefixo `sha256=`).
3. `internal/onboarding/infrastructure/http/server/handlers/whatsapp_verify_handler.go` e `whatsapp_inbound_handler.go`.
4. Tabela `meta_processed_messages(wamid TEXT PK, processed_at TIMESTAMPTZ)` no schema onboarding (adicionar à migration 0009 ou nova 0012).
5. Test unitário e integração cobrindo: assinatura válida/inválida, idempotência por WAMID, ATIVAR vs fallback, erro 4xx/5xx do client.

## Monitoramento
- `meta_signature_invalid_total` (já listada na techspec §9.2).
- `meta_inbound_messages_total{kind}`.
- `onboarding_outreach_sent_total{result}`.

---
epic_id: E3
slug: onboarding-magic-token
title: Onboarding via magic token (landing → checkout → wa.me → ATIVAR)
status: pending
blocked_by: [E1]
blocks: [E4]
source_bundle: .agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md
source_discoveries:
  - docs/discoveries/discovery-onboarding-flow.md
artifacts:
  prd: null
  techspec: null
  tasks: null
next_skill: create-prd
target_module: internal/onboarding/
---

# Épico E3 — Onboarding Magic Token

## Bloqueio

**Este épico É BLOQUEADO PELO ÉPICO E1 (`identity-foundation`).**

A ativação `ATIVAR <token>` precisa criar/atualizar `User` via `UserRepository` (entregue por E1) e usar `NewWhatsAppNumber` para normalizar o número que abre o chat (entregue por E1).

**Pode rodar em paralelo a E2** (`billing-pipeline`) — ambos dependem de E1, mas não entre si **na fase de PRD/techspec**. Em runtime, E3 consome o resultado do webhook de E2 (`signup_tokens.status = PAID`) e chama `EntitlementService.Invalidate` de E2 ao ativar.

**Acoplamento operacional com E2:** o token marcado como `PAID` é populado pelo `BillingEventProcessor` de E2 ao processar `compra_aprovada`. Para testes E2E completos, ambos precisam estar implementados; testes unitários e de integração de E3 podem mockar essa borda.

**PRD e techspec podem ser escritos em paralelo a E1**, mas execução de tarefas espera E1 atingir `status: implemented`.

## Contexto e motivação

A landing `mecontrola.app.br` promete experiência 100% WhatsApp, com pagamento direto na landing ("Sem app pra baixar"). O desafio é vincular o número que pagou (informado no checkout, frágil) ao número que vai usar (autenticado pelo Meta no WhatsApp).

A discovery de onboarding dita o fluxo: Landing → Checkout Kiwify (com `?s={token}`) → Thank-you page **própria** (não Kiwify) → deep link `wa.me` com `ATIVAR <token>` pré-preenchido → bot resolve token + cria `User` + vincula `Subscription`. A identidade do WhatsApp é garantida pelo próprio canal.

Sem este épico, o cliente paga e o backend não sabe quem é. Os usuários ficam órfãos; suporte vira gargalo.

## Escopo incluído

- Módulo `internal/onboarding/` (novo, separado).
- Agregado `SignupToken` em `internal/onboarding/domain` com UUID v4 opaco, TTL 7 dias, estados `PENDING → PAID → CONSUMED` e `EXPIRED` via job de cleanup.
- Tabela `signup_tokens` em Postgres (token PK, plan_code, status, whatsapp_input, whatsapp_real, email, user_id FK, subscription_id FK, provider_order_id, timestamps, fallback_reason).
- Endpoint `POST /api/checkout-session` com rate limit (10/min por IP) que cria token `PENDING` e retorna URL Kiwify com `?s={token}`.
- Thank-you page **própria** em `mecontrola.app.br/obrigado?token={s}` com deep link `wa.me?text=ATIVAR%20{token}` e fallback copy-paste visível. Auto-redirect em mobile após 2s. **Decidir na techspec** se hospedada no repo da landing ou em domínio da API.
- Handler do comando `ATIVAR <token>` no router de mensagens do WhatsApp, idempotente (reenvio do mesmo token pelo mesmo número responde "já ativado"; outro número responde "código já usado").
- Ativação atômica via `SELECT ... FOR UPDATE`: upsert `users` por `whatsapp_number`, vincula `subscription.user_id`, marca `signup_token.status = CONSUMED`. Tudo na mesma transação.
- Integração com `EntitlementService.Invalidate(ctx, userID)` de E2 imediatamente após ativação (negativo do cache invalidado).
- Job horário de outreach: para tokens `PAID` há > 2h sem `CONSUMED`, dispara template WhatsApp Business pré-aprovado para `whatsapp_input` normalizado via `NewWhatsAppNumber`.
- Fallback de ativação por match E.164: se mensagem qualquer (não só `ATIVAR`) chega de número que casa `whatsapp_input` de um token `PAID`, ativa registrando `fallback_reason = phone_number_match`.
- Job diário de cleanup que marca tokens com `expires_at < now()` como `EXPIRED`.
- Métricas: `onboarding_checkout_session_created_total`, `onboarding_token_paid_total`, `onboarding_activation_consumed_total{path}`, `onboarding_pending_paid_tokens` (gauge), `time_from_paid_to_consumed_seconds` (histograma).
- Logs estruturados com mascaramento de PII (`whatsapp_input`, `whatsapp_real`, `email`).

## Fora de escopo

- Webhook Kiwify e marcação de token como `PAID` — vive em E2 (`BillingEventProcessor` chama `MarkTokenPaid`).
- Implementação completa de `Subscription` e máquina de estados — E2.
- Implementação do `EntitlementService` (cache + decisão) — E2; este épico apenas chama `Invalidate`.
- Painel admin web para suporte reverter ativação ou conceder entitlement manual.
- Trial gratuito alternativo ("CTA secundário vai direto ao bot") — não prometido pela landing atual.
- Suporte multi-país no normalizador de telefone (BR-only).
- Detecção e bloqueio automático de padrão de fraude além do básico ("token já usado por outro número" → alerta para suporte).

## Restrições inegociáveis

- Fluxo único: Landing → Checkout Kiwify (com `?s={token}`) → Thank-you page **própria** → deep link `wa.me` com `ATIVAR <token>` → bot. Sem caminho alternativo no MVP.
- TTL do token: 7 dias. Curto demais aumenta uso de fallback; longo demais vira passe livre.
- Token é UUID v4 opaco. Não enumerável.
- Estados: `PENDING → PAID → CONSUMED`; `EXPIRED` via cleanup. Sem estados intermediários.
- Ativação atômica: `FOR UPDATE` no token, upsert user, vincula subscription, marca consumed — tudo em uma transação.
- User só é criado quando `ATIVAR` chega. Antes disso, `subscription.user_id` é nullable (ou tabela `pending_subscriptions`).
- Handler `ATIVAR` é idempotente: reenvio retorna mensagem amigável, nunca erro 500.
- Fallback obrigatório: (1) job horário de outreach via template WhatsApp Business; (2) ativação por match E.164. Sem fallbacks adicionais.
- LGPD: `whatsapp_input` é PII; mascarar em logs; oferecer mecanismo de deletion request.
- Idempotência do handler `ATIVAR` baseada em `(token, whatsapp_real)` — mesmo token + mesmo número = "já ativado"; mesmo token + número diferente = "código já usado".

## Critérios de aceite

- **CA-01:** `POST /api/checkout-session` cria token `PENDING` e retorna URL Kiwify com `?s={token}` em < 200ms.
- **CA-02:** Rate limit em `/api/checkout-session` (10/min/IP) verificado em teste.
- **CA-03:** Thank-you page renderiza com link `wa.me` e auto-redirect em mobile testado em iOS e Android (smoke manual).
- **CA-04:** Handler `ATIVAR` cobre 100% das transições: token inexistente, expirado, ainda `PENDING`, `PAID` (ativa), `CONSUMED` pelo mesmo número, `CONSUMED` por outro número.
- **CA-05:** Ativação atômica testada (transação) com falha simulada no meio do fluxo — rollback completo, token volta para `PAID`.
- **CA-06:** Job horário de outreach roda em sandbox e dispara template para `whatsapp_input` válido após 2h.
- **CA-07:** Fallback de match E.164 ativa e registra `fallback_reason = phone_number_match`.
- **CA-08:** Idempotência do handler testada (5 envios do mesmo `ATIVAR <token>` produzem o mesmo estado final).
- **CA-09:** Funil de métricas reportado: `checkout_session_created`, `token_paid`, `activation_consumed{path}`, `time_from_paid_to_consumed_seconds`.
- **CA-10:** PII mascarada em logs (`whatsapp_input`, `email`).
- **CA-11:** Lint `depguard` verde; `internal/onboarding/domain` sem import de I/O.

## Dependências externas

- **Kiwify:** propagação de `?s={token}` no payload do webhook (UTM ou custom field). Validar com H7 antes do PRD de E2.
- **Landing page** (`LimaTeixeiraTecnologia/mecontrola-landingpage`): integração com `POST /api/checkout-session` no botão "Assinar"; substituição dos `CHECKOUT_URL_*` placeholders por URL real do endpoint API. Hosting da thank-you page (decidir na techspec).
- **WhatsApp Business API:** template pré-aprovado na Meta para outreach (`activation_reminder` com 1 variável).
- **Postgres:** tabela `signup_tokens` via migration.
- **Redis:** rate limit em `/api/checkout-session`.

## Pré-requisitos não-técnicos

- Template WhatsApp Business `activation_reminder` aprovado pela Meta antes de habilitar job de outreach em produção.
- Decisão de hospedagem da thank-you page (landing repo vs domínio API). Impacta CORS, deploy e SEO.
- Janela coordenada com Marketing para substituir `CHECKOUT_URL_*` placeholders na landing apenas quando E2 + E3 estiverem em staging com smoke test passando.
- Conta WhatsApp Business com número oficial do bot para deep link `wa.me/{numero_bot}`.

## Próximos passos sugeridos

```bash
# 1. PRD
ai-spec create-prd
#  → consome docs/epics/epic-03-onboarding-magic-token.md + bundle (blocos A, E, H)
#  → produz .specs/prd-onboarding-magic-token/prd.md

# 2. Techspec (pode iniciar em paralelo a E1)
ai-spec create-technical-specification
#  → decisão de hospedagem da thank-you page
#  → integração com landing (CORS, deploy)
#  → ADR sobre rate limit (Redis vs token bucket)

# 3. Tasks
ai-spec create-tasks

# 4. Execução — ESPERA E1 estar implemented
ai-spec execute-all-tasks
```

## Riscos residuais

- **R-01 (alto):** `wa.me` com texto pré-preenchido pode não abrir corretamente em desktop ou versões antigas — mitigado por copy-paste fallback visível.
- **R-02 (médio):** Race condition `ATIVAR` chega antes do webhook `compra_aprovada` (Pix lento) — mitigado por mensagem "pagamento ainda processando, tente em 1 minuto".
- **R-03 (médio):** Token usado em outro número (cartão clonado tentando ativar em número diferente) — mitigado por bloqueio + alerta para suporte.
- **R-04 (baixo):** Compra duplicada (cliente clica 2x) gera 2 tokens em `PAID` e 2 subscriptions — mitigado por ativar a primeira e disparar alerta na segunda para reembolso manual.
- **R-05 (médio):** Template WhatsApp Business demora a ser aprovado pela Meta — atraso operacional; mitigado iniciando o processo de aprovação em paralelo ao desenvolvimento.

## Referências

- Bundle: `.agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md` (blocos **A. Layout**, **E. Onboarding**, **H. Segurança/LGPD**).
- Discovery: `docs/discoveries/discovery-onboarding-flow.md`.
- Landing page: `LimaTeixeiraTecnologia/mecontrola-landingpage` (planos em `src/lib/content.ts`).
- Épico bloqueador: `docs/epics/epic-01-identity-foundation.md`.
- Épico paralelo/co-dependente em runtime: `docs/epics/epic-02-billing-pipeline.md`.
- Governança: `CLAUDE.md`, `AGENTS.md`.

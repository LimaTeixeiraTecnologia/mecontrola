---
epic_id: E2
slug: billing-pipeline
title: Pipeline de cobrança recorrente (webhook Kiwify → subscription → entitlement)
status: prd_done
blocked_by: [E1]
blocks: [E4]
source_bundle: .agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md
source_discoveries:
  - docs/discoveries/discovery-billing-hotmart-kiwify.md
artifacts:
  prd: .specs/prd-billing-pipeline/prd.md
  techspec: null
  tasks: null
next_skill: create-technical-specification
target_module: internal/billing/
---

# Épico E2 — Billing Pipeline

## Bloqueio

**Este épico É BLOQUEADO PELO ÉPICO E1 (`identity-foundation`).**

O `BillingEventProcessor` precisa chamar `UserRepository.UpsertByWhatsAppNumber` (entregue por E1) para vincular `Subscription` recebida do webhook. O `EntitlementService.Check` precisa consumir a função pura `IsEntitled(sub, now) bool` declarada em `identity/domain` (entregue por E1).

**Pode rodar em paralelo a E3** (`onboarding-magic-token`) — ambos dependem de E1, mas não entre si.

**PRD e techspec podem ser escritos em paralelo a E1**, mas execução de tarefas espera E1 atingir `status: implemented`.

## Contexto e motivação

A landing `mecontrola.app.br` promete planos Mensal (R$ 29,90), Trimestral (R$ 80,73) e Anual (R$ 297,80) com pagamento via PIX/cartão. Sem este pipeline, o backend não captura cobrança, não conhece estado da assinatura, e não tem como bloquear acesso de inadimplente — qualquer mensagem dispara LLM mesmo sem pagamento.

A discovery de billing dita: webhook handler faz **3 coisas** (verifica assinatura → dedup → publica via outbox); processador async aplica máquina de estados canônica; reconciliação horária protege contra webhook perdido; cache de entitlement com TTL inteligente serve decisão em < 5ms.

## Escopo incluído

- Módulo `internal/billing/` (novo, separado de `internal/finance/`).
- Webhook ingress `/webhooks/kiwify` (3 passos: verifica assinatura/token → dedup `(provider, external_event_id)` em `webhook_events` via `INSERT ... ON CONFLICT DO NOTHING` → persiste raw payload + publica via outbox; retorna 200 em < 2s).
- Agregado `Subscription` em `internal/billing/domain` com `period_length` por plano (30d/90d/365d).
- Máquina de estados canônica: `TRIALING|ACTIVE|PAST_DUE|CANCELED_PENDING|EXPIRED|REFUNDED`. Implementa contrato `Subscription` mínimo declarado em `identity/domain` (E1).
- `BillingEventProcessor` (consumidor do outbox) como **único ponto de mutação** de estado de subscription. Idempotente por `event_id`. Ignora evento se `occurred_at` representa regressão.
- Interface `BillingProvider` em `internal/billing/application` + adapter Kiwify em `internal/billing/infrastructure`. Preparada para Asaas/Pagar.me/Stripe sem refator do processor.
- Tabelas Postgres: `webhook_events` (event store imutável, JSONB), `subscriptions`, `billing_transactions`.
- `EntitlementService.Check(ctx, userID) (Decision, error)` em `internal/billing/application` com:
  - Cache Redis: `TTL = min(period_end - now, 1h)`.
  - Negative cache de 5min para "sem subscription".
  - Invalidação **síncrona** após commit no `BillingEventProcessor` (ordem fixa: Postgres → cache → notificação).
  - Reutiliza função pura `IsEntitled` de `identity/domain` (E1).
- Reconciliação horária via API Kiwify nas subscriptions `ACTIVE`/`PAST_DUE` (batch + rate limit 100 req/min). Divergência dispara evento sintético no mesmo `BillingEventProcessor`.
- Métricas: `billing_webhook_received_total`, `billing_webhook_failed_total`, `billing_event_processed_total`, `billing_subscription_state_total`, `entitlement_check_total`, `entitlement_cache_hit_ratio`.
- Mapeamento Kiwify → estado canônico em adapter (não no processor).
- Logs estruturados com mascaramento de PII (CPF, email, telefone do payload).

## Fora de escopo

- Sweep diário full de reconciliação dos últimos 90 dias (vai para E4 pós-MVP).
- Dashboard MRR/churn (E4).
- Override administrativo de entitlement (`entitlement_overrides`) — escopo de operações pós-MVP.
- Painel admin web para suporte fazer reembolso manual (pós-MVP).
- Implementação real de outros providers (Asaas, Pagar.me, Stripe) — interface preparada, mas sem adapter.
- Trial gratuito (não prometido pela landing; flag opcional para experimentos futuros).
- Rate limit por usuário no gate LLM (entra em E4 ou PRD próprio).

## Restrições inegociáveis

- Webhook ingress faz **3 coisas e nada mais**: verifica assinatura → dedup → persiste raw + publica outbox. Sem parse de regra de negócio, sem decisão de estado.
- `BillingEventProcessor` é o **único** ponto de mutação. Reconciliação dispara evento sintético; nunca atualiza estado direto.
- Idempotência por `event_id` obrigatória em todo consumidor de outbox.
- Cache de entitlement: `TTL = min(period_end - now, 1h)` + negative cache 5min. Sem TTL fixo eterno. Invalidação síncrona após commit.
- `Subscription.period_length` por plano. Sem constante hardcoded.
- `webhook_events` é event store imutável. Nunca apagar; usar para replay e auditoria.
- Reconciliação horária inegociável no MVP. Sweep diário full = E4.
- Estado canônico interno é independente de provider. Mapeamento provider → canônico mora em adapter.
- 1 user = 1 subscription ativa (regra global do bundle).

## Critérios de aceite

- **CA-01:** Webhook ingress retorna 200 em < 2s p99 com payload Kiwify real.
- **CA-02:** Mesmo evento Kiwify enviado 5x produz mesmo estado final (idempotência testada).
- **CA-03:** Eventos fora de ordem (`subscription_renewed` antes de `compra_aprovada`) processam corretamente sem regressão de estado.
- **CA-04:** Cobertura 100% da máquina de estados (6 estados × principais transições).
- **CA-05:** `EntitlementService.Check` serve decisão em < 5ms p99 com cache quente.
- **CA-06:** Reconciliação horária detecta e corrige divergência simulada (mock de API Kiwify).
- **CA-07:** Smoke E2E com Postgres + Redis + mock de webhook Kiwify cobrindo compra → ativação → entitlement granted → cancelamento → entitlement denied.
- **CA-08:** PII (CPF, email, telefone) mascarada em todos os logs do módulo.
- **CA-09:** Lint `depguard` verde; `internal/billing/domain` sem import de I/O.

## Dependências externas

- **Kiwify:** API REST `public-api.kiwify.com`; webhook configurado apontando para `/webhooks/kiwify`; secret de assinatura em vault; eventos habilitados: `compra_aprovada`, `subscription_renewed`, `subscription_late`, `subscription_canceled`, `compra_reembolsada`, `chargeback`.
- **Postgres:** novas tabelas via migration.
- **Redis:** já existente; cache de entitlement e outbox streams.
- **WhatsApp Business API:** notificação de mudança de estado (best-effort via `events.Bus`).

## Pré-requisitos não-técnicos

- **Bloqueador:** validar H7 do bundle — **propagação de `?s={token}` no webhook Kiwify** — com compra real R$ 1 Pix em sandbox da Kiwify. Se Kiwify não propagar, plano B é UTM ou campo customizado obrigatório no formulário; impacta diretamente o PRD de E3.
- Definir produto na Kiwify para cada plano (mensal/trimestral/anual) e capturar `plan_code` real.
- Token de webhook Kiwify provisionado em vault dev/staging/prod.
- Definir URL pública dos webhooks por ambiente.
- Conta de teste WhatsApp Business com template aprovado para notificações de mudança de estado (opcional no MVP).

## Próximos passos sugeridos

```bash
# Antes de tudo: validar propagação de ?s={token} com compra real R$ 1 Pix em sandbox Kiwify.

# 1. PRD
ai-spec create-prd
#  → consome docs/epics/epic-02-billing-pipeline.md + bundle (blocos A, C, D, G)
#  → produz .specs/prd-billing-pipeline/prd.md

# 2. Techspec (pode iniciar em paralelo a E1)
ai-spec create-technical-specification

# 3. Tasks
ai-spec create-tasks

# 4. Execução — ESPERA E1 estar implemented
ai-spec execute-all-tasks
```

## Riscos residuais

- **R-01 (alto):** Kiwify não propagar `?s={token}` — quebra magic token; fallback E.164 tem janela de erro. Mitigação: validar antes do PRD.
- **R-02 (médio):** Webhook fora de ordem ou duplicado — mitigado por idempotência + dedup + verificação de `occurred_at`.
- **R-03 (médio):** Cache de entitlement stale durante mudança de estado — mitigado por TTL inteligente + invalidação síncrona pós-commit.
- **R-04 (médio):** Rate limit Kiwify (100 req/min) estoura em reconciliação com > 6k subs — mitigado por batch e revisão de capacidade em 5k subs.
- **R-05 (baixo):** Hotmart deixado fora do MVP — interface `BillingProvider` permite plugar depois sem refator.

## Referências

- Bundle: `.agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md` (blocos **A. Layout**, **C. Billing**, **D. Entitlement**, **G. Plataforma**).
- Discovery: `docs/discoveries/discovery-billing-hotmart-kiwify.md`.
- Épico bloqueador: `docs/epics/epic-01-identity-foundation.md`.
- PRD de E1 (consumido como dependência): `.specs/prd-identity-foundation/prd.md`.
- Governança: `CLAUDE.md`, `AGENTS.md` seção "Outbox vs events.Bus".

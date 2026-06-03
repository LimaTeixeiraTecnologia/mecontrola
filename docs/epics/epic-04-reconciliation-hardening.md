---
epic_id: E4
slug: reconciliation-hardening
title: Hardening de reconciliação, observabilidade e operações (pós-MVP)
status: backlog
blocked_by: [E2, E3]
blocks: []
source_bundle: .agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md
source_discoveries:
  - docs/discoveries/discovery-billing-hotmart-kiwify.md
  - docs/discoveries/discovery-identity-entitlement.md
  - docs/discoveries/discovery-onboarding-flow.md
artifacts:
  prd: null
  techspec: null
  tasks: null
next_skill: null
target_module: cross-module (billing, identity, onboarding, platform)
---

# Épico E4 — Reconciliation Hardening

## Bloqueio

**Este épico É BLOQUEADO PELOS ÉPICOS E2 (`billing-pipeline`) E E3 (`onboarding-magic-token`).**

Reconciliação diária full, alertas refinados, dashboard MRR/churn e runbook completo só fazem sentido depois que o pipeline básico (webhook + processador + entitlement) e o fluxo de onboarding estejam operacionais. Sem produção real rodando, não há sinal para calibrar thresholds.

**Status:** `backlog` — **não deve ter PRD aberto agora**. Reabrir quando o MVP estiver em produção e: (a) algum incidente justificar hardening, ou (b) volume justificar (ex.: > 5k subs), ou (c) operação manual virar gargalo.

## Contexto e motivação

A discovery de billing entrega o "production-proof mínimo" no MVP: idempotência, reconciliação horária e máquina de estados única. O **hardening completo** (sweep diário full nos últimos 90 dias, alertas refinados por threshold calibrado, dashboard MRR/churn, runbooks operacionais maduros, rate limit por usuário no gate LLM) entra **apenas quando dados reais de produção forem suficientes para calibrar**.

Tratar este épico como "fazemos depois" é decisão consciente. O custo de adiar é: (a) primeiros incidentes vão demandar diagnóstico manual; (b) primeiro mês pode ter blind spots em métricas; (c) suporte responde a tickets de forma artesanal. Esse custo é aceitável no MVP em troca de chegar mais rápido ao primeiro cliente pagante.

## Escopo incluído (provisório, a refinar quando reabrir)

- **Reconciliação diária full** dos últimos 90 dias para todas as subscriptions, com batch + rate limit Kiwify.
- **Alertas operacionais refinados** com thresholds calibrados em dados reais:
  - Webhook falhando 3x consecutivas.
  - Fila/stream de outbox crescendo (`size > N` por > 5min).
  - Reconciliação divergindo (`divergence_count > N` por dia).
  - `pending_paid_tokens > N` (threshold definido após primeiro mês).
  - Latência do `BillingEventProcessor` (`p99 > N segundos`).
- **Dashboard MRR/churn** (Grafana ou similar): MRR mensal, churn voluntário, churn involuntário, taxa de falha de cobrança, conversão de funil (checkout → ativação → engajamento).
- **Runbook completo de operações** para:
  - Reembolso manual.
  - Override de entitlement (criar `entitlement_overrides` se ainda não existir).
  - Replay de webhook (consumir `webhook_events.status = 'RECEIVED'` antigos).
  - Atender pedido de exclusão LGPD com janela de 30 dias + job de anonimização.
  - Trocar número de WhatsApp via CLI admin (consumindo `LinkNewNumber` de E1).
- **Rate limit por usuário** no gate LLM (ex.: 30 mensagens/h por user) com Redis counter.
- **Job de anonimização LGPD** para usuários com `deleted_at > 30 dias`.
- **Métricas avançadas** complementando MVP: histograma de latência de cada estágio do pipeline; trace correlation por `event_id` ponta-a-ponta.
- **Admin web mínimo** (se justificado por volume de suporte): listar subs, override de entitlement, lista de webhooks falhando.

## Fora de escopo

- Multi-provider real (Asaas/Pagar.me/Stripe) — abre PRD próprio quando ticket médio ou MRR justificar.
- Plano família/equipe — abre brainstorm + PRD próprio.
- Suporte a múltiplos países no `WhatsAppNumber` — abre PRD próprio.
- Sistema completo de antifraude (além de "token usado em outro número → alerta").
- Sistema de cupons além do que a Kiwify oferece nativamente.

## Restrições inegociáveis

- Reconciliação respeita a máquina de estados única do `BillingEventProcessor` — divergência dispara evento sintético, nunca atualiza estado direto.
- Override de entitlement registra `granted_by`, `reason` e timestamp; é auditado.
- Rate limit por usuário não pode bloquear comandos administrativos (whitelist `ATIVAR`, `/ajuda`, `/cancelar`, `/contato`).
- Anonimização LGPD preserva chaves de integridade referencial (não viola FK), apenas substitui PII por placeholder.

## Critérios de aceite (a refinar quando reabrir)

- **CA-01:** Sweep diário processa 100% das subscriptions dos últimos 90 dias sem estourar rate limit Kiwify.
- **CA-02:** Alertas configurados disparam em incidente simulado (chaos test) com `MTTD < 5min`.
- **CA-03:** Dashboard MRR/churn atualiza com defasagem máxima de 5min.
- **CA-04:** Runbooks testados em "fire drill" com pessoa de suporte que não escreveu o código.
- **CA-05:** Rate limit por usuário aplica e libera após janela; comandos administrativos passam livres.
- **CA-06:** Job de anonimização LGPD processa em janela noturna sem impactar SLA de produção.

## Dependências externas

- **E2 e E3 em produção** com dados reais por > 4 semanas.
- **Provedor de observabilidade** (Grafana Cloud, Datadog, Honeycomb — decisão fora deste épico).
- **Time de suporte definido** com responsabilidades claras antes de operacionalizar runbooks.

## Pré-requisitos não-técnicos

- Decisão de stack de observabilidade.
- Definição de SLOs / SLIs (MTBF de webhook, latência p99 do entitlement, tempo médio de ativação).
- Acordos de operação com Marketing e Suporte sobre limites do gate LLM.

## Próximos passos sugeridos

**Não abrir PRD agora.** Reabrir este épico quando:
- MVP estiver em produção por > 4 semanas com dados reais; ou
- Incidente concreto demandar hardening pontual (abrir PRD apenas do hardening necessário); ou
- Volume bater 5k subs ativas (revisar capacidade); ou
- Suporte demandar runbooks formais por aumento de tickets.

Quando reabrir:

```bash
# Eventualmente:
ai-spec create-prd      # PRD próprio para a fatia priorizada
ai-spec create-technical-specification
ai-spec create-tasks
ai-spec execute-all-tasks
```

## Riscos residuais

- **R-01 (médio):** Adiar hardening por > 4 meses aumenta dívida operacional acumulada; suporte vira gargalo. Mitigação: revisar trimestralmente se algum item de E4 já é urgente.
- **R-02 (médio):** Sem dashboard MRR/churn, decisões de produto e marketing ficam às cegas. Mitigação: query SQL manual em janela mensal até dashboard existir.
- **R-03 (baixo):** Sem rate limit por usuário, um cliente paga pode disparar muito LLM e estourar custo. Mitigação: alertar em `entitlement_check_total` por user atingir N/dia.

## Referências

- Bundle: `.agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md` (bloco **I. Roadmap**, item 4).
- Discovery billing: `docs/discoveries/discovery-billing-hotmart-kiwify.md` (seção 12 — checklist production-proof).
- Discovery identity: `docs/discoveries/discovery-identity-entitlement.md` (seção 7 — checklist).
- Discovery onboarding: `docs/discoveries/discovery-onboarding-flow.md` (seção 9 — checklist).
- Épicos bloqueadores: `docs/epics/epic-02-billing-pipeline.md`, `docs/epics/epic-03-onboarding-magic-token.md`.

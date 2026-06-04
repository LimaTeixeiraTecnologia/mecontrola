# ADR-009 — `BillingEventProcessor` único mutador com idempotência via `billing_event_applications`

## Metadados

- **Título:** Garantia de idempotência do processor via tabela dedicada
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de domínio + plataforma
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-04, RF-14, RF-21..27, F-4), `techspec.md` §process_billing_event, `internal/platform/outbox/handler.go`

## Contexto

`outbox.Dispatcher` entrega at-least-once. RF-21 obriga handler idempotente por `event.ID()`. PRD declara `BillingEventProcessor` como único ponto de mutação de `Subscription` — webhook ingress nunca muta estado, reconciliação publica evento sintético em vez de UPDATE direto.

Existem dois níveis de idempotência possíveis:
- Por `outbox.event.ID` (granularity do outbox).
- Por `webhook_event.id` (granularity do business event).

São o mesmo conceito apenas se cada webhook produzir exatamente 1 outbox event. No fluxo de reconciliação, eventos sintéticos têm `outbox.event.ID` novo mas referenciam um `webhook_event` sintético também — `webhook_event.id` é o que identifica o **business event** unicamente.

## Decisão

Tabela `billing_event_applications (event_id TEXT PK, subscription_id TEXT FK, applied_at TIMESTAMPTZ)` registra cada aplicação de `webhook_event` em uma `Subscription`. PK em `event_id` (= `webhook_event.id`).

`ProcessBillingEventUseCase.Handle(evt outbox.Event)`:
1. Decode payload do outbox: extrai `webhookEventID`.
2. Lookup `webhook_events.payload` (ADR-001).
3. Parse `CanonicalEvent` via adapter.
4. Open UoW transacional:
   - Resolve `User` via `UserResolver.UpsertByWhatsAppNumber`.
   - Find current `Subscription` (pode ser nil para primeira ativação).
   - Aplica state machine: `sub, transition := stateMachine.Apply(existing, canonical, userID, now)`.
   - Se `transition.IsNoop()` (evento stale, RF-25): retorna sem persistir, sem registrar application.
   - Caso contrário: `RecordApplication(eventID, sub.ID, now)` via `INSERT INTO billing_event_applications ... ON CONFLICT DO NOTHING`.
   - Se `recorded == false` (já aplicado em retry anterior): retorna sem `Upsert`.
   - Caso contrário: `Upsert(sub)`, `MarkProcessed(webhookEventID, now)`.
5. Pós-commit: `cache.Invalidate(userID)`, `events.Bus.Publish(StateChangedEvent)`.

**Classificação de erros para outbox:**
- `ErrPayloadDecode`, `ErrUnknownKiwifyEventType` → wrapped com `outbox.ErrPermanent` → DLQ.
- Erros Postgres transitórios (deadlock, connection refused) → não wrapped → Dispatcher retenta.

**Erros de domínio (`ErrIllegalTransition`):** wrapped com `outbox.ErrPermanent` — transição ilegal sinaliza estado inconsistente ou bug, retry não resolve.

## Alternativas Consideradas

### Idempotência via `subscription.last_event_at` + `OCCURRED_AT` check apenas

- Vantagem: zero tabela extra.
- Desvantagem: dois eventos com mesmo `occurred_at` (race) aplicariam dois lados; lógica de "stale" não cobre replay exato do mesmo evento.
- Rejeitada por insuficiência.

### Dedup no próprio outbox por `event_id` (subscription_name UNIQUE)

- Vantagem: outbox já tem `(event_id, subscription_name) UNIQUE` em `outbox_deliveries`.
- Desvantagem: dedup é por entrega, não por aplicação ao business state. Reconciliação publica evento sintético com `outbox.event_id` novo, mas business `webhook_event_id` é o mesmo se for repasse. Insuficiente.
- Rejeitada por confundir camadas.

### Upsert direto em `subscriptions` por `(provider, external_subscription_id)` como dedup natural

- Vantagem: UPSERT é idempotente naturalmente.
- Desvantagem: evento `subscription_renewed` precisa ler estado anterior para avançar `period_end`. UPSERT não basta — precisa lógica de transição.
- Rejeitada por não cobrir estado.

## Consequências

### Benefícios Esperados

- Idempotência forte por `event_id` — garantia formal.
- Auditoria: cada aplicação é rastreável (`subscription_id` + `applied_at`).
- Replay de evento original ou reconciliation é seguro: 2ª aplicação é no-op.

### Trade-offs e Custos

- Tabela cresce linearmente com eventos processados — ~80 bytes/linha; em 5 anos a 10k eventos/dia ≈ 1.4 GB. Aceitável.
- 1 INSERT extra por evento — sob 50 eventos/tick batch do Dispatcher, custo negligível.

### Riscos e Mitigações

- **Risco:** `ON CONFLICT DO NOTHING` mascara inconsistência (aplicação prévia em subscription_id diferente). **Mitigação:** PK é só `event_id`; conflito significa que evento já foi aplicado (não importa em qual subscription). Auditoria via `applied_at` revela momento.
- **Risco:** transação que insere `application` aborta após commit do outbox publisher → retry processa mesmo evento gerando 2ª linha. **Mitigação:** outbox publish e application insert estão na mesma transação do processor (UoW); abort reverte ambos.

## Plano de Implementação

1. Migration `0009_billing_schema.up.sql` cria `billing_event_applications` com FK CASCADE para `subscriptions`.
2. `WebhookEventRepository.RecordApplication(ctx, eventID, subID, at) (bool, error)` retorna `(true, nil)` em insert sucesso, `(false, nil)` em conflict.
3. `ProcessBillingEventUseCase` chama `RecordApplication` ANTES de `Upsert`; se `false`, no-op.
4. Integration test (CA-02): processa mesmo `webhook_event_id` 5x → 1 linha em `billing_event_applications`, 1 row final em `subscriptions` no mesmo estado.
5. Métrica `billing_event_processed_total{outcome="duplicate_application"}` quando `recorded=false`.

## Monitoramento e Validação

- Métricas:
  - `billing_event_processed_total{outcome ∈ "applied", "duplicate_application", "ignored_stale", "ignored_unknown", "dlq"}`.
  - `billing_event_applications_count` (gauge via `count(*)`).
- Span OTel `billing.event.process` com atributos `webhook_event_id`, `outcome`.

## Impacto em Documentação e Operação

- Runbook de replay: para reprocessar evento, basta republicar no outbox apontando para mesmo `webhook_event_id` — application table garante no-op.
- AGENTS.md billing documenta contrato.

## Revisão Futura

- Se número de eventos crescer >> 1M/mês, considerar particionamento de `billing_event_applications` por mês.

# ADR-001 — Outbox event carrega pointer mínimo `{webhook_event_id, provider}`

## Metadados

- **Título:** Pointer-based outbox payload para eventos do webhook ingress
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de produto + plataforma
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-05, RF-22), `techspec.md` §F-1/F-4, `AGENTS.md` "Outbox vs events.Bus", `internal/platform/outbox/event.go:16-83`

## Contexto

O fluxo de ingresso de webhook precisa publicar um evento no `outbox.Publisher` indicando que um payload Kiwify foi recebido para que o `BillingEventProcessor` consuma de forma assíncrona via `outbox.Dispatcher`. `outbox.NewEvent` exige `Payload json.RawMessage` válido. Duas tabelas já guardam o payload bruto: `webhook_events` (event store imutável com `UNIQUE (provider, external_event_id)`) é single source of truth.

A questão é o que vai dentro do `outbox.Event.Payload`. RF-07 do PRD ordena que o webhook ingress não interprete payload de negócio. Confronto adicional: `outbox_events.payload` é JSONB e Dispatcher transporta o blob via `ClaimReady`; payloads grandes oneram o batch e a tabela.

## Decisão

O `outbox.Event` publicado pelo `IngestKiwifyWebhookUseCase` carrega payload mínimo:

```json
{ "webhook_event_id": "01HXYZ...", "provider": "kiwify" }
```

O `ProcessBillingEventUseCase` (handler registrado) faz `WebhookEventRepository.FindRawPayload(webhookEventID)` ao receber o evento, recuperando o payload bruto da `webhook_events`. `webhook_events` permanece como single source of truth para o payload e para auditoria/replay.

## Alternativas Consideradas

### Self-contained (payload integral no outbox)

- Vantagem: processor não depende de `webhook_events` em runtime.
- Desvantagem: duplicação de PII em duas tabelas (`webhook_events.payload` + `outbox_events.payload`); evento grande (10–50 KB) onera o `Dispatcher.ClaimReady` batch; contradição conceitual (event store deveria ter 1 cópia).
- Rejeitada por duplicação de PII e overhead em batch.

### Híbrido (metadados parseados + ref)

- Vantagem: processor vê metadados (event_type, external_event_id, occurred_at) sem SELECT.
- Desvantagem: parsing parcial no ingress contradiz parcialmente RF-07; metadados ficam duplicados e podem divergir se schema evoluir.
- Rejeitada por viola RF-07 e por adicionar superfície de drift.

## Consequências

### Benefícios Esperados

- `outbox_events` permanece leve (< 100 bytes por evento).
- `webhook_events` é a única fonte de verdade — qualquer reprocessamento/replay parte dela.
- Conformidade com RF-07 (ingress não interpreta payload).
- Schema do payload Kiwify pode evoluir sem afetar contrato do outbox.

### Trade-offs e Custos

- Processor faz 1 SELECT extra por evento em `webhook_events` (PK lookup, < 1ms em Postgres com índice).
- Se `webhook_events` for inacessível durante processamento (e.g., DB readonly mode), processor falha — mas neste cenário a stack inteira já está degradada.

### Riscos e Mitigações

- **Risco:** registro em `webhook_events` deletado fora de auditoria → processor não acha payload. **Mitigação:** anonimização preserva metadados e payload anonimizado; nunca hard delete (ADR-007). DDL `REVOKE ALL ... FROM PUBLIC` reduz superfície.

## Plano de Implementação

1. Criar `internal/billing/infrastructure/outbox/event_payload.go` com `EncodeReceivedPayload(id) (json.RawMessage, error)` e `DecodeReceivedPayload(json.RawMessage) (ReceivedPayload, error)`.
2. `IngestKiwifyWebhookUseCase` chama `EncodeReceivedPayload` após `InsertIfNew` retornar `inserted=true`.
3. `ProcessBillingEventUseCase` chama `DecodeReceivedPayload` no início do `Handle`.
4. Adicionar caso de teste: payload decodificado contém `webhook_event_id` e `provider` corretos.

## Monitoramento e Validação

- Métrica `billing_event_processed_total{outcome}` com `outcome="lookup_failed"` em caso de FindRawPayload falhar.
- Span `billing.event.process` inclui span filho `webhook_events.find_raw_payload` com latência.

## Impacto em Documentação e Operação

- `internal/billing/AGENTS.md` documenta o contrato do payload.
- Runbook de replay descreve: para reprocessar, publicar novo outbox event apontando para mesmo `webhook_event_id`.

## Revisão Futura

- Revisitar se o número de webhooks > 100 req/s sustentado — o 1 SELECT extra pode justificar cache no processor.

# ADR-001 — Schema Two-Table (`outbox_events` + `outbox_deliveries`)

## Metadados

- **Título:** Schema two-table com granularidade de delivery por subscription
- **Data:** 2026-06-02
- **Status:** Aceita
- **Decisores:** Tech lead backend
- **Relacionados:** PRD `prd-outbox-event-driven` v4; techspec `.specs/prd-outbox-event-driven/techspec.md`; ADR-016 (PRD foundation)

## Contexto

Toda implementação de Transactional Outbox precisa decidir entre:

1. **Single-table** (`outbox_events`): evento + estado de entrega na mesma linha; fan-out lógico via consulta.
2. **Two-table** (`outbox_events` + `outbox_deliveries`): evento imutável + 1 linha de delivery por subscription registrada.

A diferença materializa diretamente a granularidade de observabilidade, retry e DLQ. Discovery aprovado (Alt 2 do scorecard) selecionou two-table; esta ADR formaliza.

PRD declara (RF-04, RF-08, RF-16) que cada par `(event, handler)` precisa ter ciclo de vida independente, métrica própria e DLQ próprio.

## Decisão

Schema two-table:

- `outbox_events` (PK `id TEXT` = ULID) — **imutável após insert**. Armazena `event_type`, `event_version`, `aggregate_type`, `aggregate_id`, `partition_key` (nullable), `payload JSONB`, `headers JSONB`, `occurred_at`, `created_at`.
- `outbox_deliveries` (PK `id BIGSERIAL`) — uma linha por par `(event_id, subscription_name)`. Constraint `UNIQUE (event_id, subscription_name)` garante idempotência do publish (RF-04). Carrega `status`, `attempts`, `next_retry_at`, `last_error`, `processed_at`, `dead_letter_at`, `claimed_at`, `claimed_by` (hostname-pid, D-11).
- Foreign key `outbox_deliveries.event_id REFERENCES outbox_events(id) ON DELETE CASCADE` — housekeeping apaga primeiro deliveries, depois eventos órfãos.

## Alternativas Consideradas

- **Single-table com bitmask de delivery por subscription**. Vantagem: 1 INSERT por evento. Desvantagens: retry/DLQ por subscription exige tabela auxiliar de qualquer forma; histórico de tentativas perdido; observabilidade granular impossível.
- **Single-table + tabela de DLQ separada apenas para falhas terminais**. Mantém problema da observabilidade de retries em progresso.
- **Three-table** (`events` + `subscriptions_status` + `dlq`). Excesso de joins; sem ganho.

## Consequências

**Benefícios**:
- DLQ, retry, métrica por subscription nativos via filtro `WHERE subscription_name = $1`.
- Falha de um handler não bloqueia ou contamina outros (RF-08).
- Re-enfileiramento manual via SQL simples (RF-17, runbook).

**Custos**:
- +1 INSERT por handler na transação do publish. Para volumetria alvo (1–3 handlers/evento), aceitável; monitorado.
- 2 tabelas para entender em vez de 1.

**Riscos / Mitigações**:
- Tamanho de `outbox_deliveries` cresce mais rápido — mitigado por housekeeping diário + índices parciais.
- Casos com >5 handlers/evento começam a pressionar — métrica `subscriptions_per_event_type`, revisão obrigatória.

## Plano de Implementação

`migrations/0002_outbox.up.sql` + `down.sql`. Idempotente via `CREATE TABLE IF NOT EXISTS`. Detalhe completo na techspec.

## Monitoramento e Validação

- `outbox.deliveries.dlq.total{subscription_name}` por subscription.
- Tamanho de `outbox_deliveries` exposto via gauge `outbox.deliveries.total` (collector via `Stats`).
- Validação: integration test confirma constraint UNIQUE rejeita duplicatas.

## Revisão Futura

Reavaliar se número médio de handlers por evento ultrapassar 5 sustentadamente (batching de insert vira candidato) ou se schema migration para particionamento por hash for necessária.

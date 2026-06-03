# ADR-002 — Coordenação multi-instância via `FOR UPDATE SKIP LOCKED`

## Metadados

- **Título:** Coordenação entre réplicas do worker usando `FOR UPDATE SKIP LOCKED`
- **Data:** 2026-06-02
- **Status:** Aceita
- **Decisores:** Tech lead backend
- **Relacionados:** PRD `prd-outbox-event-driven` v4 (R-05); techspec; ADR-001 (schema)

## Contexto

O Dispatcher é executado dentro do `cmd/worker`, que pode rodar com **N réplicas** simultaneamente (Fly.io / Kubernetes). Cada delivery em `outbox_deliveries` precisa ser processada **exatamente uma vez por vez** (zero double-processing — RF-14). Restrição R-05 do PRD: sem leader election externo, sem ZooKeeper/etcd, sem advisory locks. Apenas Postgres.

Há essencialmente 4 técnicas para coordenar acesso concorrente entre réplicas em Postgres puro:

1. `SELECT ... FOR UPDATE SKIP LOCKED` — claim explícito com bloqueio por linha.
2. `pg_advisory_lock` — lock global por chave (proibido por R-05).
3. Leader election externo + worker único — proibido por R-05.
4. Optimistic locking via `version` column — sofre de muitos retries em alta concorrência.

## Decisão

Adotar **`SELECT ... FOR UPDATE SKIP LOCKED`** dentro de um `UPDATE ... WHERE id IN (subquery)` para claim transacional. A subquery seleciona deliveries `pending` com `next_retry_at <= now()`, ordena por `id` e aplica `LIMIT $batchSize`. O outer `UPDATE` marca `status='claimed', claimed_at=now(), claimed_by=$instance, attempts=attempts+1` e retorna as linhas claimadas via `RETURNING`.

```sql
UPDATE outbox_deliveries d
   SET status = 'claimed', claimed_at = now(), claimed_by = $1, attempts = d.attempts + 1, updated_at = now()
 WHERE d.id IN (
       SELECT id FROM outbox_deliveries
        WHERE status = 'pending' AND next_retry_at <= now()
        ORDER BY id LIMIT $2
        FOR UPDATE SKIP LOCKED
 )
 RETURNING d.id, d.event_id, d.subscription_name, d.attempts;
```

O reaper (cron `@every 1m`) usa a mesma técnica para liberar `claimed` stuck (D-17), evitando race com Dispatcher que esteja terminando de marcar uma linha.

## Alternativas Consideradas

- **`pg_advisory_lock`**: proibido por R-05.
- **Leader election externo**: proibido por R-05; adiciona componente fora do Postgres.
- **Optimistic version + retry**: em alta concorrência (3+ dispatchers) gera muitos retries em loop; pior throughput.
- **Partitioning por hash de aggregate_id + um worker por partição**: requer atribuição manual de partições; quebra horizontal scaling pelo Fly.io / k8s; complexidade desnecessária para a volumetria alvo.

## Consequências

**Benefícios**:
- Zero coordenador externo. Stack continua "Postgres-only".
- Escalável linearmente com número de réplicas até o limite de polling pressure (~5 réplicas com tick=500ms).
- Recuperação automática de crash via reaper sem leader election.
- Comportamento bem documentado e exercitado em Postgres 9.5+.

**Custos**:
- `FOR UPDATE` adquire bloqueio por linha — embora `SKIP LOCKED` evite contenção, há custo de WAL e checkpoint.
- Em fila vazia, cada Dispatcher executa ~2 queries/s (`tick=500ms`); com 3 réplicas isso é ~6 qps de polling.

**Riscos / Mitigações**:
- **Crash entre `UPDATE claimed` e `MarkProcessed`** → delivery presa em `claimed`. Mitigado pelo reaper `@every 1m` + `stuck_after=5m`.
- **Polling agressivo demais com muitas réplicas** → CPU do Postgres sobe. Mitigado por monitoramento e tick configurável; alvo MVP 1–3 réplicas.
- **`SKIP LOCKED` não respeita `ORDER BY` globalmente** — duas réplicas podem pegar linhas fora da ordem absoluta. Aceitável: ordem garantida apenas dentro de cada batch e por `aggregate_id`; ordenação global FIFO está fora-de-escopo (FE-05).

## Plano de Implementação

`storage_pgx.go` implementa `ClaimReady(ctx, batchSize, instanceID) ([]Claim, error)` com a query acima. Integration test `concurrency_integration_test.go` valida com 3 dispatchers + 1000 deliveries que nenhuma é processada mais de uma vez.

## Monitoramento e Validação

- `outbox.poll.duration_ms` — custo do claim por ciclo.
- `outbox.poll.batch_size` — quantos itens retornaram (gauge zero em fila vazia).
- Validação: assert do integration test verifica `SELECT event_id, subscription_name, COUNT(*) FROM outbox_deliveries WHERE status='processed' GROUP BY 1,2 HAVING COUNT(*)>1` retorna 0 linhas.

## Revisão Futura

Promover para LISTEN/NOTIFY se p99 de entrega > 2s sustentado por 7d, ou se número de réplicas precisar passar de 5.

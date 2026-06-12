# Runbook: Modulo Transactions

**Servico:** `internal/transactions`
**Owner:** Plataforma
**Dashboard:** [Transactions Overview](../../docs/dashboards/transactions-overview.json)
**Alertas:** [docs/alerts/transactions.yaml](../../docs/alerts/transactions.yaml)

---

## Cenario 1: Consumer Travado

**Alerta:** `TransactionsOutboxConsumerLagHigh`
**Metrica:** `transactions_outbox_consumer_lag_seconds > 5` por 5min
**Severidade:** critical

### Sintomas

- Consumer lag acima de 5 segundos por mais de 5 minutos.
- Painel `Outbox Consumer Lag (s)` no dashboard mostra valor crescente ou plateau alto.
- `MonthlySummary` desatualizado para usuarios afetados.

### Diagnostico

1. Verificar logs do worker para erros de processamento:
   ```
   module="transactions" operation="consumer.monthly_summary_recompute.handle"
   ```
2. Verificar se o processo worker esta em execucao (`cmd/worker`).
3. Checar se o coalescer esta retendo eventos (timer nao drenado):
   ```sql
   SELECT COUNT(*), MIN(created_at), MAX(created_at)
   FROM outbox_events
   WHERE processed = false
     AND aggregate_type LIKE 'transactions.%'
   ORDER BY created_at;
   ```
4. Verificar se ha lock ou deadlock no banco de dados bloqueando o consumer.

### Remediacao

**Drenar manualmente e resetar cursor:**

1. Se o worker travar em retry loop, reiniciar o processo:
   ```bash
   kubectl rollout restart deployment/mecontrola-worker
   ```
2. Se eventos acumulados apos reinicio, verificar evento especifico com erro:
   ```sql
   SELECT id, type, aggregate_id, payload, retry_count, last_error
   FROM outbox_events
   WHERE processed = false
     AND aggregate_type LIKE 'transactions.%'
     AND retry_count > 3
   ORDER BY created_at
   LIMIT 20;
   ```
3. Para drenar manualmente eventos presos, marcar como dead-letter apos inspecao:
   ```sql
   UPDATE outbox_events
   SET processed = true, dead_letter = true, processed_at = NOW()
   WHERE id = '<event_id>';
   ```
4. Monitorar metrica `transactions_outbox_consumer_lag_seconds` ate retornar a zero.

**Metrica de referencia:** `transactions_outbox_consumer_lag_seconds`

---

## Cenario 2: Drift Detectado

**Alerta:** `TransactionsMonthlySummaryDriftDetected`
**Metrica:** `increase(transactions_monthly_summary_drift_total{kind="detected"}[1d]) > 0` por 15min
**Severidade:** warning

### Sintomas

- O job `MonthlySummaryReconcilerJob` detectou divergencia entre `monthly_summary.total_cents` e `SUM(transactions) + SUM(card_invoice_items)` para um ou mais `(user_id, ref_month)`.
- Painel `Monthly Summary Drift by Kind` no dashboard mostra incremento.
- Usuarios podem ver saldo mensal incorreto.

### Diagnostico

1. Identificar pares `(user_id, ref_month)` afetados via logs:
   ```
   module="transactions" operation="reconcile_monthly_summary" level="warn"
   ```
2. Confirmar drift com query manual:
   ```sql
   SELECT
     ms.user_id,
     ms.ref_month,
     ms.income_total_cents AS projected_income,
     ms.expense_total_cents AS projected_expense,
     SUM(CASE WHEN t.direction = 'income' THEN t.amount_cents ELSE 0 END) AS actual_income,
     SUM(CASE WHEN t.direction = 'outcome' THEN t.amount_cents ELSE 0 END) AS actual_expense
   FROM monthly_summaries ms
   LEFT JOIN transactions t ON t.user_id = ms.user_id
     AND DATE_TRUNC('month', t.occurred_at AT TIME ZONE 'America/Sao_Paulo') = ms.ref_month::date
   WHERE ms.ref_month = '<YYYY-MM>'
   GROUP BY ms.user_id, ms.ref_month, ms.income_total_cents, ms.expense_total_cents
   HAVING
     ms.income_total_cents != SUM(CASE WHEN t.direction = 'income' THEN t.amount_cents ELSE 0 END)
     OR ms.expense_total_cents != SUM(CASE WHEN t.direction = 'outcome' THEN t.amount_cents ELSE 0 END);
   ```
3. Verificar se houve evento perdido no outbox (evento publicado mas nao processado):
   ```sql
   SELECT id, type, aggregate_id, occurred_at
   FROM outbox_events
   WHERE aggregate_type LIKE 'transactions.%'
     AND processed = false
     AND occurred_at < NOW() - INTERVAL '1 hour';
   ```

### Remediacao

**Investigar evento perdido e recomputar:**

1. Se evento perdido identificado, forcar reprocessamento publicando evento de reconciliacao ou chamando endpoint interno de recompute (se disponivel).
2. Para recompute manual de um par `(user_id, ref_month)`:
   - Publicar evento de recompute via API interna ou via script que chame o use case `RecomputeMonthlySummary` diretamente.
3. Confirmar que apos reprocessamento a divergencia some:
   ```sql
   SELECT drift_detected FROM monthly_summaries
   WHERE user_id = '<uid>' AND ref_month = '<YYYY-MM>';
   ```
4. Monitorar `transactions_monthly_summary_drift_total{kind="detected"}` para garantir que nao ha novos drifts.

**Metrica de referencia:** `transactions_monthly_summary_drift_total`, `transactions_monthly_summary_recompute_duration_seconds`

---

## Cenario 3: Dead-Letter

**Alerta:** `TransactionsOutboxDeadLetterIncreasing`
**Metrica:** `increase(transactions_outbox_dead_letter_total[15m]) > 0`
**Severidade:** critical

### Sintomas

- Eventos do modulo transactions nao puderam ser processados apos todas as tentativas e foram movidos para dead-letter.
- Painel `Dead-Letter Events` no dashboard mostra incremento.
- Metrica `transactions_outbox_dead_letter_total` aumentando.
- `MonthlySummary` pode estar desatualizado para usuarios afetados.

### Diagnostico

1. Identificar eventos no dead-letter:
   ```sql
   SELECT id, type, aggregate_id, payload, retry_count, last_error, created_at
   FROM outbox_events
   WHERE dead_letter = true
     AND aggregate_type LIKE 'transactions.%'
   ORDER BY created_at DESC
   LIMIT 50;
   ```
2. Analisar `last_error` para identificar causa:
   - **Payload malformado**: erro de deserializacao JSON → bug no producer ou schema drift.
   - **Falha de idempotencia**: `event_id` duplicado inesperado → investigar dupla publicacao.
   - **Erro de repositorio**: banco indisponivel durante processamento → verificar conectividade.
3. Verificar logs do consumer no periodo de ocorrencia:
   ```
   module="transactions" level="error" operation="consumer.monthly_summary_recompute"
   ```

### Remediacao

**Replay manual ou descarte com auditoria:**

**Opcao A — Replay manual** (quando causa raiz corrigida):

1. Identificar eventos no dead-letter que devem ser reprocessados.
2. Resetar flags para reprocessamento:
   ```sql
   UPDATE outbox_events
   SET dead_letter = false, processed = false, retry_count = 0, last_error = NULL
   WHERE id IN ('<event_id_1>', '<event_id_2>');
   ```
3. Aguardar consumer reprocessar e monitorar logs.
4. Confirmar que `transactions_monthly_summary_drift_total` nao aumenta apos replay.

**Opcao B — Descarte com auditoria** (quando evento invalido ou obsoleto):

1. Registrar o evento descartado na tabela de auditoria antes de marcar como ignorado:
   ```sql
   INSERT INTO audit_log (entity_type, entity_id, action, actor, details, occurred_at)
   VALUES ('outbox_event', '<event_id>', 'dead_letter_discarded', 'ops', '<justificativa>', NOW());
   ```
2. Marcar evento como descartado:
   ```sql
   UPDATE outbox_events
   SET processed = true, dead_letter = true, processed_at = NOW(),
       last_error = 'manually_discarded: <justificativa>'
   WHERE id = '<event_id>';
   ```
3. Verificar se o `MonthlySummary` dos usuarios afetados precisa de recompute manual.
4. Monitorar `transactions_outbox_dead_letter_total` para confirmar que nao ha novos eventos.

**Metrica de referencia:** `transactions_outbox_dead_letter_total`, `transactions_idempotency_replay_total`

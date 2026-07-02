# Runbook: Deploy Anti-Storm e Triagem de Alertas WhatsApp

PRD: `.specs/prd-whatsapp-ordenacao-idempotencia/`
ADR-004: `.specs/prd-whatsapp-ordenacao-idempotencia/adr-004-observabilidade-e-deploy-seguro.md`
Task: 7.0 (7.4/7.5/7.7)

## 1. Deploy Seguro (stop-first + stop_grace_period)

### Invariante

O `compose.swarm.yml` garante para server-1, server-2, worker-1, worker-2:

```yaml
deploy:
  update_config:
    order: stop-first
    parallelism: 1
  stop_grace_period: 30s
```

`stop-first` assegura que a instancia antiga e encerrada antes de a nova subir, eliminando
o janela dual-writer que causava duplicidade de claims e desordem de FIFO.

`stop_grace_period: 30s` da tempo ao processo para completar o claim em voo, fechar o span
OTel e drenar goroutines antes do SIGKILL do Swarm.

### Procedimento de deploy

1. Construir e publicar a imagem: `IMAGE_TAG=<sha> make build push`.
2. Verificar variaveis de ambiente antes de atualizar a stack:

   ```bash
   grep OTEL_SERVICE_VERSION deployment/compose/compose.swarm.yml
   ```

3. Atualizar a stack:

   ```bash
   docker stack deploy -c deployment/compose/compose.swarm.yml mecontrola
   ```

4. Monitorar a rolling update (uma replica por vez):

   ```bash
   watch docker service ps mecontrola_worker-1
   watch docker service ps mecontrola_worker-2
   ```

5. Confirmar zero replicas `Running` da versao anterior antes de prosseguir.

### Gate CI anti-storm (execucao obrigatoria antes de merge em main)

```bash
# 1. Confirmar stop-first em todos os servicos de producao
for svc in server-1 server-2 worker-1 worker-2; do
  count=$(grep -A 20 "^  $svc:" deployment/compose/compose.swarm.yml \
    | grep -c "order: stop-first")
  [ "$count" -gt 0 ] || echo "FAIL: stop-first ausente em $svc"
done

# 2. Confirmar stop_grace_period >= 30s
for svc in server-1 server-2 worker-1 worker-2; do
  count=$(grep -A 20 "^  $svc:" deployment/compose/compose.swarm.yml \
    | grep -c "stop_grace_period: 30s")
  [ "$count" -gt 0 ] || echo "FAIL: stop_grace_period ausente em $svc"
done

# 3. Confirmar OTEL_SERVICE_VERSION presente
grep -q "OTEL_SERVICE_VERSION" deployment/compose/compose.swarm.yml \
  || echo "FAIL: OTEL_SERVICE_VERSION ausente"

# 4. Confirmar OTEL_TRACE_SAMPLE_RATE=1 nos 4 servicos
count=$(grep "OTEL_TRACE_SAMPLE_RATE" deployment/compose/compose.swarm.yml \
  | grep -c '"1"')
[ "$count" -ge 4 ] || echo "FAIL: OTEL_TRACE_SAMPLE_RATE!=1 em algum servico"
```

## 2. Triagem: Dead-letter (alert OutboxDeadLetter)

Sintoma: `outbox_dead_letter_total` aumentou; evento com `status=4`.

### Passos

1. Identificar o evento:

   ```sql
   SELECT id, type, aggregate_id, attempts, next_attempt_at, last_error
   FROM outbox_events
   WHERE status = 4
   ORDER BY next_attempt_at DESC
   LIMIT 20;
   ```

2. Inspecionar o erro no campo `last_error`. Causas comuns:
   - Payload malformado (JSON invalido para o consumer).
   - Consumer com bug que sempre retorna erro para um tipo de evento.
   - Timeout sistematico do LLM/tool (verificar `agents_whatsapp_inbound_timeout_total`).

3. Reparar o evento se possivel (corrigir payload):

   ```sql
   UPDATE outbox_events SET status = 0, attempts = 0, next_attempt_at = NOW()
   WHERE id = '<event_id>';
   ```

4. Descartar definitivamente (se o evento for irrecuperavel):

   ```sql
   UPDATE outbox_events SET status = 5
   WHERE id = '<event_id>';
   ```

5. Verificar que o FIFO do usuario retomou processando o proximo evento:

   ```sql
   SELECT id, status, attempts FROM outbox_events
   WHERE metadata->>'user_id' = '<user_id>'
   ORDER BY next_attempt_at ASC;
   ```

## 3. Triagem: Lag p95 alto (alert OutboxLagP95High)

Sintoma: `outbox_lag_seconds` p95 > 30s.

### Passos

1. Verificar ritmo de despacho:

   ```sql
   SELECT date_trunc('minute', published_at) AS minute,
          COUNT(*) AS dispatched
   FROM outbox_events
   WHERE status = 2
     AND published_at > NOW() - INTERVAL '10 minutes'
   GROUP BY 1 ORDER BY 1;
   ```

2. Verificar backlog:

   ```sql
   SELECT COUNT(*) FROM outbox_events WHERE status = 0;
   ```

3. Verificar se o dispatcher esta rodando (uma replica por servico):

   ```bash
   docker service ps mecontrola_worker-1 --filter "desired-state=running"
   ```

4. Se o backlog crescer indefinidamente, escalar replicas do worker temporariamente
   (revertendo para 2 replicas ativas):

   ```bash
   docker service scale mecontrola_worker-1=2
   ```

   Atencao: isso pode reativar dual-writer temporariamente; monitorar `outbox_claim_deferred_total`.

## 4. Triagem: Outbound vazio (alert WhatsAppOutboundEmpty)

Sintoma: `agents_whatsapp_inbound_total{outcome="no_reply"}` aumentou.

### Passos

1. Buscar spans no Tempo filtrando `outcome=no_reply` via traceparent propagado.
2. Verificar se o agente retornou `Content = ""` na `agent.Outcome`.
3. Verificar logs do consumer:

   ```bash
   docker service logs mecontrola_worker-1 2>&1 | grep "no_reply"
   ```

4. Reproduzir localmente enviando a mensagem do usuario que causou o problema.

## 5. Triagem: Timeout LLM/tool (alert WhatsAppInboundTimeoutHigh)

Sintoma: `agents_whatsapp_inbound_timeout_total` aumentou.

### Passos

1. Confirmar que o timeout configurado e menor que `STUCK_AFTER` (5m):

   ```bash
   grep AGENT_LLM_TIMEOUT deployment/compose/compose.swarm.yml
   grep STUCK_AFTER deployment/compose/compose.swarm.yml
   ```

2. Verificar latencia do OpenRouter no periodo:

   ```bash
   grep "openrouter" /var/log/mecontrola/worker-1.log | grep "duration_ms"
   ```

3. Se timeout sistematico, aumentar `AGENT_LLM_TIMEOUT` mantendo-o abaixo de `STUCK_AFTER - 30s`.

## 6. Referencias

- ADR-004: `.specs/prd-whatsapp-ordenacao-idempotencia/adr-004-observabilidade-e-deploy-seguro.md`
- Alertas: `docs/alerts/whatsapp-dead-letter.yaml`
- Dashboard: `docs/dashboards/mecontrola-observabilidade-whatsapp.json`
- PRD RF-13..22: `.specs/prd-whatsapp-ordenacao-idempotencia/prd.md`

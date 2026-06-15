# Runbook: Teste de Alertas Prometheus

**Objetivo:** Validar que cada regra de alerta em `deployment/monitoring/prometheus-rules.yaml`
dispara corretamente e chega ao destinatário via AlertManager.

## Pré-requisito

Stack de observabilidade rodando:
```bash
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  --profile observability up -d
```

Verificar:
- Prometheus: `curl -s http://localhost:9090/-/ready`
- AlertManager: `curl -s http://localhost:9093/-/ready`
- node-exporter: `curl -s http://localhost:9100/metrics | head -5`

## 1. DiskSpaceLow (disco > 80%)

Simular enchendo um disco temporário:
```bash
fallocate -l $(df / --output=avail | tail -1 | awk '{print ($1 - 100) * 1024}') /tmp/fill_disk.bin
```

Aguardar 5 minutos. Verificar alerta em `http://localhost:9090/alerts`.
Limpar: `rm /tmp/fill_disk.bin`

## 2. MemoryPressure (RAM > 90%)

Simular com stress-ng:
```bash
apt-get install -y stress-ng
stress-ng --vm 1 --vm-bytes 90% --timeout 300s &
```

Aguardar 5 minutos. Verificar alerta.
Limpar: `kill %1`

## 3. PostgreSQLDown

Parar o container postgres:
```bash
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  stop postgres
```

Aguardar 1 minuto. Verificar alerta `PostgreSQLDown` no Prometheus e e-mail no destinatário.
Restaurar: `docker compose ... start postgres`

## 4. HighErrorRate (5xx > 1%)

Gerar requisições com resposta 5xx:
```bash
for i in $(seq 1 200); do
  curl -sf "http://localhost:8080/api/v1/nonexistent-route-that-returns-500" || true
done
```

Aguardar 5 minutos (a regra usa janela de 5m). Verificar alerta.

## 5. SSLCertificateExpiringSoon

Simular certificado próximo da expiração (não é possível sem acesso ao cert). Alternativa:
alterar a regra temporariamente para `< 365 * 24 * 3600` e verificar se dispara para qualquer cert válido.

```bash
kubectl patch prometheusrule ... (ou editar /etc/prometheus/rules.yaml e reload)
curl -X POST http://localhost:9090/-/reload
```

Reverter após validação.

## 6. BackupTooOld (backup > 25h)

Verificar se a métrica existe:
```bash
curl -s http://localhost:9100/metrics | grep backup_last_success
```

Se não existir, o cron ainda não rodou. Executar manualmente:
```bash
sudo -u postgres pgbackrest --stanza=mecontrola --type=full backup
```

Para testar o alerta: editar `/var/lib/node_exporter/textfile_collector/pgbackrest.prom`
e definir um timestamp antigo (> 25h):
```bash
echo "backup_last_success_timestamp_seconds $(date -d '26 hours ago' +%s)" \
  | sudo tee /var/lib/node_exporter/textfile_collector/pgbackrest.prom
```

Aguardar 30 minutos (alerta tem `for: 30m`). Reverter após validação.

## 7. PostgreSQLCacheHitLow / PostgreSQLBufferHitLow

Verificar se postgres_exporter está exportando as métricas:
```bash
curl -s http://localhost:9187/metrics | grep pg_stat_database_blks
```

Se vazio, verificar permissões do usuário (exige `pg_monitor` role):
```bash
docker exec mecontrola-postgres-1 psql -U mecontrola mecontrola_db \
  -c "GRANT pg_monitor TO mecontrola;"
```

Para simular cache miss baixo: executar queries sequenciais intensas com `enable_seqscan=on`.

## Verificação de E-mail

Após cada alerta:
1. Verificar no Prometheus: `http://localhost:9090/alerts` — status deve ser `firing`
2. Verificar no AlertManager: `http://localhost:9093/#/alerts` — deve aparecer o alerta
3. Verificar inbox do e-mail configurado em `ALERTMANAGER_TO_EMAIL`

## Teste de Resolved

Após resolver a condição (restaurar serviço, liberar disco, etc.):
- O alerta deve aparecer como `resolved` no AlertManager
- E-mail de "RESOLVED" deve chegar ao destinatário

## Checklist de Validação

```
[ ] DiskSpaceLow — dispara + e-mail recebido + resolved após limpeza
[ ] MemoryPressure — dispara + e-mail recebido + resolved após liberar memória
[ ] PostgreSQLDown — dispara em < 2 min + e-mail recebido + resolved ao reiniciar
[ ] HighErrorRate — dispara após 5 min + e-mail recebido
[ ] SSLCertificateExpiringSoon — regra validada manualmente
[ ] BackupTooOld — dispara após 30 min com timestamp antigo + e-mail recebido
[ ] PostgreSQLCacheHitLow — métricas disponíveis no postgres_exporter
[ ] PostgreSQLBufferHitLow — métricas disponíveis no postgres_exporter
```

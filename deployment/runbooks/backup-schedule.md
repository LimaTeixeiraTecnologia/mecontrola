# Runbook: Agendamento e Verificação de Backup pgBackRest

**Referências:** `deployment/pgbackrest/pgbackrest.conf`, `deployment/pgbackrest/crontab.txt`, `deployment/scripts/pgbackrest-schedule.sh`, `deployment/runbooks/restore-pitr.md`

## Visão Geral

O backup pgBackRest é executado dentro do container postgres (imagem custom `mecontrola-postgres:*`) e persiste no S3 com criptografia AES-256. O agendamento é provisionado na VPS host por `pgbackrest-schedule.sh`, que instala um systemd-timer (ou cron fallback) chamando `docker exec` no container.

## Agendamento Ativo

| Tipo | Frequência | Horário UTC |
|------|-----------|-------------|
| Full | Semanal | Domingo 05:00 |
| Diferencial | Diária | Seg-Sáb 05:00 |
| Incremental | 6 em 6 horas | 00:00, 06:00, 12:00, 18:00 |
| Métricas | 30 em 30 min | Contínuo |

## Instalação do Agendamento (RF-01)

Execute uma única vez na VPS de produção como root:

```bash
cd /opt/mecontrola
bash deployment/scripts/pgbackrest-schedule.sh
```

O script é **idempotente**: pode ser re-executado sem efeitos colaterais.

Verificação pós-instalação:

```bash
# systemd (preferido)
systemctl list-timers --no-pager | grep pgbackrest

# cron fallback
cat /etc/cron.d/pgbackrest

# wrapper instalado
cat /usr/local/bin/pgbackrest-run.sh
```

## Verificação de Estado do Backup

```bash
# Info completo dos stanzas
PG_CONTAINER=$(docker ps --filter "name=mecontrola_postgres\\." --format "{{.Names}}" | head -1)

# Info completo dos stanzas
docker exec "$PG_CONTAINER" pgbackrest --stanza=mecontrola info

# Checagem de archive (retorna não-zero se archive-push tiver falha)
docker exec "$PG_CONTAINER" pgbackrest --stanza=mecontrola check
```

Saída esperada de `pgbackrest info`:
- Backup full: `backup type = full`, `backup stop time` ≤ 7 dias atrás
- Backup diff: `backup type = diff`, `backup stop time` ≤ 25h atrás

## Métricas e Alertas (RF-02)

O script `deployment/scripts/pgbackrest-backup-metrics.sh` exporta textfile para o node-exporter:

```
/var/lib/node_exporter/textfile_collector/pgbackrest.prom
```

Métricas exportadas:
- `pgbackrest_backup_age_seconds{type="full|diff|incr"}` — segundos desde o último backup
- `pgbackrest_backup_last_success_timestamp_seconds{type="..."}` — epoch do último backup
- `pgbackrest_archive_push_failed` — 1 se `pgbackrest check` falhou

**Alertas ativos:**
- `BackupFullStale` (critical): backup full > 7 dias
- `BackupDiffStale` (critical): backup diff > 25h
- `ArchivePushFailed` (critical): archive-push com falha
- `mc-backup-full-stale`, `mc-backup-diff-stale`, `mc-backup-archive-push-failed` (Grafana)

### Teste forçado de alerta de staleness

Para validar que o alerta dispara:

```bash
# Zerar o textfile simulando backup inexistente (valores 0 = age=999999s)
echo 'pgbackrest_backup_age_seconds{stanza="mecontrola",type="full"} 999999' \
  > /var/lib/node_exporter/textfile_collector/pgbackrest.prom

# Aguardar scrape (30s) e verificar alerta no Grafana
# Normalizar rodando o metrics collector novamente:
bash /opt/mecontrola/deployment/scripts/pgbackrest-backup-metrics.sh
```

## Guard de Imagem em Deploy (RF-03)

O deploy aborta automaticamente se `POSTGRES_IMAGE` não for a imagem custom:

**Em `deploy-swarm.sh`**: lê `deployment/config/prod.env` e falha se `POSTGRES_IMAGE` estiver vazio ou corresponder a `postgres:*`.

**Em `render-stack.py`**: bloqueia renderização quando `ENVIRONMENT=production` e `POSTGRES_IMAGE` não é a imagem custom.

Mensagem de erro esperada ao usar imagem default:
```
ERRO: deploy abortado — POSTGRES_IMAGE nao e a imagem custom com pgBackRest
      Valor: 'postgres:16-alpine' resolve para imagem sem pgBackRest
      Producao exige mecontrola-postgres:<tag>. Configure POSTGRES_IMAGE em deployment/config/prod.env
```

Para corrigir: definir `POSTGRES_IMAGE=ghcr.io/limateixeiratecnologia/mecontrola-postgres:<tag>` em `deployment/config/prod.env`.

## Construção de Nova Imagem Custom

A imagem `mecontrola-postgres` é buildada em `.github/workflows/ci-cd.yml` (job `build-image`). Para forçar rebuild:

```bash
# Tag a ser registrada em prod.env após o build
export IMAGE_TAG="$(date -u +%Y%m%d-%H%M%S)"
docker build -t mecontrola-postgres:${IMAGE_TAG} deployment/postgres/
```

## Troubleshooting

| Sintoma | Causa provável | Ação |
|---------|---------------|------|
| Timer inativo (`systemctl list-timers` vazio) | `pgbackrest-schedule.sh` não rodado | Rodar o script como root |
| `pgbackrest check` falha | Conectividade S3 perdida ou `archive_mode=off` | Verificar `PGBACKREST_S3_KEY`, reconectar S3 |
| `pgbackrest_backup_age_seconds` = 999999 | Script selecionou o container `postgres-exporter` em vez do `postgres`, ou `pgbackrest info` falhou | Verificar filtro do container (`mecontrola_postgres\\.`) e rodar `pgbackrest info` manualmente no container correto |
| Deploy falha com "POSTGRES_IMAGE" | `prod.env` com imagem default | Atualizar `POSTGRES_IMAGE` no `prod.env` com tag atual da imagem custom |
| Textfile vazio após 30 min | Container postgres parado | Verificar `docker ps`, reiniciar serviço se necessário |

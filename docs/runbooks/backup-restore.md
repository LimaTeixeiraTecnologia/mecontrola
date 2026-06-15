# Runbook — Restore de Backup + Smoke Queries

## Visão Geral

Este runbook documenta o procedimento de restore do backup criptografado do banco de dados
mecontrola em um container Postgres efêmero, incluindo smoke queries de validação.

O script `deployment/scripts/pg-restore-smoke.sh` automatiza todo o fluxo:
baixar dump cifrado (rclone) → descriptografar (age) → restaurar (Postgres efêmero) →
smoke queries (3 tabelas críticas) → cleanup.

---

## Pré-requisitos

### Binários no host de execução

| Ferramenta | Versão mínima | Instalação |
|---|---|---|
| `rclone` | v1.65+ | `curl https://rclone.org/install.sh | sudo bash` |
| `age` | v1.1+ | `brew install age` / `apt install age` / binário em https://github.com/FiloSottile/age/releases |
| `docker` | v24+ | https://docs.docker.com/get-docker/ |
| `psql` | 15+ | `apt install postgresql-client` / `brew install libpq` |

Verificação rápida:
```bash
for cmd in rclone age docker psql; do command -v "$cmd" && "$cmd" --version 2>&1 | head -1 || echo "AUSENTE: $cmd"; done
```

### Chave privada age

A chave privada age (`key.txt`) deve estar disponível no host. A chave pública correspondente
(`AGE_RECIPIENT`) foi usada para criptografar o dump via `pg-dump.sh`.

```bash
# Verificar que a chave existe e é legível
ls -la /etc/age/key.txt
```

Nunca versionar a chave privada. Armazenar em volume seguro ou secret manager.

### Configuração rclone

O remote `backup` deve estar configurado apontando para o bucket de armazenamento offsite:

```bash
rclone config show backup
rclone lsf backup:mecontrola-backups/ | tail -5
```

### Variáveis de ambiente

Criar `/etc/pg-restore.env` (ou exportar antes de executar):

```bash
BACKUP_REMOTE=backup:mecontrola-backups
AGE_KEY_FILE=/etc/age/key.txt
POSTGRES_DB=mecontrola_db
POSTGRES_USER=mecontrola
RESTORE_PORT=15432
SMOKE_SCHEMA=mecontrola
POSTGRES_IMAGE=postgres:16-alpine
# Opcional: comando de alerta em caso de falha (ex: curl para Slack/Grafana OnCall)
ALERT_CMD=
```

---

## Execução Manual

```bash
# Opção A — via variáveis de ambiente já exportadas
bash deployment/scripts/pg-restore-smoke.sh

# Opção B — apontando para env file customizado
PG_RESTORE_ENV_FILE=/etc/pg-restore.env bash deployment/scripts/pg-restore-smoke.sh

# Opção C — via Task (recomendado localmente)
task security:backup-restore-smoke
```

Saída esperada em caso de sucesso:

```
[2026-06-12T00:00:00Z] === pg-restore-smoke: inicio ===
[2026-06-12T00:00:00Z] Pre-requisitos OK: rclone, age, docker, psql
[2026-06-12T00:00:00Z] Localizando ultimo dump em backup:mecontrola-backups...
[2026-06-12T00:00:00Z] Baixando mecontrola_db_20260612T000000Z.sql.gz.age de backup:mecontrola-backups...
[2026-06-12T00:00:00Z] Download concluido: /tmp/pg-restore-smoke/latest.age (12M)
[2026-06-12T00:00:00Z] Descriptografando com age...
[2026-06-12T00:00:00Z] Descriptografia concluida: /tmp/pg-restore-smoke/dump.sql (58M)
[2026-06-12T00:00:00Z] Subindo container Postgres efemero pg-restore-smoke-12345 na porta 15432...
[2026-06-12T00:00:00Z] Postgres pronto apos 3s
[2026-06-12T00:00:00Z] Restaurando dump em mecontrola_db...
[2026-06-12T00:00:00Z] Restore concluido
[2026-06-12T00:00:00Z] Executando smoke queries no schema mecontrola...
[2026-06-12T00:00:00Z] OK: mecontrola.users count=142
[2026-06-12T00:00:00Z] OK: mecontrola.cards count=318
[2026-06-12T00:00:00Z] OK: mecontrola.transactions count=5042
[2026-06-12T00:00:00Z] Smoke queries OK: users cards transactions
[2026-06-12T00:00:00Z] === pg-restore-smoke: SUCESSO — exit 0 ===
```

---

## Cron de validação automática

Configurar no host de staging (não na VPS de produção — restore consome recursos):

    sudo crontab -e

Linha:

    0 4 * * 0 root PG_RESTORE_ENV_FILE=/etc/pg-restore.env bash /opt/mecontrola/pg-restore-smoke.sh >> /var/log/pg-restore-smoke.log 2>&1

Frequência: domingo 04:00 UTC. Alerta de falha via ALERT_CMD do env file
(ex: curl -s -X POST <webhook>).

## Smoke queries cobertas

- count(*) > 0 em users, cards, transactions
- JOIN transactions.user_id = users.id (prova FK válida no dump)
- max(created_at) em transactions (freshness — alerta se > 7 dias)
- public.schema_migrations: versão mais alta aplicada

## Idempotência

Cada execução usa porta TCP aleatória (20000-24999) e container nomeado por PID,
permitindo execuções paralelas e re-runs sem colisão.

### Verificação e teste manual

Verificar que o cron foi registrado:

```bash
crontab -l | grep pg-restore-smoke
```

Testar a linha cron manualmente antes de aguardar o agendamento:

```bash
PG_RESTORE_ENV_FILE=/etc/pg-restore.env bash /opt/mecontrola/deployment/scripts/pg-restore-smoke.sh
echo "Exit code: $?"
```

### Alertas de falha

O script aceita `ALERT_CMD` — qualquer comando shell executado em caso de falha (exit != 0).
Exemplos:

```bash
# Slack via webhook
ALERT_CMD='curl -s -X POST "$SLACK_WEBHOOK" -H "Content-type: application/json" -d "{\"text\":\"[ALERTA] pg-restore-smoke falhou em staging\"}"'

# Email via sendmail
ALERT_CMD='echo "pg-restore-smoke falhou em staging" | mail -s "[ALERTA] Backup restore" ops@mecontrola.com.br'

# Grafana OnCall via webhook API
ALERT_CMD='curl -s -X POST "$GRAFANA_ONCALL_URL" -H "Authorization: Basic $GRAFANA_ONCALL_TOKEN" -d "{\"title\":\"pg-restore-smoke falhou\"}"'
```

---

## Critérios de Sucesso

O restore é considerado bem-sucedido quando:

1. Script finaliza com exit 0.
2. Logs mostram `SUCESSO` na última linha.
3. Smoke queries retornam `count > 0` para as três tabelas: `users`, `cards`, `transactions`.
4. Container Postgres efêmero foi removido (`docker ps | grep pg-restore-smoke` vazio).
5. Arquivo `/tmp/pg-restore-smoke/dump.sql` foi removido (dump cifrado mantido para auditoria).

---

## Troubleshooting

### Chave age inválida ou ausente

```
ERROR: Chave age nao encontrada: /etc/age/key.txt
```

Verificar:
```bash
ls -la /etc/age/key.txt
head -1 /etc/age/key.txt  # deve ser: AGE-SECRET-KEY-...
```

Testar descriptografia manual:
```bash
age -d -i /etc/age/key.txt /tmp/pg-restore-smoke/latest.age | gunzip | head -5
```

### Espaço em disco insuficiente

```
gunzip: write error: No space left on device
```

Verificar espaço em `/tmp`:
```bash
df -h /tmp
```

Usar diretório alternativo com mais espaço:
```bash
WORK_DIR=/data/restore PG_RESTORE_ENV_FILE=/etc/pg-restore.env bash deployment/scripts/pg-restore-smoke.sh
```

### Postgres não sobe (porta ocupada)

```
ERROR: Postgres nao ficou pronto em 30s
```

Verificar se a porta já está em uso:
```bash
ss -tlnp | grep 15432
docker ps | grep pg-restore-smoke
```

Liberar e usar porta alternativa:
```bash
docker rm -f pg-restore-smoke-* 2>/dev/null || true
RESTORE_PORT=25432 PG_RESTORE_ENV_FILE=/etc/pg-restore.env bash deployment/scripts/pg-restore-smoke.sh
```

### Nenhum dump encontrado no bucket

```
ERROR: Nenhum dump .sql.gz.age encontrado em backup:mecontrola-backups
```

Verificar conteúdo do bucket:
```bash
rclone lsf backup:mecontrola-backups/ --format "tp"
```

Verificar configuração do rclone remote:
```bash
rclone config show backup
rclone about backup:
```

### Smoke query falhou (tabela não encontrada)

```
ERROR: Smoke query falhou: mecontrola.users — tabela nao encontrada ou query com erro
```

Verificar se o schema existe no dump restaurado:
```bash
PGPASSWORD=<restore_password> psql -h 127.0.0.1 -p 15432 -U mecontrola -d mecontrola_db \
  -c "\dn" \
  -c "SELECT tablename FROM pg_tables WHERE schemaname='mecontrola' ORDER BY tablename;"
```

Se o schema for diferente, ajustar `SMOKE_SCHEMA`:
```bash
SMOKE_SCHEMA=public PG_RESTORE_ENV_FILE=/etc/pg-restore.env bash deployment/scripts/pg-restore-smoke.sh
```

---

## Referências

- Script: `deployment/scripts/pg-restore-smoke.sh`
- Backup script: `deployment/scripts/pg-dump.sh`
- Receita Task: `task security:backup-restore-smoke` (ver `taskfiles/security.yml`)
- Requisitos: RF-11, RF-12, RF-13, RF-14 em `.specs/prd-pre-golive-hardening/prd.md`

# Tarefa 6.0: Ajustar Configuração do PostgreSQL, pgBouncer e pgBackRest

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ajustar as configurações de PostgreSQL, pgBouncer e pgBackRest para atender aos requisitos de PITR, proteção contra queries runaway, pooling e backup off-VM.

<requirements>
- Cobrir RF-13: pool de conexões dimensionado para 2+2 réplicas + jobs + admin.
- Cobrir RF-14: `archive_mode=on`, `statement_timeout=30s`.
- Cobrir RF-15: backups full semanal, diff diário, incr a cada 6h, retenção 30 dias.
- Cobrir RF-16: backups off-VM no AWS S3 com criptografia.
</requirements>

## Subtarefas

- [ ] 6.1 Atualizar `deployment/postgres/postgresql.conf`:
  - `max_connections = 100`
  - `superuser_reserved_connections = 3`
  - `wal_level = replica`
  - `archive_mode = on`
  - `archive_command = 'pgbackrest --stanza=mecontrola archive-push %p'`
  - `statement_timeout = 30s`
  - tuning de memória para KVM2.
- [ ] 6.2 Atualizar `deployment/pgbouncer/pgbouncer.ini`:
  - `max_client_conn = 300`
  - `default_pool_size = 15`
  - `max_db_connections = 60`
- [ ] 6.3 Revisar `deployment/pgbackrest/pgbackrest.conf`:
  - bucket, região, retention, criptografia.
- [ ] 6.4 Criar crontab em `deployment/pgbackrest/crontab.txt` para full/diff/incr.
- [ ] 6.5 Executar `pgbackrest-setup.sh` fase 1 e 2 em staging.
- [ ] 6.6 Testar restore PITR em instância isolada.

## Detalhes de Implementação

Ver seção "5. Banco de Dados e pgBackRest" de `techspec.md`. `archive_mode=on` requer reinício do PostgreSQL. O tuning de memória deve ser conservador para KVM2:

```conf
shared_buffers = 512MB
effective_cache_size = 1.5GB
work_mem = 16MB
maintenance_work_mem = 128MB
```

Cron:

```cron
# Full semanal (domingo 02:00 BRT -> 05:00 UTC)
0 5 * * 0 ... pgbackrest --type=full backup
# Diff diário
0 5 * * 1-6 ... pgbackrest --type=diff backup
# Incremental a cada 6h
0 */6 * * * ... pgbackrest --type=incr backup
```

## Critérios de Sucesso

- `pgbackrest check` retorna sucesso.
- `pgbackrest info` mostra backups full/diff/incr no S3.
- `SHOW statement_timeout;` no PostgreSQL retorna `30s`.
- `SHOW archive_mode;` retorna `on`.
- pgBouncer aceita conexões de 2 servers + 2 workers sem estourar `max_db_connections`.
- Restore PITR funciona em instância de teste.

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste de backup full/diff/incr em staging.
- [ ] Teste de restore PITR em container isolado.
- [ ] Teste de carga de conexões via pgbouncer.
- [ ] Teste de query runaway: executar `SELECT pg_sleep(60)` e verificar timeout em 30s.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/postgres/postgresql.conf`
- `deployment/pgbouncer/pgbouncer.ini`
- `deployment/pgbackrest/pgbackrest.conf`
- `deployment/scripts/pgbackrest-setup.sh`
- `deployment/pgbackrest/crontab.txt` (novo)

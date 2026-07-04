# Plano — Diagnóstico de Capacidade do PostgreSQL + S3 para 10k/15k (KVM2 Hostinger)

## Context

Executar o prompt `docs/prompts/2026-07-03-prompt-diagnostico-postgres-s3-10k-15k.md`: diagnóstico **100% read-only, zero implementação**, para determinar se o PostgreSQL de produção (Swarm single-node na KVM2 da Hostinger, 2vCPU/8GB, IP 187.77.45.48) — com pgBouncer, pgBackRest e backup/PITR em S3 — suporta 4 alvos avaliados separadamente: `10k ativos/dia`, `15k ativos/dia`, `10k simultâneos em pico`, `15k simultâneos em pico`. Hoje há 0 usuários. Veredito fechado por alvo (`atende hoje` / `atende com ajustes obrigatórios` / `não atende` / `não comprovado`), sem falso positivo: o que não tiver prova objetiva vira `não comprovado`/gap explícito.

Fatos já confirmados no pré-check:
- SSH read-only funciona (`root@187.77.45.48`, host `srv1761537`, up 17 dias, load 0.05).
- Stack real no Swarm: caddy, otel-lgtm, node-exporter, pg-tunnel, pgbouncer 1.25.2, postgres (imagem custom `mecontrola-postgres:mastra-20260629`), postgres-exporter, **server-1/server-2, worker-1/worker-2** (2+2 réplicas — mais processos app que o assumido em auditorias anteriores).
- **`mecontrola_migrate` está `0/1`** — sinal de drift/falha já conhecido (memória: config validation + baseline rename); precisa ser reconfirmado e classificado como achado.
- Todos os arquivos-fonte citados no prompt existem, incluindo `deployment/pgbackrest/crontab.txt` (agendamento) e `docs/runs/2026-07-03-evidencia-restore.md` (possível prova de restore) — ambos posteriores às auditorias que apontavam "sem agendamento / restore nunca testado". Não assumir os achados antigos: revalidar.

## Mandatos (herdados do prompt — inegociáveis)

- Nenhuma escrita no host, nenhum deploy/restart/tuning/migração/backup/restore em produção.
- Só comandos read-only via SSH (inspeção SO/Swarm/configs/logs/métricas + SQL read-only `pg_settings`/`pg_stat_*`/`pg_database_size` + `pgbackrest info/check` read-only).
- Toda afirmação com evidência (host, repo `file:line`, ou doc oficial). Sem benchmark de blog. Sem prova → `não comprovado`.
- Partir de `cmd/server/server.go` e `cmd/worker/worker.go` para a pressão da app; proibido partir de `internal/platform/runtime`.
- Drift host ↔ `deployment/` = achado crítico.

## Execução (5 frentes, paralelizáveis via subagents onde marcado)

### Fase 1 — Topologia real em produção (SSH)
Comandos read-only: `docker service ls/ps/inspect` (postgres, pgbouncer, server-1/2, worker-1/2, migrate, pg-tunnel), `docker config ls`, inspect de mounts/volumes/networks/healthchecks/limits, versão real do Postgres (`SHOW server_version`), versão pgBouncer, portas publicadas. Confirmar a qual endpoint server/worker/migrate se conectam (env `DATABASE_*`/DSN não sensível + código de bootstrap) → mapa `app → pgbouncer → postgres → volume → pgbackrest → S3`. Registrar drift vs `compose.swarm.yml` (inclui migrate 0/1 e a contagem 2×server+2×worker).

### Fase 2 — Orçamento real de capacidade (SSH)
- Host: `nproc`, `free`, `df`, `vmstat/iostat` (se disponíveis), load, `docker stats --no-stream`.
- Postgres (SQL read-only via container): `pg_settings` efetivos, `pg_database_size`, top tabelas/índices (`pg_stat_user_tables`, `pg_relation_size`), conexões (`pg_stat_activity` por estado), autovacuum (`pg_stat_user_tables.last_autovacuum`), checkpoints/WAL (`pg_stat_bgwriter`/`pg_stat_wal`), `archive_command` status (`pg_stat_archiver`).
- pgBouncer: `SHOW POOLS; SHOW STATS; SHOW CLIENTS; SHOW CONFIG` via psql no pgbouncer (read-only).
- Análise de coerência: `max_connections=100` / `max_db_connections=30` / `default_pool_size=20` vs **2 servers + 2 workers** e o pool Go configurado no código (max open conns por processo).

### Fase 3 — Backup/PITR/restore: configurado vs comprovado (SSH + repo)
- `pgbackrest info` (inventário full/diff/incr, timestamps, tamanhos), `pgbackrest check` se seguro, `pg_stat_archiver` (archived_count, failed_count, last_archived_time), backlog de spool/queue, crontab real no host (`crontab -l`, `/etc/cron*`) vs `deployment/pgbackrest/crontab.txt`.
- Ler `docs/runs/2026-07-03-evidencia-restore.md` e julgar criteriosamente: restore/PITR **real executado** (onde? com que dados? RTO medido?) ou apenas projeção/runbook. Isso decide se recovery é `comprovado` ou `não comprovado` — não confundir com capacidade de tráfego.

### Fase 4 — Pressão real da aplicação sobre o banco (codebase — subagent Explore)
Partindo de `cmd/server/server.go` + `cmd/worker/worker.go` (+ `whatsapp_wiring.go`): enumerar entrypoints HTTP/webhook que tocam banco, workers/jobs/outbox (dispatcher, reaper, claim particionado), fan-out e concorrência (nº workers × goroutines), configuração do pool `database/sql` por processo, superfícies de contenção (advisory locks, claim `FOR UPDATE`, embeddings, workflow_runs), padrão de escrita por mensagem WhatsApp (nº de transações/inserts por inbound). Derivar req/s e tx/s estimados por alvo a partir do fluxo real — sem inventar taxa: onde faltar dado de comportamento de usuário (msgs/dia/usuário), declarar premissa explícita ou `não comprovado`.

### Fase 5 — Síntese por alvo + relatório
- Cruzar Fases 1–4 com docs oficiais (PostgreSQL 16, pgBouncer, pgBackRest, Docker Swarm, S3 durability) apenas para limites/semântica de componente, nunca como benchmark de capacidade.
- Ler os relatórios anteriores (`2026-07-03-relatorio-analise-infra-hostinger-kvm2-10k.md`, `2026-07-03-evidencia-carga-envelopes.md`, `2026-07-03-orcamento-recursos.md`, `relatorio-auditoria-producao.md`) como insumo, revalidando o que mudou.
- Produzir o relatório no formato obrigatório do prompt (seções 1–11, todas as tabelas, classificação fechada por alvo, 10 perguntas finais respondidas explicitamente).

## Entregável

`docs/runs/2026-07-03-diagnostico-capacidade-postgres-s3-10k-15k.md` — relatório completo no formato mandatório do prompt (única escrita permitida, no repo local; nada no host).

## Verificação

- Checklist contra o prompt: todas as 11 seções presentes, 4 alvos com classificação fechada, cada linha de tabela com evidência (comando/saída do host, `file:line` do repo, ou doc oficial nomeada).
- Nenhum comando não-read-only executado no host (auditável pelo transcript).
- Perguntas finais do §11 respondidas uma a uma, incluindo "foi possível fechar 0 gaps?" com resposta honesta.

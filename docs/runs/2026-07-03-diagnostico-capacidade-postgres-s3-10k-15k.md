# Diagnóstico de Capacidade do PostgreSQL

Data: 2026-07-03
Prompt de origem: `docs/prompts/2026-07-03-prompt-diagnostico-postgres-s3-10k-15k.md`
Método: auditoria read-only (SSH prod + repositório + codebase), zero mudança em produção.

---

## 1. Escopo e método

**Objetivo.** Determinar, com prova objetiva e sem falso positivo, se o PostgreSQL de produção — como está construído e operado (pgBouncer + pgBackRest + S3, Docker Swarm single-node, Hostinger KVM 2) — suporta 4 alvos avaliados separadamente: `10k ativos/dia`, `15k ativos/dia`, `10k simultâneos em pico`, `15k simultâneos em pico`. Estado de negócio atual: **0 usuários ativos** (confirmado: pgBouncer logando `0 xacts/s, 0 queries/s`; tabelas de negócio com 0–1 linhas).

**Ambiente analisado.**
- Host: `ssh root@187.77.45.48` (`srv1761537`), Ubuntu kernel 6.8.0-124, Docker Swarm single-node, up 17 dias.
- Repositório prod: `/opt/mecontrola` em `ef7b31e` — **idêntico ao HEAD local** (zero deploy-lag).
- Repositório local: working tree `mecontrola` @ `ef7b31e`.

**Comandos read-only executados no host** (íntegra, nenhum comando de escrita/restart/deploy):
`nproc`, `free -m`, `df -h`, `cat /proc/loadavg`, `swapon --show`, `uname -r`, `lsblk`, `sar -d`, `ss -tlnp`, `ufw status`, `iptables -L INPUT -n`, `ps aux` (runner CI), `crontab -l`, `ls /etc/cron.d`, `systemctl list-timers`, `docker service ls/ps/inspect`, `docker stats --no-stream`, `docker service logs --tail`, `docker exec <postgres> psql` (somente `SHOW`, `pg_settings`, `pg_stat_*`, `pg_database_size`, `pg_control_checkpoint`, `schema_migrations`), `docker exec <postgres> pgbackrest info` e `pgbackrest check` (verificação sem backup novo), leitura de configs montadas (`pgbouncer.ini`, `pgbackrest.conf`). Do host local: `nc -z 187.77.45.48 15432/5432` (teste de conectividade TCP, sem autenticação).

**Arquivos do repositório analisados.** `AGENTS.md`, `go.mod` (Go 1.26.4, `robfig/cron/v3`), `README.md` (§ Backup/restore), `cmd/server/server.go`, `cmd/server/whatsapp_wiring.go`, `cmd/worker/worker.go`, `cmd/migrate/migrate.go`, `configs/config.go`, `deployment/compose/compose.swarm.yml`, `deployment/postgres/postgresql.conf`, `deployment/pgbouncer/pgbouncer.ini`, `deployment/pgbackrest/pgbackrest.conf`, `deployment/pgbackrest/crontab.txt`, `deployment/scripts/pgbackrest-schedule.sh`, `deployment/runbooks/{deploy,restore-pitr,restore-vps,backup-schedule}.md`, `deployment/monitoring/`, `internal/platform/outbox/{dispatcher,storage_postgres}.go`, `internal/platform/worker/job/{scheduler,adapter,types}.go`, `internal/platform/agent/runtime.go`, `internal/platform/database/postgres/postgres.go`, `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`, `migrations/000001_initial_schema.up.sql`, `deployment/config/prod.env`, `docs/runs/2026-07-03-relatorio-analise-infra-hostinger-kvm2-10k.md`, `docs/runs/2026-07-03-evidencia-restore.md`, `docs/runs/2026-07-03-evidencia-carga-envelopes.md`, `docs/runs/2026-07-03-orcamento-recursos.md`, `docs/runs/2026-07-03-relatorio-auditoria-producao.md`.

**Documentações oficiais consultadas (semântica de componente, nunca benchmark).**
- PostgreSQL 16 — Resource Consumption (`shared_buffers`, `work_mem`), Continuous Archiving & PITR (`archive_command`, `archive_timeout`, `recovery_target_timeline` default `latest`), `pg_stat_archiver`/`pg_stat_checkpointer`.
- pgBouncer — configuração (`pool_mode=transaction` e suas limitações de estado de sessão; `default_pool_size`, `max_db_connections`, `admin_users`).
- pgBackRest User Guide — retenção (`repo1-retention-full/diff`), archiving assíncrono (`archive-async`, spool), repositório S3, cifra AES-256-CBC.
- Docker — Swarm resources (limits/reservations), publicação de portas e interação iptables/ufw ("Packet filtering and firewalls": portas publicadas contornam regras ufw de INPUT).
- AWS S3 — durabilidade de design 99,999999999% (11 noves) do objeto armazenado.

**Regras de decisão.** (1) Toda célula de tabela ancorada em evidência host, `file:line` ou doc oficial; (2) configuração ≠ prova de capacidade; (3) backup configurado ≠ restore comprovado; (4) ausência de prova ⇒ `não comprovado`; (5) drift host↔repo ⇒ achado crítico; (6) alvos avaliados separadamente, sem mistura.

---

## 2. Topologia real comprovada

| componente | funcao | onde roda | como se conecta | persistencia | limite declarado | evidencia no host | evidencia no repo |
|---|---|---|---|---|---|---|---|
| postgres 16.14 | banco primário (único) | Swarm 1/1, imagem custom `mecontrola-postgres:mastra-20260629-191935` | recebe de pgbouncer, exporter, pg-tunnel, migrate | volume `mecontrola_postgres-data`; conf bind ro | cpus 1.0 / mem 2G (res. 0.25/512M) | `docker service inspect` + `SHOW server_version`=16.14 | `compose.swarm.yml:118-148` |
| pgbouncer 1.25.2 | pool transaction | Swarm 1/1, `edoburu/pgbouncer:v1.25.2-p0` | `postgres:5432`; escuta `:6432` | stateless | cpus 0.25 / 128M | inspect + ini efetivo lido no container | `compose.swarm.yml:150-189` |
| server-1/server-2 | API HTTP + webhooks | Swarm 1/1 cada (2 réplicas) | `DB_HOST=pgbouncer:6432`, `DB_MAX_CONNS=10` | stateless (read_only, tmpfs) | cpus 0.75 / 512M cada | env do service inspecionado | `compose.swarm.yml:285-377` |
| worker-1/worker-2 | jobs + outbox dispatcher | Swarm 1/1 cada (2 réplicas) | `DB_HOST=pgbouncer:6432`, `DB_MAX_CONNS=10` | stateless | cpus 0.50 / 256M cada | env do service inspecionado | `compose.swarm.yml:379-469` |
| migrate | migrações one-shot | Swarm 0/1 (`restart: none`) | **`DB_HOST=postgres:5432` direto** | — | cpus 0.25 / 256M | `docker service ps`: `Complete 2 minutes ago` após 3 `Failed` hoje | `compose.swarm.yml:247-283` |
| pg-tunnel (socat) | acesso remoto ao banco | Swarm 1/1 | `postgres:5432`; **publica `15432` host mode** | stateless | cpus 0.05 / 16M | porta 15432 em LISTEN via docker-proxy; **acessível da internet (nc OK)** | `compose.swarm.yml:558-596` |
| postgres-exporter | métricas | Swarm 1/1 | `postgres:5432` direto | stateless | 0.25 / 128M | inspect + conexões em `pg_stat_activity` | `compose.swarm.yml:191-215` |
| caddy | TLS/proxy | Swarm 1/1 | server-1/2; publica 80/443 host mode | volume caddy-data | 0.25 / 128M | inspect + `ss` | `compose.swarm.yml:471-509` |
| otel-lgtm | observabilidade | Swarm 1/1 | recebe OTLP; publica 3000 | 4 volumes | 1.0 / 1228M | `docker stats`: 495MiB/1.2GiB (maior consumidor) | `compose.swarm.yml:511-556` |
| pgBackRest | backup/PITR | dentro do container postgres | S3 `mecontrola-backups-660838763799-use1` (us-east-1), AES-256-CBC | repo em S3; spool local | — | `pgbackrest info`: stanza ok; `check` OK em 3,7s | `pgbackrest.conf` (repo=container, idênticos) |

**Mapa real do caminho de conexão (comprovado):**
`server/worker (2+2 réplicas, 10 conns máx cada) → pgbouncer:6432 (transaction, pool 20, max_db 30) → postgres:5432 → volume local mecontrola_postgres-data → pgbackrest (archive-push assíncrono por WAL) → S3 us-east-1 (AES-256-CBC)`.
Fora desse caminho, conectam **direto ao postgres:5432**: migrate, postgres-exporter e pg-tunnel (este último com 6 conexões DBeaver idle observadas em `pg_stat_activity`).

**Drifts host↔repo detectados:**
1. **CRÍTICO** — `deployment/runbooks/backup-schedule.md` declara "Agendamento Ativo" (full dom 05:00, diff seg–sáb, incr 6/6h via `pgbackrest-schedule.sh`); no host **não existe** `crontab` root, `/etc/cron.d/pgbackrest`, systemd-timer nem `/usr/local/bin/pgbackrest-run.sh`. O script existe no repo e **nunca foi executado na VPS**.
2. `deployment/pgbouncer/pgbouncer.ini` (referência host-based) declara `admin_users=mecontrola`; o efetivo no container é `admin_users=postgres` (gerado por env da imagem edoburu). Consequência: `SHOW POOLS/STATS` negado ao usuário da aplicação (`FATAL: not allowed`) — lacuna de observabilidade do pooler.
3. `mecontrola_migrate 0/1` não é drift: é one-shot com `restart_policy: none` (`compose.swarm.yml:270-271`); última execução `Complete` hoje, `schema_migrations=2, dirty=f`. As 3 falhas anteriores hoje precederam o fix `ef7b31e`.
4. CI runner GitHub Actions roda no host de produção (`/home/github-runner` + `/opt/actions-runner`, processo `Runner.Listener` ativo) — não declarado em `deployment/`.

---

## 3. Prova do host e do orçamento de recursos

| recurso | capacidade observada no host | consumo atual | limite/reservation declarados | margem | status |
|---|---|---|---|---|---|
| CPU | 2 vCPU (`nproc`), QEMU | load 0.17 / 0.17 / 0.17 | soma de limits ≈ 5.05 cpus (oversubscribed 2,5×) | ampla em idle; limits não são prova sob carga | ok hoje; **não comprovado sob carga** |
| RAM | 7 940 MB | 1 818 MB usados; 6 122 MB disponíveis | soma de limits steady-state 5 340 MB (orçamento aprovado em `docs/runs/2026-07-03-orcamento-recursos.md`) | ~1,1 GB sobre o orçamento | ok |
| Swap | 4 096 MB | 768 KB usados | — | intocado | ok |
| Disco | 96 GB (sda, não-rotacional/NVMe declarado Hostinger) | 17 GB usados / 80 GB livres (18%) | — | 80 GB | ok hoje; ver crescimento §8 |
| IO | `sar -d`: sda média 12 tps, %util ~0,1, await 0,27 ms | idle | — | sem medição sob carga (pgbench proibido) | **não comprovado** para 10k/15k |
| postgres (container) | limite 2 GiB | 126,4 MiB (6,2%) | 1.0 cpu / 2G | 1,9 GiB | ok (banco de 12 MB) |
| otel-lgtm | limite 1,2 GiB | 495 MiB (40%) | 1.0 cpu / 1228M | maior consumidor do host em idle | risco de co-locação com sampling 100% (§9) |
| server/worker (4) | limites 512M/256M | 11–12 MiB cada | 0.75/0.50 cpu | ampla | ok em idle |

Banco atual: **12 MB** (`pg_database_size`). Conexões abertas: 1 active + 13 idle + 6 background — detalhe: 7 server-conns idle do pgbouncer (min_pool_size=5 + exporter), 6 DBeaver via pg-tunnel, dentro de `max_connections=100` com folga.

---

## 4. Prova da configuracao do PostgreSQL

Origem dupla comprovada: `deployment/postgres/postgresql.conf` (bind ro no container, `compose.swarm.yml:130`) ↔ `pg_settings` no host — **sem divergência** em nenhum parâmetro verificado.

| parametro | valor atual (host) | origem | impacto | adequado? |
|---|---|---|---|---|
| server_version | 16.14 | imagem custom | major suportada, patch atual da série 16 | sim |
| max_connections | 100 | postgresql.conf | teto físico; app chega via pool de 30 | sim para topologia atual |
| shared_buffers | 512MB (65536×8kB) | postgresql.conf | 25% do limite 2G do container | sim |
| effective_cache_size | 1536MB | postgresql.conf | coerente com limite 2G | sim |
| work_mem | 16MB | postgresql.conf | 16MB×sorts concorrentes; com ≤30 backends, pior caso teórico ~480MB+ | sim com pool 30; monitorar |
| maintenance_work_mem | 128MB | postgresql.conf | autovacuum/index build | sim |
| wal_level=replica, archive_mode=on | confirmados | postgresql.conf | habilita PITR | sim |
| archive_command | `pgbackrest --stanza=mecontrola archive-push %p` | postgresql.conf | WAL→S3 | sim (operacional, §6) |
| archive_timeout | 600s | postgresql.conf | RPO teto 10 min em idle | sim |
| max_wal_size / min_wal_size | 2GB / 256MB | postgresql.conf | espaçamento de checkpoints | sim |
| checkpoint_timeout / completion_target | 300s / 0.9 | postgresql.conf | 1423 timed vs 13 requested (`pg_stat_checkpointer`) = checkpoints saudáveis, não forçados | sim |
| autovacuum | on | default/conf | manutenção | sim (sem carga p/ avaliar tuning) |
| random_page_cost | 1.1 | postgresql.conf | adequado a SSD | sim |
| extensões | vector 0.8.3, pg_trgm, pgcrypto, unaccent | `pg_extension` | pgvector p/ embeddings HNSW | sim |
| schema_migrations | version=2, dirty=f | tabela | migrations 100% aplicadas (fix ef7b31e) | sim |

WAL gerado desde o start de hoje (19:47 UTC): 89 739 registros / 15 MB — desprezível com 0 usuários; sem base para extrapolar sob carga (marcado como dado ausente em §8).

---

## 5. Prova da configuracao do pgBouncer

Valores **efetivos** lidos do `/etc/pgbouncer/pgbouncer.ini` dentro do container (gerado por env, `compose.swarm.yml:153-165`):

| parametro | valor atual | origem | impacto | adequado? |
|---|---|---|---|---|
| pool_mode | transaction | compose env | multiplexa 40 client conns em ≤20 server conns; **exige** `pg_advisory_xact_lock` (já adotado) e proíbe estado de sessão | sim |
| max_client_conn | 300 | compose env | teto de clientes; app usa ≤40 (4×10) + jobs | sim |
| default_pool_size | 20 | compose env | 20 backends para transações curtas; em multiplexação 3–5:1 sustenta 60–100 tx simultâneas curtas | sim para os alvos /dia; ver §8 |
| min_pool_size / reserve_pool_size | 5 / 5 | compose env | warm pool + burst | sim |
| max_db_connections | 30 | compose env | teto real no postgres (30 < max_connections=100, sobra p/ exporter/tunnel/migrate) | sim |
| server_idle_timeout | 600 | compose env | recicla conexões ociosas | sim |
| auth_type | scram-sha-256 | compose env | auth forte | sim |
| admin_users | **postgres** (repo ini diz `mecontrola`) | default da imagem edoburu | `SHOW POOLS/STATS` **negado** ao usuário da aplicação — sem visibilidade direta de waiters/saturação do pooler via psql | **não** — lacuna de observabilidade + drift documental |
| carga atual | `0 xacts/s, 0 queries/s` | log do pgbouncer | confirma 0 usuários | — |

Coerência aritmética comprovada: 2 servers + 2 workers × `DB_MAX_CONNS=10` (env prod + `configs/config.go:713`, aplicado em `cmd/server/server.go:73-79` e `cmd/worker/worker.go:79-85`) = 40 client conns ≤ 300; servidos por ≤20+5 backends ≤ 30 `max_db_connections` ≤ 100 `max_connections`. A cadeia fecha sem estrangulamento de configuração.

---

## 6. Prova do backup, PITR e restore

| item | estado | evidencia | impacto operacional | veredito |
|---|---|---|---|---|
| Stanza pgBackRest | ok, cifra AES-256-CBC | `pgbackrest info`: `status: ok` | base íntegra | comprovado |
| Archive WAL contínuo → S3 | **operacional** | `pg_stat_archiver`: 78 arquivados, último hoje 20:47 UTC; `pgbackrest check` OK (WAL 0x60 confirmado no repo em 3,7s); 30 falhas históricas com `last_failed_time=2026-06-28` (anterior, resolvido) | RPO ≤ 10 min sustentado pelo WAL | comprovado |
| Backup full | 3 fulls no S3: 28/06 (×2) e **01/07 15:52** — nenhum desde então | `pgbackrest info` | full "fresco" depende de execução manual | configurado, **não operacional como rotina** |
| Backup diff/incr | **zero** diffs, **zero** incrs no repo | `pgbackrest info` (só 3 fulls listados) | retenção diff=7 nunca exercitada | não operacional |
| Agendamento (full/diff/incr) | **AUSENTE no host** | sem crontab root, sem `/etc/cron.d/pgbackrest`, sem timer, sem `/usr/local/bin/pgbackrest-run.sh`; script `deployment/scripts/pgbackrest-schedule.sh` existe no repo e nunca foi executado; runbook `backup-schedule.md` declara "Ativo" | cadeia full+WAL cresce sem novo full ⇒ RTO de PITR cresce sem teto; retenção nunca roda | **gap crítico + drift documental** |
| Backlog archive queue | vazio (spool 16K, 1 entrada) | `du /var/spool/pgbackrest` | sem represamento | comprovado |
| Retenção efetiva | full=4, diff=7 declaradas; **nunca exercitadas** (só 3 fulls existem) | `pgbackrest.conf` + `info` | política não testada na prática | não comprovado |
| Restore PITR | **apenas documentado/projetado** | `docs/runs/2026-07-03-evidencia-restore.md` declara: "Nenhum restore real foi executado"; RTO ~25 min/SLO 45 min = projeção; RF-04/RF-05 NÃO COMPROVADO | capacidade de recuperar após incidente sem prova | **não comprovado** |
| Restore VPS completo | apenas runbook + projeção (~56 min/SLO 90 min) | mesmo documento | idem | **não comprovado** |
| Anomalia de timeline | archive contém WAL de **timeline 2** (`wal archive min/max: 000000010000000000000008/000000020000000000000009`) enquanto o cluster de produção roda **timeline 1** (`pg_control_checkpoint`) | `pgbackrest info` + SQL | PostgreSQL 16 usa `recovery_target_timeline=latest` por default: um PITR ingênuo pode seguir a timeline 2 divergente (origem não identificada — provável promote de ensaio antigo no mesmo stanza) e recuperar histórico errado ou falhar | **bloqueio objetivo para PITR confiável** |

**Distinção obrigatória:** a capacidade de *atender tráfego* (§3–§5, §7–§8) é independente da capacidade de *recuperar após incidente*. Hoje: o arquivamento contínuo de WAL está **comprovado**; a rotina de backups está **inoperante**; o restore está **não comprovado**; e a anomalia de timeline compromete a confiabilidade do PITR até ser explicada/sanada. `Tem backup configurado` ≠ `restore comprovado` — e aqui nem a rotina de backup está de pé.

---

## 7. Pressao real da aplicacao sobre o banco

Derivada de `cmd/server/server.go`, `cmd/server/whatsapp_wiring.go`, `cmd/worker/worker.go` (mandato do prompt), com `file:line`.

**Caminhos do `server` (2 réplicas).** Rotas registradas em `server.go:129-259`: identity, categories, billing (webhook Kiwify), onboarding público, cards, transactions, WhatsApp (`/api/v1/whatsapp/inbound|status`, `whatsapp_wiring.go:20-65`), health. O webhook WhatsApp inbound é o caminho quente: síncrono no request faz apenas escrita leve — dedup `InsertIfAbsent(wamid)`, resolução de principal (SELECT users + INSERT auth_event) e **INSERT em `outbox_events`** (`internal/platform/whatsapp/dispatcher/dispatcher.go:104-181`) — e retorna. Rate limit do webhook: **600 req/min + burst 100 por réplica** (`prod.env:190-191`, `configs/config.go:1335-1336`).

**Caminhos do `worker` (2 réplicas).** ~19 jobs agendados via `robfig/cron` (`worker.go:177-327`), todos com `OverlapPolicy=OverlapSkip` por default (`internal/platform/worker/job/adapter.go:21`, `scheduler.go:80-87`): housekeepings diários em batch (500–10 000), reconciliação Kiwify horária, materialização de recorrências diária, reapers. O dominante é o **outbox-dispatcher**: tick 500ms, batch 50 (`prod.env`), claim com `FOR UPDATE SKIP LOCKED` ordenado por `(occurred_at, created_at, id)` (`internal/platform/outbox/storage_postgres.go:82-84`).

**Principais escritores.** Consumo de mensagem do agent (`whatsapp_inbound_consumer.go:115-176` → `internal/platform/agent/runtime.go:81-152`): por mensagem processada ≈ **8–10 INSERTs/UPDATEs** — outbox (2: inbound + embedding-index), platform_runs (2), platform_messages (3), platform_threads (0–1), platform_embeddings (1, assíncrono) — mais N queries de negócio via tools (transactions, cards, budgets). Maiores leitores: histórico de mensagens (`messages.Recent` LIMIT 20), listagens de transações/categorias.

**Risco de burst.** A ingestão (webhook) desacopla via outbox; o burst se acumula como backlog em `outbox_events`, não como pressão direta no Postgres.

**Risco de lock/contenção.** (a) Índice único `outbox_events_user_inflight_uidx` (`migrations/000001:70-72`) serializa 1 evento in-flight por usuário — correto para ordenação, sem contenção cruzada entre usuários; (b) claim usa SKIP LOCKED — sem briga entre os 2 workers; (c) `pg_advisory_lock(424242)` só no migrate (`cmd/migrate/migrate.go:56`); (d) **não há transação de banco aberta durante a chamada LLM** (nenhum `BeginTx`/UoW em `runtime.go` — verificado): as conexões não ficam presas durante o LLM, apenas a *goroutine* do handler.

**Risco de saturação de pool.** Baixo nos alvos /dia: transações curtas em transaction pooling. O pool não é o primeiro gargalo.

**Gargalo estrutural comprovado (o achado central).** O processamento do batch claimado é **serial** (`dispatcher.go:93-97`, loop `for _, row := range rows`), o job não sobrepõe ticks (`OverlapSkip`), e cada handler é limitado por **`OUTBOX_DISPATCHER_HANDLER_TIMEOUT=10s`** (`prod.env:72`, aplicado em `dispatcher.go:136`) — que **prevalece sobre** o `AGENT_INBOUND_TIMEOUT` default de 90s (`configs/config.go:1349`; ausente no prod.env), pois o contexto pai já expira em 10s. Como a chamada LLM (OpenRouter, `gpt-4o-mini` + tool loop) roda **dentro** do handler:
- Concorrência máxima do caminho do agent no cluster inteiro = **2** (1 por worker).
- Vazão máxima teórica = `2 ÷ latência_média_do_handler`. Com LLM+tools em 2–6s: **~0,33–1,0 msg/s**.
- Qualquer interação LLM > 10s é morta pelo timeout ⇒ run falho + retry (até 3, `OUTBOX_RETRY_MAX_ATTEMPTS`), consumindo capacidade.

---

## 8. Analise por alvo

**Premissa declarada P1 (não comprovável hoje, 0 usuários):** 3–5 mensagens WhatsApp por usuário ativo/dia, com pico concentrado em janela de ~12h a 2× a média e burst 3×. Toda conclusão dependente de P1 está marcada. **Dado ausente:** latência real do handler do agent com LLM em produção (nunca medida); IOPS do disco sob carga (medição proibida pelo mandato read-only).

### 8.1 10k ativos/dia

- **Comprovado:** ingestão suporta — 10k×5=50k msgs/dia ⇒ média 0,58 req/s, pico ~1,2 req/s, burst ~3,5 req/s, contra rate limit de 20 req/s agregado (2×600/min) e ~3 writes leves/webhook ⇒ ~10 writes/s no pico: ordem de grandeza trivial para Postgres em SSD com checkpoints saudáveis. Pool: dezenas de tx curtas/s ≪ 60–100 tx simultâneas sustentáveis pelos 20 backends. Disco: com P1, ~50k msgs/dia ⇒ ~0,5–0,8 GB/dia entre platform_messages + embeddings (1536 dims ≈ 6 KB + índice HNSW) + WAL ⇒ 80 GB livres duram meses, com housekeeping de outbox (90d) e retenção do ledger ativas.
- **Não comprovado:** (a) que o backlog do outbox drena no pico — demanda de pico 0,7–1,2 msg/s (P1) **excede ou empata** com a vazão máxima estrutural de 0,33–1,0 msg/s do consumo serial ⇒ fila cresce durante horas de pico e a latência de resposta ao usuário degrada de segundos para minutos; (b) zero teste de carga executado (harness k6 existe — `docs/runs/2026-07-03-evidencia-carga-envelopes.md` — nunca rodado); (c) rate limit do OpenRouter sob rajada.
- **Gargalo primário:** consumo serial do outbox (2 handlers, 10s cap) com LLM inline — **não é o PostgreSQL**.
- **Gargalo secundário:** OpenRouter (latência/rate limit externo).
- **Primeiro componente a saturar:** fila `outbox_events` (backlog), depois nada do banco: Postgres é o 3º+ na ordem de saturação.
- **Impacto do backup/restore neste alvo:** com escrita real, a ausência de fulls agendados faz a cadeia full(01/07)+WAL crescer sem teto ⇒ RTO de PITR cresce dia a dia; restore não comprovado + anomalia de timeline 2 = incidente com perda potencialmente irrecuperável no RTO prometido.
- **Classificação final: `nao comprovado`** — projeção favorável ao Postgres em si, mas desfavorável ao pipeline de consumo sob P1, sem nenhuma prova de carga real, e com gates de recuperação reprovados.

### 8.2 15k ativos/dia

- **Comprovado:** ingestão idem (1,7× o alvo anterior; ainda ≪ 20 req/s).
- **Não comprovado:** tudo do 8.1, agravado: demanda de pico ~1,0–1,7 msg/s ⇒ **excede** a vazão estrutural máxima mesmo no melhor caso de latência LLM ⇒ backlog cresce estruturalmente; crescimento de disco ~0,8–1,2 GB/dia encurta o horizonte de 80 GB para ~2–3 meses sem re-dimensionar retenções.
- **Gargalo primário/secundário/primeiro a saturar:** idem 8.1.
- **Impacto do backup/restore:** idem 8.1, com cadeia de WAL maior e RTO pior.
- **Classificação final: `nao comprovado`** (projeção estrutural desfavorável; sem prova de carga; recuperação reprovada).

### 8.3 10k simultaneos em pico

- **Comprovado (objetivamente, contra o alvo):** 10 000 requisições simultâneas de usuários distintos encontram rate limit agregado de ~20 req/s + burst 200 (`prod.env:190-191`) ⇒ >97% recebem 429 imediato (Meta re-tenta, o pico vira fila); ainda que toda a ingestão entre, o consumo é serial-2 com teto de 0,33–1,0 msg/s ⇒ drenar 10k eventos leva **2,8–8,4 horas**. Nenhum SLO conversacional sobrevive. O PostgreSQL sequer é exercitado: satura-se a camada de aplicação antes.
- **Não comprovado:** nada relevante pende — a insuficiência é estrutural e demonstrável por configuração e código (`prod.env:72,190-191`; `dispatcher.go:93-97,136`; `adapter.go:21`).
- **Gargalo primário:** rate limit do webhook + consumo serial do outbox. **Secundário:** OpenRouter. **Primeiro a saturar:** webhook HTTP (429), depois fila outbox.
- **Impacto do backup/restore:** irrelevante para o veredito (falha antes).
- **Classificação final: `nao atende`.**

### 8.4 15k simultaneos em pico

- **Comprovado:** idem 8.3 com 1,5× a demanda; drenagem 4,2–12,6 horas.
- **Classificação final: `nao atende`.**

---

## 9. Gaps, lacunas e bloqueios objetivos

| item | categoria | evidencia | impacto | severidade | bloqueia qual alvo | acao obrigatoria |
|---|---|---|---|---|---|---|
| Agendamento pgBackRest ausente no host (runbook diz "Ativo") | gap + drift crítico | §6; sem cron/timer/wrapper; último full 01/07; zero diff/incr | RTO de PITR cresce sem teto; retenção nunca roda | crítica | todos | executar `deployment/scripts/pgbackrest-schedule.sh` na VPS e validar timer |
| Restore/PITR nunca executado | não comprovado | `docs/runs/2026-07-03-evidencia-restore.md` (auto-declarado) | RTO/RPO são projeção; recuperação de incidente sem prova | crítica | todos (gate de production-ready) | restore drill real em ambiente isolado com RTO medido |
| WAL de timeline 2 no archive vs cluster em timeline 1 | bloqueio objetivo | `pgbackrest info` + `pg_control_checkpoint` | PITR default (`recovery_target_timeline=latest`, PG16) pode seguir timeline divergente | alta | todos (recovery) | identificar origem; sanear archive (expire/stanza) ou documentar `--target-timeline=1` no runbook |
| Consumo do outbox serial (2 handlers) com LLM inline e cap 10s | gap estrutural | `dispatcher.go:93-97,136`; `adapter.go:21`; `prod.env:72` | teto 0,33–1,0 msg/s no cluster; picos formam backlog de horas | crítica p/ capacidade | 10k/dia (marginal), 15k/dia, 10k/15k pico | paralelizar consumo entre usuários preservando `user_inflight_uidx`; reconciliar timeout com latência LLM real |
| `OUTBOX_DISPATCHER_HANDLER_TIMEOUT=10s` < latência LLM possível (90s default nunca alcançável) | gap de config | `prod.env:72` vs `configs/config.go:1349`; contexto pai expira antes | interações LLM legítimas >10s falham e re-tentam ×3 | alta | 10k+/dia | medir p95 do handler com LLM real; ajustar os dois timeouts coerentemente |
| Zero teste de carga executado | lacuna de prova | `docs/runs/2026-07-03-evidencia-carga-envelopes.md`: harness pronto, execução pendente | nenhum alvo pode ser declarado "atende" | crítica p/ veredito | 10k/15k dia | rodar `task loadtest:suite:envelope-b` contra staging + variante com latência LLM |
| Porta 15432 (pg-tunnel→postgres) acessível da internet | risco de segurança | `nc 187.77.45.48 15432` OK do exterior; docker-proxy contorna ufw (comportamento documentado Docker/iptables) | superfície de ataque direta ao banco (scram é a única barreira) | alta | todos (operação) | restringir por IP em DOCKER-USER/ufw route ou remover publicação e usar SSH tunnel |
| `SHOW POOLS/STATS` negado (admin_users=postgres) + drift do ini de referência | lacuna de observabilidade | §5 | sem visibilidade de waiters/saturação do pooler | média | diagnóstico em incidente | expor STATS_USERS=mecontrola via env do compose; corrigir ini de referência |
| `OTEL_TRACE_SAMPLE_RATE="1"` (100%) em prod | gap de custo/CPU | `compose.swarm.yml:299,346,393,439` | sob 10k+/dia, tracing integral pressiona otel-lgtm (já o maior consumidor: 495MiB idle) e o disco | média | 10k/15k dia | reduzir a 0.1 no compose (env já previa 0.1) |
| CI runner GitHub Actions no host de produção | risco operacional | processo `Runner.Listener` ativo; `/opt/actions-runner` | build rouba 2 vCPU/IO do banco durante deploys/CI | média | picos coincidentes com CI | mover runner p/ fora do host prod (ou limitar cgroup) |
| IOPS/latência de disco sob carga nunca medidos | dado ausente | mandato read-only impede benchmark; sar só em idle | dimensionamento de WAL/checkpoint sob carga é projeção | média | 15k/dia | medir durante o load test (não em prod idle) |
| Comportamento de usuário (msgs/user/dia) desconhecido | dado ausente | 0 usuários | P1 é premissa, não fato | média | todos /dia | instrumentar coorte real antes de escalar |

---

## 10. Menor plano de evolucao seguro

| alvo | mudanca minima | motivo tecnico | custo/impacto | risco mitigado | prova exigida |
|---|---|---|---|---|---|
| todos | rodar `pgbackrest-schedule.sh` na VPS (já existe no repo) | reativar rotina full/diff/incr e retenção | R$ 0; minutos | RTO crescente; retenção morta | timer/cron visível no host + novo full no `pgbackrest info` |
| todos | restore drill real (PITR em container isolado, conforme runbook) com RTO cronometrado + sanear timeline 2 | transformar recuperação de projeção em prova | R$ 0 (usa S3 existente); ~2h de trabalho | perda irrecuperável em incidente | `docs/runs` com checklist executado e RTO medido |
| todos | restringir 15432 por IP de origem (DOCKER-USER) ou despublicar | fechar acesso direto ao banco da internet | R$ 0 | intrusão/bruteforce no Postgres | `nc` externo falhando; regra visível |
| 10k/dia | executar harness k6 envelope B + cenário com latência LLM real/stub | única forma de sair de `não comprovado` | R$ 0 (harness pronto); staging efêmero | falso positivo de capacidade | relatório k6 com p95<500ms e fila outbox drenando |
| 10k/dia | paralelizar handlers do outbox por usuário (worker pool N=8–16 goroutines, mantendo `user_inflight_uidx` e claim SKIP LOCKED) + timeout coerente com p95 LLM | eleva teto de 0,33–1 msg/s para ~N msg/s sem hardware novo; o Postgres atual absorve (writes curtos) | mudança de código contida; sem infra nova | backlog em pico | re-execução do load test com backlog estável |
| 10k/dia | `OTEL_TRACE_SAMPLE_RATE=0.1` em prod | reduzir pressão do otel-lgtm (co-locado) | R$ 0 | contenção de CPU/disco no host único | compose atualizado + consumo do otel-lgtm estável |
| 15k/dia | tudo acima + re-executar load test a 1,5×; só então decidir upgrade KVM 4 (gatilhos objetivos já definidos em `docs/runs/2026-07-03-orcamento-recursos.md`) | não comprar hardware sem prova de necessidade (CPU>70%/30min etc.) | +R$ 16–72/mês somente se gatilho disparar | gasto prematuro | métricas do gatilho sob carga 1,5× |
| 10k/15k pico | **fora do menor caminho**: exigiria repensar rate limit, consumo massivamente paralelo e provavelmente broker/sharding (ADR-001 já prevê gatilho) | pico simultâneo de 10k+ é incompatível com single-node serial por design | alto | — | não recomendado agora (0 usuários; demanda hipotética) |

**Ajustes desnecessários hoje (anti-desperdício):** aumentar `max_connections`; subir RAM do Postgres além de 2G (banco de 12 MB); multi-node/HA; broker de mensagens; sharding do outbox; réplica de leitura. Nenhum tem necessidade provada e todos adicionam custo/complexidade sem atacar o gargalo real (consumo serial + provas ausentes).

---

## 11. Veredito final fechado

- **Hoje, o PostgreSQL atual com a topologia atual suporta 10k ativos/dia?** `nao comprovado`. O banco em si tem projeção favorável (ingestão, pool, WAL, disco); mas não existe nenhum teste de carga executado, o pipeline de consumo (serial, 2 handlers, cap 10s com LLM inline) tem teto estimado abaixo do pico sob a premissa P1, e os gates de recuperação estão reprovados.
- **Hoje, o PostgreSQL atual com a topologia atual suporta 15k ativos/dia?** `nao comprovado`, com projeção estrutural desfavorável (demanda de pico excede o teto de consumo mesmo no melhor caso).
- **Hoje, o PostgreSQL atual com a topologia atual suporta 10k simultaneos em pico?** `nao atende` — prova objetiva: rate limit 600/min+burst 100 por réplica e consumo serial de 2 handlers ⇒ 429 em massa e drenagem de horas. O Postgres nem chega a ser o componente exercitado.
- **Hoje, o PostgreSQL atual com a topologia atual suporta 15k simultaneos em pico?** `nao atende` (a fortiori).
- **O backup em S3 está apenas configurado ou está operacionalmente comprovado?** Parcial: o **archive contínuo de WAL está operacionalmente comprovado** (78 segmentos, último hoje; `pgbackrest check` OK; 3 fulls íntegros no S3 com AES-256). A **rotina de backups full/diff/incr NÃO está operacional**: agendamento ausente no host, último full em 01/07, zero diff/incr — em contradição com o runbook.
- **O restore/PITR está comprovado ou apenas documentado/projetado?** Apenas documentado/projetado (`docs/runs/2026-07-03-evidencia-restore.md` auto-declara "Nenhum restore real foi executado"; RTO é projeção). Agravante: WAL de timeline 2 no archive torna o PITR default ambíguo.
- **Qual é o primeiro bloqueio técnico real para 10k?** O consumo serial do outbox com LLM inline (2 handlers no cluster, timeout 10s) — gargalo de aplicação, não de banco — somado à ausência total de prova de carga.
- **Qual é o primeiro bloqueio técnico real para 15k?** O mesmo, com déficit estrutural ainda no melhor caso de latência LLM; secundariamente o crescimento de disco (~0,8–1,2 GB/dia projetado) sobre 80 GB livres.
- **Qual é o menor caminho econômico e seguro para fechar os gaps?** Custo ~zero em infra: (1) instalar o agendamento pgBackRest já versionado; (2) restore drill real com RTO medido + sanear timeline 2; (3) restringir a porta 15432; (4) rodar o harness k6 já pronto (envelope B + latência LLM); (5) paralelizar o consumo do outbox por usuário e reconciliar os timeouts; (6) sampling 0.1. Upgrade KVM 4 só se os gatilhos objetivos dispararem sob carga real.
- **Foi possível fechar 0 gaps, 0 lacunas e 0 ressalvas com prova objetiva?** **Não.** Foram encontrados 2 bloqueios críticos de recuperação (agendamento ausente + restore não comprovado), 1 anomalia de PITR (timeline 2), 1 gargalo estrutural de capacidade (consumo serial), 1 exposição de segurança (15432), 2 lacunas de prova (carga e IOPS) e 2 dados ausentes (comportamento de usuário, latência LLM). Nenhum dos 4 alvos pode ser declarado `atende hoje` com honestidade.

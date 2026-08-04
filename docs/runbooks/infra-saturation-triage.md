# Runbook — Triagem de Saturação de Infraestrutura (Collector, Pool DB, Disco)

- Origem: Tarefa 7.0 do PRD `.specs/prd-observabilidade-golden-signals-otel/` (RF-10).
- Escopo: alertas provisionados em `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`
  para saturação de infraestrutura que não é coberta pelos runbooks de deploy/backup existentes.

## 1. `mc-collector-export-fail` — Falha na exportação de métricas do collector

Sintoma: o OpenTelemetry Collector (`otel-lgtm`) não consegue enviar métricas ao Prometheus
por 5 minutos (`otelcol_exporter_send_failed_metric_points_total` crescendo).

```bash
# Logs do collector embutido no otel-lgtm
docker service logs --since 15m mecontrola_otel-lgtm | grep -i "exporter\|prometheus" | tail -n 50

# Conectividade interna: o Prometheus embutido escuta em localhost dentro do mesmo container
docker exec $(docker ps --filter "name=mecontrola_otel-lgtm" --format "{{.Names}}" | head -1) \
  wget -q -O- http://localhost:9090/-/ready
```

Causas prováveis e ação:
- Prometheus (Mimir embutido) sem memória/disco (ver `observabilidade-spof-retention.md` para
  retenção e SPOF aceito) → verificar uso de disco do volume `otel-lgtm-prometheus`.
- Fila interna do exporter saturada (`otelcol_exporter_queue_size`) → reiniciar o serviço:
  ```bash
  docker service update --force mecontrola_otel-lgtm
  ```
- Indisponibilidade é tolerada operacionalmente (perda de sinais, não de dado financeiro) — não
  é gatilho de rollback da aplicação.

## 2. `mc-pgbouncer-pool-saturation` — Conexões reais ao Postgres acima de 80% do teto

Sintoma: `pg_stat_database_numbackends{datname="mecontrola_db"}` acima de 24 (80% de
`max_db_connections=30`, ver `deployment/pgbouncer/pgbouncer.ini`) por 5 minutos.

```bash
# Conexões ativas por estado
docker exec $(docker ps --filter "name=mecontrola_postgres" --format "{{.Names}}" | head -1) \
  psql -U mecontrola -d mecontrola_db -c \
  "SELECT state, count(*) FROM pg_stat_activity WHERE datname = 'mecontrola_db' GROUP BY state ORDER BY 2 DESC;"

# Queries mais longas em execução (candidatas a segurar conexão)
docker exec $(docker ps --filter "name=mecontrola_postgres" --format "{{.Names}}" | head -1) \
  psql -U mecontrola -d mecontrola_db -c \
  "SELECT pid, now() - query_start AS duration, state, query FROM pg_stat_activity WHERE state != 'idle' ORDER BY duration DESC LIMIT 10;"
```

Ação:
1. Se há queries anômalas travando conexão: `SELECT pg_terminate_backend(<pid>);` (avaliar impacto
   antes de matar transação).
2. Verificar `DB_MAX_CONNS` (padrão `10`, ver `deployment/config/prod.env`) dos processos
   `server-1/2` e `worker-1/2` — múltiplas réplicas podem somar mais que `max_db_connections=30`
   do pgBouncer sob pico.
3. Companheiro: alerta `mc-pgbouncer-client-queue` (warning) mede o pool `database/sql` da
   própria aplicação (app → pgbouncer); se ele também estiver disparando, o gargalo é na
   aplicação, não no pgBouncer.
4. Gatilho de upgrade documentado na anotação do alerta: espera sustentada > 5/s por 10 min
   indica necessidade de aumentar `DB_MAX_CONNS`/`max_db_connections` ou revisar concorrência.

## 3. `mc-disk-low-bytes` — Disco com menos de 10 GiB livres

Sintoma: `node_filesystem_avail_bytes{mountpoint="/"}` abaixo de 10 GiB por 5 minutos —
mais crítico que o alerta percentual `mc-disk-usage-high` (warning, 80%).

```bash
# Espaço livre atual
df -h /

# Execução imediata do prune (idempotente, já documentado como referência do alerta)
bash deployment/scripts/docker-prune.sh

# Maiores consumidores em /var/lib/docker
docker system df -v
```

Ação:
1. Rodar `deployment/scripts/docker-prune.sh` imediatamente (já agendado via
   `deployment/scripts/docker-prune.timer`; a execução manual acelera a liberação).
2. Se o prune não recuperar espaço suficiente, investigar volumes de dados (Postgres, Loki,
   Tempo, Prometheus) via `docker system df -v` antes de qualquer remoção manual.
3. Espaço crítico persistente é gatilho para upgrade de disco da VPS — registrar decisão em
   `docs/runs/`.

## 4. `mc-runtime-goroutine-growth` — Crescimento sustentado de goroutines (causa)

Alerta de **causa**, não de sintoma: `severity: warning`, roteado para e-mail/ticket, nunca
pagina isoladamente. Serve para diagnóstico precoce de goroutine leak antes que ele vire
saturação de CPU/memória.

Séries de runtime do processo (`contrib/instrumentation/runtime` v0.69.0, semconv `go.*`):

```bash
# Contagem de goroutines por processo (api e worker)
curl -sG "$PROM/api/v1/query" --data-urlencode 'query=max by (job) (go_goroutine_count)'

# Heap em uso vs. alvo do GC — heap subindo junto com goroutines indica retenção real
curl -sG "$PROM/api/v1/query" --data-urlencode 'query=go_memory_used_bytes{go_memory_type="heap"}'
curl -sG "$PROM/api/v1/query" --data-urlencode 'query=go_memory_gc_goal_bytes'

# GOMAXPROCS efetivo (contexto para interpretar a contagem)
curl -sG "$PROM/api/v1/query" --data-urlencode 'query=go_processor_limit'
```

Triagem:

1. Identificar o `job` no alerta (`mecontrola-api` ou `mecontrola-worker`) e abrir a row
   "Runtime do processo (Go)" em `deployment/dashboards/mecontrola-infra.json`.
2. Distinguir leak de carga: se `go_goroutine_count` sobe **e não volta** após o tráfego cair,
   é leak; se acompanha o tráfego e retorna à baseline, é carga.
3. Se `go_memory_used_bytes{go_memory_type="heap"}` cresce junto, as goroutines estão retendo
   memória — suspeitar de consumer/producer sem cancelamento, workflow suspenso sem limpeza
   ou goroutine de background sem shutdown cooperativo (ver R6 em `AGENTS.md`).
4. Coletar dump para confirmar o call site:
   ```bash
   docker exec <container> kill -QUIT 1   # goroutine dump no stderr do processo
   docker logs <container> --tail 500
   ```
5. Mitigação imediata: restart do processo afetado (recupera o leak, não corrige a causa);
   registrar o dump e abrir ticket. Rollback só se o crescimento começou em um deploy
   identificável — ver `deployment/runbooks/rollback.md`.

O threshold de 3000 é baseline operacional inicial, não SLO formal; recalibrar após observar
a distribuição real pós-deploy.

## Referências

- `deployment/pgbouncer/pgbouncer.ini` — `max_db_connections=30`, `default_pool_size=20`.
- `deployment/config/prod.env` — `DB_MAX_CONNS`.
- `deployment/scripts/docker-prune.sh`, `deployment/scripts/docker-prune.timer`.
- `deployment/runbooks/observabilidade-spof-retention.md` — SPOF aceito do `otel-lgtm`.
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` — definição dos alertas.

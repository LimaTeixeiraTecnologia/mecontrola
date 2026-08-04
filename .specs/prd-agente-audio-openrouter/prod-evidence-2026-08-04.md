# Evidencia de Producao: agente audio OpenRouter

Data da coleta: 2026-08-04
Host: `mecontrola-vps`
Modo: somente leitura

## Inventario

- Host remoto: `srv1761537`
- Uptime: 49 dias
- Stack Swarm ativa:
- `mecontrola_server-1`: `1/1`, healthy
- `mecontrola_server-2`: `1/1`, healthy
- `mecontrola_worker-1`: `1/1`, healthy
- `mecontrola_worker-2`: `1/1`, healthy
- `mecontrola_postgres`: `1/1`, healthy
- `mecontrola_pgbouncer`: `1/1`, healthy
- `mecontrola_otel-lgtm`: `1/1`, healthy
- `mecontrola_postgres-exporter`: `1/1`, healthy
- `mecontrola_node-exporter`: `1/1`, healthy
- `mecontrola_caddy`: `1/1`, healthy

## Banco

- PostgreSQL: `16.14`
- `schema_migrations`: version `14`, `dirty=false`
- Tabelas relevantes:
- `platform_runs`: `181`
- `platform_messages`: `398`
- `agents_write_ledger`: `46`
- `consumer_processed_messages`: `198`
- `outbox_events`: `902`
- `whatsapp_message_status`: `0`
- `platform_runs` por status:
- `succeeded`: `178`
- `failed`: `3`
- `pg_stat_activity`:
- `idle/client`: `10`
- `active`: `1`, correspondente a coleta read-only
- `pg_locks`: apenas locks concedidos na amostra (`AccessShareLock=1`, `ExclusiveLock=1`)
- `outbox_events` por status:
- `status=3`: `902`
- `status=4`: `0` retornado na consulta de dead-letter

## Metricas

Fonte: Grafana proxy para Prometheus, datasource uid `prometheus`.

Ultimas 24h:

- `whatsapp_dispatcher_route_total{outcome="agent"}`: aproximadamente `13`
- `whatsapp_dispatcher_route_total{outcome="invalid"}`: aproximadamente `47`
- `agents_whatsapp_inbound_total{outcome="success"}`: aproximadamente `11`
- `agent_runs_total{agent_id="mecontrola-agent",status="succeeded"}`: aproximadamente `7`
- `agent_llm_provider_call_total{model="openai/gpt-4o-mini",status="ok"}`: aproximadamente `20`
- `agent_llm_provider_call_total{model="openai/text-embedding-3-small",status="ok"}`: aproximadamente `28`
- `agent_llm_provider_errors_total`: sem serie retornada na janela consultada
- `http_server_request_count_total` 5xx: sem serie retornada na janela consultada
- `http_server_request_duration_seconds` p95 em 1h para `mecontrola-api`: `0.00475s`
- `pg_database_size_bytes{datname="mecontrola_db"}`: `22821911`

Series existentes por prefixo:

- Agent: `agent_llm_provider_call_total`, `agent_llm_provider_latency_seconds_*`, `agent_runs_total`, `agent_tool_invocations_total`, `agents_whatsapp_inbound_total`, `agents_whatsapp_route_total`
- WhatsApp: `whatsapp_dispatcher_route_total`, `whatsapp_dedup_housekeeping_duration_seconds_*`, `whatsapp_ratelimit_*`
- HTTP: `http_server_request_count_total`, `http_server_request_duration_seconds_*`, `http_client_request_*`
- Postgres: `pg_*`
- Go exporter atual: `go_goroutines`, `go_memstats_*`, `go_threads`

Nao ha series `agents_audio_*` ou STT em producao antes da implementacao.

## Tracing e Logs

- Grafana datasource uids:
- Prometheus: `prometheus`
- Loki: `loki`
- Tempo: `tempo`
- Tempo search retornou traces recentes de `mecontrola-api`, incluindo `GET /healthz` e spans de onboarding.
- Loki labels: `deployment_environment`, `service_instance_id`, `service_name`.
- Query Loki por WhatsApp retornou logs de `whatsapp.ratelimit.cleanup`.
- Query Loki por erros em `mecontrola-api|mecontrola-worker` nao retornou erro na consulta executada.
- `docker service logs` nas ultimas 6h retornou erro recorrente fora do escopo de audio: job `billing-reconciliation` com Kiwify `401`.
- `docker service logs` retornou tambem erro de shutdown de observabilidade por `tracer_provider force flush: context canceled`.

## Implicacoes Para Techspec

- Migration de audio deve ser `000015`, pois producao esta em `14` limpa.
- Volume atual de agente/WhatsApp e baixo; tabela de auditoria por WAMID com PK e sem backfill e suficiente.
- A amostra de banco nao mostrou backlog dead-letter nem lock aguardando; migration append-only pequena continua adequada.
- Dashboards/alerts de audio devem usar nomes novos `agents_audio_*` e labels de baixa cardinalidade.
- Consultas operacionais devem usar `job` para metricas Prometheus de app; `service_name` aparece em Loki/Tempo.
- Nao usar nomes `go_*` documentais sem confirmar no Prometheus real, porque producao atual expõe `go_goroutines` e `go_memstats_*`.
- Kiwify 401 e shutdown flush sao riscos operacionais existentes, mas nao bloqueiam audio.

## Evidencia Especifica de Audio Real

Ver `.specs/prd-agente-audio-openrouter/whatsapp-audio-payload-evidence-2026-08-04.md`.

Resumo:

- O envio de audio real gerou `auth.principal_established` publicado e `agents.whatsapp.inbound.v1` em falha.
- O payload persistido do evento inbound contem apenas `message_id`, `peer`, `text`, `user_id`.
- `text` existe com tamanho `0`.
- `audio` e `media_id` estao ausentes.
- Nao houve `platform_runs` nem `platform_messages` na janela; o audio nao entrou no runtime agentivo.

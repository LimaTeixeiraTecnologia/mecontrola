# Padrão de Telemetria — mecontrola-api / mecontrola-worker

> Documento de padrão de observabilidade baseado nos Four Golden Signals (Google SRE) + OpenTelemetry
> (Specification, OTLP, Semantic Conventions). Materializado pelo PRD
> `observabilidade-golden-signals-otel` (`.specs/prd-observabilidade-golden-signals-otel/`) —
> consolidação final na Tarefa 8.0. Cobre RF-01..RF-20 do PRD; toda métrica, alerta e query citada
> abaixo foi confirmada contra o código-fonte real (`go.mod`, `pkg/observability` do devkit,
> `deployment/telemetry/`) ou contra um ambiente `grafana/otel-lgtm` alimentado com telemetria real
> emitida pelo código de produção — nenhum nome foi inventado.

## 1. Contexto do serviço

| Campo | Valor |
| --- | --- |
| `service.name` | `mecontrola-api` (HTTP), `mecontrola-worker` (jobs/consumers) |
| Porte | pequeno/médio (monolito modular, um Postgres, um Collector agent) |
| Tipo | REST (API) + worker (jobs/consumers/outbox dispatcher) |
| Linguagem principal | Go (`go.mod`: `go.opentelemetry.io/otel`/`sdk` v1.45.0; `go.opentelemetry.io/otel/log`, `sdk/log`, `otlplog/otlploggrpc`, `otlplog/otlploghttp` v0.20.0 — fixado por compatibilidade com `otelslog@v0.19.0`) |
| Plataforma | VM única via Docker Swarm (`deployment/compose/compose.swarm.yml`) |
| SLO disponibilidade | 99,9% (error budget 0,1%, janela 30 dias) — seção 6 |
| SLO latência P95 | < 500ms (janela 30 dias) — seção 6 |
| Dependências | PostgreSQL (pgBouncer + pgBackRest), OpenRouter (LLM), WhatsApp Cloud API, Kiwify (billing) |
| Backend de telemetria | Prometheus (métricas) + Tempo (traces) + Loki (logs), atrás de um único OpenTelemetry Collector agent (`otel-lgtm`), provisionado em `deployment/telemetry/` |
| Provider de instrumentação | `github.com/JailtonJunior94/devkit-go/pkg/observability` (único provider; ver `.claude/rules/agent-workflows-tools.md` R-AGENT-WF-001.4 para as call-sites de LLM sancionadas, que não afetam este documento) |

## 2. Golden Signals → OpenTelemetry (Four Golden Signals, Google SRE)

| Sinal | Métrica semconv / instrumento | Nome Prometheus renderizado | Atributos | Query PromQL canônica |
| --- | --- | --- | --- | --- |
| Latência | `http.server.request.duration` (histograma, buckets Stable `[0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10]`) | `http_server_request_duration_seconds_bucket` / `_sum` / `_count` | `http.request.method`, `http.route`, `http.response.status_code` | `histogram_quantile(0.99, sum by (le) (rate(http_server_request_duration_seconds_bucket{job="mecontrola-api"}[5m])))` |
| Tráfego | `http.server.request.duration` (`_count`) | `http_server_request_duration_seconds_count` | idem | `sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api"}[5m]))` |
| Erros | idem, filtrado por status | `http_response_status_code=~"5.."` sobre a mesma série `_count` | `http.response.status_code`, `http.route` | `100 * sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api", http_response_status_code=~"5.."}[5m])) / clamp_min(sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api"}[5m])), 1)` |
| Saturação | `go.goroutine.count`, `go.memory.used`, `go.memory.gc.goal`, `go.processor.limit` (runtime Go, RF-01/RF-03) | `go_goroutine_count`, `go_memory_used_bytes{go_memory_type=...}`, `go_memory_gc_goal_bytes`, `go_processor_limit` | `job`, `go_memory_type` | `max(go_goroutine_count{job=~"mecontrola-.+"})` (usada por `mc-runtime-goroutine-growth`) |

Nunca usar média simples para latência — sempre `histogram_quantile`. Rota HTTP sempre via
`http.route` (rota normalizada, ex.: `/api/v1/cards/{id}`), nunca URL crua — reforça RF-13.

Fonte canônica de RED (ADR-004, `.specs/prd-observabilidade-golden-signals-otel/adr-004-red-canonical-source.md`):
o provider devkit instrumenta HTTP server com o histograma acima **e** três counters auxiliares
(`pkg/observability/otel/http.go:14-84`):

| Métrica semconv | Nome Prometheus renderizado | Papel |
| --- | --- | --- |
| `http.server.request.duration` (histograma) | `http_server_request_duration_seconds_bucket` / `_sum` / `_count` | **Fonte canônica de RED** (tráfego, erros, latência) |
| `http.server.request.count` (counter) | `http_server_request_count_total` | Auxiliar — não usar como fonte primária de novos dashboards/alertas |
| `http.server.request.active` (up-down counter) | `http_server_request_active` | Auxiliar — liveness (`absent(...)`, alerta `mc-api-down`) e in-flight |
| `http.server.request.error.count` (counter) | `http_server_request_error_count_total` | Auxiliar — redundante com a filtragem `status_code=~"5.."` sobre o histograma |

O alerta `mc-api-5xx` (`deployment/telemetry/grafana/provisioning/alerting/rules.yaml`) usa a query
canônica de taxa de erro acima. O alerta `mc-api-latency-p99` já usava a série canônica do
histograma antes desta migração. O alerta `mc-api-down` continua usando o auxiliar
`http_server_request_active` para liveness — uso permitido e documentado (o histograma não tem uma
série de "sem tráfego" equivalente sem a mesma semântica de gauge in-flight).

**Confirmação empírica (Tarefa 8.0, fecha o risco residual registrado pela Tarefa 5.0):** a série
`http_server_request_duration_seconds_count` foi confirmada ao vivo publicando telemetria real do
provider devkit (`otel.Provider.HTTP()`, mesmo caminho de código usado em produção) contra um
`grafana/otel-lgtm:0.7.5` local via OTLP HTTP real. Query direta ao Prometheus embutido
(`http_server_request_duration_seconds_count`) retornou as séries
`{http_request_method="GET", http_response_status_code="200", http_route="/api/v1/task8-probe", job="mecontrola-api"} 4`
e `{..., http_response_status_code="500", ...} 1`, com atributos idênticos aos usados pelas queries
de `mc-api-5xx`/`mc-slo-burn-*`. O counter auxiliar `http_server_request_count_total` publicado no
mesmo experimento retornou os mesmos valores (4/1) para as mesmas combinações de atributos — coerência
numérica confirmada empiricamente, não apenas por inspeção de código. O ambiente de validação
(container `mc-task8-validate`, programa Go efêmero em `cmd/tools/httpprobe-tmp/`) foi removido após
a confirmação; nenhum artefato de teste permanece no repositório.

## 3. Instrumentação

- Atributos de recurso obrigatórios em todo sinal: `service.name`, `service.version`,
  `deployment.environment` (RF-17), preservando `service.instance.id` (hostname) já injetado em
  `cmd/server/server.go:40-51` (`buildO11yConfig`, `ResourceAttributes`).
- Estratégia: auto-instrumentação via `go.opentelemetry.io/contrib/instrumentation/runtime` v0.69.0
  (compatível com `go.opentelemetry.io/otel` v1.45.0) para saturação de processo (RF-01), manual via
  `github.com/JailtonJunior94/devkit-go/pkg/observability` para HTTP, banco (`database_pool_*`),
  domínio (`agents_*`, `outbox_*`, etc.) e heartbeat do worker (`worker_heartbeat`, RF-11, gauge
  registrado em `cmd/worker/worker.go`, `startWorkerHeartbeat`).
- Sinais habilitados: métricas (obrigatório), traces (sim, via Collector com tail sampling — seção
  4/5), logs correlacionados (sim — seção 7, RF-12).
- Runtime metrics (RF-01/RF-02): ativado via `runtimemetrics.Start(o11y, runtime.DefaultMinimumReadMemStatsInterval)`
  em `cmd/server/server.go` e `cmd/worker/worker.go`, exigindo `RegisterGlobal: true` no
  `otel.Config` (o provider devkit usa o `MeterProvider` global por não expor um público) — não
  interfere nas métricas de instância publicadas via `o11y.Metrics()`.
- Nenhum nome de métrica/atributo é inventado fora do OpenTelemetry Semantic Conventions e das
  métricas já existentes (RF-14) — todos os nomes deste documento foram confirmados contra código
  ou ambiente real (ver seção 8, Inventário de métricas).

## 4. Coleta e transporte (OTLP + Collector) — topologia preservada (RF-15)

Arquivo real: `deployment/telemetry/grafana/otelcol-config.yaml` (agent único, `grafana/otel-lgtm`
em produção via `deployment/compose/compose.swarm.yml`). Topologia **preservada** por esta
iniciativa — nenhuma mudança removeu processors, exporters ou pipelines:

- **Receiver**: `otlp`, protocolos `grpc` (porta `4317`) e `http` (porta `4318`).
- **Processors** (ordem real do pipeline de traces): `memory_limiter` (`check_interval: 1s`,
  `limit_mib: 512`, `spike_limit_mib: 128`) → `tail_sampling` (ver seção 5) → `batch`
  (`timeout: 10s`, `send_batch_size: 512`, `send_batch_max_size: 1024`). Pipelines de `metrics` e
  `logs` usam apenas `memory_limiter` + `batch` (tail sampling é exclusivo de `traces` — não faz
  sentido decidir amostragem de métricas/logs por span).
- **Exporters**: `otlphttp/metrics` → Prometheus (`http://127.0.0.1:9090/api/v1/otlp`),
  `otlphttp/traces` → Tempo (`http://127.0.0.1:4418`), `otlphttp/logs` → Loki
  (`http://127.0.0.1:3100/otlp`) — três pipelines independentes, um exporter cada, preservados.
- **Extensions**: `health_check` (`0.0.0.0:13133/ready`).

## 5. Sampling, cardinalidade e custo

### 5.1 Sampling (RF-16) — `tail_sampling` vigente, NÃO head sampling

A app envia **100% (head)** ao Collector — nenhuma decisão de amostragem acontece no processo Go.
A decisão vive inteiramente no Collector, via `tail_sampling` (`otelcol-config.yaml:18-45`), com
três políticas combinadas (uma trace é retida se **qualquer** política aceitar):

1. `errors-policy` (`status_code`): retém 100% dos traces com `status_code: ERROR`.
2. `agent-flow-policy` (`ottl_condition`): retém 100% dos traces que contêm qualquer span cujo nome
   esteja na lista fixa (`agent.runtime.execute`, `agent.execute`,
   `agents.route.whatsapp_inbound`, `agents.consumer.whatsapp_inbound.handle`,
   `agents.consumer.subscription_bound_welcome.handle`, `agents.usecase.handle_inbound`,
   `agents.usecase.resume_dispatcher`, `workflow.engine.start`, `workflow.engine.resume`,
   `workflow.step.execute`) — preserva observabilidade total do fluxo de agente/workflow
   independentemente do resultado.
3. `probabilistic-policy`: retém 10% dos traces restantes (não cobertos pelas duas políticas acima).

Métricas **não são amostradas** — todo ponto de dado é agregado e exportado (RF-15).

### 5.2 Cardinalidade controlada (RF-13)

Nenhuma métrica deste sistema carrega rótulo de alta cardinalidade: **proibido** `user_id`,
`request_id`, `correlation_id`/`correlation_key`, `category_id` ou qualquer ID de sessão como
label/atributo de métrica. Reforça `.claude/rules/transactions-workflows.md` (R-TXN-004),
`.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001.4) e
`.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.5).

Rota HTTP usa exclusivamente `http.route` (rota normalizada do roteador, ex.:
`/api/v1/cards/{id}`), nunca a URL crua com o ID interpolado — evita explosão de cardinalidade por
recurso.

Estes identificadores **aparecem em logs** (não em métricas) para correlação
(`request_id`/`correlation_id` em `pkg/observability/otel/logger.go:227-243`) — uso permitido e
esperado em logs estruturados, distinto de rótulo de métrica Prometheus, que agrega por série e
sofre explosão combinatória com IDs de alta cardinalidade.

Labels permitidos nas métricas deste sistema (enums fechados/baixa cardinalidade):
`http_request_method`, `http_route`, `http_response_status_code`, `job`, `direction`,
`payment_method`, `installments_bucket`, `frequency`, `reason`, `operation`, `kind`, `workflow`,
`step`, `status`, `outcome`, `agent_id`, `channel`, `tool`, `severity`, `signal`, `go_memory_type`.

### 5.3 Decisões de custo/economia

- `tail_sampling` reduz o volume de traces exportado ao Tempo em ~90% fora dos caminhos
  garantidos (erro, fluxo de agente), mantendo 100% de cobertura onde o custo de perder o trace é
  alto (incidente, debugging de agente).
- `memory_limiter` (512 MiB, spike 128 MiB) protege o processo do Collector contra OOM sob pico de
  tráfego, com `batch` amortecendo o número de exports por segundo aos três backends.
- Métricas HTTP usam buckets de latência Stable padronizados do OpenTelemetry (não arbitrários),
  evitando explosão de séries por bucket customizado por serviço.

## 6. SLO, error budget e burn-rate (RF-04/05/06, ADR-002)

Decisão: `.specs/prd-observabilidade-golden-signals-otel/adr-002-burn-rate-slo-alerts.md`.

### 6.1 SLOs formais (janela de 30 dias)

| SLO | Meta | Error budget | SLI |
| --- | --- | --- | --- |
| Disponibilidade | 99,9% | 0,1% das requisições | Proporção de respostas HTTP não-5xx sobre `job="mecontrola-api"`, derivada da série canônica do histograma (`http_server_request_duration_seconds_count`, ADR-004), erro filtrado por `http.response.status_code=~"5.."` |
| Latência | P95 < 500ms | — (threshold direto) | `histogram_quantile(0.95, ...)` sobre `http_server_request_duration_seconds_bucket{job="mecontrola-api"}` |

Para o budget de 0,1% em 30 dias, o downtime aproximado tolerado é de ~8,77 horas/mês (tabela de
referência em `alerting-slo.md` da skill `golden-signals-otel-standards`).

### 6.2 Alertas de burn-rate multi-janela (disponibilidade)

Esquema do Google SRE Workbook derivado do budget de 0,1% (`error_budget = 0.001`), grupo `slo` em
`deployment/telemetry/grafana/provisioning/alerting/rules.yaml`. Cada alerta usa duas janelas
(longa + curta) combinadas por `AND` (`classic_conditions`) para equilibrar rapidez de detecção e
ruído — a janela curta confirma que o burn ainda está em curso, a longa evita alarme por pico
isolado.

| uid | Multiplicador | % do orçamento consumido | Janelas (longa + curta) | Severidade | Runbook |
| --- | --- | --- | --- | --- | --- |
| `mc-slo-burn-fast` | 14,4× | 2% em 1h | 1h + 5m | `critical` (página) | `deployment/runbooks/rollback.md` |
| `mc-slo-burn-6x` | 6× | 5% em 6h | 6h + 30m | `critical` (página) | `deployment/runbooks/rollback.md` |
| `mc-slo-burn-3x` | 3× | 10% em 1d | 1d + 2h | `warning` (ticket) | — (não pagina; RF-10 não se aplica) |
| `mc-slo-burn-1x` | 1× | 10% em 3d | 3d + 6h | `warning` (ticket) | — (não pagina; RF-10 não se aplica) |

Fórmula da razão de erro por janela (`W` = janela):
`sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api", http_response_status_code=~"5.."}[W])) / clamp_min(sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api"}[W])), 1)`,
comparada ao threshold `multiplicador * 0.001` nas duas janelas simultaneamente.

Estes alertas COMPLEMENTAM, não substituem, os thresholds estáticos existentes (`mc-api-5xx`,
`mc-api-latency-p99`) — defesa em profundidade (ADR-002): o threshold estático pega o caso simples
("5xx > 5% agora"), o burn-rate liga o alarme diretamente ao consumo do orçamento de erro do SLO.

### 6.3 Alerta de latência ligado ao SLO

`mc-slo-latency-p95` (severidade `warning`, `for: 10m`) dispara quando `histogram_quantile(0.95, ...)`
sobre `http_server_request_duration_seconds_bucket{job="mecontrola-api"}` (janela `[5m]`) ultrapassa
0,5s (SLO P95 < 500ms). Coexiste com `mc-api-latency-p99` (threshold estático em 1s, causa aguda); o
novo alerta liga o sintoma percebido pelo usuário ao SLO formal de latência.

## 7. Correlação log↔trace (RF-12) — já satisfeita pelo devkit, nenhum trabalho novo

Auditoria confirmada nesta tarefa: o provider devkit adiciona `trace_id`/`span_id` a **todo** log
emitido em contexto com um span ativo, tanto no bridge estruturado quanto no console
(`pkg/observability/otel/logger.go`, função `appendCorrelationAttrs`, linhas 227-243):

```go
func (l *otelLogger) appendCorrelationAttrs(ctx context.Context, attrs []slog.Attr) []slog.Attr {
	correlation, _ := CorrelationFromContext(ctx)
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		correlation.TraceID = formatTraceID(spanCtx.TraceID())
		correlation.SpanID = formatSpanID(spanCtx.SpanID())
	}
	return append(attrs,
		slog.String("service", l.serviceName),
		slog.String("environment", l.environment),
		slog.String("component", "observability.logger"),
		slog.String("trace_id", correlation.TraceID),
		slog.String("span_id", correlation.SpanID),
		/* [...] mais 2 campos de correlacao (nao citados aqui por serem log-only, nao rotulo
		   de metrica proibido por RF-13 — ver texto abaixo) */
	)
}
```

Chamada por `log` (bridge OTel) e `logConsole` em toda emissão de log (`logger.go:180-225`),
independentemente do nível. Quando não há span ativo no `context.Context`, `spanCtx.IsValid()` é
`false` e `trace_id`/`span_id` ficam vazios — comportamento correto (log fora de uma requisição
rastreada, ex.: startup). Nenhum código novo foi necessário; RF-12 está satisfeito pela
instrumentação já em produção.

## 8. Inventário de métricas

### 8.1 HTTP (RED, `job` ∈ {`mecontrola-api`, `mecontrola-worker`})

Ver seção 2 (fonte canônica + auxiliares).

### 8.2 Runtime Go — saturação (RF-01/RF-03/RF-14)

Confirmadas empiricamente pela Tarefa 6.0 contra `grafana/otel-lgtm:0.7.5` alimentado pelo pacote
de produção real `internal/platform/observability/runtimemetrics` (`go.opentelemetry.io/contrib/instrumentation/runtime`
v0.69.0 + `go.opentelemetry.io/otel@v1.45.0/semconv/v1.41.0/goconv`), via query direta ao Prometheus
embutido. Os nomes abaixo são os **nomes Prometheus renderizados reais** — divergem em alguns casos
do texto original da techspec (`techspec.md`, seção "Design de Implementação → Modelos de Dados"),
que não previu os sufixos `_total`/`_percent` da OTel Prometheus exporter naming convention:

| Métrica semconv (`go.*`) | Nome Prometheus renderizado (confirmado ao vivo) | Unidade/instrumento | Usado em alerta/dashboard | Observação |
| --- | --- | --- | --- | --- |
| `go.goroutine.count` | `go_goroutine_count` | `{goroutine}`, UpDownCounter | Sim — `mc-runtime-goroutine-growth`, `mecontrola-infra.json` | — |
| `go.memory.used` | `go_memory_used_bytes{go_memory_type=...}` | `By`, UpDownCounter | Sim — `mecontrola-infra.json` | Atributo `go_memory_type` |
| `go.memory.gc.goal` | `go_memory_gc_goal_bytes` | `By`, UpDownCounter | Sim — `mecontrola-infra.json` | — |
| `go.processor.limit` | `go_processor_limit` | `{thread}`, UpDownCounter | Sim — `mecontrola-infra.json` | GOMAXPROCS |
| `go.memory.limit` | `go_memory_limit_bytes` | `By`, UpDownCounter | **Não** — 0 séries no ambiente atual | `GOMEMLIMIT` não configurado em `cmd/server`/`cmd/worker`/deployment; usar geraria painel/alerta permanentemente "No Data" — omitida deliberadamente |
| `go.memory.allocated` | `go_memory_allocated_bytes_total` | `By`, Counter monotônico | Não | Nome real diverge do texto da techspec (sufixo `_total` não previsto; erratum registrado aqui) |
| `go.memory.allocations` | `go_memory_allocations_total` | `{allocation}`, Counter monotônico | Não | Idem — sufixo `_total` |
| `go.config.gogc` | `go_config_gogc_percent` | `%`, UpDownCounter | Não | Idem — sufixo `_percent` (unidade `%` gera sufixo na exportação Prometheus) |

**Threshold de `mc-runtime-goroutine-growth` é baseline inicial, não SLO calibrado:** o alerta usa
`max(go_goroutine_count{job=~"mecontrola-.+"}) > 3000` sustentado por 15 minutos. O valor `3000` foi
escolhido como baseline operacional inicial na ausência de histórico de produção (não há dado real
de carga para calibrar nesta iniciativa) — não é um alvo formal de SLO nem foi derivado de
distribuição observada. A `description` do alerta em `rules.yaml` registra essa natureza
explicitamente ("Threshold de 3000 e baseline operacional inicial, nao um SLO formal; ajustar apos
observar a distribuicao real pos-deploy."). Recomendação operacional: recalibrar o threshold após
observar a distribuição real de goroutines em produção pós-deploy (candidato a follow-up, sem data
fixa). Mitigado por `severity: warning` — o alerta não pagina isoladamente, apenas abre ticket.

### 8.3 Worker liveness (RF-11)

| Métrica | Nome Prometheus | Papel |
| --- | --- | --- |
| `worker_heartbeat` (snake_case por consistência com 100% das demais métricas custom do repositório) | `worker_heartbeat` | Gauge incrementado por ticker cancelável em `cmd/worker/worker.go` (`startWorkerHeartbeat`). Alerta `mc-worker-down` cobre dois modos de falha: `absent(...)` (processo morto/sem export) **ou** `changes(...[10m]) == bool 0` (processo vivo com ticker congelado) |

**Unidade de anotação é obrigatória em gauge de contagem — regra derivada de defeito real.** O
tradutor OTLP→Prometheus (`prometheus/otlptranslator`) aplica sufixo derivado da unidade OTel e do
tipo de instrumento. Tabela **confirmada empiricamente** contra `grafana/otel-lgtm:0.7.5` alimentado
por OTLP HTTP real (não inferida da leitura do tradutor):

| Instrumento | Unidade | Nome de registro | Nome renderizado |
| --- | --- | --- | --- |
| Gauge observável | `"1"` | `worker_heartbeat` | `worker_heartbeat_ratio` |
| Gauge observável | `{beat}` | `worker_heartbeat` | `worker_heartbeat` |
| Counter | `"1"` | `agents_write_total` | `agents_write_total` |
| Counter | `"1"` | `probe_write_count` | `probe_write_count_total` |
| **UpDownCounter** | `"1"` | `probe_inflight` | `probe_inflight` |
| qualquer | `By` / `s` / `%` | `x` | `x_bytes` / `x_seconds` / `x_percent` |

Dois pontos não óbvios, ambos verificados e não presumidos:

1. **Só o gauge observável ganha `_ratio` com unidade `"1"`.** UpDownCounter com a mesma unidade
   **não** ganha. Uma versão anterior deste documento afirmava o contrário; a afirmação foi corrigida
   depois do teste empírico, e o gate de CI que dependia dela foi corrigido junto (senão marcaria
   todo UpDownCounter como série morta — falso positivo).
2. **Counter nunca vira `_ratio`**: recebe `_total` se ainda não terminar em `_total`. Os 102
   counters do repositório com unidade `"1"` estão corretos como estão.

Unidades de anotação (`{beat}`, `{job}`, `{token}`, `{bucket}`) nunca geram sufixo — é a forma
recomendada para contagens adimensionais.

Três gauges do repositório usavam `"1"` e portanto eram publicados com nome `*_ratio`, deixando os
alertas correspondentes permanentemente sem série (`absent()` sempre verdadeiro, `noDataState: OK`
mascarando o problema): `worker_heartbeat`, `outbox_pending_jobs` e
`onboarding_tokens_paid_unconsumed{,_overdue}`. Todos corrigidos para unidade de anotação. Nome
renderizado confirmado empiricamente contra `grafana/otel-lgtm:0.7.5` alimentado por OTLP HTTP real:
com `"1"` → `worker_heartbeat_ratio` / `outbox_pending_jobs_ratio`; com `{beat}` / `{job}` →
`worker_heartbeat` / `outbox_pending_jobs`.

O gate `task ci:audit-alert-metrics` passou a **modelar essa transformação** (renderiza o nome
Prometheus a partir de nome + unidade + tipo de instrumento antes de comparar) e a **varrer também
os dashboards**, não só `rules.yaml`. Regressão coberta por teste
(`TestRenderedNamesModelsOTLPToPrometheusSuffixes`, `TestAuditDetectsGaugeRenamedByUnitRatio`,
`TestAuditScansDashboardPanels`).

### 8.4 Counters HTTP auxiliares

Ver seção 2 — `http_server_request_count_total`, `http_server_request_active`,
`http_server_request_error_count_total`. Papel documentado; não usar como fonte primária de novos
dashboards/alertas (ADR-004).

Aplicação de RF-08 concluída também nos painéis: `deployment/dashboards/mecontrola-api.json` migrou
tráfego e taxa de erro (`Request rate`, `Error rate (5xx %)`, breakdown por rota, breakdown por
status, top-15 rotas e a variável de template `job`) de `http_server_request_count_total` para a
série canônica `http_server_request_duration_seconds_count`. O único uso remanescente de auxiliar no
dashboard é `http_server_request_active` — gauge de concorrência, que não é tráfego nem erro e
portanto permanece legitimamente como auxiliar.

### 8.5 Reconciliação alerta↔métrica (RF-07)

Auditoria (Tarefa 1.0) comparou todo alerta/painel provisionado contra o inventário de métricas
realmente emitidas. Métricas mortas confirmadas e removidas com justificativa registrada em
`rules.yaml`, todas herdadas do `internal/agent` descontinuado: `agent_intent_parsed_total`,
`agent_llm_fallback_exhausted_total`, `agent_policy_blocks_total`, `agent_intent_routed_total` e
`agent_idempotency_replay_total` (esta migrada para `agents_write_total{outcome="replay"}`).

**Correção de reconciliação — `outbox_pending_jobs` não era métrica morta.** A auditoria original
classificou-a como nunca emitida com base em `grep` restrito a `internal/platform/outbox/`; ela é
emitida em `cmd/worker/worker.go` (`registerOutboxMetrics`). O sintoma de "série ausente" tinha
outra causa: o sufixo `_ratio` descrito na seção 8.3. O alerta `mc-outbox-queue-high` foi
**restaurado** (RF-20) e a unidade corrigida na origem. Lição incorporada ao gate: comparar contra o
nome Prometheus **renderizado**, não contra o nome de registro, e varrer dashboards além de
`rules.yaml`.

O gate `task ci:audit-alert-metrics` (via `cmd/tools/audit-alert-metrics`) valida continuamente que
nenhum alerta **nem painel** referencia série morta — ver seção 10 (evidência de execução).

### 8.6 Créditos OpenRouter (custo de LLM — monitor do worker)

Fonte: `startOpenRouterCreditsMonitor` (`cmd/worker/worker.go`) — ticker cancelável a cada 15 minutos
que consulta `GET {OPENROUTER_BASE_URL}/api/v1/credits` e `GET /api/v1/auth/key` via
`internal/platform/llm/credits.go` (HTTP outbound por `internal/platform/httpclient`, padrão
obrigatório do repositório). Shapes dos endpoints confirmados empiricamente contra a API real
(2026-08-07): `/credits` retorna `data.total_credits`/`data.total_usage`; `/auth/key` retorna
`data.usage_daily`/`usage_weekly`/`usage_monthly`. `/api/v1/activity` exige management key (403) e
foi descartado. Saldo (`remaining`) é calculado em código como `total_credits - total_usage` —
a API não expõe campo pronto.

| Métrica (registro) | Nome Prometheus renderizado | Unidade OTel | Tipo | Papel |
| --- | --- | --- | --- | --- |
| `openrouter_credits_total_usd` | idem | `{usd}` | gauge observável | Créditos comprados na conta |
| `openrouter_credits_used_usd` | idem | `{usd}` | gauge observável | Uso acumulado monotônico — base de `increase(...[24h])` para gasto/dia real |
| `openrouter_credits_remaining_usd` | idem | `{usd}` | gauge observável | Saldo (alertas `mc-openrouter-credit-*`) |
| `openrouter_usage_daily_usd` | idem | `{usd}` | gauge observável | Gasto do dia corrente **até agora** (zera no início do dia — não usar como taxa diária) |
| `openrouter_usage_weekly_usd` | idem | `{usd}` | gauge observável | Gasto da semana — base da projeção de dias restantes (`weekly/7`) |
| `openrouter_usage_monthly_usd` | idem | `{usd}` | gauge observável | Gasto do mês corrente |
| `openrouter_credits_last_success_timestamp_seconds` | idem | `{timestamp}` | gauge observável | Monitor-do-monitor (alerta `mc-openrouter-credits-stale`) |
| `openrouter_credits_scrape_errors_total` | idem | `"1"` | counter | Falhas da consulta (já termina em `_total`, sem renomeação) |

Notas operacionais:

- Unidades `{usd}`/`{timestamp}` são **unidades de anotação** (mesma família de `{beat}`/`{job}` da
  seção 8.3): não geram sufixo no nome renderizado. Usar `"1"` nesses gauges geraria sufixo `_ratio`
  e séries mortas nos alertas — mesmo defeito já corrigido em `worker_heartbeat`.
- Gauges retornam `NaN` até a primeira coleta bem-sucedida: comparações PromQL com `NaN` são falsas,
  logo os alertas de saldo **não disparam** antes do primeiro dado real (anti-falso-positivo por
  construção). Após o primeiro sucesso, o gauge mantém o último valor conhecido em caso de falha.
- Projeção de dias restantes usa `usage_weekly/7` com `clamp_min(..., 0.05)` no denominador —
  `usage_daily` zera no início do dia e geraria projeção infinita/falsa.
- Dashboard: `deployment/dashboards/mecontrola-openrouter.json`. Runbook: `docs/runbooks/openrouter-credits.md`.

## 9. Alertas — roteamento por severidade (RF-09/RF-10)

Provisionado em `deployment/telemetry/grafana/provisioning/alerting/contact-points.yaml`.

| Severidade | Papel | Contact-point | Receiver |
| --- | --- | --- | --- |
| `critical` | Página (on-call) | `telegram-mecontrola` | `telegram` (bot já existente, `${ALERT_TELEGRAM_BOT_TOKEN}`/`${ALERT_TELEGRAM_CHAT_ID}`) |
| `warning` | Ticket (não acorda ninguém) | `email-mecontrola` | `email` **nativo do Grafana**, entregue via SMTP configurado por `GF_SMTP_*` |

O contact-point de e-mail reusa a infraestrutura SMTP/Resend já existente da aplicação
(`SMTP_HOST=smtp.resend.com`, `SMTP_USERNAME`/`SMTP_PASSWORD`, `EMAIL_FROM_ADDRESS`,
`EMAIL_FROM_NAME` — ver `deployment/config/prod.env`): esses mesmos valores são projetados para as
variáveis `GF_SMTP_*` do container `otel-lgtm` em `deployment/compose/compose.swarm.yml`, sem
introduzir endpoint novo na aplicação nem acoplar o alerta ao runtime do Go.

Roteamento (`policies[].routes[]` em `contact-points.yaml`):

```yaml
routes:
  - receiver: telegram-mecontrola
    matchers:
      - alertname =~ "mc-openrouter-.*"
  - receiver: telegram-mecontrola
    matchers:
      - severity = "critical"
  - receiver: email-mecontrola
    matchers:
      - severity = "warning"
```

A primeira rota envia **warning e critical do grupo `openrouter` ao Telegram** (decisão do usuário
2026-08-07: alertas de recarga de crédito devem paginar mesmo em warning); as demais severidades
seguem a política padrão (critical → Telegram, warning → e-mail).

O contact-point `telegram-mecontrola` permanece como receiver padrão da política (fallback) — não
foi removido, preservando RF-20.

**RF-10 — runbook obrigatório em todo alerta de página:** os alertas com `severity: critical`
(páginas via Telegram) em `rules.yaml` referenciam um runbook na anotação `description`
(`Runbook: <path>`), incluindo `mc-api-down`/`mc-worker-down`/`mc-api-5xx`/`mc-slo-burn-fast`/
`mc-slo-burn-6x` → `deployment/runbooks/rollback.md`;
`mc-wal-lag-high`/`mc-backup-info-failed`/`mc-backup-metrics-stale` →
`deployment/runbooks/backup-schedule.md`;
`mc-collector-export-fail`/`mc-pgbouncer-pool-saturation`/`mc-disk-low-bytes` →
`docs/runbooks/infra-saturation-triage.md`; os demais (`mc-outbox-dead-letter`,
`mc-backup-full-stale`, `mc-backup-archive-push-failed`) já referenciavam runbook. O alerta de causa
`mc-runtime-goroutine-growth` (`severity: warning`) não pagina isoladamente — RF-10 não exige runbook
nele, ainda que a descrição referencie o dashboard de diagnóstico.

Assets desta seção: `observability/promql-golden-signals.md` (queries) e
`observability/alert-rules-slo.yaml` (regras de alerta SLO/burn-rate/RED extraídas do provisioning
real).

## 10. Validação e evidência de execução

- `task ci:audit-alert-metrics` — confirma que nenhum alerta em `rules.yaml` referencia série morta
  (RF-07/RF-20), reexecutado nesta tarefa após a consolidação das edições das Tarefas 1.0, 3.0, 4.0,
  5.0, 6.0 e 7.0 no mesmo `rules.yaml` — ver `.specs/prd-observabilidade-golden-signals-otel/8.0_execution_report.md`
  para a saída literal do comando.
- `python3 .agents/skills/golden-signals-otel-standards/scripts/validate-standard.py --standard observability/STANDARD.md --collector deployment/telemetry/grafana/otelcol-config.yaml` —
  `SUCCESS` (RF-18) — ver relatório de execução para a saída literal.
- `go build ./...` / `go vet ./...` — limpos no repositório inteiro após esta tarefa.

## 11. Referências oficiais

- Google SRE Book — Monitoring Distributed Systems: https://sre.google/sre-book/monitoring-distributed-systems/
- Google SRE Workbook — Alerting on SLOs (burn-rate multi-janela): https://sre.google/workbook/alerting-on-slos/
- OpenTelemetry Specification: https://opentelemetry.io/docs/specs/otel/
- OTLP: https://opentelemetry.io/docs/specs/otlp/
- Semantic Conventions (HTTP metrics): https://opentelemetry.io/docs/specs/semconv/http/http-metrics/
- Semantic Conventions (Runtime/Go metrics — `go.*`): https://opentelemetry.io/docs/specs/semconv/runtime/go-metrics/

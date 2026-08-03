<!-- spec-hash-prd: 5b903694f14214c5f0683252fce5838b97a94465f3d3f7ecf8b7210174ced803 -->

# Especificação Técnica: Observabilidade — Fechamento de Gaps e Reconciliação

## Resumo Executivo

Esta especificação implementa o fechamento de gaps residuais sobre a stack de observabilidade production-grade já existente (`deployment/telemetry/`), sem reconstruí-la. O trabalho tem cinco frentes independentes e de baixo acoplamento: (1) instrumentar a saturação do processo Go em `cmd/server` e `cmd/worker` com `go.opentelemetry.io/contrib/instrumentation/runtime` v0.69.0, ativando o MeterProvider global via `RegisterGlobal: true` (o provider devkit não expõe getter de MeterProvider); (2) formalizar SLO 99,9%/P95<500ms e adicionar alertas de burn-rate multi-janela no Grafana provisionado, ao lado dos thresholds existentes; (3) auditar e reconciliar `provisioning/alerting/rules.yaml` contra o inventário de métricas vivas, corrigindo/removendo alertas que referenciam séries mortas do `internal/agent` descontinuado; (4) adicionar um contact-point de e-mail e roteamento por severidade (página→Telegram, ticket→e-mail); (5) materializar `observability/STANDARD.md` documentando a topologia real e validá-lo. A correlação log↔trace já é fornecida pelo devkit e será apenas confirmada. Todas as mudanças são aditivas ou corretivas; nenhuma remove instrumentação, dashboard ou alerta válido.

Decisões materiais estão registradas em ADRs: `adr-001` (RegisterGlobal + runtime), `adr-002` (burn-rate SLO), `adr-003` (reconciliação alerta↔métrica), `adr-004` (fonte canônica de RED).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes modificados ou adicionados:

- `cmd/server/server.go` (modificado): `otel.Config{RegisterGlobal: true}` + chamada a `runtime.Start()` após `otel.NewProvider`.
- `cmd/worker/worker.go` (modificado): idem, com `ServiceNameWorker`; adicionalmente emite um heartbeat de liveness (RF-11) — um gauge `worker.heartbeat` atualizado por um ticker cancelável no bootstrap do worker, usando `o11y.Metrics().Gauge`, com shutdown cooperativo junto ao lifecycle do worker.
- `internal/platform/observability/runtimemetrics/` (novo pacote fino): função `Start(o11y, minInterval) error` que encapsula `runtime.Start(...)` e a decisão de intervalo, isolando a dependência da lib de contrib.
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` (modificado): novos grupos `slo` (burn-rate) e `runtime`; correção/remoção de alertas com métrica morta.
- `deployment/telemetry/grafana/provisioning/alerting/contact-points.yaml` (modificado): novo contact-point de e-mail + política de roteamento por severidade.
- `deployment/dashboards/mecontrola-api.json` e/ou novo painel (delegado à skill `otel-grafana-dashboards`): saturação de runtime e visão de SLO.
- `observability/STANDARD.md` + `observability/promql-golden-signals.md` + `observability/alert-rules-slo.yaml` (novos): padrão auditável e assets, validados por `scripts/validate-standard.py`.

Componentes preservados intactos (não-regressão): `otelcol-config.yaml` (Collector + tail sampling), pipelines OTLP, exporters LGTM, dashboards e alertas válidos existentes.

Fluxo de dados (inalterado na espinha dorsal): `app (server/worker) --OTLP 4317/4318--> Collector (memory_limiter, tail_sampling, batch) --> Prometheus/Tempo/Loki --> Grafana (dashboards + unified alerting) --> Telegram/e-mail`.

## Design de Implementação

### Interfaces Chave

Pacote novo `internal/platform/observability/runtimemetrics` — adapter fino sobre a lib de contrib (segue R-ADAPTER-001: zero comentários, responsabilidade única):

```go
package runtimemetrics

import (
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
)

func Start(o11y observability.Observability, minInterval time.Duration) error {
	return runtime.Start(runtime.WithMinimumReadMemStatsInterval(minInterval))
}
```

Racional: `runtime.Start` usa `otel.GetMeterProvider()` (global) por default. Como o provider devkit chama `otel.SetMeterProvider` apenas quando `RegisterGlobal` é `true` (`pkg/observability/otel/config.go:244-245`) e não expõe getter público de MeterProvider (só `Tracer()/Logger()/Metrics()/HTTP()`, `config.go:337-345`), o único caminho é ativar o global. O parâmetro `o11y` é recebido para manter a fronteira do pacote e permitir log de bootstrap, mas a lib resolve o meter via global.

### Modelos de Dados

Não há entidades de domínio. Os "modelos" são as séries de métrica emitidas pela lib v0.69.0 (caminho default `go.*`, verificado em `runtime.go:43-50` do módulo):

| Métrica (OTel) | Série Prometheus (aprox.) | Unidade | Uso |
|---|---|---|---|
| `go.goroutine.count` | `go_goroutine_count` | {goroutine} | goroutine leak |
| `go.memory.used` | `go_memory_used_bytes` | By | pressão de heap/stack (attr `go.memory.type`) |
| `go.memory.limit` | `go_memory_limit_bytes` | By | limite `GOMEMLIMIT` |
| `go.memory.allocated` | `go_memory_allocated_bytes` | By | alocação acumulada |
| `go.memory.allocations` | `go_memory_allocations` | {allocation} | contagem de alocações |
| `go.memory.gc.goal` | `go_memory_gc_goal_bytes` | By | alvo de heap do GC |
| `go.processor.limit` | `go_processor_limit` | {thread} | GOMAXPROCS |
| `go.config.gogc` | `go_config_gogc` | % | GOGC |

O nome Prometheus exato depende do exporter do Collector (sufixos de unidade). A implementação DEVE confirmar o nome renderizado no Prometheus antes de fixá-lo nos alertas/dashboards e registrá-lo no `STANDARD.md` (RF-14).

### Endpoints de API

Não há novos endpoints HTTP. Os endpoints OTLP (4317/4318) e o `/metrics` do Collector já existem.

## Pontos de Integração

- Grafana Unified Alerting (provisionado): novos alertas seguem o schema existente em `rules.yaml` (`groups[].rules[]` com `data[].model.expr`, `datasourceUid: prometheus`, `condition`/`threshold`). O contact-point de e-mail segue o schema de `contact-points.yaml` (`contactPoints[].receivers[]` + `policies[].routes[]` com `matchers` por `severity`).
- E-mail de ticket: usar o receiver `email` NATIVO do Grafana, configurado via `GF_SMTP_*` (host, usuário, senha, from). Provisionado em `contact-points.yaml` como um novo contact-point `email-mecontrola`; não requer código na aplicação nem endpoint novo, e não acopla o alerta ao runtime da app. Descartado o webhook para o serviço de e-mail da app (Resend) por criar endpoint e acoplamento desnecessários.
- OpenRouter/LLM: sem impacto (regra: LLM só nas call-sites sancionadas).

## Abordagem de Testes

### Testes Unitários

- `runtimemetrics.Start`: teste que verifica que a chamada não retorna erro com um MeterProvider global de teste registrado (usar `sdkmetric.NewMeterProvider` + `otel.SetMeterProvider` no setup; assert nil error e presença dos instrumentos via `metricdata` reader). Mock apenas do reader; sem mock da lib.
- Sanidade de config: teste em `cmd/server`/`cmd/worker` (ou no bootstrap) garantindo que o `otel.Config` construído tem `RegisterGlobal == true`.

### Testes de Integração

> Critérios avaliados: (a) fronteira de IO crítica = exportação OTLP para o Collector; (b) risco de o global não registrar quebrar runtime metrics silenciosamente. Duas condições atendidas → integração recomendada, porém de baixo custo.

- Teste de integração leve (build tag `//go:build integration`): sobe o provider com `RegisterGlobal: true` apontando para um Collector/OTLP mock (ou um `sdkmetric` in-memory reader via global), chama `runtimemetrics.Start`, força uma coleta e assert que `go.goroutine.count` é observado. Isso protege contra regressão do wiring global.
- Reconciliação (RF-07): um teste/script de auditoria que extrai todos os nomes de métrica referenciados em `provisioning/alerting/rules.yaml` e confronta com o inventário de métricas emitidas (grep no código + lista devkit conhecida), falhando se algum alerta referenciar série não-emitida. Este script vira gate de CI.

### Testes E2E

Validação manual/observacional pós-deploy: confirmar no Grafana que os painéis de runtime populam, que os alertas de burn-rate avaliam sem erro (`execErrState`), e que uma página de teste chega ao Telegram e um ticket de teste ao e-mail.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Reconciliação (RF-07) primeiro** — auditar `rules.yaml`, corrigir/remover alertas com métrica morta e criar o script de auditoria/gate. Faz o estado atual ficar honesto antes de adicionar coisas novas; sem dependência de código.
2. **Instrumentação de runtime (RF-01/02/03)** — pacote `runtimemetrics`, `RegisterGlobal: true` em server e worker, `runtime.Start` no bootstrap; confirmar nomes Prometheus.
3. **SLO + burn-rate (RF-04/05/06)** — novos grupos de alerta derivados do error budget; depende de os nomes de série de latência/erro estarem confirmados.
4. **Fonte canônica de RED (RF-08)** — migrar `mc-api-5xx` para a série do histograma ou justificar; depende de 3 para consistência de queries.
5. **E-mail + roteamento (RF-09/10)** — contact-point `email-mecontrola` (receiver `email` nativo do Grafana via `GF_SMTP_*`) e política por severidade (página→Telegram, ticket→e-mail); runbooks referenciados.
5b. **Worker heartbeat + staleness (RF-11)** — gauge `worker.heartbeat` por ticker cancelável no bootstrap do worker + alerta `absent()`/staleness em `rules.yaml`, espelhando `mc-api-down`. Independente das demais frentes.
6. **Runtime nos dashboards (RF-19)** — delegado à skill de dashboards; depende de 2.
7. **STANDARD.md + validação (RF-18)** — consolida topologia real, inventário de métricas, cardinalidade e decisões; roda `scripts/validate-standard.py`.

### Dependências Técnicas

- Adicionar `go.opentelemetry.io/contrib/instrumentation/runtime v0.69.0` ao `go.mod` (compatível com `otel v1.44.0`, já presente).
- Acesso de escrita a `deployment/telemetry/grafana/provisioning/` e `observability/`.
- SMTP configurado no Grafana (`GF_SMTP_*`) para o receiver de e-mail.

## Monitoramento e Observabilidade

- Métricas novas: `go.*` de runtime (RF-01), exportadas pelo pipeline existente.
- Alertas novos: grupo `slo` (burn-rate disponibilidade, RF-05), alerta de latência SLO (RF-06), alerta de causa de runtime (RF-03), tudo em `rules.yaml` provisionado.
- Dashboards: painel de runtime + SLO (RF-19, delegado).
- Logs: correlação `trace_id`/`span_id` já emitida pelo devkit (`pkg/observability/otel/logger.go:229-238`) — confirmar, sem novo trabalho (RF-12).

## Considerações Técnicas

### Decisões Chave

- ADR-001 — Ativar `RegisterGlobal: true` e instrumentar runtime via lib de contrib (em vez de reimplementar métricas de runtime com `o11y.Metrics()`).
- ADR-002 — Adicionar alertas de burn-rate multi-janela no Grafana provisionado, coexistindo com os thresholds estáticos existentes.
- ADR-003 — Política de reconciliação alerta↔métrica: corrigir para métrica viva ou remover; gate de CI de auditoria.
- ADR-004 — Fonte canônica de RED = série do histograma `http_server_request_duration_seconds_count`; counters devkit rebaixados a auxiliares.

### Riscos Conhecidos

- **Ativar o MeterProvider global pode afetar código que lê o global**: hoje o global está desligado e nada o consome (instrumentação usa `o11y.Metrics()`/`o11y.Tracer()` de instância). Mitigação: auditar usos de `otel.GetMeterProvider`/`otel.GetTracerProvider` (esperado zero no app) antes do merge; teste de não-regressão das métricas de instância. Rollback: reverter `RegisterGlobal` para o zero value.
- **Nome Prometheus das séries `go.*` incerto até o deploy** (sufixos de unidade do exporter): mitigação em RF-14 — confirmar a série renderizada antes de fixar alerta/dashboard; não hardcodar sem confirmação.
- **`go.schedule.duration` indisponível** (exige `metric.Producer` no Reader, não exposto pelo devkit): documentado como limitação (Fora de Escopo), não implementado sem mudança upstream no devkit.
- **Drift adicional não detectado**: a auditoria RF-07 pode revelar mais alertas/painéis com métricas mortas além dos seis confirmados; o gate de CI cobre a detecção contínua.

### Conformidade com Padrões

- R-ADAPTER-001 (`.claude/rules/go-adapters.md`): pacote `runtimemetrics` é adapter fino, zero comentários.
- R-TXN-004, R-WF-KERNEL-001.4, R-AGENT-WF-001.5: cardinalidade controlada nas métricas e alertas.
- `.claude/rules/go-testing.md`: testes de use case (não se aplica diretamente; o pacote é de plataforma, testado com reader in-memory).
- Go — versão de `go.mod` respeitada; nova dependência compatível com `otel v1.44.0`.

### Arquivos Relevantes e Dependentes

- `cmd/server/server.go:63-82`, `cmd/worker/worker.go:68-80` — construção do `otel.Config` e `NewProvider`.
- `internal/platform/observability/runtimemetrics/` — novo pacote.
- `deployment/telemetry/grafana/otelcol-config.yaml` — Collector (preservado).
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` — alertas (modificado).
- `deployment/telemetry/grafana/provisioning/alerting/contact-points.yaml` — roteamento (modificado).
- `deployment/dashboards/mecontrola-api.json` — dashboard (estendido, delegado).
- `observability/STANDARD.md` e assets — novos.
- Devkit (referência, module cache): `pkg/observability/otel/config.go:53,68,116-131,244-245,337-345`, `http.go:14-84`, `logger.go:229-238`.
- Lib de runtime (referência): `runtime.go:24-156` (v0.69.0).

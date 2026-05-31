# Tarefa 4.0: `internal/infrastructure/observability` — OTel OTLP + slog + redaction de PII

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Compor o `devkit-go/pkg/observability` em `internal/infrastructure/observability` provendo **OTel OTLP gRPC** (traces + metrics + logs correlated) + **`slog` via `otelslog`** + **redaction automática de PII** (`phone`, `password`, `token`, `card_number`, `amount`) + exportação para **Grafana Cloud free tier** (D-02). Cobre **RF-11** integralmente. Habilita as métricas custom da foundation (`bootstrap_duration_seconds`, `events_published_total`, `health_probe_status`).

<requirements>
- Factory `NewProvider(cfg *configs.Config) (*Provider, shutdown func(context.Context) error, error)`.
- Exporter OTLP gRPC configurado pelo `O11yConfig.ExporterEndpoint` + `ExporterProtocol` (default `grpc`) + `ExporterInsecure` (default `false`) + headers via env `OTEL_EXPORTER_OTLP_HEADERS` (basic auth Grafana).
- Resource attributes: `service.name=mecontrola`, `service.version=<from cfg>`, `deployment.environment=<from cfg>`.
- TraceSampleRate configurável via `O11yConfig.TraceSampleRate`; default 1.0 (100%) — D-21.
- Logs estruturados JSON via `slog` + `otelslog` carregando `request_id`, `trace_id`, `span_id` automaticamente.
- Redaction de PII enforced — proibido emitir trace/span/log com `phone`, `password`, `token`, `card_number`, `amount` sem mascaramento.
- Métricas custom registradas no startup: `bootstrap_duration_seconds`, `events_published_total{event_name,outcome}`, `health_probe_status{check}`.
- Shutdown drena spans/logs/metrics no SIGTERM via `Shutdowner` do devkit-go.
</requirements>

## Subtarefas

- [ ] 4.1 Criar `internal/infrastructure/observability/provider.go` com `NewProvider(cfg) (*Provider, shutdown, error)` composando `pkg/observability` do devkit-go.
- [ ] 4.2 Configurar resource attributes (service.name/version/deployment.environment) a partir do cfg.
- [ ] 4.3 Configurar exporter OTLP gRPC com endpoint + headers + insecure flag do cfg.
- [ ] 4.4 Inicializar `slog` global via `otelslog` (handler que propaga trace_id/span_id).
- [ ] 4.5 Registrar redaction list de PII em `internal/infrastructure/observability/redaction.go` (`phone`, `password`, `token`, `card_number`, `amount`).
- [ ] 4.6 Criar `internal/infrastructure/observability/metrics.go` registrando métricas custom da foundation.
- [ ] 4.7 Criar `internal/infrastructure/observability/provider_test.go` com mock de exporter; valida que (a) shutdown drena; (b) log com campo `phone` é redatado; (c) span carrega resource attributes esperados.
- [ ] 4.8 Documentar em `internal/infrastructure/observability/doc.go` o contrato + invariantes (redaction obrigatória).

## Detalhes de Implementação

Ver techspec §"Monitoramento e Observabilidade" + §"Estratégia de Erros" + D-02 (Grafana Cloud). Não duplicar.

## Critérios de Sucesso

- `NewProvider(cfg)` retorna `*Provider` com TraceProvider + MeterProvider + LoggerProvider ligados; `shutdown(ctx)` drena.
- Span de teste exportado contém `service.name=mecontrola` + `service.version=<sha>` + `deployment.environment=<env>`.
- `slog.Info("evt", "phone", "+5511999999999")` produz log com `phone="***"` (mascarado).
- Métricas custom registradas via `meter.Float64Histogram("bootstrap_duration_seconds")` (assert na criação).
- `go test ./internal/infrastructure/observability/... -race` verde.
- Cobre RF-11 integralmente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `otel-grafana-dashboards` — gerar dashboards Grafana iniciais para a foundation (painel "Plataforma" com latência HTTP, DB pool, bootstrap_duration) a partir das métricas custom registradas nesta task

## Testes da Tarefa

- [ ] Testes unitários: `provider_test.go` (mock exporter — assert spans/logs/metrics + redaction + shutdown).
- [ ] Testes de integração: smoke local com docker-compose `otel-collector` opcional; não bloqueia a task (validação real em 10.0 com Grafana Cloud).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/infrastructure/observability/provider.go`
- `internal/infrastructure/observability/redaction.go`
- `internal/infrastructure/observability/metrics.go`
- `internal/infrastructure/observability/provider_test.go`
- `internal/infrastructure/observability/doc.go`
- `go.mod`, `go.sum`

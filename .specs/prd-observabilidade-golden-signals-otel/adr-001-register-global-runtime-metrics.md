# Registro de Decisão Arquitetural (ADR-001)

## Metadados

- **Título:** Ativar MeterProvider global (`RegisterGlobal: true`) e instrumentar saturação de runtime via `contrib/instrumentation/runtime`
- **Data:** 2026-08-03
- **Status:** Aceita
- **Decisores:** Engenharia de plataforma (on-call)
- **Relacionados:** PRD `.specs/prd-observabilidade-golden-signals-otel/prd.md` (RF-01, RF-02, RF-03, RF-14), techspec.md, ADR-004

## Contexto

Hoje não há métricas de saturação do processo Go (goroutines/heap/GC); só saturação de host via node-exporter. A biblioteca oficial `go.opentelemetry.io/contrib/instrumentation/runtime` (v0.69.0, compatível com `otel v1.44.0`) emite, no caminho default, as séries `go.*` (`go.goroutine.count`, `go.memory.*`, `go.memory.gc.goal`, `go.processor.limit`, `go.config.gogc`). Sua função `runtime.Start` resolve o meter via `otel.GetMeterProvider()` global.

Restrições verificadas no provider devkit (module cache):
- `Provider` expõe apenas `Tracer()/Logger()/Metrics()/HTTP()` — não há getter público de `MeterProvider` (`pkg/observability/otel/config.go:337-345`).
- `otel.SetMeterProvider` só é chamado quando `config.RegisterGlobal` é `true` (`config.go:244-245`).
- `NewProvider` não aplica defaults ao `Config` (`config.go:116-131`); a app monta `otel.Config{}` literal sem `RegisterGlobal` em `cmd/server/server.go:63-74` e `cmd/worker/worker.go:68-79`, deixando o global desligado.

## Decisão

Ativar `RegisterGlobal: true` no `otel.Config` de server e worker e chamar `runtime.Start` (via um pacote adapter fino `internal/platform/observability/runtimemetrics`) logo após `otel.NewProvider`. A instrumentação de runtime passa a emitir `go.*` pelo pipeline OTLP já existente. `go.schedule.duration` (histograma via `metric.Producer`) fica de fora por exigir registro no Reader, que o devkit constrói internamente e não expõe.

## Alternativas Consideradas

- **Passar um MeterProvider explícito via `runtime.WithMeterProvider`**: inviável — o devkit não expõe seu `sdkmetric.MeterProvider`. Rejeitada por falta de acesso.
- **Reimplementar métricas de runtime manualmente com `o11y.Metrics()`**: reinventa a lib oficial, arrisca nomes fora do semconv (viola RF-14) e mais manutenção. Rejeitada por robustez/economia.
- **Fork/patch do devkit para expor o MeterProvider**: fora do controle do repositório; maior custo. Rejeitada.

## Consequências

### Benefícios Esperados

- Fecha o gap de saturação do processo com a lib oficial e nomes canônicos.
- Zero nova série inventada; reuso do pipeline existente (economia).

### Trade-offs e Custos

- Registrar o MeterProvider/TracerProvider global muda estado de processo antes desligado.
- Nova dependência no `go.mod`.

### Riscos e Mitigações

- **Risco:** código que leia o global passe a se comportar diferente. **Impacto:** baixo — instrumentação atual usa instância (`o11y.*`), não o global. **Mitigação:** auditar `otel.GetMeterProvider`/`otel.GetTracerProvider` (esperado zero no app) e teste de não-regressão das métricas de instância. **Rollback:** reverter `RegisterGlobal` ao zero value.
- **Risco:** nome Prometheus das séries `go.*` incerto (sufixos de unidade). **Mitigação:** confirmar a série renderizada antes de fixar em alerta/dashboard (RF-14).

## Plano de Implementação

1. Adicionar `go.opentelemetry.io/contrib/instrumentation/runtime v0.69.0` ao `go.mod`.
2. Criar `internal/platform/observability/runtimemetrics` com `Start(o11y, minInterval)`.
3. Setar `RegisterGlobal: true` em server e worker; chamar `runtimemetrics.Start` após `NewProvider`.
4. Confirmar nomes Prometheus; adicionar painel (RF-19) e alerta de causa de runtime (RF-03).

## Monitoramento e Validação

- Sucesso: `go.goroutine.count` e `go.memory.used` visíveis no Prometheus/Grafana para `mecontrola-api` e `mecontrola-worker`.
- Teste de integração de wiring global (build tag `integration`).
- Reverter se as métricas de instância existentes regredirem.

## Impacto em Documentação e Operação

- `observability/STANDARD.md`: registrar as séries `go.*` e o nome Prometheus confirmado.
- Runbook de saturação de runtime referenciado pelo alerta de causa.

## Revisão Futura

- Revisitar se o devkit passar a expor o MeterProvider (permitindo `WithMeterProvider` e o Producer de `go.schedule.duration`) ou se a app crescer para múltiplos serviços.

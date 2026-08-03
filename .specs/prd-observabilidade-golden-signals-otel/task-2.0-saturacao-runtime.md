# Tarefa 2.0: Saturação de runtime do processo Go (server + worker)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar métricas de saturação do processo Go (goroutines/heap/GC) ao pipeline OTLP já existente, para API (`cmd/server`) e worker (`cmd/worker`). Hoje só há saturação de host via node-exporter; goroutine leak, pressão de heap e comportamento do GC são invisíveis. A tarefa instrumenta ambos os processos com a lib oficial `go.opentelemetry.io/contrib/instrumentation/runtime` v0.69.0 (compatível com `otel v1.44.0`), ativando o MeterProvider global (`RegisterGlobal: true`) porque o provider devkit não expõe getter público de MeterProvider e a lib resolve o meter via global. Cobre RF-01, RF-02, RF-14 e RF-17. Todas as mudanças são aditivas; nenhuma métrica/span de instância existente pode regredir.

<requirements>
- Adicionar `go.opentelemetry.io/contrib/instrumentation/runtime v0.69.0` ao `go.mod` (compatível com `otel v1.44.0` já presente); não atualizar `otel` para satisfazer a dependência.
- Criar o pacote fino `internal/platform/observability/runtimemetrics` com `func Start(o11y observability.Observability, minInterval time.Duration) error` que encapsula `runtime.Start(runtime.WithMinimumReadMemStatsInterval(minInterval))` (adapter fino R-ADAPTER-001, zero comentários, responsabilidade única).
- Ativar `RegisterGlobal: true` no `otel.Config` de `cmd/server/server.go` (linhas 63-74) e `cmd/worker/worker.go` (linhas 68-79); chamar `runtimemetrics.Start` logo após `otel.NewProvider` em ambos.
- Preservar os atributos de recurso `service.name`/`service.version`/`deployment.environment` + `service.instance.id` já injetados (RF-17).
- Séries emitidas são exatamente as do caminho default `go.*` da v0.69.0; nenhum nome pode ser inventado (RF-14).
- Auditar `otel.GetMeterProvider`/`otel.GetTracerProvider` no app (esperado zero) antes do merge; ativar o global não pode quebrar as métricas de instância via `o11y.Metrics()` (RF-02).
- Zero comentários em Go de produção (R-ADAPTER-001).
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar a dependência `go.opentelemetry.io/contrib/instrumentation/runtime v0.69.0` ao `go.mod`/`go.sum` e criar o pacote `internal/platform/observability/runtimemetrics` com `Start(o11y, minInterval)`.
- [ ] 2.2 Ativar `RegisterGlobal: true` e chamar `runtimemetrics.Start` após `otel.NewProvider` em `cmd/server/server.go` e `cmd/worker/worker.go`; auditar leitores do MeterProvider/TracerProvider global no app.

## Detalhes de Implementação

Ver techspec.md — seções "Design de Implementação › Interfaces Chave" (assinatura de `runtimemetrics.Start` e o racional do global), "Modelos de Dados" (tabela das séries `go.*` e o alerta de RF-14 sobre confirmar o nome Prometheus renderizado) e "Arquivos Relevantes e Dependentes". Ver ADR-001 (`adr-001-register-global-runtime-metrics.md`) para o contexto/decisão completos.

Racional (ADR-001, verificado no module cache do devkit-go v0.5.5): o `Provider` expõe apenas `Tracer()/Logger()/Metrics()/HTTP()` — sem getter público de `MeterProvider` (`pkg/observability/otel/config.go:337-345`); `otel.SetMeterProvider` só é chamado quando `config.RegisterGlobal` é `true` (`config.go:244-245`); `NewProvider` não aplica defaults, e a app monta `otel.Config{}` literal sem `RegisterGlobal` (`cmd/server/server.go:63-74`, `cmd/worker/worker.go:68-79`), deixando o global desligado. `runtime.Start` resolve o meter via `otel.GetMeterProvider()` global — portanto ativar o global é o único caminho.

Métricas emitidas (caminho default `go.*` da v0.69.0 — NÃO inventar nomes): `go.goroutine.count`, `go.memory.used`, `go.memory.limit`, `go.memory.allocated`, `go.memory.allocations`, `go.memory.gc.goal`, `go.processor.limit`, `go.config.gogc`. O nome Prometheus exato depende dos sufixos de unidade do exporter do Collector; confirmar a série renderizada no Prometheus antes de fixá-la em alerta/dashboard e registrá-la no `STANDARD.md` (RF-14). `go.schedule.duration` fica fora de escopo (exige `metric.Producer` no Reader, não exposto pelo devkit).

Risco (ADR-001 / techspec "Riscos Conhecidos"): ativar o MeterProvider global muda estado de processo antes desligado e pode afetar código que leia `otel.GetMeterProvider`/`otel.GetTracerProvider` (esperado zero no app). Mitigação: auditar esses usos antes do merge + teste de não-regressão das métricas de instância. Rollback: reverter `RegisterGlobal` ao zero value.

## Critérios de Sucesso

- `go.goroutine.count` e `go.memory.used` visíveis no Prometheus/Grafana para `mecontrola-api` e `mecontrola-worker`.
- `otel.Config` construído em server e worker tem `RegisterGlobal == true`.
- Nenhuma métrica de instância existente (via `o11y.Metrics()`) regride após ativar o global.
- `runtimemetrics.Start` retorna `nil` com um MeterProvider global registrado; nenhum nome de série fora do conjunto `go.*` oficial.
- Zero comentários em Go de produção adicionado; `go build`/`go vet`/`go test -race` e lint passam no escopo alterado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Cobertura esperada:

- Unit de `runtimemetrics.Start`: no setup registrar `sdkmetric.NewMeterProvider` + `otel.SetMeterProvider`; asserir `nil` error e `go.goroutine.count` observado via reader in-memory (`metricdata`); sem mock da lib.
- Sanidade de config: teste garantindo `RegisterGlobal == true` no `otel.Config` construído em server e worker.
- Integração leve (build tag `//go:build integration`): sobe o provider com `RegisterGlobal: true` apontando para reader in-memory/Collector mock, chama `runtimemetrics.Start`, força coleta e assere que a métrica `go.goroutine.count` é exportada (protege contra regressão do wiring global).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `go.mod` / `go.sum` — nova dependência `go.opentelemetry.io/contrib/instrumentation/runtime v0.69.0`.
- `internal/platform/observability/runtimemetrics/` — novo pacote fino (`Start` + testes).
- `cmd/server/server.go` (linhas 63-74) — `RegisterGlobal: true` + `runtimemetrics.Start` após `NewProvider`.
- `cmd/worker/worker.go` (linhas 68-79) — idem para o worker.

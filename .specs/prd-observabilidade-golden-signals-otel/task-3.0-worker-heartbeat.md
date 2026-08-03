# Tarefa 3.0: Heartbeat de liveness do worker + alerta de staleness

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Dar ao worker um sinal de liveness observável e um alerta de ausência (staleness), cobrindo RF-11. O worker passa a emitir um gauge `worker.heartbeat` atualizado por um ticker CANCELÁVEL no bootstrap (`cmd/worker/worker.go`) via `o11y.Metrics().Gauge`, com shutdown cooperativo junto ao lifecycle do worker (goroutine sem leak, cancelável por `context`). Em `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` adiciona-se um alerta `absent()`/staleness que dispara se o heartbeat parar, espelhando o alerta `mc-api-down` da API (que usa `absent(http_server_request_active{job="mecontrola-api"})`). Esta frente é independente das demais, mas depende da Tarefa 2.0 por compartilhar o bootstrap do worker editado lá.

<requirements>
- Emitir um gauge `worker.heartbeat` atualizado por um ticker cancelável no bootstrap do worker (`cmd/worker/worker.go`) via `o11y.Metrics().Gauge`.
- Goroutine do ticker SEMPRE cancelável, com shutdown cooperativo junto ao lifecycle do worker; sem leak (R6 / goroutines canceláveis).
- Adicionar em `rules.yaml` um alerta `absent()`/staleness que dispara se o heartbeat parar, espelhando o schema e o comportamento de `mc-api-down`.
- Não remover nem regredir o alerta `mc-api-down` nem qualquer alerta válido existente (não-regressão, RF-20).
- Nenhum rótulo de alta cardinalidade na métrica (RF-13); zero comentários em Go de produção (R-ADAPTER-001).
- Depende da Tarefa 2.0 (compartilha o bootstrap do worker editado nela).
</requirements>

## Subtarefas

- [ ] 3.1 Emitir o gauge `worker.heartbeat` por ticker cancelável no bootstrap do worker (`cmd/worker/worker.go`) via `o11y.Metrics().Gauge`, com shutdown cooperativo e sem leak.
- [ ] 3.2 Adicionar o alerta `absent()`/staleness do heartbeat em `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`, espelhando `mc-api-down`.

## Detalhes de Implementação

Ver techspec.md — "Arquitetura › Visão Geral dos Componentes" (o item de `cmd/worker/worker.go` descreve o gauge `worker.heartbeat` por ticker cancelável com shutdown cooperativo) e "Sequenciamento de Desenvolvimento › item 5b" (heartbeat + staleness espelhando `mc-api-down`, independente das demais frentes). Ver ADR-001 (`adr-001-register-global-runtime-metrics.md`) para o contexto do bootstrap do worker compartilhado com a Tarefa 2.0.

O alerta segue o schema existente de `rules.yaml` (`groups[].rules[]` com `data[].model.expr`, `datasourceUid: prometheus`, `condition`/`threshold`), usando `absent()` sobre a série renderizada do heartbeat — confirmar o nome Prometheus da série antes de fixá-la, análogo ao `mc-api-down` que usa `absent(http_server_request_active{job="mecontrola-api"})`. Nenhum rótulo de alta cardinalidade na métrica (RF-13).

## Critérios de Sucesso

- Gauge `worker.heartbeat` visível no Prometheus/Grafana para o worker enquanto o processo está vivo.
- A goroutine do ticker encerra deterministicamente no shutdown do worker (cancelada por context); sem leak observado.
- Alerta de staleness avalia sem erro (`execErrState`) e dispara quando o heartbeat cessa; `mc-api-down` e demais alertas válidos permanecem intactos.
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

- Unit do emissor de heartbeat: o ticker atualiza o gauge periodicamente e é cancelado no shutdown (context cancelado encerra a goroutine sem leak).
- Validação do alerta de staleness em `rules.yaml`: sintaxe/schema válidos e a expressão `absent()` referencia a série viva do heartbeat, espelhando `mc-api-down`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `cmd/worker/worker.go` — gauge `worker.heartbeat` por ticker cancelável, shutdown cooperativo (compartilha o bootstrap editado na Tarefa 2.0).
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` — novo alerta `absent()`/staleness espelhando `mc-api-down`.

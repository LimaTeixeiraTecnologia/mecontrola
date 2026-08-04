# Tarefa 6.0: Alerta de causa de runtime + painéis de runtime e SLO nos dashboards

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar em `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` um alerta de CAUSA de saturação de runtime do processo Go (ex.: crescimento sustentado de `go.goroutine.count` ou de `go.memory.used` acima do alvo) com `severity: warning`, que serve para diagnóstico/ticket e NÃO pagina humano isoladamente. Estender os dashboards em `deployment/dashboards/` (`mecontrola-api.json` e/ou um novo painel) com a saturação de runtime do processo e a visão de SLO/burn-rate; a geração/edição do JSON de dashboard é delegada à skill `otel-grafana-dashboards`. Usar exclusivamente os nomes de métrica `go.*` reais confirmados no ambiente. Cobre RF-03 e RF-19. Depende das Tarefas 2.0 (métricas `go.*` já emitidas) e 4.0 (SLO para o painel).

<requirements>
- Adicionar em `rules.yaml` um alerta de causa de saturação de runtime (ex.: crescimento sustentado de `go.goroutine.count` ou `go.memory.used` acima do alvo) com `severity: warning`.
- O alerta de causa NÃO pode paginar humano isoladamente (sem `severity: critical` nem roteamento de página); serve para diagnóstico/ticket.
- Estender os dashboards em `deployment/dashboards/` (`mecontrola-api.json` e/ou novo painel) com saturação de runtime do processo e visão de SLO/burn-rate.
- Usar apenas os nomes de métrica `go.*` reais confirmados no ambiente (RF-14); nenhum nome inventado.
- A geração/edição do JSON de dashboard é delegada à skill `otel-grafana-dashboards`.
- Não regredir dashboards nem alertas válidos existentes.
</requirements>

## Subtarefas

- [x] 6.1 Confirmar no ambiente os nomes de série `go.*` renderizados no Prometheus (ex.: `go.goroutine.count`, `go.memory.used`) antes de fixar queries de alerta/painel.
- [x] 6.2 Adicionar em `rules.yaml` o alerta de causa de saturação de runtime com `severity: warning`, sem roteamento de página, no schema do Grafana unified alerting já usado no arquivo.
- [x] 6.3 Estender os dashboards (`mecontrola-api.json` e/ou novo painel) com a saturação de runtime do processo e a visão de SLO/burn-rate, delegando a geração/edição do JSON à skill `otel-grafana-dashboards`.
- [x] 6.4 Confirmar que o painel de runtime popula com dados e que o alerta de causa avalia sem erro e não tem severidade de página.

## Detalhes de Implementação

Ver techspec.md, seção "Sequenciamento de Desenvolvimento → Ordem de Build" (item 6, runtime nos dashboards depende do item 2 = Tarefa 2.0; SLO no painel vem da Tarefa 4.0) e "Monitoramento e Observabilidade" (alerta de causa de runtime em `rules.yaml`; painel de runtime + SLO delegado). A tabela de séries `go.*` (nomes OTel e aproximação Prometheus) está em techspec.md, seção "Design de Implementação → Modelos de Dados"; o nome Prometheus exato depende do exporter e DEVE ser confirmado antes de fixar (RF-14). A não-regressão de dashboards/alertas válidos segue RF-20.

## Critérios de Sucesso

- Existe em `rules.yaml` um alerta de causa de saturação de runtime com `severity: warning` que não pagina isoladamente (sem `severity: critical` nem rota de página).
- O alerta de causa avalia sem erro (`execErrState` limpo).
- Os dashboards em `deployment/dashboards/` cobrem a saturação de runtime do processo e a visão de SLO/burn-rate, e o painel de runtime popula com dados reais.
- Todas as queries usam nomes `go.*` reais confirmados no ambiente; nenhum nome inventado.
- Nenhum dashboard ou alerta válido existente removido ou renomeado indevidamente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `golden-signals-otel-standards` — mapeamento de saturação e queries PromQL.
- `otel-grafana-dashboards` — geração/edição de painéis Grafana para métricas OTel.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Validar que o alerta de causa de runtime avalia sem erro e não tem `severity: critical` nem roteamento de página, e que o painel de runtime popula com dados.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` — alerta de causa de saturação de runtime (`severity: warning`).
- `deployment/dashboards/mecontrola-api.json` — painéis de saturação de runtime e SLO/burn-rate (edição delegada à skill de dashboards).

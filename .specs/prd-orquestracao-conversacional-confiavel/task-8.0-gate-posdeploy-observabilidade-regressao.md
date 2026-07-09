# Tarefa 8.0: Gate pós-deploy + observabilidade + contrato de regressão

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o caminho production-ready: gate pós-deploy monitorado (agregados produtivos com amostra mínima e
margem), observabilidade/runbook/alertas e o contrato de regressão que garante que nenhuma funcionalidade
existente regride.

<requirements>
- RF-42: `tool-call-accuracy` redefinida na agregação (denominador exclui `outcome ∈ {clarify, replay}`).
- RF-43: gate pós-deploy monitora agregados com critério explícito de rollback (falhas, scorers,
  truncamento, escrita duplicada) e amostra mínima.
- RF-49: linha base de referência (19 succeeded / 4 failed em 23 runs; 0,304 / 0,149 / 0,565).
- RF-50: demonstrar melhora contra a baseline sem aumento de truncamento/escrita duplicada/resposta
  vazia/falha de persistência.
- RF-51: amostra mínima (N ≥ 100 runs ou ≥ 14 dias) + margem absoluta por métrica (scorers ≥ baseline +
  0,05; taxa de falha ≤ baseline) antes de promover/reverter.
- RF-52: decisão de promover/reverter por evidência rastreável por `run_id`.
- RF-53: nenhum novo alerta crítico de privacidade, truncamento ou escrita duplicada após o deploy.
- RF-54: nenhuma tool removida/renomeada/oculta/alterada sem história própria.
- RF-55: contrato público (`BuildMeControlaAgent`, `AgentRuntime`, `RunStore`, `ThreadGateway`,
  `MessageStore`, `WorkingMemory`, schemas strict, workflows) preservado.
- RF-56/RF-57: fluxos existentes cobertos por teste/golden/evidência equivalente (contrato de regressão).
</requirements>

## Subtarefas

- [ ] 8.1 Queries de agregação sobre `platform_runs`+`platform_scorer_results`+`workflow_runs` para o
  gate pós-deploy, com `tool-call-accuracy` redefinida (exclui `clarify`/`replay`).
- [ ] 8.2 Definir amostra mínima (N ≥ 100 / ≥ 14 dias) e margens absolutas por métrica; documentar no
  runbook.
- [ ] 8.3 Dashboards/alertas: `agent_run_truncated_total`, `agent_run_update_errors_total`,
  `agent_message_append_errors_total`, scorers comportamentais, escrita duplicada; comparação por versão
  do agente.
- [ ] 8.4 Contrato de regressão: inventário das tools/workflows/contratos públicos + verificação de que
  nada foi removido/renomeado/alterado; cobertura por teste/golden/evidência.
- [ ] 8.5 Runbook de promoção/rollback com decisão rastreável por `run_id`.

## Detalhes de Implementação

Ver `adr-005-golden-harness-gate.md` e `techspec.md` → "Monitoramento e Observabilidade" e "tool-call-
accuracy redefinida". Métricas com cardinalidade fechada (RF-33). Gate pós-deploy é
consulta/observabilidade + runbook, não código de runtime. Reusa scorers (5.0), golden (6.0) e runtime
observável (2.0).

## Critérios de Sucesso

- Gate pós-deploy computável por consulta, com amostra mínima e margem absoluta; decisão rastreável por
  `run_id`.
- Dashboards/alertas para os contadores novos e scorers; comparação por versão.
- Contrato de regressão verificado: nenhuma tool/workflow/contrato público removido ou alterado.
- Runbook de promoção/rollback publicado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `postgresql-production-standards` — queries de agregação sobre Postgres de produção para o gate pós-deploy.
- `otel-grafana-dashboards` — dashboards/alertas Grafana para métricas OTel dos runs, scorers e falhas sanitizadas.

## Testes da Tarefa

- [ ] Testes unitários: lógica de cálculo do gate (amostra mínima, margem, `tool-call-accuracy`
  redefinida) sobre dados sintéticos.
- [ ] Testes de integração: consultas de agregação contra Postgres (testcontainers) com dados semeados.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- Runbook/dashboards de produção (`docs/runbooks/`, `docs/dashboards/`, `docs/alerts/`)
- Consultas de agregação (gate pós-deploy) e testes correspondentes
- Inventário de contrato de regressão (tools/workflows/contratos públicos)

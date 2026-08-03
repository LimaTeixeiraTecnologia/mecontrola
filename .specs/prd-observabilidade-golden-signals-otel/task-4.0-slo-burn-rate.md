# Tarefa 4.0: SLO + alertas de burn-rate multi-janela + alerta de latência SLO

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Formalizar os SLOs do serviço — disponibilidade 99,9% (error budget 0,1%) e latência P95 < 500ms, ambos em janela de 30 dias, documentados no `observability/STANDARD.md` — e adicionar em `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` um grupo `slo` com quatro alertas de burn-rate multi-janela (esquema do Google SRE Workbook derivado do budget de 0,1%) mais um alerta de latência ligado ao SLO P95 < 500ms. Os alertas de burn-rate e de latência SLO complementam, não substituem, os thresholds estáticos existentes (`mc-api-5xx`, `mc-api-latency-p99`), que são PRESERVADOS (defesa em profundidade). O SLI de disponibilidade usa a série canônica do histograma HTTP (ADR-004). Cobre RF-04, RF-05, RF-06; segue ADR-002. Depende das Tarefas 1.0 e 5.0.

<requirements>
- Formalizar e documentar no `observability/STANDARD.md` o SLO de disponibilidade 99,9% (error budget 0,1%) e o SLO de latência P95 < 500ms, ambos em janela de 30 dias.
- Adicionar um grupo `slo` em `rules.yaml` com quatro alertas de burn-rate de disponibilidade derivados do budget de 0,1%: página em 14,4× (2% do orçamento, janelas 1h e 5m); página em 6× (5%, 6h e 30m); ticket em 3× (10%, 1d e 2h); ticket em 1× (10%, 3d e 6h).
- Adicionar um alerta de latência ligado ao SLO P95 < 500ms, coexistindo com o `mc-api-latency-p99` existente.
- PRESERVAR os thresholds estáticos existentes (`mc-api-5xx`, `mc-api-latency-p99`); não substituí-los.
- O SLI de disponibilidade DEVE usar a série canônica do histograma `http_server_request_duration_seconds_count` (erro filtrado por `http.response.status_code=~"5.."`), conforme ADR-004; reusar `job="mecontrola-api"` como nos alertas existentes.
- Cada alerta de página DEVE referenciar um runbook na anotação `description`, seguindo o padrão já usado no `rules.yaml`.
- O schema DEVE seguir o Grafana unified alerting já usado em `rules.yaml`: `groups[].rules[]` com `data[].model.expr`, `datasourceUid: prometheus`, `condition`/`threshold`.
</requirements>

## Subtarefas

- [ ] 4.1 Documentar no `observability/STANDARD.md` os SLOs de disponibilidade 99,9% (budget 0,1%) e latência P95 < 500ms em 30 dias, com a tabela de burn-rate (multiplicadores e janelas).
- [ ] 4.2 Confirmar a série e os rótulos de erro/total do SLI de disponibilidade (série canônica do histograma, ADR-004) antes de fixar as queries.
- [ ] 4.3 Escrever o grupo `slo` em `rules.yaml` com as quatro regras de burn-rate (14,4× 1h+5m página; 6× 6h+30m página; 3× 1d+2h ticket; 1× 3d+6h ticket), no schema do Grafana unified alerting já usado no arquivo.
- [ ] 4.4 Adicionar o alerta de latência SLO P95 < 500ms coexistindo com o `mc-api-latency-p99`.
- [ ] 4.5 Referenciar um runbook na `description` de cada alerta de página (RF-10).
- [ ] 4.6 Confirmar que os thresholds estáticos existentes permanecem intactos e que os novos alertas avaliam sem `execErrState`.

## Detalhes de Implementação

Ver techspec.md, seção "Sequenciamento de Desenvolvimento → Ordem de Build" (item 3, SLO + burn-rate depende dos nomes de série de latência/erro confirmados; itens 1 e 4 = Tarefas 1.0 e 5.0) e "Considerações Técnicas → Decisões Chave" (ADR-002). O contrato completo está em `adr-002-burn-rate-slo-alerts.md` (esquema de 4 janelas do Google SRE Workbook derivado do budget de 0,1%, coexistência com thresholds como defesa em profundidade, SLI sobre a série canônica do histograma via ADR-004, reuso de `job="mecontrola-api"`, runbook por alerta de página). A fonte canônica de RED está fixada em `adr-004-red-canonical-source.md`.

## Critérios de Sucesso

- SLOs de disponibilidade 99,9% (budget 0,1%) e latência P95 < 500ms em 30 dias documentados no `observability/STANDARD.md`.
- O grupo `slo` em `rules.yaml` contém exatamente as quatro regras de burn-rate com os multiplicadores e janelas especificados (14,4×/6×/3×/1×) e o alerta de latência SLO P95 < 500ms.
- Os thresholds estáticos existentes (`mc-api-5xx`, `mc-api-latency-p99`) permanecem presentes e inalterados.
- Todos os novos alertas avaliam sem `execErrState` e disparam de forma coerente sob injeção de erro.
- Cada alerta de página referencia um runbook na `description`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `golden-signals-otel-standards` — SLO, error budget e burn-rate multi-janela orientados a sintoma.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Validar que todos os alertas do grupo `slo` e o alerta de latência SLO avaliam sem `execErrState` e disparam de forma coerente em um teste de injeção de erro.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` — grupo `slo` (burn-rate) e alerta de latência SLO adicionados; thresholds existentes preservados.
- `observability/STANDARD.md` — SLO, error budget e tabela de burn-rate documentados.

# Tarefa 9.0: Runbook, dashboards e readiness operacional

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar operação production-ready para áudio: runbook de triagem, dashboards/alertas, thresholds, canary e checklist de readiness pós-deploy. A tarefa fecha operação, não implementa regra financeira.

<requirements>
- Cobrir RF-28, RF-29 e RF-46.
- Runbook deve cobrir falha de download, falha STT, incerteza técnica alta, latência alta, custo alto e regressão golden.
- Alertas devem usar thresholds da techspec.
- Dashboards devem usar datasources reais Prometheus, Loki e Tempo quando aplicável.
- Não incluir PII, WAMID, telefone, media id ou transcrição em queries/labels.
</requirements>

## Subtarefas

- [ ] 9.1 Criar ou atualizar runbook em `deployment/runbooks` para áudio WhatsApp/STT.
- [ ] 9.2 Criar dashboards ou painéis Grafana para volume, erro, incerteza, p95, tamanho, duração e custo.
- [ ] 9.3 Criar alertas com `stt_error_rate_15m > 5%`, `transcription_uncertain_rate_15m > 20%`, `transcription_p95_15m > 8s`, `audio_cost_microusd_1h > 120000` e `audio_false_success > 0`.
- [ ] 9.4 Documentar canary com `AudioEnabled=false` por default e critérios para habilitação.
- [ ] 9.5 Documentar rollback e evidências mínimas pós-deploy.
- [ ] 9.6 Validar cardinalidade de queries/labels.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Monitoramento e Observabilidade`, `Alertas iniciais` e `Riscos e Mitigacoes`.

Evidências de produção já levantadas:
- Prometheus datasource `prometheus`
- Loki datasource `loki`
- Tempo datasource `tempo`
- `.specs/prd-agente-audio-openrouter/prod-evidence-2026-08-04.md`

## Critérios de Sucesso

- Runbook permite triagem sem acesso a áudio bruto.
- Dashboards não usam labels de alta cardinalidade.
- Threshold de custo horário é numérico e rastreável.
- Readiness pós-deploy distingue authoring-ready, merge-ready e production-ready.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — O runbook deve refletir o fluxo real do consumidor agentivo e seus outcomes.
- `domain-modeling-production` — A operação precisa preservar linguagem de outcomes/reasons fechados.
- `design-patterns-mandatory` — A tarefa deve evitar arquitetura operacional excessiva sem evidência.
- `otel-grafana-dashboards` — A tarefa gera ou atualiza dashboards Grafana.
- `golden-signals-otel-standards` — A tarefa define painéis e alertas por latência, tráfego, erro, saturação/custo.

## Testes da Tarefa

- [ ] Validação sintática de YAML/JSON de dashboards/alertas quando aplicável.
- [ ] Grep de labels proibidos em dashboards/alerts.
- [ ] Revisão do runbook contra RF-46.
- [ ] Se houver Go alterado, rodar gates proporcionais do pacote afetado.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/runbooks/`
- `deployment/telemetry/grafana/`
- `.specs/prd-agente-audio-openrouter/prod-evidence-2026-08-04.md`

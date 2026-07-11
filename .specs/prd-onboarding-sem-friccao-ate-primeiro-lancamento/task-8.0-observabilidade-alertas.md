# Tarefa 8.0: Atualizar observabilidade, alertas e runbook

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Garantir que onboarding, pendência de lançamento, mensagens, runs, eventos de outbox, métricas e traces continuem auditáveis, com cardinalidade controlada e sem expor dados sensíveis. Adicionar alerta crítico de confirmação positiva sem transação ativa em até 30s.

<requirements>
- RF-31: onboarding, pendência de lançamento, mensagens e respostas continuam rastreáveis em `workflow_runs`, `workflow_steps`, `platform_runs`, `platform_messages` e `outbox_events`.
- RF-32: métricas de onboarding, workflow, inbound WhatsApp e escrita financeira continuam disponíveis com cardinalidade controlada.
- RF-33: traces de inbound WhatsApp permitem correlacionar retomada de workflow, execução de agente, chamada LLM, criação de orçamento, criação de 💳 e escrita financeira.
- RF-34: logs, métricas e traces não expõem dados sensíveis do cliente como rótulos de alta cardinalidade.
</requirements>

## Subtarefas

- [ ] 8.1 Revisar instrumentação existente nos workflows, guards, pending-entry e tools.
- [ ] 8.2 Garantir labels permitidos (`workflow`, `step`, `status`, `outcome`, `agent_id`, `guard`, `decision`) sem `user_id`, telefone, `wamid`, categoria ou IDs de entidade.
- [ ] 8.3 Adicionar contador de decisões do guard `card_provenance` por `agent_id`, `guard` e `decision`.
- [ ] 8.4 Adicionar alerta crítico: confirmação positiva sem transação ativa em até 30s.
- [ ] 8.5 Atualizar runbook operacional para troubleshooting de falso sucesso financeiro.

## Detalhes de Implementação

Ver `techspec.md` — seções **Monitoramento e Observabilidade** e **Arquivos Relevantes**. O alerta deve ser adicionado em `docs/alerts/whatsapp-dead-letter.yaml` ou `docs/alerts/mecontrola-agent-gate-posdeploy.yaml`, conforme padrão existente.

## Critérios de Sucesso

- Métricas mantêm cardinalidade controlada; nenhum label de PII ou IDs de entidade.
- Alerta crítico de confirmação sem transação em 30s está configurado.
- Runbook documenta como investigar falso sucesso financeiro.
- Dashboards/alertas validados com `otel-grafana-dashboards` quando aplicável.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — observabilidade de workflows, agentes, tools e guards.
- `otel-grafana-dashboards` — geração/atualização de dashboards Grafana para os novos sinais.

## Testes da Tarefa

- [ ] Inspeção de labels de métricas.
- [ ] Validação de alertas.
- [ ] Revisão de runbook.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `docs/alerts/whatsapp-dead-letter.yaml`
- `docs/alerts/mecontrola-agent-gate-posdeploy.yaml`
- `scripts/loadtest/whatsapp-inbound.js`
- `taskfiles/loadtest.yml`

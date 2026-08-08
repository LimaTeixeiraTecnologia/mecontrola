# Tarefa 6.0: Alerta Grafana mc-audio-duration-exceeded-rate

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar o alerta `mc-audio-duration-exceeded-rate` ao grupo `audio` de regras Grafana existente, consumindo a métrica já emitida `agents_audio_inbound_total` com labels `outcome` e `reason`, sem nenhuma instrumentação nova (ADR-004).

<requirements>
- Critério de sucesso do PRD: guardrail com monitoramento ativo da taxa de `duration_exceeded`.
- ADR-004: janela de 1h, threshold inicial 0.30, severidade informativa, threshold declaradamente provisório por ausência de baseline.
</requirements>

## Subtarefas

- [ ] 6.1 Adicionar a regra `mc-audio-duration-exceeded-rate` em `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`, no grupo `audio` (`:1279`), seguindo o formato das regras irmãs (`mc-audio-stt-error-rate`, `:1283`; `mc-audio-transcription-uncertain-rate`, `:1315`).
- [ ] 6.2 Expressão: razão entre `rate(agents_audio_inbound_total{outcome="rejected",reason="duration_exceeded"}[1h])` e `rate(agents_audio_inbound_total[1h])`, maior que 0.30, com severidade informativa e anotações explicando o significado e a provisionalidade do threshold.
- [ ] 6.3 Validar os labels da query contra o painel genérico existente (`deployment/dashboards/agent-audio-whatsapp.json:98`), que já agrega por `outcome` e `reason`.
- [ ] 6.4 Documentar o alerta e a interpretação da taxa no runbook `deployment/runbooks/audio-whatsapp-stt.md` (seção de triagem), em coordenação com a tarefa 5.0 para não gerar conflito de edição no mesmo arquivo; se 5.0 ainda não merged, consolidar a nota no mesmo ponto do runbook.
- [ ] 6.5 Nenhuma mudança em código Go, dashboards ou métricas.

## Detalhes de Implementação

Ver `techspec.md` seção `Monitoramento e Observabilidade` e ADR-004 (expressão, threshold, severidade, mitigação de falso positivo na primeira semana e revisão em 30 dias).

## Critérios de Sucesso

- YAML válido e carregável pelo provisionamento do Grafana (validação sintática local do arquivo).
- Query PromQL usa exatamente os labels `outcome` e `reason` já presentes na métrica, sem labels novos e sem aumento de cardinalidade.
- Regra presente no grupo `audio` com nome, janela, threshold e severidade conforme ADR-004.

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

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`
- `deployment/dashboards/agent-audio-whatsapp.json` (somente leitura, referência de labels)
- `deployment/runbooks/audio-whatsapp-stt.md`
- `.specs/prd-limite-audio-20-segundos-whatsapp/adr-004-alerta-dedicado-duration-exceeded.md`

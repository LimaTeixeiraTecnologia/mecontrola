# Tarefa 4.0: Sampling de traces proporcional e ajuste do gate anti-storm

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Produção roda com **100% de trace sampling** (`compose.swarm.yml` força `"1"`, sobrepondo `prod.env=0.1`, e o gate anti-storm exige isso). Sob carga de 10k isso explode CPU/storage de observabilidade. Reduzir para amostragem proporcional preservando 100% dos traces com erro.

<requirements>
- RF-11: reduzir o sampling em produção (error-biased), alinhando compose e prod.env num valor efetivo único.
- RF-12: ajustar o gate deploy-anti-storm para permitir o sampling reduzido controlado.
- RF-13: documentar o SPOF de observabilidade single-node aceito e a retenção dos sinais.
</requirements>

## Subtarefas

- [ ] 4.1 Definir e alinhar `OTEL_TRACE_SAMPLE_RATE` efetivo (ex.: 0.1) entre `compose.swarm.yml` e `prod.env`, removendo a divergência.
- [ ] 4.2 Configurar error-biased sampling (tail-sampling no OTel Collector `otelcol-config.yaml` ou parent-based no SDK) para reter 100% de traces com erro.
- [ ] 4.3 Ajustar `scripts/ci/deploy-anti-storm.sh` para validar o novo valor acordado, mantendo as demais invariantes.
- [ ] 4.4 Documentar SPOF de observabilidade aceito e retenção (30 d) em runbook.

## Detalhes de Implementação

Ver `techspec.md` REQ-04. Não reabrir o risco que o gate anti-storm original mitigava (versão OTEL consistente entre réplicas).

## Critérios de Sucesso

- Sampling de produção reduzido e coerente entre compose e env; um único valor efetivo.
- Traces com erro continuam 100% amostrados (validado em teste).
- Gate anti-storm ajustado e verde; documentação de SPOF/retention publicada.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `otel-grafana-dashboards` — ajustar sampling/coleta OTel e validar traces/erros na stack Grafana.

## Testes da Tarefa

- [ ] Testes unitários (gate anti-storm: valida novo valor, rejeita divergência)
- [ ] Testes de integração (traces com erro retidos 100%; volume de traces normais reduzido)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/compose/compose.swarm.yml` (OTEL_TRACE_SAMPLE_RATE)
- `deployment/config/prod.env`
- `deployment/telemetry/grafana/otelcol-config.yaml`
- `scripts/ci/deploy-anti-storm.sh`, `taskfiles/ci.yml`
- `deployment/runbooks/` (documentação de SPOF/retention)

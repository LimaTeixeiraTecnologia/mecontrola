# Tarefa 7.0: Deploy da main em produção e alerta de drift de versão

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Produção roda a imagem `571425f`, atrás da `main` (correções mergeadas, ex.: confirmação honesta, não estão em prod). Deployar a `main` corrente e instituir alerta de drift para que produção não fique defasada silenciosamente.

<requirements>
- RF-19: deployar a main atual em produção (sair de 571425f) com verificação de saúde.
- RF-20: gate/alerta de drift entre OTEL_SERVICE_VERSION em execução e o HEAD de main.
</requirements>

## Subtarefas

- [ ] 7.1 Deployar a `main` corrente em produção pelo caminho atual e confirmar saúde pós-deploy (`/healthz`, `docker service ls`).
- [ ] 7.2 Implementar alerta/gate de drift que compara `OTEL_SERVICE_VERSION` em execução com o `HEAD` de `main`, disparando quando divergir além de um limiar de tempo.
- [ ] 7.3 Documentar o procedimento e o limiar no runbook de deploy.

## Detalhes de Implementação

Ver `techspec.md` REQ-07. Esta tarefa usa o **caminho de deploy atual** (F0/urgente) e deve completar **antes** da Tarefa 3.0 (que reescreve/remove o pipeline). Preservar rollback automático.

## Critérios de Sucesso

- `docker service ls`/`OTEL_SERVICE_VERSION` refletem a `main`; app saudável.
- Alerta de drift ativo e testado (dispara ao simular defasagem de versão).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `otel-grafana-dashboards` — criar o alerta de drift de versão sobre a métrica de versão exposta.

## Testes da Tarefa

- [ ] Testes unitários (lógica de comparação de versão, se implementada em código)
- [ ] Testes de integração (deploy da main saudável; alerta de drift dispara em cenário defasado)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/scripts/deploy-swarm.sh`, `deployment/runbooks/deploy.md`
- `deployment/compose/compose.swarm.yml` (OTEL_SERVICE_VERSION)
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`
- `.github/workflows/ci-cd.yml` (job de verificação de drift, se agendado no CI)

# Tarefa 1.0: Backup pgBackRest agendado, alertado e com guard de imagem

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

pgBackRest opera em produção (imagem custom, AES-256, WAL→S3), mas **sem agendamento** (nenhum crontab/systemd-timer; último full 2026-07-01) e o deploy default aponta para uma imagem sem pgBackRest. Automatizar o agendamento de forma versionada, alertar staleness e blindar a imagem.

<requirements>
- RF-01: agendar pgBackRest (full semanal, diff diário, incr 6h) versionado e idempotente.
- RF-02: alertar backup stale/ausente e falha de archive-push.
- RF-03: falhar deploy de produção se POSTGRES_IMAGE não for a imagem custom com pgBackRest.
</requirements>

## Subtarefas

- [ ] 1.1 Criar script idempotente (`deployment/scripts/pgbackrest-schedule.sh`) que instala systemd-timer/cron a partir de `deployment/pgbackrest/crontab.txt`, disparando `pgbackrest backup` no container postgres.
- [ ] 1.2 Expor "idade do último backup" como métrica scrapeável (textfile do node-exporter ou coletor simples) e criar regra de alerta de staleness + falha de archive-push nas provisioning rules do Grafana/Prometheus.
- [ ] 1.3 Adicionar guard em `deploy-swarm.sh`/`render-stack.py` que aborta se `POSTGRES_IMAGE` resolver para `postgres:*-alpine` em produção.
- [ ] 1.4 Documentar o procedimento de agendamento e verificação em runbook.

## Detalhes de Implementação

Ver `techspec.md` REQ-01. Reusar `deployment/pgbackrest/{pgbackrest.conf,crontab.txt}` e `deployment/scripts/pgbackrest-setup.sh`. Não duplicar credenciais S3 no repo.

## Critérios de Sucesso

- `pgbackrest --stanza=mecontrola info` mostra full ≤ 7 d e diff ≤ 24 h após o agendamento ativo.
- Alerta de staleness dispara em teste forçado (backup atrasado simulado) e limpa ao normalizar.
- Deploy de produção falha explicitamente com imagem postgres default; passa com a custom.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `taskfile-production` — automatizar o agendamento/guard como target versionado e integrável à pipeline.
- `otel-grafana-dashboards` — criar a regra de alerta de staleness de backup na stack de observabilidade.

## Testes da Tarefa

- [ ] Testes unitários (script de guard: caminho de imagem default falha, custom passa)
- [ ] Testes de integração (agendamento aplica e `pgbackrest info` reflete backup fresco; alerta dispara/limpa)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/scripts/pgbackrest-schedule.sh` (novo), `deployment/scripts/pgbackrest-setup.sh`
- `deployment/pgbackrest/crontab.txt`, `deployment/pgbackrest/pgbackrest.conf`
- `deployment/scripts/deploy-swarm.sh`, `deployment/scripts/render-stack.py`
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`, `deployment/monitoring/prometheus-rules.yaml`
- `deployment/runbooks/restore-pitr.md` (referência)

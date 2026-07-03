# Tarefa 5.0: Orçamento de recursos, pool de conexões e alerta de saturação

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Os `limits` do compose somam 6,77 GB RAM (~87% de 8 GB antes do overhead) e 5,55 vCPU (2,8× o host), amparados só por swap. O pool tem folga estreita (40 conexões cliente vs `MAX_DB_CONNECTIONS=60`). Sanear o orçamento e alertar saturação, sem estrangular o p95.

<requirements>
- RF-14: worst-case de memória do compose cabe com margem em 8 GB (host − overhead).
- RF-15: dimensionar e alertar saturação do pool (pgBouncer + DB_MAX_CONNS) com folga para jobs/retry.
- RF-16: documentar o orçamento aprovado e o gatilho objetivo de upgrade KVM2→KVM4.
</requirements>

## Subtarefas

- [ ] 5.1 Rever `limits`/`reservations` em `compose.swarm.yml` (reduzir tetos superdimensionados de server/worker/otel-lgtm) para que a soma de `limits` de memória caiba com margem; manter `reservations` como garantia de scheduling.
- [ ] 5.2 Revisar `DB_MAX_CONNS`/pgBouncer (`DEFAULT_POOL_SIZE`, `MAX_DB_CONNECTIONS`) para folga sobre 4 processos + jobs de background.
- [ ] 5.3 Adicionar alerta de saturação de pool (postgres-exporter/pgBouncer stats) em Grafana.
- [ ] 5.4 Documentar o orçamento aprovado e o gatilho de upgrade KVM2→KVM4 (métrica + limiar).

## Detalhes de Implementação

Ver `techspec.md` REQ-05. Calibrar os cortes de `limits` com base no p95 medido na Tarefa 6.0 (não cortar às cegas). Mudança em `configs/config.go`, se houver, segue `go-implementation` + `R-DTO-VALIDATE-001` quando aplicável.

## Critérios de Sucesso

- Soma de `limits` de memória do compose cabe com margem sobre 8 GB menos overhead.
- Alerta de saturação de pool ativo e testado (dispara sob pressão sintética).
- Documento de orçamento + gatilho de upgrade publicado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `otel-grafana-dashboards` — criar o alerta/painel de saturação de pool na stack Grafana.

## Testes da Tarefa

- [ ] Testes unitários (se `configs/config.go` mudar defaults de pool: testify/suite)
- [ ] Testes de integração (alerta de pool dispara sob pressão; stack sobe dentro do novo orçamento)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/compose/compose.swarm.yml` (limits/reservations)
- `deployment/config/prod.env` (DB_MAX_CONNS), `deployment/pgbouncer/pgbouncer.ini`
- `configs/config.go` (se defaults mudarem)
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`
- `docs/runs/<data>-orcamento-recursos.md` (novo)

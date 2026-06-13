# Tarefa 8.0: Smoke staging + métrica + dashboard

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fecha o PRD com smoke test em staging (validar que eventos reais populam `aggregate_user_id`), dashboard Grafana com painéis dedicados, e validação operacional da métrica `outbox_events_inserted_total{has_user_id}`.

<requirements>
- M-02: métrica em staging mostra `has_user_id="true"` ≥ 99%
- M-08: dashboard Grafana inclui painel "Outbox Adoption"
- RF-18, RF-19, RF-20: validações finais
- Alerta operacional configurado: `rate(outbox_events_inserted_total{has_user_id="false"}[5m]) / rate(outbox_events_inserted_total[5m]) > 0.01` por 10 min
- Runbook documentando interpretação da métrica e procedimento se alerta disparar
</requirements>

## Subtarefas

- [ ] 8.1 Smoke staging:
  - Aplicar migration `000017` em staging.
  - Deploy do código com 1.0–7.0 mergeado.
  - Disparar 1 evento de cada módulo (transação criada, expense committed, subscription bound, principal established).
  - Validar `SELECT count(*) FROM outbox_events WHERE aggregate_user_id IS NOT NULL` aumenta para cada evento.
- [ ] 8.2 Atualizar `docs/dashboards/outbox.json` (ou criar) com painéis:
  - "Outbox Adoption %": `sum(rate(outbox_events_inserted_total{has_user_id="true"}[5m])) / sum(rate(outbox_events_inserted_total[5m]))`.
  - "Outbox Missing User ID Rate": `rate(outbox_events_inserted_total{has_user_id="false"}[5m])`.
- [ ] 8.3 Configurar alerta em `docs/alerts/outbox.yaml` (ou equivalente) com regra acima.
- [ ] 8.4 Criar `docs/runbooks/outbox-aggregate-user-id.md` cobrindo:
  - Significado da métrica.
  - Procedimento se alerta disparar (consultar logs warn estruturados `outbox.event.missing_aggregate_user_id`, identificar event_type, abrir PR para popular).
  - Política de adição à allowlist (ADR-004).
- [ ] 8.5 Validar adversarialmente o alerta: forçar 1 evento sem user_id em staging (via teste manual), confirmar que métrica reflete e alerta dispara.

## Detalhes de Implementação

Ver techspec seção "Monitoramento e Observabilidade" + ADR-001 (critério v2: 30 dias de coverage ≥ 99.99% antes de promover validação obrigatória).

## Critérios de Sucesso

- Métrica `has_user_id="true"` ≥ 99% em staging por 24h.
- Dashboard JSON válido (`jq .`).
- Alerta configurado e validado adversarialmente.
- Runbook revisado, sem TODOs.
- `task lint && task test && task vulncheck` PASS.

## Skills Necessárias

<!-- MANDATÓRIO -->

- `otel-grafana-dashboards` — painel "Outbox Adoption" com métricas Prometheus + alerta operacional (M-02, M-08)

## Testes da Tarefa

- [ ] Smoke staging com 4 módulos disparando eventos
- [ ] Métrica verificada via `/metrics` interno
- [ ] Dashboard JSON validado
- [ ] Alerta validado adversarialmente

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `docs/dashboards/outbox.json` (novo ou modificado)
- `docs/alerts/outbox.yaml` (novo ou modificado)
- `docs/runbooks/outbox-aggregate-user-id.md` (novo)

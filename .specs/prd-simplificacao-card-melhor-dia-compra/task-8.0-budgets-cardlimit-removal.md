# Tarefa 8.0: budgets — remoção cirúrgica do alerta de limite de cartão (ADR-004)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Remover **cirurgicamente** a trilha `ThresholdAlertCardLimit` de `internal/budgets`, eliminando a
dependência de `cards.limit_cents`, **sem** tocar nos alertas de categoria (`ThresholdAlertCategory`) e
metas (`ThresholdAlertGoal`). Auditar dashboards/alertas para remover o label `kind="card_limit_near"`.

<requirements>
- RF-15: remover a feature de alerta por limite de cartão; preservar categoria e metas intactos.
- Reduzir o tipo fechado `ThresholdAlertKind` a `Category`/`Goal` (state-as-type mantido).
- Remover leitura de `cards.limit_cents` (`card_threshold_reader`) — fecha RF-05 no lado budgets.
- Métrica `budgets_threshold_alerts_dispatched_total` deixa de emitir `kind="card_limit_near"`.
</requirements>

## Subtarefas

- [ ] 8.1 Remover arquivos: `application/interfaces/card_threshold_reader.go`, `infrastructure/repositories/postgres/card_threshold_reader.go`, `application/interfaces/mocks/card_threshold_reader.go`.
- [ ] 8.2 Editar `domain/services/threshold_workflow.go`: remover const `ThresholdAlertCardLimit` e seus `case` em `String()` e `thresholdForKind()`.
- [ ] 8.3 Editar `application/interfaces/repository_factory.go` + `infrastructure/repositories/factory.go`: remover método `CardThresholdReader`.
- [ ] 8.4 Editar `application/usecases/evaluate_threshold_alerts.go`: remover `cardReader`, `activeCards`, `buildCardSnapshots`; guarda vira `if len(active) == 0`.
- [ ] 8.5 Editar `application/usecases/notify_threshold_alert.go`, `infrastructure/repositories/postgres/threshold_alert_sent_repository.go`, `infrastructure/messaging/database/consumers/threshold_alert_notifier.go`: remover `case` de `card_limit_near`/`ThresholdAlertCardLimit`.
- [ ] 8.6 Editar `module.go`: remover `cfg.Card`/`cardRatio` do `ThresholdConfig`; regenerar `mocks/repository_factory.go`.
- [ ] 8.7 Testes: remover `TestDecideAlerts_CardLimit` e os 3 testes de cartão de `evaluate_threshold_alerts_test.go`; ajustar os 5 testes que faziam `EXPECT().CardThresholdReader(...)`.
- [ ] 8.8 Auditar `docs/dashboards/*budgets*`/`docs/alerts/*budgets*`: remover séries/queries que filtram `kind="card_limit_near"`.

## Detalhes de Implementação

Ver `techspec.md` §"Remoção cirúrgica em budgets" (lista fechada) e ADR-004. `buildSnapshots`
(categoria/metas) **não é tocado**. Sem migration em budgets; linhas legadas `card_limit_near` ficam inertes.

## Critérios de Sucesso

- Grep por `ThresholdAlertCardLimit`/`CardThresholdReader`/`ActiveCardForScan`/`card_limit_near` no módulo budgets retorna vazio.
- Suíte de budgets verde (categoria/metas preservados).
- Nenhum módulo lê `cards.limit_cents`.
- Dashboards/alertas sem referência ao label removido.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `otel-grafana-dashboards` — auditar e atualizar dashboards/alertas Grafana para remover a série do label `kind="card_limit_near"` (subtarefa 8.8).

## Testes da Tarefa

- [ ] Testes unitários: `threshold_workflow_test.go`, `evaluate_threshold_alerts_test.go` (ajustados; categoria/metas verdes).
- [ ] Testes de integração: `threshold_alerts_job_integration_test.go` (categoria/metas continuam disparando).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- Remover: `internal/budgets/application/interfaces/card_threshold_reader.go`, `infrastructure/repositories/postgres/card_threshold_reader.go`, `application/interfaces/mocks/card_threshold_reader.go`
- Editar: `internal/budgets/domain/services/threshold_workflow.go`, `application/interfaces/repository_factory.go`, `infrastructure/repositories/factory.go`, `application/usecases/{evaluate_threshold_alerts,notify_threshold_alert}.go`, `infrastructure/repositories/postgres/threshold_alert_sent_repository.go`, `infrastructure/messaging/database/consumers/threshold_alert_notifier.go`, `module.go`, mocks
- `docs/dashboards/*budgets*`, `docs/alerts/*budgets*`

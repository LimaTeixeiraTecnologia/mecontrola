# Tarefa 7.0: Avaliação assíncrona de alerta + consumer interno + ListAlerts

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar a avaliação assíncrona de alertas (RT-24): consumer interno `expense_committed_consumer` que despacha `budgets.expense.committed.v1` para o use case `EvaluateAlert`, que recalcula gasto agregado por raiz, atualiza `budgets_threshold_states` via `UpsertIfTransition` e insere linha em `budgets_alerts` somente quando há transição real `false→true`. Cobre todas as regras de supressão (rascunho, retroativo, stale, rate-limit). Expõe `ListAlerts` para `GET /api/v1/budgets/alerts`.

<requirements>
- Consumer idempotente por `event.ID` (garantia base do outbox); rejeições não derrubam o dispatcher.
- `EvaluateAlert` impõe a sequência: (1) recálculo da soma da raiz; (2) busca de budget — se não ACTIVE, suprime com métrica (RF-57); (3) `ThresholdEvaluator(spent, planned, currently_crossed)`; (4) `UpsertIfTransition` no estado; (5) decisão de inserção em `budgets_alerts`.
- Estado final do alerta no MVP (RF-64a/d):
  - `pending_delivery` quando transição real e cutoff = competência corrente. Imediatamente `delivered` no MVP (sem provider real).
  - `suppressed_stale` quando recálculo mostra que não há mais cruzamento (RF-56b).
  - `suppressed_retroactive` quando `committed_at` aponta para competência anterior à corrente (RF-60c/d).
  - `rate_limited` quando `CountDelivered` ≥ 10 para a tupla (RF-61/61a).
- Avaliação descarta evento com `committed_at < threshold_state.last_evaluated_committed_at` (idempotência out-of-order).
- `ListAlerts` é paginado por cursor (`opaque base64` codificando `(created_at, id)`), filtra por `competence`, `root_slug`, `threshold`, restrito ao `user_id` autenticado. Retorna apenas `pending_delivery|delivered` (RF-64c).
- Consumer registrado em `BudgetsModule.EventHandlers` para wiring em `cmd/worker/worker.go` (tarefa 10.0).
- Zero comentários em `.go` de produção.
</requirements>

## Subtarefas

- [ ] 7.1 `application/dtos/input/list_alerts_input.go` + `application/dtos/output/alert_output.go`.
- [ ] 7.2 `application/usecases/evaluate_alert.go` + unit tests cobrindo as 5 saídas (delivered, suppressed_stale, suppressed_retroactive, rate_limited, no-op por já cruzado).
- [ ] 7.3 `application/usecases/list_alerts.go` + unit tests cobrindo paginação cursor, filtros, isolamento por `user_id`.
- [ ] 7.4 `infrastructure/messaging/database/consumers/expense_committed_consumer.go` (adapter fino: deserializa envelope, chama `EvaluateAlert`). Tipo de evento exato: `"budgets.expense.committed.v1"`.
- [ ] 7.5 Integration test do consumer com producer mock: commita despesa, dispatcher entrega, alerta persiste.
- [ ] 7.6 Atualizar `mockery.yml`.

## Detalhes de Implementação

Ver **Fluxo de Dados** (caminho assíncrono), seção **Endpoints de API** e ADRs [`adr-002`](./adr-002-outbox-event-unico-expense-committed.md), [`adr-003`](./adr-003-threshold-state-tabela-dedicada.md) na `techspec.md`.

Cursor: codifica `(created_at_ts_micro, id)` em base64 url-safe; decoder valida formato e ordem. Limite default 50, máximo 200.

## Critérios de Sucesso

- Replay at-least-once do mesmo `event.ID` não duplica alerta nem incrementa contador.
- Cenário out-of-order: evento antigo chega depois de novo → `UpsertIfTransition` ignora; estado preserva `currently_crossed` do mais recente.
- Edição que reduz gasto abaixo do limiar marca `currently_crossed=false`; próxima despesa que cruza dispara novo alerta (RF-60b).
- `GET /api/v1/budgets/alerts` retorna apenas alertas do `Principal.UserID` (isolamento RF-71b validado em integration test).
- Cobertura ≥ 85% nos use cases; linter limpo.

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

- `internal/budgets/application/dtos/input/list_alerts_input.go` (novo)
- `internal/budgets/application/dtos/output/alert_output.go` (novo)
- `internal/budgets/application/usecases/{evaluate_alert,list_alerts}.go` (novo)
- `internal/budgets/infrastructure/messaging/database/consumers/expense_committed_consumer.go` (novo)
- `internal/budgets/infrastructure/messaging/database/consumers/expense_committed_consumer_integration_test.go` (novo)
- Referência: `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go`

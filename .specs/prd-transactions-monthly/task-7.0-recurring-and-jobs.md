# Tarefa 7.0: Use cases Recurring + `RecurringMaterializerJob` (double-layer) + `MonthlySummaryReconcilerJob`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa CRUD de `RecurringTemplate`, o use case `MaterializeRecurringForDay`, os 2 jobs (recurring materializer diário + monthly summary reconciler), e o producer fino. Idempotência **double-layer real** (audit fix #6): `pg_try_advisory_xact_lock` como first-cut + PK `(template_id, ref_month)` como race-final.

<requirements>
- Use cases CRUD em `application/usecases/`: `create_recurring_template.go`, `update_recurring_template.go`, `delete_recurring_template.go`, `get_recurring_template.go`, `list_recurring_templates.go`.
- `materialize_recurring_for_day.go`: filtra templates por `day_of_month == today.Day()` em fuso BR; para cada, `TryAdvisoryLock` → `InsertIfAbsent` → consome `RecurringWorkflow.DecideMaterializeForDay` → delega a `CreateTransaction` ou `CreateCardPurchase` conforme `payment_method`.
- `recurring_template_repository.go` com `db` campo: CRUD + `FindActiveByDayOfMonth(ctx, day, asOf, cursor, batchSize)` retornando em batches de 200.
- `recurring_materialization_repository.go` com `db` campo: `TryAdvisoryLock` (via `pg_try_advisory_xact_lock(hashtext(template_id || ref_month))`) + `InsertIfAbsent` (`INSERT ... ON CONFLICT DO NOTHING` — race-final).
- `recurring_template_event_publisher.go` fino: 3 eventos (`created/updated/deleted`).
- Job `RecurringMaterializerJob` em `infrastructure/jobs/handlers/`: fino, calcula `today := time.Now().In(saoPaulo)` e chama `usecase.MaterializeForDay(ctx, today)`. Métrica `transactions_recurring_materialize_{attempt_total,skipped_total,duration_seconds}`.
- Job `MonthlySummaryReconcilerJob`: lookback 48h via `TransactionsConfig.MonthlySummaryReconcilerLookbackHours`; compara `SUM(transactions) + SUM(card_invoice_items)` vs `monthly_summary` por `(user_id, ref_month)` com atividade; corrige drift + métrica `transactions_monthly_summary_drift_total{kind}`.
- Templates de crédito (`payment_method=credit_card`) só com `card_id` presente (check constraint + use case).
- Materialização **não retroativa**: template criado depois do `day_of_month` no mês corrente só materializa no próximo mês (RF-31).
- `occurred_at` da materialização = `today 12:00 America/Sao_Paulo`.
- Integration tests: re-execução no mesmo dia → 1 só materialização; concorrência → `skipped_total{reason="lock_not_acquired"}`; drift artificial corrigido pelo reconciler.
</requirements>

## Subtarefas

- [ ] 7.1 5 use cases CRUD `RecurringTemplate` + unit tests com mocks.
- [ ] 7.2 `materialize_recurring_for_day.go` + unit test cobrindo credit_card vs débito.
- [ ] 7.3 `recurring_template_repository.go` + integration test cobrindo `FindActiveByDayOfMonth` em batches.
- [ ] 7.4 `recurring_materialization_repository.go` + integration test cobrindo `TryAdvisoryLock` + `InsertIfAbsent` race.
- [ ] 7.5 `recurring_template_event_publisher.go` + integration test.
- [ ] 7.6 `RecurringMaterializerJob` handler fino + integration test "duas execuções → uma materialização".
- [ ] 7.7 `MonthlySummaryReconcilerJob` handler fino + integration test "drift detectado e corrigido".

## Detalhes de Implementação

Referência: techspec "Visão Geral dos Componentes" / `jobs/handlers/`, ADR-002 (double-layer idempotency), audit fix #6 (correção retórica). RF-27, RF-29..RF-34, RF-37, RF-40.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/transactions/application/usecases/...` cobertura ≥ 85% nos novos use cases.
- Integration test concorrente (2 goroutines disparando job no mesmo `(template_id, ref_month)`): exatamente 1 inserção em `transactions_recurring_materializations`; outra goroutine incrementa `skipped_total{reason="lock_not_acquired"}` ou `{reason="already_materialized"}`.
- Integration test reconciler: drift artificial de 100 cents em `transactions_monthly_summary` é corrigido na próxima execução; métrica `drift_total{kind="detected"}` e `{kind="corrected"}` incrementadas.
- Templates de crédito com `card_id=null` rejeitados na criação (`400 validation_error`).
- Materialização com `today < template.day_of_month` no mês corrente NÃO acontece (sem retroatividade).
- Zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests dos 5 CRUD use cases + `MaterializeRecurringForDay`.
- [ ] Integration test `recurring_template_repository_integration_test.go`.
- [ ] Integration test `recurring_materialization_repository_integration_test.go` (lock + PK race).
- [ ] Integration test `recurring_materializer_job_integration_test.go` (idempotência).
- [ ] Integration test `monthly_summary_reconciler_job_integration_test.go` (drift detection + correction).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/application/usecases/{create,update,delete,get,list}_recurring_template.go` (novos)
- `internal/transactions/application/usecases/materialize_recurring_for_day.go` (novo)
- `internal/transactions/application/usecases/reconcile_monthly_summary.go` (novo)
- `internal/transactions/infrastructure/repositories/postgres/recurring_template_repository.go` (novo)
- `internal/transactions/infrastructure/repositories/postgres/recurring_materialization_repository.go` (novo)
- `internal/transactions/infrastructure/messaging/database/producers/recurring_template_event_publisher.go` (novo)
- `internal/transactions/infrastructure/jobs/handlers/recurring_materializer_job.go` (novo)
- `internal/transactions/infrastructure/jobs/handlers/monthly_summary_reconciler_job.go` (novo)
- Testes `*_test.go` e `*_integration_test.go` correspondentes.

# Tarefa 9.0: Jobs billing — reconciliação horária e housekeeping de kiwify_events

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar dois jobs em `internal/billing/infrastructure/jobs/handlers/`: (a) `ReconciliationJob` `@every 1h` que invoca `KiwifyClient.ListSalesUpdatedSince(checkpoint - 15m, now)` e reusa os use cases do webhook (Tarefa 5.0) para corrigir divergências (RF-18); (b) `KiwifyEventsHousekeepingJob` que apaga linhas de `billing_kiwify_events` com retention > 90d (ADR-008 / LGPD básica). Ambos são `worker.Job` registrados via `internal/platform/worker/job.NewAdapter`.

<requirements>
- Implementações como métodos de struct expondo `Name()`, `Schedule()`, `Run(ctx) error` conforme `internal/platform/worker.Job`.
- Registro feito via `job.NewAdapter` (ou `NewAdapterWithPolicy`); nunca consumir worker direto.
- `Run(ctx)` é cancelável; respeita shutdown cooperativo (R6 / AGENTS.md §Worker).
- Reconciliação consome o **mesmo** use case do webhook (idempotência via `event_key` cobre dedup).
- Checkpoint avança somente após sucesso completo (todas as páginas processadas sem erro fatal).
- Housekeeping apaga em lote pequeno e respeita `BillingConfig.KiwifyEventsRetentionDays` (default 90).
- Logs estruturados via `slog`; métricas Prometheus conforme techspec §9.2.
- Sem panic; erros propagam para o `WorkerManager` registrar e re-tentar conforme política.
</requirements>

## Subtarefas

- [ ] 9.1 `jobs/handlers/reconciliation_job.go`: estrutura, `Name()`, `Schedule()` (`BillingConfig.ReconciliationSchedule`), `Run(ctx)` com paginação e checkpoint atomic.
- [ ] 9.2 `jobs/handlers/kiwify_events_housekeeping_job.go`: estrutura, `Schedule()` (`BillingConfig.KiwifyEventsHousekeepingSchedule`), `Run(ctx)` com DELETE batched.
- [ ] 9.3 Sintonia com `BillingConfig` para schedules, window overlap (15 min) e retention (90 d).
- [ ] 9.4 Métrica `billing_reconciliation_corrections_total` incrementada por trigger aplicado durante a corrida.
- [ ] 9.5 Logs `billing.reconciliation.run` com `window_start, window_end, sales_seen, corrections_applied, duration_ms`.
- [ ] 9.6 Unit tests: checkpoint só avança em sucesso, paginação completa, reuso idempotente do use case; housekeeping respeita janela e batch size.

## Detalhes de Implementação

- Pseudocódigo da reconciliação em techspec §7.4; `windowStart = checkpoint - 15m` absorve clock skew.
- O `KiwifyClient.ListSalesUpdatedSince` (Tarefa 7.0) já trata paginação, rate limit e retry; o job não duplica esses comportamentos.
- `reconcileSale` traduz uma sale em pseudo-evento `order_approved` ou `order_refunded` conforme `sale.status`; sale já processada pelo webhook é no-op (idempotência por `event_key`).
- Housekeeping: `DELETE FROM billing_kiwify_events WHERE received_at < now() - interval '90 days' LIMIT N` em loop curto até esgotar; respeita `ctx.Done()`.
- `WorkerManager` é configurado no `cmd/worker` (Tarefa 10.0); aqui só entregamos `worker.Job` registráveis.

## Critérios de Sucesso

- `go build ./internal/billing/infrastructure/jobs/handlers/...` verde.
- `go test -race -count=1 ./internal/billing/infrastructure/jobs/handlers/...` cobre paginação, falha parcial e checkpoint, housekeeping batch.
- Integ test (testcontainers Postgres + stub HTTP Kiwify): sale com `status='refunded'` reconciliada gera transição REFUNDED via mesmo use case do webhook; sale já processada é no-op (RF-11).
- Housekeeping remove linhas com `received_at > 90d` sem afetar linhas dentro da janela.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests dos dois jobs.
- [ ] Integ test de reconciliação E2E com stub HTTP.
- [ ] Integ test de housekeeping (limpa > 90d).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/infrastructure/jobs/handlers/{reconciliation_job,kiwify_events_housekeeping_job}.go` + `_test.go`
- Referência: `internal/platform/worker/job/adapter.go`, `internal/platform/worker/manager.go`.
- Referência: techspec §7.4 (reconciliação), §8.4 (LGPD), ADR-006 e ADR-008.

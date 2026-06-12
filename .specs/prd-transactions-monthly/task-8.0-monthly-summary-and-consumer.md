# Tarefa 8.0: MonthlySummary projection + consumer com coalescing 1500ms

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa a projeção `MonthlySummary` (use cases `Recompute`, `Get`, `ListEntries`), o repositório postgres, e o `MonthlySummaryRecomputeConsumer` com **debounce/coalescing por `(user_id, ref_month)`** em janela 1500ms (ADR-004). Drain de timers pendentes no shutdown (`graceful-lifecycle.md`).

<requirements>
- Use cases em `application/usecases/`: `recompute_monthly_summary.go`, `get_monthly_summary.go`, `list_monthly_entries.go`. `Recompute` lê `SUM(transactions)` e `SUM(card_invoice_items)` por `(user_id, ref_month)` filtrando `deleted_at IS NULL`; upsert em `transactions_monthly_summary`.
- `list_monthly_entries.go` retorna extrato unificado combinando `transactions` + `card_invoice_items` ordenado por `created_at DESC`, paginado por cursor base64.
- `GetMonthlySummary` para mês sem projeção retorna `200` com totais zerados + `updated_at=null` (RF-28).
- `monthly_summary_repository.go` com `db` campo: `Upsert(ctx, userID, refMonth, income, outcome, updatedAt)`, `Get`, `ListActiveSince` (para reconciler), `ListEntries` (extrato unificado via `UNION ALL`).
- `messaging/database/consumers/monthly_summary_recompute_consumer.go` fino: recebe `outbox.Envelope`, parse `payload.RefMonthsAffected` (ou single `ref_month` em eventos `transactions.transaction.*`), schedula recompute por `(user_id, ref_month)` no coalescer.
- Coalescer interno em `consumers/internal/coalescer.go`: `map[key]*time.Timer` protegido por `sync.Mutex`; método `Schedule(key, fn)` reseta timer se existir, cria se não; método `Stop()` cancela todos os timers pendentes e executa `fn` síncrono para cada chave restante até `ShutdownTimeout`.
- Métrica `transactions_monthly_summary_coalesce_factor` (Histogram — eventos colapsados por recompute).
- Janela configurável via `OutboxConfig.MonthlySummaryDebounceWindow` default 1500ms.
- Consumer registrado via `consumer.NewAdapter` para eventos `transactions.transaction.{created|updated|deleted}.v1` e `transactions.card_purchase.{created|updated|deleted}.v1`.
- Integration test cobrindo: 10 eventos da mesma chave em 200ms → 1 recompute; eventos de chaves distintas não coalescem; `Stop()` durante pendência drena.
</requirements>

## Subtarefas

- [ ] 8.1 `recompute_monthly_summary.go` + unit test cobrindo soft-delete filtrado.
- [ ] 8.2 `get_monthly_summary.go` + unit test cobrindo mês sem projeção (200 + zeros).
- [ ] 8.3 `list_monthly_entries.go` (UNION ALL transactions + card_invoice_items, cursor base64) + unit test.
- [ ] 8.4 `monthly_summary_repository.go` + integration test (upsert idempotente, version, ListEntries cursor).
- [ ] 8.5 `consumers/internal/coalescer.go` + unit test (10 eventos mesma chave → 1 fn; Stop drena).
- [ ] 8.6 `consumers/monthly_summary_recompute_consumer.go` (fino) + integration test (envelope → coalescer → usecase).
- [ ] 8.7 Configs `OutboxConfig.MonthlySummaryDebounceWindow` (`OUTBOX_MONTHLY_SUMMARY_DEBOUNCE_WINDOW`) com default 1500ms.

## Detalhes de Implementação

Referência: techspec "Visão Geral dos Componentes" / `consumers/`, ADR-004 (debounce/coalescing). RF-23 a RF-28, RF-39, RF-40.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/transactions/...` passa, incluindo testes do coalescer.
- Integration test consumer: 10 eventos da mesma chave em 200ms produzem 1 entrada em `transactions_monthly_summary`; métrica `coalesce_factor` registra valor ≈ 10.
- Integration test extrato unificado: lista combina `transactions` + `card_invoice_items` ordenado por `created_at DESC` com cursor preservando ordem.
- `Stop()` do coalescer drena timers pendentes (assert via método `len(pending)` antes/depois).
- Recompute filtra `deleted_at IS NULL` em ambas as queries.
- Zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests dos 3 use cases.
- [ ] Unit test do coalescer (mesma chave coalesce; chaves distintas não; Stop drena).
- [ ] Integration test `monthly_summary_repository_integration_test.go`.
- [ ] Integration test `monthly_summary_recompute_consumer_integration_test.go` (E2E com outbox + dispatcher fake).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/application/usecases/{recompute,get}_monthly_summary.go` (novos)
- `internal/transactions/application/usecases/list_monthly_entries.go` (novo)
- `internal/transactions/infrastructure/repositories/postgres/monthly_summary_repository.go` (novo)
- `internal/transactions/infrastructure/messaging/database/consumers/monthly_summary_recompute_consumer.go` (novo)
- `internal/transactions/infrastructure/messaging/database/consumers/internal/coalescer.go` (novo)
- Testes `*_test.go` e `*_integration_test.go` correspondentes.
- `configs/config.go` (modificado — `OutboxConfig.MonthlySummaryDebounceWindow`)

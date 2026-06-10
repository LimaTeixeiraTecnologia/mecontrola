# Tarefa 8.0: Eventos cross-module + pending events reaper job

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar a ingestão de despesas por evento (RF-32..RF-41) e a máquina de estados de pendentes (RF-38/RF-39a). Compreende o consumer `external_expense_consumer` (registra `source` da allowlist, delega para `UpsertExpense`), o use case `apply_pending_event` (decide outcome contra estado canônico), e o job periódico `pending_events_reaper` (poll 30s + expiry 24h embutido). ADR-005.

<requirements>
- Consumer reject evento com `source` ausente ou fora da allowlist (RF-32a/c) sem alterar estado financeiro.
- Evento sem campos obrigatórios (`event_id`, `source`, `external_transaction_id`, `occurred_at`, `user_id`, `operation`, `version`) → estado `failed` permanente (RF-34).
- Identidade `(user_id, source, external_transaction_id)` é dedup canônico (RF-35/42). Duplicate por mesmo `event.ID` é dedup do outbox.
- Versão monotônica: criação exige `version=1` (RF-36a); update/delete exige `expected_version == current_version + 1` (RF-36); regressivo ou já aplicado é no-op (RF-37); lacuna ou pre-creation vai para `pending` por até 24h (RF-38).
- `occurred_at` registrado para auditoria, **não** decide autoridade concorrente (RF-41).
- Job `pending_events_reaper`: schedule env-configurável (default 30s); por execução, `ListReady(200)` com `FOR UPDATE SKIP LOCKED`; expira (RF-39) → `expired`; aplicáveis → `apply_pending_event`.
- `apply_pending_event` chama `UpsertExpense`/`DeleteExpense` com `expected_version` recalculado; outcomes: `applied`, `obsolete_idempotent`, ainda pendente (sem transição).
- Job exporta métricas `budgets_pending_events_total{state,source}` e `budgets_pending_oldest_seconds{source}` (RF-39c, RT-25).
- Zero comentários em `.go` de produção.
</requirements>

## Subtarefas

- [ ] 8.1 `application/usecases/apply_pending_event.go` + unit tests cobrindo `applied`, `obsolete_idempotent`, ainda pendente, expirado.
- [ ] 8.2 `infrastructure/messaging/database/consumers/external_expense_consumer.go` (adapter fino: valida envelope, valida `source` na allowlist, decide entre `UpsertExpense` direto OU `PendingEventRepository.Insert`).
- [ ] 8.3 `infrastructure/jobs/handlers/pending_events_reaper.go` implementando interface `worker.Job` (Name/Schedule/Run).
- [ ] 8.4 Integration test do reaper cobrindo: aplicação tardia, expiração 24h, idempotência entre execuções consecutivas.
- [ ] 8.5 Integration test do consumer cobrindo: rejeita source fora da allowlist, persiste pending quando há lacuna, aplica direto quando versão é a próxima esperada.
- [ ] 8.6 Atualizar `configs/` para incluir `BUDGETS_PENDING_REAPER_INTERVAL` (default 30s) e `BUDGETS_PENDING_TTL_HOURS` (default 24).

## Detalhes de Implementação

Ver **Pontos de Integração** (produtores cross-module externos), seção **Modelos de Dados** (`budgets_expense_events_pending`) e ADR-005 (`pending-events-reaper-job`) na `techspec.md`. Padrão de job espelha `internal/billing/infrastructure/jobs/handlers/reconciliation_job.go`.

A allowlist é a constante Go criada em 3.0 (`internal/budgets/infrastructure/config/producers.go`).

## Critérios de Sucesso

- Integration test cobre 4 outcomes: aplicação tardia, expiração 24h, retry idempotente, evento com source inválido descartado.
- Métricas registradas após cada execução do reaper.
- Job idempotente: rodar 2 vezes consecutivas com mesma carga produz mesmo estado final.
- Linter limpo; cobertura ≥ 85%.

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

- `internal/budgets/application/usecases/apply_pending_event.go` (novo)
- `internal/budgets/infrastructure/messaging/database/consumers/external_expense_consumer.go` (novo)
- `internal/budgets/infrastructure/messaging/database/consumers/external_expense_consumer_integration_test.go` (novo)
- `internal/budgets/infrastructure/jobs/handlers/pending_events_reaper.go` (novo)
- `internal/budgets/infrastructure/jobs/handlers/pending_events_reaper_integration_test.go` (novo)
- `configs/` (incluir `BudgetsConfig`)
- Referência: `internal/billing/infrastructure/jobs/handlers/reconciliation_job.go`, `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go`

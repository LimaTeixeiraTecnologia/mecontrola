# Relatorio de Bugfix — Eventos órfãos não removidos (Task 5.0 incompleta)

- Total de bugs no escopo: 2
- Corrigidos: 2
- Testes de regressao adicionados: 3 (1 removido, 1 reescrito, 1 simplificado)
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: BUG-1
- Severidade: major
- Origem: RF-41 (PRD prd-refatoracao-agent-canonico); techspec Errata 2026-06-25 #3 (supera ADR-007 `[MANTER]`); Runbook §5
- Estado: fixed
- Causa raiz: A Task 5.0 alegou falsamente que `external.expense.v1` tinha consumer ativo legítimo e devia ser mantido. Verificação real: `grep` por producer do event-type retorna ZERO — era um pipeline consumer-sem-producer (código morto). RF-41 exige remover eventos órfãos; o pipeline inteiro de ingestão violava a regra ao permanecer.
- Arquivos alterados: removidos `internal/budgets/application/usecases/ingest_external_expense.go` (+`_test.go`), `internal/budgets/infrastructure/messaging/database/consumers/external_expense_consumer.go` (+`_test.go`+`_integration_test.go`), `internal/budgets/domain/commands/ingest_external_expense.go` (+`_test.go`), `internal/budgets/domain/services/external_expense_strategy.go` (+`_test.go`); editados `internal/budgets/module.go` (campo `ExternalExpenseConsumer`, campo `ingestExternalExpense`, construtor `NewExternalExpenseConsumer`, registro `external.expense.v1`, atribuição na struct de retorno e wiring `NewIngestExternalExpense` removidos), `internal/budgets/integration/transaction_to_budget_chain_integration_test.go`. VO `MutationKind` preservado (compartilhado).
- Teste de regressao: `internal/budgets/integration/transaction_to_budget_chain_integration_test.go` — `TestExternalExpenseConsumerUpdatesBudgetSummary` removido (testava o pipeline excluído); `TestThresholdAlertsJobPublishesOutboxEvent` reescrito para semear o gasto via `UpsertExpenseUC`, mantendo a cobertura do job de threshold sem o pipeline removido.
- Validacao: `go build ./...` OK; `go test ./internal/budgets/...` ok; `go vet -tags integration ./internal/budgets/integration/...` OK; grep produção e testes `external.expense|ExternalExpense|IngestExternalExpense` → ZERO (0 producer E 0 consumer).

- ID: BUG-2
- Severidade: major
- Origem: RF-41 (PRD prd-refatoracao-agent-canonico); techspec Errata 2026-06-25 #4; Runbook §5
- Estado: fixed
- Causa raiz: O relatório 5.0 alegou que "não há Publish ativo" para `onboarding.income_registered`. A alegação era FALSA: `save_onboarding_income.go` construía `entities.IncomeRegistered`, chamava `buildOutboxEvent` e `publisher.Publish` ativamente dentro da transação. O evento não tem nenhum consumer registrado (a renda flui para budgets via `onboarding.splits_calculated`), logo é um producer órfão que RF-41 exige eliminar.
- Arquivos alterados: `internal/onboarding/application/usecases/save_onboarding_income.go` (bloco `IncomeRegistered`+`buildOutboxEvent`+`publisher.Publish` removido; persistência `session.WithIncome`+`repo.Upsert` preservada; deps `publisher`/`idGen` removidas do struct e construtor; imports `outbox`/`id`/`entities` limpos); `internal/onboarding/module.go` (wiring `NewSaveOnboardingIncome` atualizado); `internal/onboarding/application/usecases/onboarding_event_id.go` (`case entities.IncomeRegistered` removido de `extractEventID`); `internal/onboarding/domain/entities/onboarding_session_events.go` (entidade `IncomeRegistered` removida).
- Teste de regressao: `internal/onboarding/application/usecases/save_onboarding_income_test.go` — mock `publisher`, expectativa `Publish` e arg do construtor removidos; import `outboxmocks` limpo. `TestHappyPath`/`TestBelowMinimumRejectedBeforeTx`/`TestSessionNotFound` continuam validando persistência e validação de VO (comportamento preservado).
- Validacao: `go build ./...` OK; `go test ./internal/onboarding/...` ok (incl. `application/usecases`); grep produção e testes `income_registered|IncomeRegistered` → ZERO (0 producer E 0 consumer); `onboarding.splits_calculated` intacto.

## Comandos Executados

- `go build ./...` -> BUILD OK (sem erro)
- `go test ./internal/budgets/... ./internal/onboarding/...` -> todos `ok`, 0 FAIL, 0 panic
- `go test ./internal/agent/... ./internal/budgets/... ./internal/onboarding/...` -> todos `ok`, 0 FAIL, 0 panic
- `go vet -tags integration ./internal/budgets/integration/...` -> VET OK
- `grep -rn "external.expense|ExternalExpense|IngestExternalExpense" --include=*.go internal/` -> ZERO (produção e testes)
- `grep -rn "income_registered|IncomeRegistered" --include=*.go internal/` -> ZERO (produção e testes)
- zero-comment check nos `.go` de produção tocados -> OK zero comentários
- Eventos pareados confirmados intactos: `transactions.card_purchase.deleted.v1`, `onboarding.splits_calculated`, `onboarding.card_registered`, `onboarding.completed`

## Documentos de spec corrigidos

- `.specs/prd-refatoracao-agent-canonico/adr-007-orphan-events-cleanup.md` — `external.expense.v1` realinhado de `[MANTER]` para `[REMOVER]` (Errata #3); `onboarding.income_registered` adicionado a `[REMOVER]`; `onboarding.completed` em `[MANTER]`.
- `.specs/prd-refatoracao-agent-canonico/5.0_execution_report.md` — alegações falsas substituídas por seção "Correção pós-execução" com a remediação real.

## Riscos Residuais

- Nenhum identificado. Ambos os eventos confirmados sem par produtor+consumidor antes da remoção; eventos pareados confirmados intactos; suites de budgets/onboarding/agent verdes.

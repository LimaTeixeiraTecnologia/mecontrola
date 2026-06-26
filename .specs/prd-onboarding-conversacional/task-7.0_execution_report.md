# Relatório de Execução — Tarefa 7.0: Working memory e limpeza de turns

# Generated: 2026-06-26T14:54:14Z

## Status

`done`

## Requisitos Funcionais Cobertos

- **RF-21**: Working memory consolidada na conclusão do onboarding (objetivo, renda, cartões, distribuição) e injetada no system prompt da operação diária via `ContextBuilder`; ausência de WM não é erro.
- **RF-24**: `recent_turns` zerados na conclusão do onboarding (`OnboardingSession.WithCompletion` define `RecentTurns = nil`).
- **RF-28 (herdado)**: Consumer `onboarding.completed` idempotente por `event_id` através da tabela `agent_processed_events`.

## Subtarefas

- [x] 7.1 Consumer `onboarding.completed` → consolida WM (markdown) via use case dedicado.
- [x] 7.2 Limpeza de `recent_turns` na conclusão.
- [x] 7.3 Garantir idempotência e ausência-tolerante de WM.

## Arquivos Alterados / Criados

### Novos
- `migrations/000022_create_agent_processed_events.up.sql`
- `migrations/000022_create_agent_processed_events.down.sql`
- `internal/agent/application/interfaces/processed_event_repository.go`
- `internal/agent/infrastructure/repositories/postgres/processed_event_repository.go`
- `internal/agent/application/usecases/consolidate_onboarding_working_memory.go`
- `internal/agent/application/usecases/consolidate_onboarding_working_memory_test.go`

### Modificados
- `internal/agent/infrastructure/messaging/database/consumers/onboarding_completed_consumer.go` — refatorado para adapter fino que delega ao use case dedicado.
- `internal/agent/infrastructure/messaging/database/consumers/onboarding_completed_consumer_test.go` — testes atualizados para o novo contrato.
- `internal/agent/infrastructure/repositories/factory.go` — expõe `ProcessedEventRepositoryFactory`.
- `internal/agent/module.go` — wiring do use case `ConsolidateOnboardingWorkingMemory` e do consumer.
- `internal/platform/workflow/infrastructure/postgres/store.go` — correção de build (tipo `Snapshot` → `workflow.Snapshot` e rename de parâmetro sombreador).
- `internal/agent/application/services/onboarding_agent.go` — adicionado import `time` faltante.
- Stubs de teste do kernel/workflow para acompanhar a nova interface `Store.ListSuspended`:
  - `internal/agent/application/workflow/transactions_write_test.go`
  - `internal/agent/application/workflow/transactions_write_kernel_test.go`
  - `internal/agent/application/services/daily_ledger_replay_test.go`
  - `internal/agent/application/services/intent_router_test.go`
  - `internal/agent/application/services/kernel_e2e_test.go`
  - `internal/agent/infrastructure/binding/forced_category_persist_regression_test.go`

## Decisões de Implementação

- Use case dedicado `ConsolidateOnboardingWorkingMemory` centraliza a regra de consolidação da WM; o consumer permanece fino (decode + delegate), sem regra de negócio, SQL ou branching.
- Idempotência por `event_id` implementada com tabela dedicada `agent_processed_events` (PK `event_id`), evitando duplicação mesmo se a WM já existir ou for atualizada por outro mecanismo.
- O markdown da WM agora inclui a distribuição planejada com percentuais formatados em português (`40,00%`).
- A limpeza de `recent_turns` já estava implementada em `OnboardingSession.WithCompletion`; foi mantida e coberta por teste existente.

## Comandos Executados

```bash
# Build
go build ./...                                  # BUILD_ALL_OK

# Testes unitários + race no escopo alterado e afetados diretos
go test -race -count=1 ./internal/agent/... ./internal/onboarding/...   # TEST_RACE_OK

# Testes unitários amplos
go test -race -short ./...                       # TEST_ALL_OK

# Vet
go vet ./internal/agent/... ./internal/onboarding/...   # VET_OK

# Lint
golangci-lint run ./internal/agent/... ./internal/onboarding/...   # 0 issues

# Drift de spec
ai-spec check-spec-drift .specs/prd-onboarding-conversacional   # OK: sem drift detectado

# Gates obrigatórios
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" "^[[:space:]]*//" internal/ configs/ cmd/ | grep -Ev "(//go:|//nolint:|// Code generated)"   # OK
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "internal/agent\|internal/transactions\|internal/billing\|internal/identity" internal/platform/workflow/   # OK
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|openai\|anthropic\|ParseInbound" internal/platform/workflow/ | grep -v "infrastructure/postgres"   # OK
f=$(find internal/agent -name daily_ledger_agent.go ! -name "*_test.go"); [ -n "$f" ] && c=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f"); [ "${c:-0}" -gt 1 ] && echo "FAIL" || echo OK   # OK
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" internal/agent/application/tools/ internal/agent/application/workflow/ 2>/dev/null   # OK
```

## Critérios de Aceite

- Após `onboarding.completed`, a WM reflete objetivo/renda/cartões/distribuição.
  - **comprovado**: teste `TestExecute_CreatesWorkingMemoryAndMarksProcessed` verifica conteúdo com "quitar dívidas", "R$ 5.000,00", "💰 Custo Fixo", "40,00%".
- `recent_turns` zerados na conclusão.
  - **comprovado**: teste `TestHappyPath_CompletedAtSetAndTurnsCleared` em `complete_onboarding_session_test.go` valida `len(p.RecentTurns) == 0`.
- Reprocessamento do evento não duplica WM.
  - **comprovado**: testes `TestExecute_AlreadyProcessed_SkipsUpsert` e `TestExecute_MarkProcessedConflict_TreatedAsProcessed` validam idempotência por `event_id`.

## Riscos Residuais

- A migration `000022_create_agent_processed_events` precisa ser aplicada no ambiente de deploy para a idempotência funcionar; sem a tabela, o consumer falhará ao marcar evento processado.
- O markdown da WM é em português e usa vírgula decimal; se o LLM de operação diária esperar formato diverso, pode ser ajustado sem impacto funcional.

## Suposições

- O `env.ID` do envelope outbox corresponde ao `event_id` do domínio (`OnboardingCompleted.EventID`), conforme implementação atual de `buildOutboxEvent`.
- A tabela `agent_processed_events` não precisa de TTL/retention nesta fase; eventos processados são imutáveis e pequenos.

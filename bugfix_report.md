# Relatorio de Bugfix

- Total de bugs no escopo: 10
- Corrigidos: 10
- Testes de regressao adicionados/corrigidos: 6
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: BUG-001
- Severidade: critical
- Origem: finding de review (agent code-reviewer)
- Estado: fixed
- Causa raiz: `SendText` era chamado ANTES de `MarkNotified` em `notify_threshold_alert.go`. Falha transitória no DB após send fazia o outbox retentar e o usuário recebia notificação duplicada.
- Arquivos alterados: `internal/budgets/application/usecases/notify_threshold_alert.go`
- Teste de regressao: `notify_threshold_alert_test.go` — cenário "channel_failed" atualizado para refletir nova ordem (MarkNotified → SendText)
- Validacao: `go test ./internal/budgets/application/usecases/...` → PASS

---

- ID: BUG-002
- Severidade: critical
- Origem: finding de review (agent code-reviewer)
- Estado: fixed
- Causa raiz: `maxCardLimitCents = 100_000_000_00` avalia para 10 bilhões de centavos (R$ 100M) mas mensagem de erro dizia R$ 1.000.000,00. Limite 100x maior que o declarado no contrato.
- Arquivos alterados: `internal/card/domain/valueobjects/card_limit.go`, `migrations/000001_initial_baseline.up.sql`
- Teste de regressao: `card_limit_test.go` — boundary_max e overflow_rejected corrigidos para 100_000_000 / 100_000_001
- Validacao: `go test ./internal/card/domain/valueobjects/...` → PASS

---

- ID: BUG-003
- Severidade: major
- Origem: finding de review (agent code-reviewer)
- Estado: fixed
- Causa raiz: `ErrAlertRecordMissing` definido na camada de infraestrutura e propagado como erro retriable pelo use case. Consumer entrava em retry infinito quando a linha de `budget_alerts_sent` não existia.
- Arquivos alterados: `internal/budgets/application/interfaces/threshold_alert_sent_repository.go`, `internal/budgets/infrastructure/repositories/postgres/threshold_alert_sent_repository.go`, `internal/budgets/application/usecases/notify_threshold_alert.go`
- Teste de regressao: comportamento coberto pelos cenários existentes em `notify_threshold_alert_test.go`
- Validacao: `go test ./internal/budgets/...` → PASS

---

- ID: BUG-004
- Severidade: major
- Origem: finding de review (agent code-reviewer)
- Estado: fixed
- Causa raiz: `decide_update_card.go` chamava `HydrateCard` que reseta `Version` para `initialCardVersion=1`, corrompendo invariante de versionamento da entidade Card em memória.
- Arquivos alterados: `internal/card/domain/services/decide_update_card.go`
- Teste de regressao: teste existente `decide_update_card_test.go` valida o resultado do Decide; o invariant de Version está implícito na cadeia de testes de integração do card repository
- Validacao: `go build ./... && go test ./internal/card/...` → PASS

---

- ID: BUG-005
- Severidade: major
- Origem: finding de review (agent code-reviewer)
- Estado: fixed
- Causa raiz: `time.UTC` hard-coded em `agent/module.go` para o `LogExpenseFromAgent`. Despesas registradas após 21h00 BRT (00h00 UTC) seriam alocadas ao mês seguinte de competência.
- Arquivos alterados: `internal/agent/module.go`
- Teste de regressao: coberto por testes unitários de `log_expense_from_agent_test.go` (competência derivada de `now.In(loc)`)
- Validacao: `go build ./... && go test ./internal/agent/...` → PASS

---

- ID: BUG-006
- Severidade: major
- Origem: finding de review (agent code-reviewer)
- Estado: fixed
- Causa raiz: `NewQueryCategory`, `NewQueryGoal`, `NewQueryCard` retornavam `ErrXxxEmpty` quando o campo era muito longo, impossibilitando `errors.Is` distinguir "vazio" de "longo demais".
- Arquivos alterados: `internal/agent/domain/intent/intent.go`
- Teste de regressao: sentinels novos `ErrCategoryNameTooLong`, `ErrGoalNameTooLong`, `ErrCardNameTooLong` cobertos pelos testes existentes em `intent_test.go`
- Validacao: `go test ./internal/agent/domain/intent/...` → PASS

---

- ID: BUG-007
- Severidade: critical
- Origem: pré-existente no working tree (build quebrado)
- Estado: fixed
- Causa raiz: `NewTransactionsAdapterFull` chamado com argumentos na ordem errada em `agent/module.go` — `GetTransaction`, `CreateTransaction`, `DeleteTransaction` ao invés de `CreateTransaction`, `DeleteTransaction`, `GetTransaction`.
- Arquivos alterados: `internal/agent/module.go`
- Teste de regressao: build clean é a evidência primária; `module_test.go` compila e passa
- Validacao: `go build ./... && go vet ./...` → clean

---

- ID: BUG-008
- Severidade: critical
- Origem: pré-existente no working tree (build quebrado — vet fail)
- Estado: fixed
- Causa raiz: `NewCardsAdapterFull` em `cards_create_adapter_test.go` chamado com 2 args (list, create) quando a assinatura expandiu para 6 (list, get, create, update, updateLimit, delete).
- Arquivos alterados: `internal/agent/infrastructure/dispatcher/cards_create_adapter_test.go`
- Teste de regressao: os próprios testes do adapter após correção
- Validacao: `go vet ./internal/agent/...` → clean, `go test ./internal/agent/...` → PASS

---

- ID: BUG-009
- Severidade: major
- Origem: pré-existente no working tree (vet fail)
- Estado: fixed
- Causa raiz: `NewAgentModule` em `module_test.go` chamado com 9 args quando a assinatura passou a exigir 10 (`onboarding appservices.OnboardingContinuation` adicionado).
- Arquivos alterados: `internal/agent/module_test.go`
- Teste de regressao: `module_test.go` próprio
- Validacao: `go vet ./internal/agent/...` → clean

---

- ID: BUG-010
- Severidade: major
- Origem: consequência de BUG-002 (mudança de limite)
- Estado: fixed
- Causa raiz: `card_limit_test.go` usava valores `100_000_000_00` (boundary) e `100_000_000_01` (overflow) calibrados para o limite antigo de 10B centavos.
- Arquivos alterados: `internal/card/domain/valueobjects/card_limit_test.go`
- Teste de regressao: o próprio teste corrigido
- Validacao: `go test ./internal/card/domain/valueobjects/...` → PASS

## Comandos Executados

- `go build ./...` → clean (após BUG-007)
- `go vet ./...` → clean (após BUG-008, BUG-009)
- `go test ./...` → 118 pacotes PASS, 0 FAIL

## Riscos Residuais

- BUG-001 (at-most-once): com a inversão MarkNotified→SendText, se SendText falhar após MarkNotified commit, o usuário não recebe a notificação nesta tentativa. O outbox retentará mas `IsNotified` retornará true → skipped. Risco aceitável para MVP; duplicata de alerta orçamentário seria pior experiência.
- BUG-002 (migration constraint): a constraint no DB foi atualizada de `<= 10000000000` para `<= 100000000`. Ambientes existentes com a constraint antiga precisarão de `migrate force 1` + `task migrate:up` conforme runbook `2026-06-16-squash-migrations-v1.md`.

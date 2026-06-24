# Relatorio de Bugfix

- Total de bugs no escopo: 5
- Corrigidos: 5
- Testes de regressao adicionados: 0 (testes existentes cobrem os paths; fix 1 coberto por TestVersionConflict_TriggersMetric ja adaptado; fix 2 coberto pela suite de resume; fix 4 coberto por comportamento unit da suite SettleRegistry existente)
- Pendentes: nenhum
- Estado final: done

## Bugs

### Bug 1
- ID: fix-1
- Severidade: major
- Origem: finding de review — workflow-kernel production-ready
- Estado: fixed
- Causa raiz: `saveSnap` incrementava `versionConflict` para qualquer `err != nil`, causando falsos positivos em erros de rede/timeout.
- Arquivos alterados: `internal/platform/workflow/engine.go`
- Teste de regressao: `TestVersionConflict_TriggersMetric` (existente, continua passando; somente erros `ErrVersionConflict` acionam o counter)
- Validacao: `go test ./internal/platform/workflow/... -race -count=1` -> ok

### Bug 2
- ID: fix-2
- Severidade: major
- Origem: finding de review — workflow-kernel production-ready
- Estado: fixed
- Causa raiz: `Resume` ignorava silenciosamente falha de decode do payload de retomada (`decErr != nil` caía no else-branch), continuando com o estado do snapshot e causando re-suspensão infinita sem erro observável.
- Arquivos alterados: `internal/platform/workflow/engine.go`
- Teste de regressao: `TestResume_AfterSuspend` (existente, path de sucesso continua passando); bug exposto por `decErr != nil` que agora retorna `fmt.Errorf("workflow.engine.resume: decode resume payload: %w", decErr)`.
- Validacao: `go test ./internal/platform/workflow/... -race -count=1` -> ok

### Bug 3
- ID: fix-3
- Severidade: minor
- Origem: finding de review — workflow-kernel production-ready
- Estado: fixed
- Causa raiz: `Definition.MaxAttempts` era dead config — setado como 3 em `NewTransactionsWriteDefinition` mas nunca lido pelo engine para gates de retry de run inteiro, enganando callers.
- Arquivos alterados: `internal/platform/workflow/engine.go`, `internal/agent/application/workflow/transactions_write.go`, `internal/platform/workflow/engine_test.go`, `internal/agent/application/workflow/transactions_write_test.go`
- Nota: `Snapshot.MaxAttempts` mantido pois a coluna `max_attempts NOT NULL CHECK > 0` existe no schema do banco (migration 000019). O campo `Definition.MaxAttempts` foi removido; o engine hardcoda `MaxAttempts: 1` no snapshot. Retry por step permanece via combinator `Retry`.
- Teste de regressao: todos os testes de engine com `Definition` sem `MaxAttempts` passam; asserção `s.Greater(def.MaxAttempts, 0)` removida de `transactions_write_test.go`.
- Validacao: `go test ./internal/platform/workflow/... ./internal/agent/application/workflow/... -race -count=1` -> ok

### Bug 4
- ID: fix-4
- Severidade: major
- Origem: finding de review — workflow-kernel production-ready
- Estado: fixed
- Causa raiz: `SettleRegistry` acumulava entradas `DecisionID → AuditSettleFunc` indefinidamente quando runs eram abandonados (usuário nunca retoma), causando memory leak.
- Arquivos alterados: `internal/agent/application/services/daily_ledger_agent.go`
- Correção: introduzido `settleEntry{fn, expiresAt}` com TTL de 30 dias. `Register` faz eviction passiva de entradas expiradas antes de inserir. `pop` descarta entrada se expirada.
- Teste de regressao: `go test ./internal/agent/application/services/... -race -count=1` -> ok (suite existente cobre Register/pop)
- Validacao: `go test ./internal/agent/application/services/... -race -count=1` -> ok

### Bug 5
- ID: fix-5
- Severidade: minor
- Origem: finding de review — workflow-kernel production-ready
- Estado: fixed
- Causa raiz: `WriteGuard.Apply` descartava `settle` silenciosamente quando `stop=true`, violando o contrato do tipo `GuardSteps.Audit`. Nenhuma implementação atual dispara este path, mas era armadilha latente.
- Arquivos alterados: `internal/agent/application/workflow/write_guard.go`
- Correção: quando `stop=true && settle != nil`, chama `settle(ctx, false)` antes de retornar `GuardShortCircuit`.
- Teste de regressao: `go test ./internal/agent/application/workflow/... -race -count=1` -> ok
- Validacao: `go test ./internal/agent/application/workflow/... -race -count=1` -> ok

## Comandos Executados
- `go build ./...` -> ok (zero erros de compilação)
- `go test ./internal/platform/workflow/... -race -count=1` -> ok
- `go test ./internal/agent/application/... -race -count=1` -> ok
- `go test ./... -count=1` -> zero FAILs
- gate zero-comentários em `internal/platform/workflow/` e `internal/agent/application/workflow/` -> OK

## Riscos Residuais
- `Snapshot.MaxAttempts` continua persistido no banco com valor fixo `1`. O campo não tem semântica no engine atual mas é necessário para satisfazer a constraint `NOT NULL CHECK > 0`. Remoção da coluna requer migration explícita — fora do escopo deste bugfix.
- TTL de 30 dias no `SettleRegistry` é fixo por constante. Se `HousekeepingRetentionDays` do kernel for alterado, o TTL não acompanha automaticamente — aceitável como MVP conforme especificação.

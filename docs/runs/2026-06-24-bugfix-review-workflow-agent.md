# Relatório de Bugfix/Revisão — Workflow Kernel + Agent HITL

- **Data:** 2026-06-24
- **Escopo:** `internal/platform/workflow`, `internal/agent` (HITL e consumo do kernel)
- **Objetivo:** Revisão final bloqueante (rodada 3) com correção de achados CRITICAL/HIGH antes de merge.

---

## Veredito

**APPROVED** — após correções, todos os achados bloqueantes foram sanados e as validações obrigatórias passam.

---

## Achados Corrigidos

### 1. `Parallel` não propagava `Suspended`/`Failed` dos filhos (CRITICAL)

**Arquivo:** `internal/platform/workflow/combinators.go`

**Problema:** o combinador `Parallel` ignorava o `StepStatus` retornado pelos steps filhos. Se um filho retornava `Suspended` ou `Failed` sem erro, o merge prosseguia como `Completed`, corrompendo o fluxo de suspensão/falha.

**Correção:**
- `Parallel.Execute` agora inspeciona os `StepOutput` de cada filho.
- Se houver `Suspended`, propaga `Suspended` com o estado/suspensão do primeiro.
- Se houver `Failed` (sem suspensão), propaga `Failed`.
- Erros continuam sendo agregados via `errors.Join`.
- A goroutine possui `defer recover()` para capturar panics e convertê-los em erro, respeitando R5.12.
- Foram extraídos helpers (`runWorkers`, `runWorker`, `joinErrors`, `collectParallelResults`) para manter a complexidade ciclomática dentro do limite do `revive`.

### 2. `Parallel` sem `recover` nas goroutines (HIGH)

**Arquivo:** `internal/platform/workflow/combinators.go`

**Correção:** adicionado `defer/recover` dentro de `runWorker`; panic é convertido em erro, as irmãs são canceladas e `wg.Done()` é garantido.

### 3. `Start` inseria run sem verificar run ativo (HIGH)

**Arquivos:** `internal/platform/workflow/engine.go`, `internal/platform/workflow/store.go`

**Problema:** `Engine.Start` inseria um novo snapshot sem verificar se já existia run `running`/`suspended` para a mesma `(workflow, correlation_key)`. O índice parcial único no banco geraria erro cru de constraint.

**Correção:**
- Adicionado `ErrRunAlreadyExists` em `store.go`.
- `Start` faz `store.Load` antes de `Insert`; se encontrar run ativa, retorna `ErrRunAlreadyExists` com span attribute `outcome=active_run_exists`.

### 4. StepRecord duplicado em `Resume` (HIGH)

**Arquivos:** `migrations/000019_create_workflow_runtime.up.sql`, `internal/platform/workflow/infrastructure/postgres/store.go`

**Problema:** em `Resume`, o step que suspendeu é reexecutado e gera um segundo `StepRecord` idêntico em `(run_id, step_id, seq, attempt)`, poluindo auditoria.

**Correção:**
- Adicionada constraint única `workflow_steps_run_seq_attempt_uidx` em `(run_id, seq, attempt)`.
- `AppendStep` agora usa `INSERT ... ON CONFLICT (run_id, seq, attempt) DO UPDATE`, garantindo idempotência por execução.

### 5. TOCTOU em `delete_last` e `edit_last` (CRITICAL)

**Arquivos:** `internal/agent/infrastructure/binding/hitl_adapters.go`, `internal/agent/domain/confirmation/draft.go`, `internal/agent/module.go`, `internal/agent/infrastructure/binding/hitl_adapters_test.go`

**Problema:** o resolver listava transações e mostrava uma descrição ao usuário, mas o executor relistava e agia sobre o "último" novamente. Entre os passos poderia entrar uma nova transação, causando delete/edit no lançamento errado.

**Correção:**
- `ConfirmState` ganhou `TargetTransactionID` e `TargetTransactionVersion`.
- `NewLastTransactionDeleterResolver` e `NewLastTransactionEditorResolver` capturam `ID`/`Version` do target e persistem no estado.
- `NewLastTransactionDeleterExecutor` e `NewLastTransactionEditorExecutor` passaram a receber apenas o deleter/editor (sem lister) e usam os valores capturados, sem relistar.
- `module.go` atualizado para as novas assinaturas.
- Testes em `hitl_adapters_test.go` ajustados e ampliados para validar uso do target e erro quando target ausente.

### 6. Autorização tautológica no transactions workflow (HIGH)

**Arquivos:** `internal/agent/application/services/agent_workflows.go`, `internal/agent/application/services/daily_ledger_agent.go`

**Problema:** a guarda `Authorize` do `transactions` workflow criava `Principal{UserID: state.UserID}` e comparava com `state.UserID`, sendo tautológica.

**Correção:**
- `Authorize` extrai o principal autenticado via `auth.FromContext(ctx)`.
- `DailyLedgerAgent.dispatchWriteKernel`, `dispatchWriteDestructive`, `continuePendingExpenseConfirmationKernel` e `continuePendingApproval` injetam `auth.Principal` no contexto com `auth.WithPrincipal`.
- O principal autenticado é convertido para o tipo local `Principal` e passado para `authorizeWrite`, que valida `effectiveUserID == principal.UserID`.

### 7. Autorização fraca no destructive workflow (HIGH)

**Arquivo:** `internal/agent/module.go`

**Problema:** a guarda `Authorize` do workflow destrutivo apenas validava se `state.UserID` era UUID válido, sem comparar com o principal autenticado.

**Correção:** a guarda extrai `auth.FromContext(ctx)` e exige `principal.UserID == uuid.Parse(state.UserID)`.

### 8. `OnSettle` nil no `DestructiveConfirmDeps` (HIGH — falso positivo)

**Arquivo:** `internal/agent/module.go`

**Avaliação:** o achado foi revisado e o `OnSettle` já estava preenchido corretamente (`settleReg.Register(...)`). Nenhuma alteração foi necessária.

---

## Outras Melhorias Técnicas

- `dispatchWriteDestructive` teve seu tamanho reduzido com extração de `initialConfirmState`, evitando violação de `function-length` do `revive`.
- `Branch` retorna erro quando nenhuma rota é encontrada (já presente no working tree, validado pelos testes).
- `Retry` valida e normaliza `RetryPolicy` (`MaxAttempts`, `BaseBackoff`, `MaxBackoff`).
- `Engine` propaga `ErrVersionConflict` como `ErrRunConflict` e lida com CAS corretamente.

---

## Evidências de Validação

```bash
# Build e vet
go build ./...
go vet ./...

# Lint no escopo alterado
golangci-lint run ./internal/platform/workflow/... ./internal/agent/... ./configs/...
# 0 issues

# Testes unitários + race
go test -race -count=1 ./internal/platform/workflow/...
go test -race -count=1 ./internal/agent/...
go test -race -count=1 ./configs/...

# Testes de integração (inclui workflow postgres store)
go test -tags=integration -race -count=1 ./internal/platform/workflow/...
go test -tags=integration -race -count=1 ./internal/agent/...
```

Resultado: **todos os pacotes passaram** (`ok` / `0 issues`).

---

## Arquivos Alterados nesta Rodada de Bugfix

- `internal/platform/workflow/combinators.go`
- `internal/platform/workflow/engine.go`
- `internal/platform/workflow/store.go`
- `internal/platform/workflow/infrastructure/postgres/store.go`
- `migrations/000019_create_workflow_runtime.up.sql`
- `internal/agent/infrastructure/binding/hitl_adapters.go`
- `internal/agent/infrastructure/binding/hitl_adapters_test.go`
- `internal/agent/domain/confirmation/draft.go`
- `internal/agent/application/services/agent_workflows.go`
- `internal/agent/application/services/daily_ledger_agent.go`
- `internal/agent/module.go`

---

## Recomendações Pós-Merge

1. **Migração 000019:** a alteração adiciona `UNIQUE (run_id, seq, attempt)`. Em ambientes onde a migration já foi aplicada, será necessário recriar a constraint ou executar migration de correção separada (000020) antes do deploy.
2. **Testes E2E manuais:** exercitar os cenários de HITL (`Apaga o Uber`, `O Uber foi 42 e não 35`) com duas mensagens simultâneas para confirmar que o TOCTOU não regressa.
3. **Testes de concorrência no Parallel:** embora os testes unitários existam, um teste de stress com muitos steps paralelos e suspensão/falha pode aumentar a confiança.

---

## Conclusão

Os achados bloqueantes identificados na revisão final foram corrigidos com mudanças mínimas e seguras, preservando a arquitetura e as fronteiras do kernel/workflow. Todas as validações obrigatórias (build, vet, lint, testes unitários e de integração) estão verdes.

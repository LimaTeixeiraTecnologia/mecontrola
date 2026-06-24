# Bugfix Report — prd-agent-platform-evolution

**Data:** 2026-06-24
**Origem:** review --effort high (rodada 1, pós execute-all-tasks)
**Total no escopo:** 3 bugs
**Corrigidos:** 3
**Testes de regressão adicionados:** 3

## Bugs Corrigidos

### bug-hitl-expiry-fallthrough (critical → fixed)

**Origem:** RF-12, RF-13, DoD task-7.0 ("expirar→fall-through")
**Arquivo:** `internal/agent/domain/confirmation/draft.go`, `internal/agent/application/workflow/steps/confirm_gate.go`, `internal/agent/application/services/daily_ledger_agent.go`

**Causa raiz:** `confirm_gate.go` sinalizava expiração apenas via `ShortCircuit=true`, indistinguível de um cancelamento normal. `continuePendingApproval` sempre retornava `(true, RouteResult{...})` em runs completos, capturando a mensagem em vez de deixá-la cair no ParseInbound.

**Fix:**
1. `draft.go`: adicionado campo `Expired bool json:"expired"` a `ConfirmState`
2. `confirm_gate.go`: setado `state.Expired=true` no path de TTL expirado (mantidos `Outcome` e `Reply` para auditoria interna)
3. `daily_ledger_agent.go`: `continuePendingApproval` verifica `result.State.Expired` e retorna `(false, RouteResult{})` para fall-through ao parser

**Teste de regressão:** `TestDestructiveKind_Expired_FallThrough` em `hitl_routing_test.go` — valida que após TTL=1ms expirar, a próxima mensagem NÃO é capturada pelo HITL (handled=false → parser recebe o texto).

**Validação:** `go test ./internal/agent/application/...` — VERDE

---

### bug-hitl-ambiguous-test-missing (major → fixed)

**Origem:** DoD task-7.0 ("4 cenários E2E: confirmar, cancelar, ambíguo→reprompt→cancela, expirar→fall-through")
**Arquivo:** `internal/agent/application/services/hitl_routing_test.go`

**Causa raiz:** Cenário ambíguo→reprompt→cancela existia apenas no nível de step (`hitl_steps_test.go`) mas não no nível de serviço (integração engine+roteamento). DoD exige evidência ao nível de serviço.

**Fix:** Adicionado `TestDestructiveKind_AmbiguousRepromptCycle` em `hitl_routing_test.go`: 3 mensagens (start → ambíguo → ambíguo) validando que o executor nunca é chamado e que o cancelamento é confirmado.

**Validação:** `go test ./internal/agent/application/services/ -run TestHITLRoutingSuite` — 6/6 PASS

---

### bug-resume-applier-dead-code (major → fixed)

**Origem:** task-2.0 (engine.Resume substituído por MergePatch)
**Arquivo:** `internal/platform/workflow/store.go`, `internal/agent/application/workflow/steps/state.go`

**Causa raiz:** Interface `ResumeApplier[S any]` e método `ExpenseState.ApplyResume` tornaram-se dead code após a task-2.0 substituir o engine.Resume por MergePatch. O engine não mais referencia `ResumeApplier`.

**Fix:**
1. Removida interface `ResumeApplier[S any]` de `store.go`
2. Removido método `ApplyResume` de `ExpenseState` em `state.go`

**Validação:** `go build ./...` — VERDE; `go test ./internal/platform/workflow/...` — VERDE

---

## Riscos Residuais

- `ConfirmState.Outcome` armazena `tools.ToolOutcome` como `int` — funcional mas sem type-safety estrita. Não corrigido neste ciclo (escopo aditivo).
- `MergePatch(base, []byte{})` retorna erro em vez de no-op (bytes verdadeiramente vazios). Guardado pelo engine com `len(resume) > 0`. Não corrigido (baixa prioridade).

## Gates R-* Finais (todos PASS)

| Gate | Resultado |
|------|-----------|
| R-WF-KERNEL-001.1 (sem import domínio) | PASS |
| R-WF-KERNEL-001.6 (zero comentários kernel) | PASS |
| R-ADAPTER-001.1 (zero comentários global) | PASS |
| R-AGENT-WF-001.1 (switch não cresceu, count=0) | PASS |

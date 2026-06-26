# Checklist de validação — antes do merge

Rode após qualquer mudança em `internal/agent`. Espelha `docs/prompts/refactor_internal_agent.md` §4
e os gates de `.claude/rules/agent-workflows-tools.md`.

## Build, vet e race

```bash
go build ./internal/agent/...
go vet ./internal/agent/...
go test -race -count=1 ./internal/agent/...
```

## Gate 1 — switch de domínio não cresce em daily_ledger_agent.go

```bash
f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
[ -n "$f" ] && cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true) && \
  { [ "${cases:-0}" -gt 1 ] && echo "FAIL: switch de domínio cresceu (cases=$cases)" || echo "OK gate1"; }
```

## Gate 2 — zero comentários em tools/ e workflow/

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/agent/application/tools/ internal/agent/application/workflow/ 2>/dev/null \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentários proibidos" || echo "OK gate2"
```

## Gate 3 — sem SQL direto em tools/ e workflow/

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agent/application/tools/ internal/agent/application/workflow/ 2>/dev/null \
  && echo "FAIL: SQL direto em tool/workflow" || echo "OK gate3"
```

## Gate 4 — pending step salvo em OutcomeClarify (R-AGENT-WF-001.7)

```bash
grep -n "OutcomeClarify" internal/agent/application/services/daily_ledger_agent.go \
  | grep -v "savePendingDraft\|buildPendingDraft\|_test\|CategoryNotFound\|CategoryHintMissing" \
  && echo "WARN: revisar se OutcomeClarify retorna sem salvar Draft" || echo "OK gate4"
```

## Gate 5 — checklist manual (R-AGENT-WF-001 / R-ADAPTER-001 / R-TESTING-001)

- [ ] Comportamento novo entrou como Workflow/Tool no seam `buildRegistry`, não como `case`.
- [ ] Tool fina: sem regra de negócio, SQL ou branching de domínio.
- [ ] `ToolOutcome`/`RunStatus`/`AwaitingKind`/`TransactionKind`/`Kind` fechados (sem string livre).
- [ ] Escrita durável passa pelo kernel write seam (`NewTransactionsWriteDefinition`, roteado por `kind.IsKernelWrite()`); leitura não duplica authz/policy.
- [ ] LLM só em `ParseInbound` (ou exceção sancionada: conversational/onboarding).
- [ ] `OutcomeClarify` sempre acompanhado de `Draft` salvo; draft limpo após uso.
- [ ] Run auditável: thread resolvido, status fechado, métricas com labels enum-only.
- [ ] Testes no padrão testify/suite (whitebox, `fake.NewProvider()`, IIFE por mock).

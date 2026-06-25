# Tarefa 3.0: Consolidar WriteGuard única no kernel

<critical>Ler o plano-fonte `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md` (Item 3) antes de iniciar.</critical>

## Visão Geral

Eliminar a duplicação da cadeia de pré-escrita (authorize → replay → policy → audit). Hoje ela existe em dois lugares: o legacy `newWriteGuard()`/`WriteGuard.Apply` (aplicado pelo `composite`) e os steps do kernel (`steps/authorize.go`, `replay.go`, `policy.go`, `audit_begin.go`). Após a consolidação, os write kinds passam exclusivamente pela cadeia do kernel; o guard legacy é removido.

<requirements>
- Uma única cadeia de guarda de escrita: os steps do kernel (`NewTransactionsWriteDefinition`).
- `newWriteGuard()`, `WriteGuard`/`GuardSteps`/`Apply` (`write_guard.go`) e o ramo de guard em `composite.Execute` removidos.
- `buildRegistry` deixa de injetar `guard` nos workflows de escrita.
- `composite` permanece para leituras/conversational (sem guard) — não removê-lo inteiro.
- `ToolOutcome`/`RunStatus` permanecem tipos fechados (R-AGENT-WF-001.3); Run auditável preservado (R-AGENT-WF-001.5).
- Switch de domínio de `daily_ledger_agent.go` não cresce (R-AGENT-WF-001.1).
- Zero comentários em Go de produção (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 3.1 Remover `newWriteGuard()` (`agent_workflows.go`) e o tipo `WriteGuard`/`GuardShortCircuit`/`SettleFunc` (`write_guard.go`) se sem outro consumidor.
- [ ] 3.2 Simplificar `composite.Execute` removendo o ramo `c.guard != nil`/`Apply`/`settle`, mantendo o caminho de leitura.
- [ ] 3.3 Ajustar `buildRegistry`/`NewIntentWorkflow` para não receber/injetar guard nos workflows de escrita.
- [ ] 3.4 Garantir que `settle` (decision audit) continua sendo chamado uma única vez via o step `audit_begin` do kernel — sem decision pendurado.
- [ ] 3.5 Remover/reescrever `TestNewWriteGuardIsNotNil` (`agent_workflows_test.go:93`) e adaptar `parity_test.go` para não depender de `NewWriteGuard`.

## Detalhes de Implementação

Ver plano-fonte (Item 3). Âncoras: `agent_workflows.go` (`newWriteGuard` ~:176, `buildKernelDefinition` ~:100), `internal/agent/application/workflow/{composite,write_guard,transactions_write}.go`, `steps/{authorize,replay,policy,audit_begin}.go`. Pré-condição: Task 2.0 concluída (mesmos arquivos de serviço).

## Critérios de Sucesso

- Todos os write kinds resolvidos pelo kernel; nenhum write passa pelo `composite` com guard.
- Ordem dos estágios idêntica à anterior (authorize→replay→policy→audit) — sem mudança de `Outcome` em conflitos.
- `parity_test.go` (adaptado) e suites do pacote workflow verdes.

## Definition of Done (DoD)

1. Guard legacy (`newWriteGuard`, `WriteGuard`, ramo de guard no `composite`) não existe mais.
2. Cadeia authorize/replay/policy/audit existe apenas como steps do kernel.
3. `composite` ainda serve leituras sem guard.
4. Nenhum decision audit fica sem `settle`.
5. Build + suites verdes; `parity_test.go` adaptado e verde.

## Critérios de Aceite (gates executáveis)

```bash
cd /Users/jailtonjunior/Git/mecontrola

grep -rn "func.*newWriteGuard\|NewWriteGuard\|type WriteGuard\|GuardShortCircuit\|c.guard.Apply" \
  internal/agent --include="*.go" --exclude="*_test.go" \
  && echo "FAIL: WriteGuard legacy ainda presente" || echo "OK"

grep -n "c.guard\|GuardShortCircuit\|settle" internal/agent/application/workflow/composite.go \
  && echo "REVISAR: composite ainda referencia guard de escrita" || echo "OK"

test "$(grep -rl 'steps.NewAuthorize\|steps.NewReplay\|steps.NewPolicy\|steps.NewAuditBegin' internal/agent --include='*.go' --exclude='*_test.go' | wc -l | tr -d ' ')" -ge 1 \
  && echo "OK: steps do kernel presentes" || echo "FAIL"

f=internal/agent/application/services/daily_ledger_agent.go
cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
[ "${cases:-0}" -gt 1 ] && echo "FAIL: switch de domínio cresceu (cases=$cases)" || echo "OK"

grep -rn --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/agent/application/tools/ internal/agent/application/workflow/ 2>/dev/null \
  | grep -Ev "(//go:|//nolint:|// Code generated)" && echo "FAIL: comentários proibidos" || echo "OK"

go build ./internal/agent/... && go test ./internal/agent/application/workflow/... ./internal/agent/application/services/
```

## Skills Necessárias

- `go-implementation` — refatoração de Go de produção (Etapas 1–5 + checklist R0–R7).
- `mastra` — WriteGuard/steps/Run auditável são padrão Workflow/Tool do agent (R-AGENT-WF-001.2/.3/.5).

## Testes da Tarefa

- [ ] Testes unitários: cada estágio (authorize/replay/policy/audit) produz o mesmo `Outcome` que antes; cruzamentos (authz-denied + replay) cobertos.
- [ ] Testes de integração: write end-to-end pelo kernel com reply idêntico ao legacy.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`.</critical>

## Arquivos Relevantes
- `internal/agent/application/services/agent_workflows.go`
- `internal/agent/application/workflow/composite.go`, `write_guard.go`, `transactions_write.go`
- `internal/agent/application/workflow/steps/{authorize,replay,policy,audit_begin}.go`
- `internal/agent/application/services/parity_test.go`, `agent_workflows_test.go` (adaptar)

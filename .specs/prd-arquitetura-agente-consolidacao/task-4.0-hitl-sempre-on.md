# Tarefa 4.0: HITL sempre-on — remover bypass sem confirmação e duplo caminho de tools

<critical>Ler o plano-fonte `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md` (Item 4), ADR-002 e ADR-003 (`.specs/prd-agent-platform-evolution/`) antes de iniciar. ATENÇÃO ao escopo: NÃO remover `dispatchWriteDestructive`/`wireBudgetCommitGate` — são o caminho canônico.</critical>

## Visão Geral

Garantir que as 4 operações destrutivas/sensíveis (`delete_last_transaction`, `edit_last_transaction`, `delete_card`, `configure_budget` commit) só sejam efetivadas após confirmação humana explícita via `destructive_confirm`. Remover o **ramo de bypass** (`confirmEngine == nil`) e o **duplo caminho de execução** dos tools de delete/edit que hoje podem mutar fora do gate.

<requirements>
- As 4 operações só executam após `confirm_gate`; não há caminho que as efetive sem confirmação (ADR-002 / R-AGENT-WF-001.7-A).
- Ramo de bypass em `dispatchWrite` (quando `confirmEngine == nil`) eliminado: kind destrutivo sem `confirmEngine` retorna erro explícito, nunca executa.
- Os tools `NewDeleteLastTransaction`/`NewEditLastTransaction`/`NewDeleteCard` deixam de ter caminho auto-executável fora do gate; a mutação ocorre só via `Executor` do `destructive_confirm`.
- `dispatchWriteDestructive` e `wireBudgetCommitGate` permanecem (caminho único canônico).
- `ConfirmState` persistido com `AwaitingConfirm` antes de retornar a pergunta; `continuePendingApproval` resolve antes de `ParseInbound`; run nunca fica `Suspended` órfão (ADR-003).
- `AwaitingApproval`/`OperationKind` como tipos fechados, nunca string solta; roteamento via `map[OperationKind]`, não `case` no switch (R-AGENT-WF-001.1/.7-A).
- Budget commit continua chamando `clearBudgetSession` no caminho do gate.
</requirements>

## Subtarefas

- [ ] 4.1 Remover o ramo de bypass em `dispatchWrite` (`daily_ledger_agent.go` ~:263-272); kind destrutivo sem `confirmEngine` → erro explícito.
- [ ] 4.2 Garantir caminho único de mutação para delete/edit/card-delete: tools deixam de ser registrados como writes auto-executáveis ou são invocados só como `Executor` do gate.
- [ ] 4.3 Confirmar ordem determinística `continuePendingApproval` → `ParseInbound` e limpeza determinística (confirma/cancela/ambíguo×2/TTL completam o run).
- [ ] 4.4 Confirmar `clearBudgetSession` no caminho do gate para `OperationBudgetCommit`.

## Detalhes de Implementação

Ver plano-fonte (Item 4) e Addendum R-AGENT-WF-001.7-A. Âncoras: `daily_ledger_agent.go` (`dispatchWrite` ~:263, `dispatchWriteDestructive` ~:662, `wireBudgetCommitGate` ~:608, `intentToOperationKind` ~:646, `finalizeConfirmResult` ~:829, `handleConfirmSuspended` ~:697, `clearBudgetSession` ~:720/843), `module.go` (executors `New*Executor` ~:555-560). Pré-condição: Tasks 1.0 (confirmEngine obrigatório) e 3.0.

## Critérios de Sucesso

- A 1ª mensagem de uma operação destrutiva sempre suspende aguardando confirmação (nunca executa).
- Não existe input que dispare delete/edit/card-delete/budget-commit sem passar pelo `confirm_gate`.
- Suites HITL verdes.

## Definition of Done (DoD)

1. Ramo de bypass sem confirmação removido.
2. Caminho único de execução para cada operação destrutiva/sensível.
3. `dispatchWriteDestructive`/`wireBudgetCommitGate` preservados como caminho canônico.
4. `ConfirmState` persistido antes da pergunta; resume antes do parse; sem run suspenso órfão.
5. `AwaitingApproval`/`OperationKind` tipos fechados; sem string solta. Build + suites HITL verdes.

## Critérios de Aceite (gates executáveis)

```bash
cd /Users/jailtonjunior/Git/mecontrola

grep -rn --exclude-dir=mocks --exclude="*_test.go" \
  'AwaitingApproval\s*=\s*"[^"]*"\|OperationKind\s*=\s*"[^"]*"' internal/agent/ \
  && echo "FAIL: estado HITL como string solta" || echo "OK"

f=internal/agent/application/services/daily_ledger_agent.go
grep -n "continuePendingApproval\|a.parser.Parse\|ParseInbound" "$f"   # continuePendingApproval ANTES do parse

cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
[ "${cases:-0}" -gt 1 ] && echo "FAIL: switch cresceu" || echo "OK"

grep -n "NewDeleteLastTransaction\|NewEditLastTransaction\|NewDeleteCard" \
  internal/agent/application/services/agent_workflows.go \
  && echo "REVISAR: confirmar que tools não mutam fora do destructive_confirm"

go build ./internal/agent/... \
  && go test ./internal/agent/application/services/ -run 'HITL' \
  && go test ./internal/agent/application/workflow/... -run 'Destructive|HITL'
```

## Skills Necessárias

- `go-implementation` — refatoração de Go de produção (Etapas 1–5 + checklist R0–R7).
- `mastra` — gate HITL, `AwaitingApproval`, `OperationKind`, resume antes do parse (R-AGENT-WF-001.7-A, ADR-002/003).

## Testes da Tarefa

- [ ] Testes unitários: 1ª msg suspende; confirma executa; cancela descarta; ambíguo×2 cancela; TTL expira → fall-through; replay não muta duas vezes.
- [ ] Testes de integração: nenhuma operação destrutiva efetiva sem passar pelo `confirm_gate`; budget commit limpa a sessão.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`.</critical>

## Arquivos Relevantes
- `internal/agent/application/services/daily_ledger_agent.go`
- `internal/agent/application/services/agent_workflows.go`
- `internal/agent/application/workflow/destructive_confirm.go`, `steps/` (HITL)
- `internal/agent/module.go` (executors)
- Suites `hitl_routing_test.go`, `hitl_budget_gate_test.go`, `destructive_confirm_test.go`, `hitl_decision_test.go`

# Tarefa 2.0: Consolidar pending step no snapshot do kernel e deduplicar helpers

<critical>Ler o plano-fonte `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md` (Item 2) e ADR-001 (`.specs/prd-agent-platform-evolution/adr-001-kernel-resume-merge-patch.md`) antes de iniciar.</critical>

## Visão Geral

Remover o caminho legacy de confirmação de pending step de categoria, tornando o `Snapshot.State` do kernel a **fonte única de verdade** no resume (ADR-001 / R-WF-KERNEL-001.7). Eliminar o side-store `PendingExpenseConfirmationGateway` quando deixar de ter consumidor real e deduplicar os helpers de match copiados verbatim entre `services/` e `steps/`.

<requirements>
- `continuePendingExpenseConfirmationLegacy` removida; resume de categoria ocorre exclusivamente via `Engine.Resume` sobre o snapshot.
- O fallback condicional kernel→legacy em `continuePendingExpenseConfirmation` é eliminado.
- Side-store de pending expense não é mais lido para reexecutar draft (viola ADR-001 se permanecer).
- Helpers `matchesExpenseConfirmation`, `matchesExpenseCancellation`, `matchCandidateByText`, `expenseCancelledText` com definição única (preferir `steps/`, sem cópia em `services/`).
- Pending step de categoria continua salvo via suspend do kernel (R-AGENT-WF-001.7), cobrindo `expense`, `income` e `card_purchase` (este via `ForceCategory`).
- Zero comentários em Go de produção (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 2.1 Remover `continuePendingExpenseConfirmationLegacy` e o ramo de fallback em `continuePendingExpenseConfirmation` (`daily_ledger_agent.go`).
- [ ] 2.2 Remover `resolvePendingCategoryConfirm`/`resolvePendingCategoryChoice`/`executePendingDraft`/`executePendingExpense`/`executePendingCardPurchase` que reidratam draft do side-store, confirmando que o `PersistFn` do kernel cobre os 3 `TransactionKind` (incl. `card_purchase` com `ForceCategory`).
- [ ] 2.3 Remover o campo `pendingExpenseConfirmation`, o adapter `NewPendingExpenseConfirmationAdapter` e a interface `PendingExpenseConfirmationGateway` se não houver outro consumidor; remover `clearPendingDraft` órfão.
- [ ] 2.4 Deduplicar os helpers de match: uma definição canônica em `steps/`, consumida por `services/` sem ciclo de import.

## Detalhes de Implementação

Ver plano-fonte (Item 2) e R-WF-KERNEL-001.7. Âncoras: `daily_ledger_agent.go` (`continuePendingExpenseConfirmation` ~:442, `...Legacy` ~:498, helpers ~:886-928), `internal/agent/application/workflow/steps/resolve_category.go` (helpers duplicados ~:133-174, `ForceCategory` ~:116/123), `tools/contracts.go` (`PendingExpenseConfirmationGateway` ~:235). Pré-condição: Task 1.0 garante kernel sempre-on.

## Critérios de Sucesso

- Resume de confirm/choice/cancel para `expense`, `income` e `card_purchase` funciona 100% via snapshot do kernel, sem `PendingExpenseConfirmationGateway`.
- Mensagem sem pending ativo continua chegando a `ParseInbound` (não some).
- `parity_test.go` e suites de pending/kernel verdes.

## Definition of Done (DoD)

1. `continuePendingExpenseConfirmationLegacy` e os executores de draft via side-store não existem mais.
2. O side-store de pending expense não é mais fonte de verdade no resume.
3. Helpers de match com definição única (sem duplicação services/↔steps/).
4. Card purchase via `ForceCategory` preservado (teste prova o ramo).
5. Build, vet e suites verdes; `parity_test.go` adaptado e verde.

## Critérios de Aceite (gates executáveis)

```bash
cd /Users/jailtonjunior/Git/mecontrola

grep -rn "continuePendingExpenseConfirmationLegacy\|resolvePendingCategoryConfirm\|resolvePendingCategoryChoice\|executePendingDraft" \
  internal/agent --include="*.go" --exclude="*_test.go" \
  && echo "FAIL: caminho legacy de pending step ainda presente" || echo "OK"

grep -n "pendingExpenseConfirmation.Load\|\.executePendingExpense\|\.executePendingCardPurchase" \
  internal/agent/application/services/daily_ledger_agent.go \
  && echo "FAIL: side-store ainda fonte de verdade no resume (viola ADR-001)" || echo "OK"

test "$(grep -rl 'func matchesExpenseConfirmation' internal/agent --include='*.go' --exclude='*_test.go' | wc -l | tr -d ' ')" = "1" \
  && echo "OK: definição única" || echo "FAIL: matchesExpenseConfirmation duplicado"

test "$(grep -rln 'expenseCancelledText =' internal/agent --include='*.go' --exclude='*_test.go' | wc -l | tr -d ' ')" = "1" \
  && echo "OK" || echo "FAIL: expenseCancelledText duplicado"

# R-WF-KERNEL-001.7 — merge-patch no resume preservado
grep -n "current = rs\|current = decoded\|current = resumed" internal/platform/workflow/engine.go \
  && echo "FAIL: resume substitui estado inteiro" || echo "OK"

go build ./internal/agent/... && go vet ./internal/agent/... \
  && go test ./internal/agent/application/services/ ./internal/agent/application/workflow/... ./internal/agent/e2e/...
```

## Skills Necessárias

- `go-implementation` — refatoração de Go de produção (Etapas 1–5 + checklist R0–R7).
- `mastra` — pending step / Thread→Run / resume são semântica exclusiva do `internal/agent` (R-AGENT-WF-001.7 / R-WF-KERNEL-001.7).

## Testes da Tarefa

- [ ] Testes unitários: resume de confirm/choice/cancel para os 3 `TransactionKind` via snapshot, com `PendingExpenseConfirmationGateway` ausente (nil).
- [ ] Testes de integração: ciclo suspend→resume end-to-end; mensagem sem pending segue para o parser.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`.</critical>

## Arquivos Relevantes
- `internal/agent/application/services/daily_ledger_agent.go`
- `internal/agent/application/workflow/steps/resolve_category.go`
- `internal/agent/application/tools/contracts.go`
- `internal/agent/module.go` (remoção do adapter de side-store)
- `internal/agent/application/services/parity_test.go` e suites de pending/kernel (adaptar)

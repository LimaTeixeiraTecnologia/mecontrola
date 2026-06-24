# ADR-006 — Kernel como Caminho Único; Remoção do Legacy de Resume e do Parity Test

## Metadados

- **Título:** Tornar o kernel o caminho único de escrita/resume e remover a coexistência legacy
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Plataforma
- **Relacionados:** PRD (RF-32, RF-36), techspec §"Kernel caminho único", `R-AGENT-WF-001`, `R-WF-KERNEL-001`

## Contexto

Durante a introdução do kernel, o agent manteve um caminho **legacy** coexistindo: flag
`kernelEnabled`, `continuePendingExpenseConfirmationLegacy`, setter morto `EnableKernel` (zero
callers) e `parity_test.go` (prova de equivalência legacy↔kernel). Hoje o legacy ainda é **fallback
ativo** em duas condições: (1) kernel desligado (`!kernelEnabled`); (2) kernel ligado mas
`Engine.Resume` retorna `RunID==uuid.Nil` (`daily_ledger_agent.go:458-460`), que é o sinal de "não há
run suspenso" (`engine.go:156-159`) — caso em que ainda pode existir um `pendingexpense.Draft` legacy.
Também há um **fallback morto** em `budget_tools.go` (else inalcançável quando a session está sempre
`Enabled()`), apoiado por `budget_configurator.go`.

## Decisão

Tornar o kernel o **caminho único** e remover o legacy, **após** satisfeitas as pré-condições:

- **PRÉ-1 (kernel sempre-on para writes):** **remover a flag
  `WorkflowKernelConfig.TransactionsWriteEnabled`** (decisão) e o gating associado;
  `attachKernel`/`attachBudgetConfigSession` deixam de ser opcionais — deps ausentes (session store,
  store factory, categories module) viram **falha de boot**, nunca fallback silencioso. Assim
  `dispatchWriteKernel` é o único produtor de suspensão de categoria.
- **PRÉ-2 (resume sempre acha run quando há espera):** com PRÉ-1, sempre que houver espera há snapshot
  kernel suspenso; o ramo `RunID==Nil` torna-se inalcançável.
- **PRÉ-3 (drenar drafts em voo):** expirar/migrar `pendingexpense.Draft` legacy em `agent_sessions`
  antes do deploy (housekeeping/TTL).
- **PRÉ-4 (paridade verde):** `parity_test` verde no momento da remoção (evidência de equivalência).

Itens removidos: `EnableKernel` (imediato, morto); `kernelEnabled` + guardas (trocar por
`a.kernelEngine != nil`); `continuePendingExpenseConfirmationLegacy` + helpers exclusivos + ramo
`:458-460`; `parity_test.go` + `runLegacy()` (migrando os casos de resume para suite kernel-only);
fallback morto de budget (`budget_tools.go` else, campo/param `BudgetConfigurator`, adapter
`budget_configurator.go` — preservando o usecase `StartBudgetConfiguration`, usado pelo onboarding).

## Alternativas Consideradas

- **Manter coexistência**: dívida permanente, dois caminhos a testar, risco de divergência. Rejeitada.
- **Remover legacy sem pré-condições**: deixaria espera de categoria órfã (usuário travado). Rejeitada
  — viola "0 gaps".

## Consequências

### Benefícios Esperados

- Um único caminho de escrita/resume; menos código, menos superfície de regressão; kernel como fonte
  única de verdade do estado suspenso (R-WF-KERNEL-001.7).

### Trade-offs e Custos

- Exige tornar kernel/session sempre-on e drenar drafts legacy antes do cutover.

### Riscos e Mitigações

- **Risco:** remoção antecipada deixa espera órfã. **Mitigação:** gates PRÉ-1..PRÉ-4; remover legacy e
  parity no **mesmo PR**, migrando casos de resume para suite kernel-only. **Rollback:** reverter PR.
- **Risco:** `StartBudgetConfiguration` removido por engano. **Mitigação:** apagar só o adapter, manter
  o usecase (confirmar uso pelo onboarding). **Gate de resíduo:** `deadcode`/`staticcheck`.

## Plano de Implementação

1. Remover `EnableKernel` (zero risco). 2. PRÉ-1 (sempre-on). 3. PRÉ-3 (drenar drafts). 4. Remover
   legacy + parity (migrar resume para kernel-only). 5. Remover fallback morto de budget. 6.
   `deadcode`/`staticcheck` + `go test -race`.

## Monitoramento e Validação

- Sucesso: suíte verde sem `parity_test`; `grep EnableKernel/continuePending*Legacy` vazio; resume
  durável coberto por suite kernel-only; nenhuma espera órfã em produção (métrica de runs suspensos
  drenados).

## Impacto em Documentação e Operação

- Runbook do agent; remover referências ao modo legacy; skill `mastra` (caminho único).

## Revisão Futura

- N/A após cutover; revisitar só se o kernel precisar de novo modo de execução.

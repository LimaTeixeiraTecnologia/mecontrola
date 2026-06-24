# Registro de Decisão Arquitetural (ADR-004)

## Metadados

- **Título:** Gate HITL de budget no ponto de commit (antes de `ActivateBudgetUC`)
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Solicitante (produto/eng) + plataforma
- **Relacionados:** `prd.md` (RF-08), `techspec.md`, ADR-002, ADR-003,
  `internal/agent/application/tools/budget_session.go`,
  `internal/agent/infrastructure/binding/budget_config.go`

## Contexto

A reconfiguração de budget já é um fluxo **multi-turn e stateful**: `BudgetSessionRunner` +
`budgetdraft.Draft` coletam alocações turno a turno (com cancelamento via "cancelar") e, ao atingir
100% (10000 basis points), **auto-commitam** chamando `CreateBudgetUC` + `ActivateBudgetUC` —
sobrescrevendo o budget vigente. O único passo realmente destrutivo é a **ativação** (sobrescrita do
budget ativo). Reescrever toda a sessão no kernel seria alto risco de regressão para um fluxo que já
funciona.

## Decisão

Inserir o gate HITL **exclusivamente no ponto de commit**: quando a sessão atinge 100% e estaria
prestes a chamar `ActivateBudgetUC`, em vez de ativar automaticamente, o fluxo **suspende** via o
workflow `destructive_confirm` (operação `OperationBudgetCommit`), exibindo a alocação final no
prompt de confirmação. A ativação só ocorre após **confirmação explícita**; cancelamento/expiração
descartam o commit **sem** sobrescrever o budget vigente. O draft de alocação completo viaja em
`ConfirmState.BudgetDraftJSON` (serializado), recuperado integralmente no resume graças ao merge-patch
(ADR-001).

A coleta multi-turn das alocações (antes dos 100%) permanece **inalterada** no `BudgetSessionRunner`
existente — o gate não embrulha a sessão inteira, apenas o commit.

## Alternativas Consideradas

- **Budget fora do HITL no MVP** (só delete/edit/card): menor superfície. **Rejeitada** pelo
  solicitante — a sobrescrita do budget é destrutiva e merece confirmação.
- **Gate em toda a sessão de budget** (migrar a sessão para o kernel): **Rejeitada** — alto risco de
  regressão num fluxo que já funciona e já tem cancelamento.

## Consequências

### Benefícios Esperados

- Protege o único passo destrutivo (ativação) sem reescrever o fluxo de coleta.
- Reuso 1:1 do mecanismo de confirmação dos demais (mesmo `confirm_gate`, mesma semântica ADR-003).
- Usuário vê a alocação final antes de sobrescrever a vigente.

### Trade-offs e Custos

- Dois mecanismos de estado coexistem no fluxo de budget: a sessão multi-turn (existente) até 100% e
  o snapshot do kernel no commit. A fronteira (handoff no commit) precisa ser clara e testada.

### Riscos e Mitigações

- **Risco:** handoff sessão→kernel no commit deixa estado inconsistente se falhar entre persistir o
  draft e suspender. **Mitigação:** o draft completo é serializado em `ConfirmState` no momento de
  Start do gate; a sessão só é considerada consumida após confirmação efetiva; cancelamento/expiração
  preservam o budget vigente (idempotência da ativação por `messageID`).
- **Risco:** dupla ativação sob replay. **Mitigação:** passo `replay` + idempotência por `messageID`
  (ADR-003).

## Plano de Implementação

1. Ajustar `BudgetSessionRunner` para, ao completar 100%, **iniciar** o gate `destructive_confirm`
   (`OperationBudgetCommit`) em vez de ativar direto.
2. `execute_destructive[OperationBudgetCommit]` chama `CreateBudgetUC` + `ActivateBudgetUC` (binding
   existente `BudgetConfigCommitterAdapter`).
3. Testes: confirma → ativa uma vez; cancela/expira → budget vigente intacto; replay → não reativa.
4. Adoção concluída quando os cenários de budget passam em E2E sem regressão da coleta multi-turn.

## Monitoramento e Validação

- `agent.hitl.*` com `operation="budget_commit"`.
- Critério de sucesso: 0 ativação de budget sem confirmação; coleta multi-turn inalterada (não
  regressão); 0 dupla ativação sob replay.

## Impacto em Documentação e Operação

- Runbook do agent: documentar o ponto de confirmação do budget e o comportamento de
  cancelamento/expiração.

## Revisão Futura

- Revisitar se a coleta de budget for futuramente migrada para o kernel (unificação de mecanismos),
  o que tornaria o handoff desnecessário.

# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Suspend/resume genérico; `pendingexpense.Draft` como estado do run do kernel
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Plataforma, dono do `internal/agent`
- **Relacionados:** PRD (RF-06, RF-15, RF-20, RF-23, RF-30); techspec (Resume; Decomposição do fluxo); ADR-002; `R-AGENT-WF-001.4/.7`

## Contexto

- Hoje o suspend/resume é bespoke e fora do workflow: `pendingexpense.Draft` é salvo em
  `agent_sessions.pending_action` e `continuePendingExpenseConfirmation` roda **antes** do
  `ParseInbound`, interpretando a resposta do usuário **sem LLM** (`resolvePendingCategoryChoice/
  Confirm`), limpando o draft após executar/cancelar.
- O PRD exige generalizar o suspend/resume no kernel e usar o `pendingexpense.Draft` como **primeiro
  consumidor** (prova de reuso), preservando o contrato (`AwaitingKind`/`TransactionKind` fechados) e a
  semântica "resume antes do parse".
- `R-AGENT-WF-001.4` proíbe LLM fora do parse; `R-AGENT-WF-001.7` exige salvar o estado de retomada ao
  clarificar.

## Decisão

- O kernel expõe suspend/resume genérico: um `Step[S]` pode retornar `StepOutput{Status: suspended,
  Suspend{Reason: SuspendAwaitingInput}}`; o engine grava `Snapshot{Status: suspended, Cursor}` e o
  `Engine.Resume(def, key, resumeBytes)` reentra no passo suspenso com o estado mesclado ao input.
- `SuspendReason` é **genérico** (`AwaitingInput`); a especificidade de domínio (`category_confirm`/
  `category_choice`) permanece no **estado** `S` do agent — o kernel não conhece categoria.
- O estado serializado `S` do workflow de transactions write **é** o `pendingexpense.Draft` (mais
  campos de replay), tornando o draft o estado do run suspenso (`workflow_runs.state`).
  `agent_sessions.pending_action` deixa de ser a fonte do draft (migração de leitura abaixo).
- A interpretação da resposta (escolha/confirmação/cancelamento) **sem LLM** migra para o handler de
  resume do passo `resolve_category` (lógica portada de `resolvePendingCategoryChoice/Confirm`).
- "Resume antes do parse" é preservado: `continuePendingExpenseConfirmation` (sob flag) chama
  `Engine.Resume` antes do `ParseInbound`; ausência de run suspenso ⇒ segue para o parse (igual hoje).

## Alternativas Consideradas

- **Manter bespoke; kernel só para fluxos novos**: duplica o conceito de suspend/resume; rejeitada
  pelo PRD (queria prova de reuso real).
- **Suspend/resume fora de escopo**: kernel sem retomada; rejeitada (reduz "production-proof").
- **`SuspendReason` carregando o awaiting kind de domínio**: vazaria domínio para o kernel; rejeitada
  (viola R-WF-KERNEL-001).

## Consequências

### Benefícios Esperados

- Prova concreta de reuso do suspend/resume genérico.
- Estado de retomada durável e idempotente (ADR-002), mais robusto que o atual (que não auditava o
  write de resume).
- LLM permanece restrito ao parse (R-AGENT-WF-001.4).

### Trade-offs e Custos

- Migração da fonte do draft (`agent_sessions` → `workflow_runs`) exige janela de drenagem.
- Reentrada no passo suspenso exige que o passo seja idempotente quanto à resolução forçada de categoria.

### Riscos e Mitigações

- **Risco:** drafts pendentes pré-cutover não existem em `workflow_runs`. **Mitigação:** sob flag,
  `continuePendingExpense...` consulta primeiro o `Store`; se vazio, faz fallback de leitura ao
  `agent_sessions.pending_action` legado por uma janela de drenagem; sem perda de estado.
- **Risco:** divergência de comportamento na interpretação de resposta. **Mitigação:** suíte de não
  regressão cobrindo choice/confirm/cancel idênticos ao atual.
- **Rollback:** flag off retorna ao caminho atual (draft em `agent_sessions`).

## Plano de Implementação

1. `ExpenseState` espelhando `pendingexpense.Draft` (round-trip `Encode/Decode` testado).
2. Passo `resolve_category` com branch (auto/ambiguous/confirm) e handler de resume sem LLM.
3. Integração de `Engine.Resume` em `continuePendingExpenseConfirmation` sob flag + fallback de leitura.
4. Conclusão: ciclo ambiguous→escolha→persistência e needs_confirm→confirm/cancel idênticos ao atual.

## Monitoramento e Validação

- `workflow_suspend_total{workflow,reason}`, `workflow_resume_total{workflow,result}`.
- Sucesso: paridade comportamental (reply/outcome) entre caminho atual e kernel; 0 LLM fora do parse.
- Revisar se a janela de drenagem do draft legado puder ser encerrada.

## Impacto em Documentação e Operação

- Skill `mastra` (`references/pending-step.md`): atualizar para refletir o draft como estado do run.
- Runbook do kernel: como inspecionar/limpar um run suspenso.

## Revisão Futura

- Estender para suspensão aninhada (dentro de `Branch`/`Parallel`) — fast-follow além do MVP.
- Encerrar fallback de leitura ao `agent_sessions` após a janela de drenagem.

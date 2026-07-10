# Registro de Decisão Arquitetural (ADR-005)

## Metadados

- **Título:** Loop de cartões um-por-vez via re-suspensão no mesmo cursor, sem limite, com reaper dedicado de onboarding
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da feature, usuário (decisões D-05, D-12)
- **Relacionados:** PRD (RF-26..RF-31a), techspec.md, ADR-004

## Contexto

O step de cartões atual cadastra **um único cartão numa passagem** (`onboarding_workflow.go:659-711`)
e é posicionado antes da metodologia. O novo fluxo exige cadastro de cartões **após a ativação**, um
por vez, repetindo a pergunta até a cliente recusar (RF-27..RF-29), sem limite máximo (D-05), sem
cartão parcial (RF-30) e sem desfazer o orçamento ativado.

Semântica do kernel: um step que retorna `StepStatusSuspended` mantém o `snap.Cursor` no seu índice
(`engine.go:331-333`) e é re-executado no resume (`engine.go:306`). Isso permite um loop natural: o
step de cartões re-suspende após cada cadastro e só completa quando a cliente diz "não".

## Decisão

Reescrever `BuildCardsStep` como **loop por re-suspensão no mesmo cursor**:

- Primeira entrada (`ResumeText==""`): `ListCards` (contagem para o texto) → suspende
  (`cardsPrompt`).
- Resume: extrai cartão (LLM). `wantsCard=false` → `CardsDone=true` → completa (avança para conclusão).
  `wantsCard=true` com dados inválidos (`DecideCardEntry`) → re-suspende (`cardsReprompt`) **sem criar
  cartão parcial** (RF-30). `wantsCard=true` válido → `CreateCard` → re-suspende (`cardsPrompt`)
  perguntando por outro cartão (D-05, sem limite).

O orçamento ativado não é tocado neste step (RF-30). Posicionado após `recurrence` (ADR-004).

**Reaper dedicado (D-12):** como o loop não tem limite e o onboarding não possuía reaper (diferente de
confirm/cardCreate/pendingEntry/budgetCreation em `module.go`), adicionar em `module.go` um
`workflow.NewStaleSuspendedReaper(store, OnboardingWorkflowID, 7*24h, 100, o11y)` com TTL generoso de
7 dias, no molde dos reapers existentes. Runs abandonados (usuário some no meio do loop ou de qualquer
suspend) são encerrados após 7 dias; quem retoma dentro da janela continua normalmente.

## Alternativas Consideradas

- **Teto de segurança (ex.: 10 cartões).** Vantagem: proteção contra loop longo acidental.
  Desvantagem: limite arbitrário não pedido pela US. Rejeitada (D-05).
- **Sem reaper de onboarding (status quo).** Vantagem: sem mudança em `module.go`. Desvantagem: runs
  abandonados persistem suspensos indefinidamente; inconsistente com os demais workflows que têm
  reaper. Rejeitada (D-12): adicionar reaper com TTL 7 dias.
- **Contador de cartões no estado em vez de `ListCards`.** Vantagem: evita IO por iteração.
  Desvantagem: fonte de verdade duplicada. Rejeitada: `ListCards` já é chamado e é autoritativo; custo
  desprezível.

## Consequências

### Benefícios Esperados

- Cadastro de múltiplos cartões um-por-vez (M-03, RF-29).
- Reaproveita a semântica de cursor do kernel sem novo combinator.
- Orçamento ativado preservado (RF-30).

### Trade-offs e Custos

- Sem limite: loop longo depende do usuário recusar; cada cartão exige turno próprio (RF-31).
- `ListCards` a cada iteração (IO leve).

### Riscos e Mitigações

- **Risco:** run suspenso indefinidamente (usuário abandona). **Mitigação:** reaper de suspensos do
  kernel (`workflow.NewStaleSuspendedReaper`, análogo a `module.go` para outros workflows) encerra
  runs abandonados; nenhum orçamento é afetado.
- **Risco:** `CreateCard`/`ListCards` falhar. **Mitigação:** `failStep` tipado (sem falso sucesso).
- **Rollback:** voltar ao step de cartão único (perde múltiplos cartões).

## Plano de Implementação

1. Reescrever `BuildCardsStep` com o loop de re-suspensão.
2. Remover do estado `CardNickname`/`CardDueDay` (mortos após remover cartão do resumo).
3. Adicionar o reaper de onboarding em `module.go` (D-12).
4. Testes: recusa imediata; um cartão; dois cartões; cartão inválido mantém loop sem criar parcial.

Concluído quando: é possível cadastrar 2+ cartões um-por-vez e encerrar com "não"; cartão inválido
não é criado e não desfaz o orçamento.

## Monitoramento e Validação

- `workflow_steps_total{step="step-cards",status}` e `workflow_suspend_total`.
- Teste de step multi-cartão + gate real-LLM (card ≥ 0,90).

## Impacto em Documentação e Operação

- Novo reaper de onboarding (TTL 7 dias) em `module.go`; documentar o TTL no runbook de onboarding e
  monitorar `workflow` runs encerrados pelo reaper.

## Revisão Futura

Revisar se surgir necessidade de limite máximo de cartões ou de cadastro em lote.

# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Criação conversacional de orçamento via Tool fina `create_budget` que inicia workflow durável HITL `budget-creation`
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma / agente financeiro
- **Relacionados:** PRD `.specs/prd-orcamento-retroativo-conversacional-e-mes-por-extenso/prd.md` (RF-01..RF-12, RF-25..RF-29), techspec.md, R-AGENT-WF-001, R-WF-KERNEL-001

## Contexto

O agente oferece criar orçamento por conversa, mas não existe tool/fluxo que persista. Na confirmação, o LLM recorre a `adjust_allocation` sobre uma competência inexistente (`EditCategoryPercentage` retorna `ErrBudgetNotFound`, `edit_category_percentage.go:75`); o run fecha `failed/usecaseError` (`runtime.go:175-238`, `agent.go:142`) e o consumer entrega o `fallbackReply` genérico (`whatsapp_inbound_consumer.go:263`). Confirmado por trace `3f4c0b8c…` (span `budgets.usecase.edit_category_percentage` no run falho de 18:09:55).

A porta `BudgetPlanner.CreateBudget(ctx, DraftBudget)` e `ActivateBudget(ctx, userID, competence)` já existem (`budget_planner.go:10,12`), com adapter mapeando DTOs (`budget_planner_adapter.go:53-112`), hoje consumidas só pelo onboarding (`onboarding_workflow.go` ~755/766/817). O substrato de workflow durável com HITL (`workflow.Engine[S].Start/Resume`, `Codec.MergePatch` RFC 7386, `StaleSuspendedReaper`, `ConfirmState`/`PendingEntryState` + Continuer no `try*` do consumer) está pronto para reuso.

A criação exige coleta multi-turno (total → distribuição por categoria até 100% → confirmação) com confirmação humana explícita antes de persistir e limpeza determinística — um caso HITL clássico.

## Decisão

1. Adicionar Tool fina `create_budget` (`internal/agents/application/tools/create_budget.go`) que **não persiste diretamente**: resolve a competência (ADR-002) e **inicia** o workflow durável `budget-creation` via `engine.Start`, retornando a primeira pergunta (total ou distribuição). Espelha o padrão das tools destrutivas que iniciam `destructive-confirm`.
2. Criar workflow `budget-creation` (`workflow.Definition[BudgetCreationState]`, `Durable:true`, `MaxAttempts:1`) com um `StepFunc` root que ramifica sobre o tipo fechado `BudgetAwaitingSlot` (`AwaitingBudgetTotal`, `AwaitingBudgetDistribution`, `AwaitingBudgetConfirm`), persistindo o snapshot antes de cada pergunta e resumindo por merge-patch antes do parse. **A coleta de total e distribuição espelha os steps do onboarding** (`BuildIncomeStep`, `BuildMethodologyStep`): cada step, no resume, invoca `a.Execute(ctx, Request{Schema: strict})` para extrair o valor estruturado (call-site sancionado R-AGENT-WF-001.4 #2, precedente do onboarding) e valida com um `Decide*` puro (`DecideIncomeCents`, `DecideAllocationsBP`), reprompt em falha. A distribuição oferece o default `_defaultDistributionBP` (40/10/10/10/30) para aceitar ou customizar — não reaproveita perfil do usuário (D1). A escrita+ativação ocorre no slot de confirmação (gate `isSim`/`isNao` determinístico), delegando a `planner.CreateBudget` + `planner.ActivateBudget` (orquestração, sem regra/SQL/branching de domínio na tool nem no consumer). `agent.Agent` é injetado no `BuildBudgetCreationWorkflow`.
3. Inserir `BudgetCreationContinuer` na cadeia `try*` do `WhatsAppInboundConsumer`, **antes** do agente/ParseInbound, para capturar os turnos seguintes de um run suspenso (Load→Resume; senão não-handled). Espelha `PendingEntryContinuer`/`DestructiveConfirmContinuer`. Chave do run por `resourceId` (o Continuer não conhece a competência no próximo inbound), com exclusão mútua: um único estado de espera ativo por `resourceId`. TTL 30min (avaliado no resume); reaper dedicado com `staleAfter` 35min.
4. Instanciar um `StaleSuspendedReaper` dedicado ao workflow `budget-creation`. `create_budget` **não** entra no `agent.WithWriteToolSet(...)` — é starter de workflow (como `delete_entry`/`update_card`), não write direto; a persistência ocorre no slot de confirmação. Idempotência garantida pela chave do run (`resourceId`) + detecção de replay do `messageID` no gate de confirmação + unicidade `(user_id, competence)`.
5. Unicidade: no slot de confirmação, `ErrBudgetConflict` (constraint `budgets_user_comp_uk`) é mapeado para mensagem "já existe" e encerra sem duplicar/ativar (cobre draft de mês futuro — RF-11/RF-12).

## Alternativas Consideradas

- **Tool persiste em um único disparo (sem workflow):** simples, mas perde HITL durável, coleta multi-turno e resume-antes-do-parse exigidos (RF-02/RF-06/RF-07); reintroduz o risco de persistir sem confirmação. Rejeitada.
- **Diálogo conduzido livremente pelo LLM (estado no histórico da thread):** é exatamente o comportamento atual que falhou (estado não durável, sem gate determinístico de confirmação, run órfão). Rejeitada por violar R-AGENT-WF-001.7.
- **Novo combinador `Sequence`/`Branch` no root do workflow:** o repo consolidou o padrão de `StepFunc` único com ramificação por `Awaiting` (pending-entry/confirm); adotar combinadores aqui divergiria do padrão vigente sem ganho. Rejeitada por consistência.

## Consequências

### Benefícios Esperados

- Elimina a promessa quebrada (oferecer sem executar) e o `failed/usecaseError` deste caminho.
- Reuso integral do kernel: sem código novo de durabilidade/merge-patch/reaper.
- Confirmação humana explícita, estado durável e limpeza determinística garantidos por construção.
- Passa a existir a série `agent_tool_invocations_total{tool="create_budget"}`.

### Trade-offs e Custos

- +1 workflow, +1 continuer, +1 reaper/job e wiring no `module.go`.
- Necessidade de exclusão mútua entre estados de espera (budget-creation vs pending-entry vs confirm): um por vez por `resourceId`.

### Riscos e Mitigações

- **Precedência no `try*`:** run suspenso deve capturar o inbound antes do agente → `tryBudgetCreation` antes do agente; testes de integração.
- **Merge-patch de `Allocations map`:** resume parcial não pode zerar a distribuição acumulada → estado autoritativo no snapshot; teste de resume `{"ResumeText":"..."}`.
- **Run órfão:** reaper dedicado + encerramento determinístico no step. Rollback: remover a tool do registro e do write set desativa o caminho sem afetar os demais.

## Plano de Implementação

1. Estado + decisões puras do workflow (`BudgetCreationState`, `BudgetAwaitingSlot`, `Decide*`).
2. `BuildBudgetCreationWorkflow` + `BuildBudgetCreationReaper` + `BudgetCreationContinuer`.
3. Tool `create_budget` + DTO `Validate()`.
4. Wiring `module.go` + `tryBudgetCreation` no consumer.
5. Testes unit + integração (Postgres) + E2E real-LLM.

## Monitoramento e Validação

- `agent_tool_invocations_total{tool="create_budget"}` > 0; `agent_runs_total{status="failed"}` deste caminho → 0.
- Contador do reaper `budget-creation` sem acúmulo anômalo de expirações.
- Gate real-LLM ≥ 0.90 nos cenários de criação/retroativo. Reverter se o gate não fechar após ajuste de instrução.

## Impacto em Documentação e Operação

- Runbook do agente: novo workflow `budget-creation`, TTL e reaper.
- Observabilidade: painel de runs por workflow inclui `budget-creation`.

## Revisão Futura

- Revisar se surgir edição/exclusão conversacional de orçamento (hoje fora de escopo) ou reaproveitamento de distribuição entre meses.

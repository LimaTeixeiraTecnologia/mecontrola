# Registro de Decisão Arquitetural (ADR)

## Metadados
- **Título:** Chave de correlação por `resourceID` + guarda de exclusividade "um fluxo de orçamento por recurso" + pré-checagem de existência/auto-draft
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Plataforma / autor da techspec
- **Relacionados:** PRD (RF-05, RF-09..RF-13, RF-33, R2, R6); techspec.md; ADR-002; `budget_creation_workflow.go`

## Contexto
Dois pontos precisam ser resolvidos com precisão para evitar defeitos:

1. **Chave do workflow.** O `budget-creation` usa chave = `resourceID` (sem competência), porque o continuer não conhece a competência no inbound seguinte (a competência vive no `State`). Um molde inicial propôs `BudgetEditKey(resourceID, competence)` na tool mas `BudgetEditKey(resourceID, "")` no continuer — inconsistência que quebraria o resume.
2. **Exclusividade de fluxo (R6).** O PRD (R6/RF-33) exige "um fluxo de orçamento por vez por recurso". Verificação adversarial no consumidor mostrou que a `tryResumeChain` (`whatsapp_inbound_consumer.go:198-201`) itera os resumers e **curto-circuita no primeiro `handled`**: enquanto qualquer fluxo está suspenso, seu resumer consome o inbound **antes** de o agente ser invocado. Como iniciar uma edição exige o agente chamar a tool `edit_budget`, é impossível iniciar um segundo fluxo enquanto outro está suspenso. Logo, um scan explícito cross-workflow no store seria **código morto**.
3. **Existência/estado (R2/RF-10/RF-13).** Editar exige orçamento existente e não-vazio; auto-draft vazio deve rotear para criação.

## Decisão
1. **Chave = `resourceID`:** `BudgetEditKey(resourceID) = resourceID + ":budget-edit"`, idêntico em tool e continuer. A competência é resolvida na tool e gravada em `State.Competence`; o resume por merge-patch nunca depende da competência na chave.
2. **Exclusividade por mecanismo existente (sem guarda nova):** a exclusividade é garantida por dois mecanismos já presentes, sem scan cross-workflow adicional:
   - **Curto-circuito da `tryResumeChain`** (`whatsapp_inbound_consumer.go:198-201`): um fluxo suspenso intercepta o inbound antes do agente, impedindo o início de um segundo fluxo enquanto o primeiro está suspenso. O `tryContinueBudgetEdit` é adicionado à cadeia (após `tryContinueBudgetCreation`, antes de `tryResolveOnboarding`).
   - **`ErrRunAlreadyExists` do kernel** (mesmo `(Workflow, CorrelationKey)`): cobre a corrida de dois inbounds concorrentes que passem a cadeia antes de qualquer suspensão — o segundo `engine.Start` falha e a tool retorna outcome `pending_flow_exists`.
   Não há scan explícito de outros workflows (seria código morto). Registrado após verificação adversarial do consumidor.
3. **Pré-checagem de existência/estado:** a tool chama `planner.GetMonthlySummary(competence)`:
   - `ErrBudgetNotFound` → outcome `offer_create` (agente oferece `create_budget`, RF-10).
   - Orçamento `AutoDraft` sem alocações (vazio) → outcome `offer_create` (rotear para criação, R2/RF-13).
   - Caso contrário → inicia o workflow com `CurrentTotalCents` preenchido.

## Alternativas Consideradas
- **Chave com competência:** permitiria fluxos paralelos por mês, mas quebra o resume (continuer sem competência) e contraria R6. Rejeitada.
- **Scan explícito cross-workflow no store** antes do `Start` (varrer `budget-creation` + `budget-edit`): rejeitada por ser **código morto** — a `tryResumeChain` já impede iniciar um segundo fluxo enquanto outro está suspenso (curto-circuito no primeiro `handled`).
- **Chave/namespace compartilhado entre create e edit** (mesmo `Workflow`): o kernel usa `Workflow` na unicidade e no roteamento do reaper/resume; unificar confundiria os dois fluxos. Rejeitada.
- **Deixar a exclusividade para o LLM/prompt:** não determinístico. Rejeitada.

## Consequências
### Benefícios Esperados
- Resume consistente (tool e continuer usam a mesma chave).
- No máximo um fluxo de orçamento ativo por recurso (R6), sem estados concorrentes.
- Editar sobre inexistente/auto-draft vazio é redirecionado à criação (R2/RF-10/RF-13).

### Trade-offs e Custos
- Nenhum scan extra por início de edição — a exclusividade reusa mecanismos existentes (zero código morto).

### Riscos e Mitigações
- Risco: corrida entre dois inbounds concorrentes iniciando edição antes de qualquer suspensão. Mitigação: `ErrRunAlreadyExists` do kernel (mesmo `(Workflow, CorrelationKey)`) → o segundo `Start` falha e a tool retorna `pending_flow_exists`.
- Risco: inbound não relacionado durante edição suspensa é tratado como resposta do fluxo. Mitigação: comportamento herdado do `budget-creation` (TTL 30min + cancelamento explícito escapam) — não é regressão.

## Plano de Implementação
1. `BudgetEditKey(resourceID)` único (idêntico em tool e continuer).
2. Adicionar `tryContinueBudgetEdit` à `tryResumeChain` após `tryContinueBudgetCreation`.
3. Tratar `ErrRunAlreadyExists` no `edit_budget` → outcome `pending_flow_exists`.
4. Pré-checagem `GetMonthlySummary` (not-found / auto-draft vazio → `offer_create`).
5. Testes de integração: race de duplo-start → `pending_flow_exists`; not-found e auto-draft vazio → `offer_create`.

## Monitoramento e Validação
- Métrica `agents_budget_edit_total{outcome="pending_flow_exists"|"offer_create"}`.
- Teste: duplo-start concorrente retorna pendência; iniciar sobre auto-draft vazio oferece criação.

## Impacto em Documentação e Operação
- Documentar a regra de exclusividade no runbook do agente.

## Revisão Futura
- Revisitar se o produto passar a permitir múltiplos orçamentos simultâneos em edição (não previsto).

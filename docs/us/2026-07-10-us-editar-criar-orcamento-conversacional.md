# Editar Orçamento por Conversa (WhatsApp) — User Story única, pronta para desenvolvimento

> Fonte: pedido do usuário ("criar/editar orçamento — módulos `internal/agents` e `internal/budgets`; analisar possibilidade de workflow; uma única US robusta pronta para desenvolvimento") + confronto direto com a base de código.
> Objetivo: permitir que o assinante edite seu orçamento mensal inteiramente por conversa no WhatsApp — alterar o total, ajustar a porcentagem de uma categoria ou refazer a distribuição inteira — sempre com confirmação humana antes de aplicar, cobrindo cada caminho de conversa que o runtime suporta.
> Data de geração: 2026-07-10
> Nome do arquivo: `2026-07-10-us-editar-criar-orcamento-conversacional.md`
> Módulo Go: `github.com/LimaTeixeiraTecnologia/mecontrola`

---

## Decisões Confirmadas (rodada de múltipla escolha, 2026-07-10)

| # | Decisão | Escolha |
|---|---------|---------|
| D-01 | Operações de edição | Ajustar % de 1 categoria **+** editar valor total **+** refazer distribuição inteira (exclui excluir/resetar) |
| D-02 | Confirmação (HITL) | **Workflow durável com confirmação** ("sim/não" antes de aplicar), espelhando a criação |
| D-03 | Criar orçamento | Já implementado — tratado como **baseline documentado**; esta US concentra-se na edição |
| D-04 | Estado alvo | **Ativo e Draft** (hoje `EditCategoryPercentage` só aceita Ativo) |
| D-05 | Cobertura de conversa | Enumerar **cada** possibilidade de conversa suportada pelo runtime, sem inventar respostas do bot |

---

## Confronto com o Codebase

### Persona e canal
Assinante do MeControla que conversa por texto no WhatsApp. Inbound entra por `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`, é roteado pelo `AgentRuntime` (`internal/platform/agent/runtime.go`); identidade via `agent.InboundIdentityFromContext()`; escrita idempotente por `wamid`.

### Baseline já implementado (criação — não é escopo de desenvolvimento desta US)
- Tool `create_budget` (`internal/agents/application/tools/create_budget.go:90`) inicia o workflow durável `budget-creation`.
- Workflow `internal/agents/application/workflows/budget_creation_workflow.go`; estado fechado `BudgetCreationState` (`budget_creation_state.go:96-110`); slots `AwaitingBudgetTotal → AwaitingBudgetDistribution → AwaitingBudgetConfirm`.
- HITL real: `budget_creation_workflow.go:232` ("Posso ativar seu orçamento com esses dados? Responda \"sim\" para confirmar ou \"não\" para cancelar."), sucesso `:323`, cancelamento `:253`/`:269`, reprompt `:265`.
- Resolução de mês por `budgetsvo.MonthReference` + `DecideCompetence(ref, now)` (`internal/budgets/domain/valueobjects/month_reference.go:114-144`).
- TTL 30min / reaper 35min (`BudgetCreationStaleAfter`, `BuildBudgetCreationReaper`); replay por `state.MessageID`.

### Estado atual da edição (parcial, sem confirmação)
- Tool `adjust_allocation` (`internal/agents/application/tools/adjust_allocation.go:58`) → `planner.EditCategoryPercentage()` → use case `internal/budgets/application/usecases/edit_category_percentage.go`.
- Ajusta a porcentagem de **uma** categoria e rebalanceia as demais por `DecideEditCategoryPercentage` (`internal/budgets/domain/services/category_percentage_workflow.go:21-74`), soma mantida em 10000 basis points.
- `adjust_allocation` **aplica na hora, sem confirmação**, sem resposta natural própria (retorna `OK bool`; narração fica com o LLM).
- `EditCategoryPercentage` exige orçamento **Ativo** (retorna `ErrBudgetNotActive`).

### Gaps confirmados (escopo de desenvolvimento desta US)
| Gap | Evidência da ausência |
|-----|-----------------------|
| G-01 | Sem use case/tool para **editar o valor total** de orçamento existente; `entities/budget.go` não expõe alteração de `total_cents` para Ativo. |
| G-02 | Sem use case/tool para **refazer a distribuição inteira** de orçamento existente (só há ajuste de 1 categoria e o distribuidor usado na ativação). |
| G-03 | **Edição sem confirmação**: `adjust_allocation` aplica imediatamente; não há workflow HITL de edição. |
| G-04 | **Edição de Draft** bloqueada por `ErrBudgetNotActive`. |
| G-05 | Sem TTL/reaper dedicado para um workflow de edição. |

### Análise de possibilidade de Workflow (pedido explícito)
**Veredito: SIM — a edição deve ser um workflow durável, reaproveitando o substrato provado do `budget-creation`.**
- O kernel `internal/platform/workflow` já oferece `Engine[S].Start/Resume`, `Store`, `Snapshot`, suspend/resume e `Codec.MergePatch` (RFC 7386) — `engine.go`, `step.go`, `codec.go`, `store.go`.
- O padrão "um `Step[S]` com switch em `state.Awaiting`, suspende pedindo input, retoma por merge-patch `{"resumeText": ...}`" já vive em `budget_creation_workflow.go` e é replicável.
- HITL, TTL, reaper, replay por `messageID` e resolução de mês são primitivos consumíveis (regra de ouro `mastra`: consumir `internal/platform/{agent,memory,workflow,tool,scorer}`, não reimplementar).
- Desenho recomendado: **um workflow de edição unificado** `budget-edit` com estado fechado e slot `Awaiting ∈ {operation, value, confirm}`. A granularidade final (unificado vs. definições irmãs) é decisão de techspec; ambas cabem no kernel sem mudança de plataforma.
- Restrições respeitadas (R-WF-KERNEL-001 / R-AGENT-WF-001): regra de domínio em `Decide*` puro de `internal/budgets/domain/services`; tools/steps finos; estados fechados; SQL só no adapter Postgres; roteamento por registry, sem `switch case intent.Kind`.

---

## Matriz de Possibilidades Conversacionais (exaustiva, sem inventar respostas)

Respostas **existentes** citadas com `arquivo:linha`; respostas de **fluxos novos** descritas por comportamento e redigidas na implementação reaproveitando o tom da criação (não especificadas aqui).

**A. Entrada e intenção**
- A1 Pedido vago ("quero mexer no orçamento") → agente desambigua a operação (copy nova).
- A2 Pedido específico ("aumenta prazeres pra 20%", "muda o total pra 4 mil", "refaz a distribuição") → roteia direto.

**B. Resolução de competência (`DecideCompetence`, `month_reference.go:120-144`)**
- B1 "deste mês" → current. B2 "mês passado" → previous. B3 "próximo mês" → next.
- B4 "junho de 2026"/"2026-06" → explicit. B5 "junho" (sem ano) → `ClarifyMissingYear` → pergunta o ano. B6 mês irreconhecível → `ClarifyUnrecognized` → pede reformular.

**C. Existência e estado**
- C1 Ativo existe → edita. C2 Draft existe → edita (D-04; hoje bloqueado). C3 Não existe → oferece criar: "Você ainda não tem um orçamento para *<mês>*. Posso te ajudar a criar um?" (`competence_reference.go:50`).

**D. Coleta do novo valor**
- D1 Total: extração LLM de "R$ 3.500,00"/"três mil e quinhentos"/"4 mil reais"/"2000" (gate `budget_creation_workflow_real_llm_test.go`); ≤ 0 ou não identificado → reprompt (análogo `:97`).
- D2 Categoria: nome → `RootSlug` (5 slugs, `root_slug.go:10-34`); porcentagem 0–100 (`edit_category_percentage.go`); fora do range → reprompt; categoria irreconhecível → clarifica.
- D3 Distribuição: modos `confirm`/`percent`/`reais` (`DecideAllocationKind`/`DecideAllocationsBP`); soma ≠ 100%/total → reprompt ("Ops, não consegui aplicar essa distribuição: …", `:158`).

**E. Confirmação (HITL) — obrigatória (D-02)**
- E1 "sim"/"confirmar"/"ok" → aplica e responde com o resumo resultante.
- E2 "não"/"cancelar" → descarta sem efeito (análogo `:253`).
- E3 Ambígua (1ª vez) → reprompt único (análogo `:265`; `RepromptCount` 0→1).
- E4 Ambígua (2ª vez) → cancela (análogo `:269`; `budgetCreationMaxReprompts = 1`).

**F. Robustez de runtime**
- F1 > 30 min → expira sem efeito; texto segue para `ParseInbound`.
- F2 `messageID`/`wamid` repetido → não reaplica (write ledger `(wamid, itemSeq, operation)`, `idempotent_write.go`).
- F3 Fluxo já pendente → informa pendência (`ErrRunAlreadyExists`).
- F4 Planner indisponível → mensagem específica, sem falso sucesso (`ErrBudgetPlannerUnavailable`).
- F5 Ajuste de 1 categoria → demais rebalanceadas proporcionalmente, soma = 10000.

---

## US-01 — Editar o orçamento mensal por conversa (total, categoria ou distribuição), com confirmação

## Declaração
Como assinante do MeControla no WhatsApp, quero editar meu orçamento mensal por conversa — alterar o valor total, ajustar a porcentagem de uma categoria ou refazer a distribuição inteira — sempre confirmando antes de aplicar, para manter meu plano correto e atualizado sem risco de mudança acidental e sem precisar recriar o orçamento.

## Contexto
- Problema: hoje só existe ajuste de uma categoria, imediato e sem confirmação, e apenas sobre orçamento Ativo; não há caminho para mudar o total nem para refazer a distribuição, e Draft não pode ser editado.
- Resultado esperado: um workflow durável de edição (`budget-edit`) que resolve a competência, identifica a operação, coleta o novo valor, mostra o resumo do impacto, pede confirmação "sim/não" e só então aplica — funcionando sobre Ativo e Draft, idempotente e com expiração segura.
- Fonte: pedido do usuário + decisões D-01..D-05 + confronto com a base de código (gaps G-01..G-05).

## Regras de Negócio
- Operações suportadas: (a) editar total; (b) ajustar % de uma categoria; (c) refazer distribuição inteira. Excluir/resetar está fora de escopo.
- Toda mutação exige confirmação humana explícita ("sim") num passo de confirmação durável; "não" descarta sem efeito (D-02).
- Editar total: novo total > 0; basis points por categoria preservados; planejado por categoria recalculado por `AllocationDistributor.Distribute` (half-even, ordem canônica).
- Ajustar categoria: porcentagem 0–100; demais categorias rebalanceadas proporcionalmente por `DecideEditCategoryPercentage`, soma mantida em 10000.
- Refazer distribuição: três modos (confirmar padrão / percentual soma 100% / reais soma total); total do orçamento é preservado.
- Estado alvo: Ativo e Draft; editar Draft não ativa o orçamento (permanece Draft); as mesmas invariantes valem para ambos (D-04).
- Competência resolvida por `DecideCompetence`; ausência de ano ou mês irreconhecível gera clarificação antes da coleta.
- Se não houver orçamento na competência, o agente oferece criar em vez de editar.
- Robustez: TTL 30 min, reaper dedicado, replay idempotente por `wamid`, reprompt único, falha-segura sem falso sucesso.
- Regra de domínio (recálculo, invariantes, rebalanceamento) vive em `Decide*` puro de `internal/budgets`; tool e step são adapters finos; estados são tipos fechados; roteamento por registry (R-AGENT-WF-001 / R-WF-KERNEL-001).

## Critérios de Aceite
```gherkin
Cenário: Editar o total, confirmado, em orçamento ativo
  Dado que o assinante tem orçamento ativo no mês corrente
  Quando ele pede para mudar o total para "4 mil" e o agente mostra o novo total e a distribuição recalculada
  E o assinante responde "sim"
  Então o total é atualizado, o planejado por categoria é recalculado mantendo os percentuais e o agente confirma o novo orçamento

Cenário: Ajustar a porcentagem de uma categoria, confirmado
  Dado que o assinante tem orçamento ativo com cinco categorias
  Quando ele pede "coloca prazeres em 20%" e o agente mostra o rebalanceamento das demais categorias
  E o assinante responde "sim"
  Então a categoria alvo passa a 20% e as demais são reajustadas proporcionalmente com soma total de 100%

Cenário: Refazer a distribuição inteira por percentual, confirmado
  Dado que o assinante tem orçamento ativo
  Quando ele envia novos percentuais para as cinco categorias que somam 100% e o agente mostra o resumo
  E o assinante responde "sim"
  Então a distribuição é atualizada mantendo o total e o agente confirma o novo orçamento

Cenário: Editar orçamento em rascunho mantém o estado Draft
  Dado que o assinante tem um orçamento em Draft não ativado
  Quando ele altera a distribuição e confirma
  Então a alteração é salva e o orçamento continua em Draft, sem ser ativado

Cenário: Pedido vago exige desambiguação da operação
  Dado que o assinante envia "quero editar meu orçamento"
  Quando o agente não identifica qual operação
  Então ele pergunta se a intenção é mudar o total, ajustar uma categoria ou refazer a distribuição

Cenário: Mês sem ano pede clarificação antes de coletar valor
  Dado que o assinante envia "ajusta prazeres em junho"
  Quando o agente resolve a referência de mês e não há ano
  Então ele pergunta o ano antes de coletar o novo valor

Cenário: Competência sem orçamento oferece criação
  Dado que não existe orçamento para a competência solicitada
  Quando o assinante pede para editar
  Então o agente responde "Você ainda não tem um orçamento para *<mês>*. Posso te ajudar a criar um?" e não tenta editar

Cenário: Valor de total não identificado gera reprompt
  Dado que o agente pediu o novo total
  Quando o assinante responde algo sem valor monetário reconhecível
  Então o agente pede novamente o valor total em reais, sem aplicar mudança

Cenário: Porcentagem fora do intervalo é recusada
  Dado que o agente aguarda a nova porcentagem de uma categoria
  Quando o assinante informa um valor maior que 100
  Então o agente recusa e pede uma porcentagem entre 0 e 100

Cenário: Distribuição com soma inválida gera reprompt
  Dado que o agente aguarda a nova distribuição
  Quando o assinante envia percentuais que não somam 100%
  Então o agente informa que não conseguiu aplicar a distribuição e pede novos valores

Cenário: Cancelamento não altera o orçamento
  Dado que o agente exibiu o resumo da edição e pediu confirmação
  Quando o assinante responde "não"
  Então nenhuma alteração é persistida e o agente informa que a edição foi cancelada

Cenário: Resposta ambígua encerra sem efeito na segunda tentativa
  Dado que o agente pediu confirmação da edição
  Quando o assinante responde de forma ambígua duas vezes
  Então o agente cancela a edição sem alterar o orçamento

Cenário: Reenvio da mesma mensagem não duplica a edição
  Dado que o assinante confirmou uma edição
  Quando a mesma mensagem de confirmação é reprocessada com o mesmo identificador
  Então o agente reconhece o replay e não aplica a edição uma segunda vez

Cenário: Expiração por inatividade libera o fluxo sem efeito
  Dado que uma edição está suspensa aguardando resposta há mais de 30 minutos
  Quando o assinante responde depois do prazo
  Então o fluxo expira sem efeito e a nova mensagem é interpretada normalmente

Cenário: Planner indisponível não gera falso sucesso
  Dado que o serviço de orçamento está indisponível no momento de aplicar
  Quando o assinante confirma a edição
  Então o agente informa a indisponibilidade e não reporta sucesso

Cenário: Fluxo de edição já pendente bloqueia novo início
  Dado que já existe uma edição pendente para o assinante no mesmo recurso
  Quando ele tenta iniciar outra edição
  Então o agente informa que há uma edição em andamento em vez de abrir uma nova
```

## Dados e Permissões
- Dados obrigatórios: competência resolvida; operação escolhida; novo valor (total em centavos, ou categoria + porcentagem 0–100, ou distribuição das 5 categorias somando 10000 basis points); `wamid` do inbound; identificador de recurso/thread.
- Perfis/permissões: assinante autenticado e dono do orçamento; escrita atribuída ao seu `userID` via `agent.InboundIdentityFromContext()`.

## Dependências
- Kernel `internal/platform/workflow` (Engine/Store/Snapshot/merge-patch, `StaleSuspendedReaper`).
- `internal/budgets`: estender `EditCategoryPercentage` para aceitar Draft (G-04); **novo** use case de edição de total preservando basis points (G-01); **novo** use case de redistribuição inteira preservando total (G-02); reuso de `AllocationDistributor.Distribute` e `DecideEditCategoryPercentage`; nova operação de domínio em `entities/budget.go` para alterar `total_cents`/distribuição respeitando invariantes.
- `internal/agents`: nova tool `edit_budget` (starter de workflow, fora do write-tool-set direto) e workflow `budget-edit` (estado fechado + slots + confirmação + reaper); porta `BudgetPlanner` estendida; `IdempotentWrite` + write ledger para as mutações; resolução de mês via `DecideCompetence`.
- `DecideAllocationKind`/`DecideAllocationsBP` (`onboarding_workflow.go`) para interpretar a distribuição.

## Fora de Escopo
- Criar orçamento (já implementado; baseline documentado — D-03).
- Excluir/resetar orçamento (não escolhido em D-01).
- Ativação do orçamento (passo separado já existente).
- Alterar total e distribuição na mesma confirmação (são operações distintas neste fluxo).
- Métricas, dashboards e alertas (tratados na observabilidade da techspec).
- Histórico/auditoria de longo prazo das edições.

## Evidências
- Entrada: pedido do usuário ("criar/editar orçamento", "todas as possibilidades de conversa", "uma única US pronta para desenvolvimento") + decisões D-01..D-05.
- Base de código: `tools/adjust_allocation.go:58`; `usecases/edit_category_percentage.go` (`ErrBudgetNotActive`); `domain/services/category_percentage_workflow.go:21-74`; `domain/services/allocation_distributor.go:20-75`; `entities/budget.go` (`BudgetStateDraft`/`BudgetStateActive`, invariantes); `month_reference.go:114-144`; `tools/competence_reference.go:50`; `workflows/budget_creation_workflow.go:143-158,224,232,253,265,269,323`; `usecases/idempotent_write.go`; `infrastructure/persistence/write_ledger_repository.go`; `interfaces/errors.go`; kernel `internal/platform/workflow/{engine,step,codec,store}.go`.
- Inferências: os fluxos de edição herdam os primitivos de robustez do fluxo de criação (TTL/reaper/replay), já provados por testes de integração e real-LLM; a copy nova reaproveita o tom da criação.
- Não evidenciado: não existem hoje use cases de editar total e de redistribuir inteiro, nem gate de confirmação para edição, nem caminho de edição de Draft, nem reaper de edição — são os gaps G-01..G-05 a implementar.

## Notas de Validação
- Cobre A1–A2, B1–B6, C1–C3, D1–D3, E1–E4, F1–F5 da matriz conversacional.
- "não inventar resposta": copies novas (desambiguação, resumo de edição, indisponibilidade, pendência) serão redigidas na implementação reaproveitando o tom existente; os cenários verificam comportamento, não texto literal novo.
- Pronta para desenvolvimento: escopo, dependências, gaps e arquivos-alvo estão explícitos abaixo.

---

## Plano de Implementação (pronto para desenvolvimento)

### `internal/budgets` (domínio + aplicação)
1. `entities/budget.go`: operação de domínio para **alterar total** (Ativo/Draft) preservando basis points e recalculando planejado; operação para **substituir distribuição** preservando total; ambos reforçando invariantes (total > 0, soma = 10000).
2. `domain/services`: `Decide*` puro para redistribuição inteira (reusar `AllocationDistributor`); reuso de `DecideEditCategoryPercentage`.
3. `application/usecases`: `EditBudgetTotal`, `RedistributeBudget`; estender `EditCategoryPercentage` para aceitar Draft. DTOs de input com `Validate()` (R-DTO-VALIDATE-001).
4. Porta/adapter `BudgetPlanner` (agents): novas operações `EditTotal`, `Redistribute`; `EditCategoryPercentage` aceitando Draft.

### `internal/agents` (workflow + tool + wiring)
5. Estado fechado `BudgetEditState` (Status/Awaiting/Operation/valores) — tipos enumerados, sem string livre.
6. Workflow `budget-edit` (`workflows/budget_edit_workflow.go`): slots `operation → value → confirm`; suspend/resume por merge-patch; confirmação HITL; expiração; replay por `messageID`.
7. Tool `edit_budget` (`tools/edit_budget.go`): starter do workflow; resolve competência via `DecideCompetence`; outcome fechado (`started`/`clarify`/`pending_edit_exists`/`offer_create`).
8. `module.go`: engine + definição + continuer + resolver no consumer + reaper dedicado (paridade com `budget-creation`).
9. `IdempotentWrite` nas mutações (`operation = edit_total | adjust_allocation | redistribute`).

### Testes e gates
10. Unit table-driven testify/suite (R-TESTING-001) para `Decide*` e steps; decisões de confirmação/expiração/replay.
11. Integração Postgres (`//go:build integration`) do workflow ponta a ponta (Ativo e Draft).
12. Gate real-LLM (`RUN_REAL_LLM=1`) para roteamento da operação e extração de valores, pass ratio ≥ 0.90 por categoria (paridade com criação).
13. Validação por risco (AGENTS.md): build, vet, test race, lint e gates de governança no módulo alterado.

## Skills obrigatórias
- `go-implementation` (sempre); `mastra` (consumir o substrato, edição como workflow/tool finos); `domain-modeling-production` (novos `Decide*`, estados fechados); `design-patterns-mandatory` (gate aplicar vs. não aplicar padrão no desenho do workflow de edição).

## Riscos e Suposições
- Risco 1: alterar o total de orçamento ativo exige nova operação de domínio preservando invariantes; sem ele a edição de total não fecha.
- Risco 2: permitir edição de Draft altera a guarda `ErrBudgetNotActive`; testes de regressão devem garantir que Ativo continua íntegro.
- Risco 3: copies novas precisam passar pelos gates real-LLM (≥ 0.90 por categoria) para evitar brittleness.
- Suposição: a granularidade (workflow `budget-edit` unificado vs. definições irmãs) é decisão de techspec; ambas cabem no kernel atual sem mudança de plataforma.

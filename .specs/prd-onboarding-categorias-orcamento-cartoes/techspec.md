<!-- spec-hash-prd: 8440885b0fb7b6f83f4ce3ac22060c27ba4987e060799f7fd61f7a9b01dc0571 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Onboarding com categorias, orçamento mensal e cartões

**PRD:** `.specs/prd-onboarding-categorias-orcamento-cartoes/prd.md` (spec-version 1)
**Skills aplicadas:** `go-implementation`, `mastra`, `domain-modeling-production`, `design-patterns-mandatory`.
**Decisão de pattern:** `select_pattern.py` → `status=reject` → **não aplicar padrão** (ADR-006).

## Resumo Executivo

A feature é um **refactor local do workflow durável de onboarding** já existente
(`internal/agents/application/workflows/onboarding_workflow.go`), consumindo o kernel genérico
`internal/platform/workflow` sem alterá-lo. Não há novo agente, tool, provider LLM, tabela, migration
ou combinator de kernel. O trabalho concentra-se em: (1) reordenar e ampliar a `Sequence` de steps;
(2) introduzir um step isolado de boas-vindas; (3) renomear a semântica de renda líquida para
orçamento mensal no estado, prompts, erros e WorkingMemory; (4) fundir metodologia+distribuição+resumo
num único step durável de **budget review** com submáquina interna fechada que permite revisão; (5)
extrair ativação e recorrência como steps próprios após a confirmação; (6) transformar o cadastro de
cartão único num **loop um-por-vez** via re-suspensão no mesmo cursor do kernel.

A decisão-âncora de arquitetura decorre da semântica de resume do kernel: `Engine.Resume` sempre
retoma no `snap.Cursor` do step suspenso (`engine.go:263`, `engine.go:306`, `combinators.go:31`); um
step só pode **re-suspender no mesmo índice** ou **completar e avançar** — nunca retornar a um step
anterior. Isso torna o loop de cartões e a revisão do resumo naturalmente expressáveis como
submáquinas internas de um step que re-suspende, e proíbe "voltar" de um step para outro.

Todos os estados permanecem tipos fechados (DMMF state-as-type): `OnboardingPhase` e um novo
`reviewAwaitKind` interno, ambos com `String`/`Parse`/`IsValid` e zero value inválido. `Decide*`
permanece puro e determinístico. LLM continua exclusivo nas extrações estruturadas via `agent.Agent`
+ OpenRouter, como já ocorre hoje.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Os componentes de fluxo vivem em `internal/agents/application/workflows/onboarding_workflow.go`
(arquivo único já existente) e no seu teste. Além disso, `internal/agents/module.go` ganha **uma
linha de wiring** para o reaper de runs suspensos do onboarding (D-12, ADR-005) — nenhum contrato
público de porta muda.

**Componentes modificados/novos:**

| Componente | Tipo | Mudança |
|-----------|------|---------|
| `OnboardingState` | struct de estado durável | renomear `IncomeCents`→`MonthlyBudgetCents`; adicionar `ReviewAwait reviewAwaitKind`; remover `CardNickname`/`CardDueDay` (mortos após remoção do resumo com cartão) |
| `OnboardingPhase` | enum fechado | novo conjunto: `Welcome, Goal, MonthlyBudget, BudgetReview, Activation, Recurrence, Cards, Conclusion` (substitui `MonthlyIncome/Methodology/Distribution/Summary`) |
| `reviewAwaitKind` | enum fechado **novo** | sub-estado do step de review: `reviewAwaitDistribution`, `reviewAwaitConfirm` |
| `BuildWelcomeStep` | step **novo** | boas-vindas isolada; suspende; no resume completa (ignora texto — D-07) |
| `BuildGoalStep` | step | remover preâmbulo de boas-vindas do prompt; manter sub-fluxo de valor opcional |
| `BuildMonthlyBudgetStep` | step (ex-`BuildIncomeStep`) | prompt = apresentação das 5 categorias (texto exato RF-11) + pergunta de orçamento mensal (D-01); grava `MonthlyBudgetCents` |
| `BuildBudgetReviewStep` | step **fundido** | funde metodologia+distribuição+resumo; submáquina `reviewAwaitDistribution`↔`reviewAwaitConfirm`; cria/recria draft budget; "não" reabre distribuição (D-09) |
| `BuildActivationStep` | step **novo/extraído** | `ActivateBudget` idempotente após confirmação (RF-22) |
| `BuildRecurrenceStep` | step **novo/extraído** | pergunta recorrência; negativa/ambígua → sem recorrência (D-11) |
| `BuildCardsStep` | step | **loop** um-por-vez via re-suspend até "não" (D-05); sem limite |
| `BuildConclusionStep` | step | reduzido a: upsert WorkingMemory + `FinalMessage` |
| `DecideMonthlyBudgetCents` | função pura (ex-`DecideIncomeCents`) | renomear; mesma regra `> 0` |
| `BuildOnboardingWorkflow` | montagem da `Sequence` | nova ordem dos steps |
| onboarding reaper | wiring em `module.go` | **novo**: `workflow.NewStaleSuspendedReaper(store, OnboardingWorkflowID, 7*24h, 100, o11y)` (D-12) |

**Componentes reutilizados sem mudança de contrato:** `interfaces.BudgetPlanner`
(`budget_planner.go:9`), `interfaces.CardManager` (`card_manager.go:9`), kernel
`internal/platform/workflow`, `memory.WorkingMemory`/`ThreadGateway`/`MessageStore`,
`money.FromCents`, `budgets/domain/valueobjects.RootSlug` (`root_slug.go:12`).

### Fluxo de Dados (Sequence de steps, ordem final)

```
Start(userID, {Phase:Welcome, UserID, PeerID})           cursor
  0. welcome         → suspend (boas-vindas)               0
  (resume: qualquer texto → complete, ignora)             →1
  1. goal            → suspend (pergunta meta)             1
     resume → extrai objetivo; (opcional) pergunta valor 1x; complete →2
  2. monthly_budget  → suspend (categorias + orçamento)    2   [D-01]
     resume → DecideMonthlyBudgetCents; inválido → reprompt (mesmo cursor); complete →3
  3. budget_review   → suspend (sugestão distribuição)     3   [submáquina]
     resume(reviewAwaitDistribution) → parse alocação → recria draft → suspend (resumo)
     resume(reviewAwaitConfirm) → "sim" complete →4 | "não" → suspend (nova distribuição) [D-09]
  4. activation      → ActivateBudget (idempotente) → complete →5   [RF-22]
  5. recurrence      → suspend (pergunta recorrência)      5
     resume → "sim" cria recorrência; "não"/ambíguo → sem recorrência; complete →6  [D-02/D-11]
  6. cards           → suspend ("adicionar cartão?")       6   [loop]
     resume → cria cartão (se válido) → re-suspend ("outro?") [mesmo cursor 6]
     resume "não" → CardsDone=true → complete →7  [D-05, sem limite]
  7. conclusion      → upsert WorkingMemory + FinalMessage → complete → Succeeded
```

Chave de correlação: `userID` opaco (`resolve_onboarding_or_agent.go:84,125,144`). Resume payload:
`{"resumeText": <mensagem>}` aplicado por merge-patch (`engine.go:249`), portanto o campo `ResumeText`
mantém a tag JSON `resumeText`; o rename só afeta o campo de orçamento mensal.

## Design de Implementação

### Interfaces Chave

Nenhuma interface de porta muda. O step de review consome as portas já existentes:

```go
type BudgetPlanner interface {
    SuggestAllocation(ctx context.Context, totalCents int64, allocations []AllocationBP) ([]AllocationCents, error)
    CreateBudget(ctx context.Context, in DraftBudget) (BudgetRef, error)
    DeleteDraftBudget(ctx context.Context, userID uuid.UUID, competence string) error
    ActivateBudget(ctx context.Context, userID uuid.UUID, competence string) error
    CreateRecurrence(ctx context.Context, userID uuid.UUID, competence string, months int) error
    GetMonthlySummary(ctx context.Context, userID uuid.UUID, competence string) (BudgetSummary, error)
}

type CardManager interface {
    ListCards(ctx context.Context, userID uuid.UUID) ([]Card, error)
    CreateCard(ctx context.Context, in NewCard) (CardRef, error)
}
```

### Modelos de Dados

**Estado durável (JSON no snapshot do kernel):**

```go
type OnboardingState struct {
    Phase              OnboardingPhase `json:"phase"`
    UserID             string          `json:"userID"`
    PeerID             string          `json:"peerID"`
    Goal               string          `json:"goal"`
    GoalValueCents     int64           `json:"goalValueCents"`
    GoalValueAsked     bool            `json:"goalValueAsked"`
    MonthlyBudgetCents int64           `json:"monthlyBudgetCents"`
    ReviewAwait        reviewAwaitKind `json:"reviewAwait"`
    CardsDone          bool            `json:"cardsDone"`
    Allocations        map[string]int  `json:"allocations"`
    Recurrence         bool            `json:"recurrence"`
    ResumeText         string          `json:"resumeText"`
    FinalMessage       string          `json:"finalMessage"`
}
```

**Enums fechados (DMMF state-as-type, `iota + 1`, zero value inválido — R5.8):**

```go
type OnboardingPhase int
const (
    PhaseWelcome OnboardingPhase = iota + 1
    PhaseGoal
    PhaseMonthlyBudget
    PhaseBudgetReview
    PhaseActivation
    PhaseRecurrence
    PhaseCards
    PhaseConclusion
)
// String()/IsValid()/ParseOnboardingPhase(s) atualizados para o novo conjunto.

type reviewAwaitKind int
const (
    reviewAwaitDistribution reviewAwaitKind = iota + 1
    reviewAwaitConfirm
)
// String()/IsValid()/parseReviewAwaitKind(s) — usados apenas dentro do pacote.
```

**Funções puras (`Decide*`, sem IO, sem `context.Context` — DMMF, R-TXN-001-análogo):**
`DecideGoal`, `DecideGoalValueCents`, `DecideMonthlyBudgetCents` (renomeada de `DecideIncomeCents`,
mesma regra `amountBRL > 0`), `DecideDistribution`, `DecideAllocationKind`, `DecideAllocationsBP`,
`DecideCardEntry`, `centsToBasisPoints` — todas reutilizadas; a única renomeação semântica é
`IncomeCents`→`MonthlyBudgetCents` nas assinaturas internas que a referenciam.

### Prompts (linguagem — RF-11/RF-13/RF-14/RF-35, M-02)

- **welcome** (novo, isolado): apresentação do MeControla reaproveitando tom/emojis atuais, **sem**
  pergunta de meta (D-10).
- **goal**: pergunta de objetivo isolada (remover o bloco de boas-vindas de `welcomeGoalPrompt`,
  `onboarding_workflow.go:455`).
- **monthly_budget** (D-01): texto exato de RF-11 (`"Antes de montar seu planejamento, deixa eu te
  mostrar como organizamos o dinheiro por aqui. Tudo vive em apenas 5 categorias: Custo Fixo,
  Conhecimento, Prazeres, Metas e Liberdade Financeira."`) + pergunta de orçamento mensal
  (`"Qual é o seu orçamento mensal?..."`), substituindo `incomePrompt` (`onboarding_workflow.go:464`)
  e `incomeReprompt` (`:466`). Reprompt específico com exemplo em reais (RF-15).
- **budget_review**: `methodologyPrompt`/`methodologyReprompt` (`:508`) e `summaryPrompt` (`:520`)
  reescritos para usar "orçamento mensal" no lugar de "renda mensal" (`:510`, `:524`); o resumo
  remove a linha de cartão (RF-21) e a de renda (RF-20/RF-21).
- **recurrence**: `conclusionRecurrencePrompt` (`:472`) mantido.
- **conclusion**: `conclusionFinalMessage` (`:534`) mantido (próximos passos; sem renda).

Gate M-02: nenhum prompt/erro/resumo/WM contém "renda líquida"/"renda mensal".

### Semântica dos steps (suspend/resume, cursor)

Cada step preserva o padrão vigente: primeira entrada com `state.ResumeText == ""` → `suspendStep`;
resume com `ResumeText` preenchido → processa e limpa `state.ResumeText = ""`. O kernel persiste o
cursor no suspend (`engine.go:333`) e retoma nele (`engine.go:306`).

- **welcome**: `ResumeText==""` → suspend(welcome). Resume → `complete` (D-07: texto ignorado;
  `state.ResumeText=""`). O step seguinte (goal) roda no mesmo `execute` com `ResumeText==""` → suspende
  a pergunta de meta (comportamento comprovado pela cadeia atual goal→income, `combinators.go:23-38`).
- **budget_review** (submáquina):
  1. `ResumeText==""` (primeira entrada): `SuggestAllocation(default)` → `ReviewAwait=reviewAwaitDistribution` → suspend(methodologyPrompt).
  2. Resume com `ReviewAwait==reviewAwaitDistribution`: extrai alocação (LLM) → `DecideAllocationKind`/`DecideAllocationsBP`; erro → suspend(methodologyReprompt) mantendo o mesmo sub-estado; sucesso → `applyDraftBudget` (helper: `GetMonthlySummary`→se draft existe `DeleteDraftBudget`; `CreateBudget`) → `SuggestAllocation(current)` → `ReviewAwait=reviewAwaitConfirm` → suspend(summaryPrompt).
  3. Resume com `ReviewAwait==reviewAwaitConfirm`: extrai sim/não (LLM) → `sim`: `complete` (avança para activation); `não`/ambíguo: `SuggestAllocation` → `ReviewAwait=reviewAwaitDistribution` → suspend(methodologyPrompt) (D-09 loop).
- **cards** (loop): `ResumeText==""` → `ListCards` (contagem para o prompt) → suspend(cardsPrompt). Resume → extrai cartão (LLM); `wantsCard=false` → `CardsDone=true` → `complete`; `wantsCard=true` inválido → suspend(cardsReprompt) [mesmo cursor, cartão parcial não criado — RF-30]; válido → `CreateCard` → suspend(cardsPrompt) [re-suspend, mesmo cursor, "outro?" — D-05].

### Tratamento de Erros (RF-39/RF-40, sem falso sucesso)

Toda falha de IO (`ListCards`, `CreateCard`, `SuggestAllocation`, `GetMonthlySummary`,
`DeleteDraftBudget`, `CreateBudget`, `ActivateBudget`, `CreateRecurrence`, `Upsert*`) retorna
`failStep(state, fmt.Errorf("agents.onboarding.<step>: <op>: %w", err))` → o kernel marca
`StepStatusFailed`/`RunStatusFailed`, grava `snap.LastError` e emite span com erro
(`engine.go:351-368`, `runStep` `:481-487`). O usecase não afirma conclusão em run não-`Succeeded`
(`resolve_onboarding_or_agent.go:149`). Ativação tolera `ErrBudgetAlreadyActive`
(`onboarding_workflow.go:847`) para idempotência no resume. Erros de validação de fronteira
(objetivo vazio, orçamento inválido, distribuição não fecha, cartão inválido) NÃO falham o run:
re-suspendem com reprompt (comportamento atual preservado).

## Pontos de Integração

- **OpenRouter via `agent.Agent`**: extrações estruturadas (`goal_with_value`, `goal_value`,
  `monthly_budget` (ex-`income`), `allocation_input`, `summary_confirm`, `recurrence`, `card`) —
  schemas `Strict: true` já existentes, apenas renome de `income_extract`→`monthly_budget_extract`.
  Sem novo provider, sem fallback chain (RF-41).
- **Postgres** (indireto via portas): budget e cartão. Nenhum SQL neste pacote (R-ADAPTER-001.2).

## Abordagem de Testes

### Testes Unitários (`onboarding_workflow_test.go`, `package workflows` whitebox — R-TESTING-001)

- `Decide*` puros por tabela: `DecideMonthlyBudgetCents` (renomeado de `DecideIncomeCents`,
  `onboarding_workflow_test.go:242`), `DecideDistribution`, `DecideAllocationsBP`,
  `DecideAllocationKind`, `DecideCardEntry`, `DecideGoal*` — sem mock (funções puras).
- `ParseOnboardingPhase`/`String` para o novo conjunto de fases (`:546`), incluindo round-trip e
  fase inválida.
- `parseReviewAwaitKind`/`String`/`IsValid` (novo enum): round-trip e zero value inválido.
- Steps com `agent.Agent` e portas mockadas (mockery, `.mockery.yml`) — testify/suite table-driven
  (args/dependencies/IIFE/SUT-in-`s.Run`): welcome (ignora texto), monthly_budget (categorias no
  prompt + reprompt inválido), budget_review (loop distribuição↔confirm, "não" reabre), cards (loop
  multi-cartão + parcial + "não"), activation idempotente, recurrence negativa/ambígua, conclusão.
- `OnboardingState` merge-patch: preserva `MonthlyBudgetCents`/`ReviewAwait`/`Allocations` quando o
  resume traz apenas `{"resumeText":...}` (espelha `TestOnboardingState_MergePatch_*`, `:216`).
- Gate M-02: teste que varre os prompts/erros/resumo e falha se contiver "renda líquida"/"renda mensal".

### Testes de Integração / Real-LLM (gate RF-42/M-05)

Sim — o projeto tem fronteira LLM crítica onde mock não garante correção (histórico de falso-verde em
extração conversacional). Harness real-LLM sob build tag de integração (`RUN_REAL_LLM=1` +
`OPENROUTER_*`), no molde já usado por features anteriores de onboarding: dirige o workflow do
welcome à conclusão com múltiplos cartões e revisão de resumo, medindo ratio de acerto ≥ 0,90 **por
categoria de extração** (goal, goal_value, monthly_budget, allocation_input, summary_confirm,
recurrence, card), com invariante semântico (drive-until-state), sem assert de keyword frágil.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Estado e enums** (`OnboardingState`, `OnboardingPhase` novo conjunto, `reviewAwaitKind`,
   `DecideMonthlyBudgetCents`) + testes puros — base sem IO. (ADR-002, ADR-003)
2. **Steps de linguagem/ordem simples**: welcome, goal (sem preâmbulo), monthly_budget (categorias +
   orçamento), conclusion reduzido. (ADR-001)
3. **budget_review** (submáquina fundida + helper `applyDraftBudget`) e **activation** step. (ADR-003, ADR-004)
4. **recurrence** step + **cards** loop. (ADR-004, ADR-005)
5. **Montagem** `BuildOnboardingWorkflow` na nova ordem + wiring do **reaper de onboarding**
   (`workflow.NewStaleSuspendedReaper(store, OnboardingWorkflowID, 7*24h, 100, o11y)`) em `module.go`,
   no molde dos reapers existentes (`module.go` confirm/cardCreate/budgetCreation). (D-12, ADR-005)
6. **Testes de step** (mock) + **gate real-LLM**.

### Dependências Técnicas

Nenhuma nova. Kernel, portas, provider e wiring já existem. Sem migration, sem infra nova.

## Monitoramento e Observabilidade

Reutiliza a instrumentação do kernel: `workflow_steps_total{workflow,step,status}`,
`workflow_step_duration_seconds`, `workflow_suspend_total`, `workflow_runs_total{workflow,status}`
(`engine.go:494-502`, `:146-151`) — cardinalidade controlada (sem `user_id`, R-WF-KERNEL-001.4). Os
novos `stepID`s (`step-welcome`, `step-monthly-budget`, `step-budget-review`, `step-activation`,
`step-recurrence`, `step-cards`, `step-conclusion`) aparecem no label `step`. Spans por step já
emitidos (`runStep`, `engine.go:464`). O usecase mantém `outcome` (`started`/`completed`/...).

## Considerações Técnicas

### Decisões Chave (ADRs)

- **ADR-001** — Reordenação da sequência + boas-vindas isolada + categorias/orçamento em mensagem
  única, sobre a semântica de cursor do kernel. (D-01, D-07; RF-01..RF-14)
- **ADR-002** — Renomeação semântica renda→orçamento mensal no estado, sem shim de compatibilidade;
  `resumeText` preservado. (D-04, D-06; RF-35, RF-43)
- **ADR-003** — Step único `budget_review` com submáquina fechada (`reviewAwaitKind`) para revisão do
  resumo reabrindo a distribuição. (D-09; RF-16..RF-23)
- **ADR-004** — Ativação e recorrência como steps próprios após confirmação; recorrência
  negativa/ambígua sem recorrência. (D-02, D-11, RF-22, RF-24, RF-25)
- **ADR-005** — Loop de cartões um-por-vez via re-suspensão no mesmo cursor, sem limite; + **reaper
  dedicado de onboarding com TTL 7 dias** para runs abandonados. (D-05, D-12; RF-26..RF-31a)
- **ADR-006** — Não aplicar design pattern GoF (selector `reject`); refactor direto. (skill design-patterns-mandatory)

Decisões técnicas adicionais registradas nas ADRs relacionadas: **D-12** (reaper de onboarding,
ADR-005) e **D-13** (competência recalculada por step — risco de virada de mês aceito, ADR-004).

### Riscos Conhecidos

| Risco | Impacto | Mitigação |
|------|---------|-----------|
| Snapshot in-flight com chave antiga `incomeCents` (D-06) | Onboarding no meio re-pergunta orçamento 1x | Aceito no PRD (RF-43); `encoding/json` ignora chave desconhecida no decode; nenhum orçamento ativado afetado |
| Step `budget_review` maior (submáquina) | Mais lógica num step; risco de estado inconsistente | Sub-estado fechado `reviewAwaitKind` (zero value inválido); helper `applyDraftBudget` idempotente (delete+create); testes de loop dedicados |
| Extração LLM abaixo de 0,90 por brittleness de teste | Falso-vermelho no gate | Harness real-LLM com drive-until-state e invariante semântico, sem keyword frágil (lição de features anteriores) |
| Loop de cartões sem limite (D-05) | Loop longo acidental / run suspenso abandonado | Cada iteração exige turno próprio (RF-31); **reaper dedicado de onboarding** com TTL 7 dias encerra runs abandonados (D-12, ADR-005) |
| Competência recalculada por step (status quo, D-13) | Onboarding cruzando a virada do mês entre criar o draft (`budget_review`) e ativar (`activation`) → ativação falha por draft de outra competência | **Risco aceito** (D-13): `failStep` tipado (sem falso sucesso); janela é rara (onboarding concluído em minutos); no resume o `budget_review` recria o draft na competência corrente antes da ativação |
| Remoção de `CardNickname`/`CardDueDay` do estado | Campos mortos após remover cartão do resumo | Decode ignora chaves antigas; sem consumidor remanescente (resumo não exibe cartão, RF-21) |

### Conformidade com Padrões

- **R-AGENT-WF-001** — comportamento novo permanece como workflow durável no consumidor
  `internal/agents`; sem `switch case intent.Kind`; estados fechados; LLM só nas call-sites
  sancionadas; Run auditável. Kernel intacto.
- **R-WF-KERNEL-001** — `internal/platform/workflow` não é alterado (opção C rejeitada em ADR-003);
  sem import de domínio, sem SQL, cardinalidade controlada.
- **R-ADAPTER-001.1** — zero comentários em Go de produção.
- **R-TESTING-001** — testify/suite whitebox table-driven; mockery.
- **R-DTO-VALIDATE-001** — não se aplica (sem input DTO novo; validação de fronteira nos steps/Decide*).
- **DMMF** (`governance.md`) — state-as-type para `OnboardingPhase`/`reviewAwaitKind`; `Decide*` puro.
- **go-implementation R5.8/R5.12/R7.6** — enums `iota+1`, sem panic, `errors.Join` onde agregar.

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/onboarding_workflow.go` — alvo principal.
- `internal/agents/application/workflows/onboarding_workflow_test.go` — testes.
- `internal/agents/application/interfaces/budget_planner.go`, `card_manager.go`, `types.go`,
  `errors.go` — portas/tipos reutilizados (sem mudança).
- `internal/agents/application/usecases/resolve_onboarding_or_agent.go` — start/resume (sem mudança).
- `internal/agents/module.go:231` — wiring `BuildOnboardingWorkflow` (assinatura preservada) + **nova
  linha** do reaper de onboarding (D-12).
- `internal/platform/workflow/{engine,combinators,step,codec}.go` — kernel consumido (sem mudança).
- `internal/budgets/domain/valueobjects/root_slug.go` — taxonomia de categorias (sem mudança).

## Itens em Aberto

Nenhum. Todas as ambiguidades de produto (D-01..D-11) e as decisões técnicas de arquitetura foram
resolvidas: estrutura de steps (ADR-003, step único fundido), reaper de onboarding (D-12, ADR-005) e
tratamento de competência por step (D-13, risco aceito, ADR-004). Nenhuma ressalva remanescente.

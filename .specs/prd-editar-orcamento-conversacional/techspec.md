<!-- spec-hash-prd: 6c5c64e617f5b58ae7e30bc8da95c9f428985534767b19ba68a89826182f4bc3 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido. -->

# Especificação Técnica — Editar Orçamento por Conversa (WhatsApp)

> PRD: `.specs/prd-editar-orcamento-conversacional/prd.md` (spec-version 1, 40 RFs).
> Skills aplicadas: `go-implementation`, `mastra`, `domain-modeling-production`, `design-patterns-mandatory`.
> Design pattern selector: **reject → não aplicar padrão** (solução direta/composição), ver ADR-001.

## Resumo Executivo

A edição conversacional de orçamento é implementada como **um workflow durável único `budget-edit`** no consumidor `internal/agents`, espelhando o mecanismo já provado do `budget-creation` (`internal/agents/application/workflows/budget_creation_workflow.go`), sem introduzir nenhum design pattern GoF (selector determinístico retornou `reject`, ADR-001). O workflow carrega um estado fechado `BudgetEditState` cujo campo `Operation` (tipo fechado `EditTotal | AdjustCategory | Redistribute`) seleciona qual valor coletar; todas as operações convergem num passo comum de confirmação HITL "sim/não" antes de aplicar (ADR-002). Uma única tool starter `edit_budget` inicia o fluxo; a tool imediata `adjust_allocation` (que mutava sem confirmação) é **removida** para eliminar o caminho de escrita sem HITL (G-03).

No domínio `internal/budgets`, três operações puras persistem via um **novo método de repositório `UpdatePlan`** que atualiza `total_cents` e as alocações **sem alterar `state`/`activated_at`** — corrigindo o efeito colateral de re-stamp de `activated_at` que o caminho atual (`EditCategoryPercentage` → `repo.Activate`) causa, e habilitando edição de rascunho (Draft) sem ativá-lo (ADR-003). As regras de negócio (recálculo do planejado preservando basis points, rebalanceamento, invariante soma=10000) vivem em funções `Decide*`/serviços puros já existentes (`AllocationDistributor.Distribute`, `DecideEditCategoryPercentage`), reutilizados sem duplicação. Robustez de runtime (TTL 30min, reaper dedicado, replay por `wamid`, exclusividade de um fluxo de orçamento por recurso, falha-segura sem falso sucesso) herda os primitivos do kernel `internal/platform/workflow` (ADR-004, ADR-005).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos — `internal/agents`:**
- `application/workflows/budget_edit_state.go` — `BudgetEditState` + tipos fechados `BudgetEditOperation`, `BudgetEditAwaiting`, `BudgetEditStatus` (com `String()`/`Parse*`/`IsValid()`).
- `application/workflows/budget_edit_decisions.go` — `Decide*` puras (`DecideBudgetEditTotal`, `DecideBudgetEditDistribution`, `DecideBudgetEditAdjust`, `DecideBudgetEditConfirmation`), constantes TTL/maxReprompts, `BudgetEditAction`.
- `application/workflows/budget_edit_workflow.go` — `BuildBudgetEditWorkflow`, step switch `(Operation, Awaiting)`, `BudgetEditKey`, `BuildBudgetEditReaper`, `executeBudgetEdit` (branch por operação).
- `application/usecases/budget_edit_continuer.go` — `BudgetEditContinuer` (resume por merge-patch, Run auditável).
- `application/tools/edit_budget.go` — tool starter `edit_budget` (pré-checagem de existência/estado + `engine.Start`).
- `application/dtos/input/*` — não aplicável (inputs via schema LLM + tool).

**Novos — `internal/budgets`:**
- `application/usecases/edit_budget_total.go` — `EditBudgetTotal` (preserva BP, recomputa planejado).
- `application/usecases/redistribute_budget.go` — `RedistributeBudget` (preserva total, substitui distribuição).
- `application/dtos/input/edit_budget_total.go`, `redistribute_budget.go` — DTOs com `Validate()` (R-DTO-VALIDATE-001).
- `domain/commands/edit_budget_total.go`, `redistribute_budget.go` — smart constructors.

**Modificados:**
- `internal/budgets/domain/entities/budget.go` — `ChangeTotal(...)` e `ReplaceDistribution(...)` (aceitam Draft e Active).
- `internal/budgets/application/interfaces/budget_repository.go` + postgres — método `UpdatePlan`.
- `internal/budgets/application/usecases/edit_category_percentage.go` — migrar persistência de `repo.Activate` para `repo.UpdatePlan`; aceitar Draft (remover guarda `IsActive`).
- `internal/budgets/module.go` — exportar `EditBudgetTotalUC`, `RedistributeBudgetUC`.
- `internal/agents/application/interfaces/budget_planner.go` + adapter — `EditBudgetTotal`, `RedistributeBudget`; `EditCategoryPercentage` Draft-capable.
- `internal/agents/module.go` — engine/agent/def/continuer/reaper/tool wiring do `budget-edit`.
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` — `WithBudgetEditResolver` + `tryContinueBudgetEdit` na cadeia de resume.
- `internal/agents/application/tools/adjust_allocation.go` — **removido** (substituído por `edit_budget` op=AdjustCategory).

### Fluxo de Dados

```
Inbound WhatsApp → AgentRuntime → LLM decide operação → tool edit_budget(operation, monthRef, [valores])
  → pré-checa competência+existência+estado (GetMonthlySummary)
     ├─ não existe        → outcome offer_create (agente oferece create_budget)      [RF-10]
     ├─ auto-draft vazio  → outcome offer_create (rotear p/ criação)                 [RF-13/R2]
     ├─ fluxo já pendente → outcome pending_flow_exists                              [RF-33/R6]
     └─ ok → engine.Start(budget-edit, key=resourceID, state{Operation,Competence})
              → se valor já veio completo/válido na mensagem: inicia em AwaitingEditConfirm (resumo direto)  [Prefill/eficiência]
  → senão: suspende pedindo o valor da operação
Inbound seguinte → consumer.tryContinueBudgetEdit → continuer.Resume(merge-patch{resumeText})
  → coleta valor (LLM extrai) → suspende no confirm com resumo (novo estado + o que muda)  [RF-24/R8]
Inbound seguinte → sim → executeBudgetEdit → planner.<op> → budgets usecase → repo.UpdatePlan
                  → run auditável fechado, resposta de sucesso                            [RF-25/RF-35]
```

## Design de Implementação

### Interfaces Chave

Porta consumida por `internal/agents` (adapter → use cases de `internal/budgets`):

```go
type BudgetPlanner interface {
    GetMonthlySummary(ctx context.Context, userID uuid.UUID, competence string) (BudgetSummary, error)
    EditBudgetTotal(ctx context.Context, userID uuid.UUID, competence string, newTotalCents int64) error
    RedistributeBudget(ctx context.Context, in DraftBudget) error
    EditCategoryPercentage(ctx context.Context, userID uuid.UUID, competence, rootSlug string, percentage int) error
    // ... métodos existentes preservados
}
```

Novo método de repositório de domínio (não altera `state`/`activated_at`):

```go
type BudgetRepository interface {
    UpdatePlan(ctx context.Context, b entities.Budget) error // UPDATE total_cents, updated_at; upsert allocations; preserva state/activated_at
    GetByUserCompetence(ctx context.Context, userID uuid.UUID, c valueobjects.Competence) (entities.Budget, error)
    // ... métodos existentes
}
```

### Modelos de Dados

Estado do workflow (tipos fechados — DMMF state-as-type):

```go
type BudgetEditOperation int // EditTotal | AdjustCategory | Redistribute (iota+1) + String/Parse/IsValid
type BudgetEditAwaiting  int // AwaitingEditValue | AwaitingEditConfirm
type BudgetEditStatus    int // Active | Completed | Cancelled | Expired

type BudgetEditState struct {
    Operation         BudgetEditOperation `json:"operation"`
    Awaiting          BudgetEditAwaiting  `json:"awaiting"`
    Status            BudgetEditStatus    `json:"status"`
    UserID            uuid.UUID           `json:"userId"`
    Competence        string              `json:"competence"`
    CurrentTotalCents int64               `json:"currentTotalCents"`
    NewTotalCents     int64               `json:"newTotalCents"`
    TargetRootSlug    string              `json:"targetRootSlug"`
    TargetPercentage  int                 `json:"targetPercentage"`
    Allocations       map[string]int      `json:"allocations"`
    ResumeText        string              `json:"resumeText"`
    ResponseText      string              `json:"responseText"`
    RepromptCount     int                 `json:"repromptCount"`
    MessageID         string              `json:"messageId"`
    IncomingMessageID string              `json:"incomingMessageId"`
    SuspendedAt       time.Time           `json:"suspendedAt"`
    Expired           bool                `json:"expired"`
}
```

Matriz de coleta por operação (passo `AwaitingEditValue`):

| Operation | Valor coletado (LLM) | Domínio ao confirmar |
|-----------|----------------------|----------------------|
| `EditTotal` | novo total BRL → `NewTotalCents` | `AllocationDistributor.Distribute(newTotal, BP_atuais)` → `Budget.ChangeTotal` → `UpdatePlan` |
| `AdjustCategory` | categoria + % → `TargetRootSlug`,`TargetPercentage` | `DecideEditCategoryPercentage` → `Distribute(total_atual)` → `Budget.ReplaceDistribution` → `UpdatePlan` |
| `Redistribute` | distribuição (confirm/percent/reais) → `Allocations` | `Distribute(total_atual, BP)` → `Budget.ReplaceDistribution` → `UpdatePlan` |

**Prefill (eficiência):** a tool `edit_budget` carrega, além de `operation` e referência de mês, os valores opcionais já ditos na mensagem que dispara a edição — `newTotalCents` (op `EditTotal`) e `targetRootSlug`+`targetPercentage` (op `AdjustCategory`). Se o valor vier **completo e válido**, o `edit_budget` inicia o workflow já com `Awaiting = AwaitingEditConfirm` (pula a coleta e vai direto ao resumo/confirmação); caso contrário inicia em `AwaitingEditValue`. A op `Redistribute` sempre coleta a distribuição num passo (entrada rica, improvável em uma mensagem). A confirmação HITL nunca é pulada.

Chave de correlação (correção sobre o molde): `BudgetEditKey(resourceID) = resourceID + ":budget-edit"` — **sem competência** (o continuer não conhece a competência no inbound seguinte; ela vive no `State`). Espelha `budget-creation` (key = resourceID). Exclusividade "um fluxo de orçamento por recurso" (R6) NÃO requer guarda nova: a `tryResumeChain` curto-circuita no primeiro resumer `handled` (`whatsapp_inbound_consumer.go:198-201`), então um fluxo suspenso intercepta o inbound antes do agente — impedindo iniciar um segundo fluxo; a corrida de duplo-start é coberta por `ErrRunAlreadyExists` → outcome `pending_flow_exists` (ADR-004).

Métodos de domínio (aceitam Draft e Active — generalização controlada):

```go
func (b *Budget) ChangeTotal(newTotalCents int64, recomputed []Allocation, now time.Time) error // total>0, sum BP=10000
func (b *Budget) ReplaceDistribution(allocs []Allocation, now time.Time) error                  // sum BP=10000 (sem guarda Active)
```

### Endpoints de API
Nenhum. A funcionalidade é conversacional (R3 do PRD; sem HTTP novo).

## Pontos de Integração

- LLM via `agent.Agent.Execute` (call-site sancionada) para extração de total/distribuição/categoria+% — OpenRouter único provider; sem LLM no kernel nem na tool (R-AGENT-WF-001.4).
- Kernel `internal/platform/workflow` (`Engine[BudgetEditState]`, `Store`, `Codec.MergePatch`, `StaleSuspendedReaper`) — consumido, não reimplementado.
- Alertas de limite: nenhum acoplamento novo; recomputados no ciclo do `ThresholdAlertsJob` existente (R5/RF-36).

## Abordagem de Testes

### Testes Unitários
- `Decide*` do workflow (`budget_edit_decisions_test.go`) — total ≤0 reprompt; distribuição ≠10000 reprompt; confirmação sim/não/ambíguo(1x reprompt, 2x cancel); expiração 31min; replay por messageID. Padrão testify/suite table-driven (R-TESTING-001).
- Domínio `budgets`: `ChangeTotal`/`ReplaceDistribution` (invariantes total>0, soma=10000, aceitam Draft e Active); `EditBudgetTotal`/`RedistributeBudget` use cases com mocks do `.mockery.yml` (fake.NewProvider, IIFE por mock).
- `AllocationDistributor`/`DecideEditCategoryPercentage` já cobertos; adicionar casos de Draft.

### Testes de Integração
Recomendados (fronteira Postgres crítica + risco de re-stamp de `activated_at`; ≥2 critérios do template atendidos). `//go:build integration` com testcontainers-go:
- `budget_edit_workflow_integration_test.go` — fluxo ponta a ponta das 3 operações sobre Ativo e Draft; verifica no banco: `total_cents`/`basis_points`/`planned_cents` atualizados, `state` e `activated_at` **inalterados**; concorrência (ErrRunAlreadyExists e guarda cross-workflow); replay idempotente por messageID.

### Testes E2E (real-LLM)
- `budget_edit_e2e_real_llm_test.go` (`//go:build integration`, `RUN_REAL_LLM=1`) — gate ≥0,90 por categoria: roteamento da operação (edit total vs adjust vs redistribute), extração de valores (total BRL, categoria+%, distribuição), clarificação de mês. Golden cases em `application/golden/cases_budget_edit.go`. Zero falso-sucesso: escrita que falha resulta em `StepStatusFailed` sem recurso (RF-35/RF-40).

## Sequenciamento de Desenvolvimento

### Ordem de Build
1. Domínio `budgets`: `Budget.ChangeTotal`/`ReplaceDistribution`, `BudgetRepository.UpdatePlan` (+postgres), commands/DTOs. (base pura, sem dependência do agente)
2. Use cases `budgets`: `EditBudgetTotal`, `RedistributeBudget`; migrar `EditCategoryPercentage` para `UpdatePlan` + Draft; exportar no `module.go`.
3. Porta/adapter `BudgetPlanner` no `internal/agents`.
4. Workflow `budget-edit`: state, decisions, workflow, continuer.
5. Tool `edit_budget` (pré-checagem) + remoção de `adjust_allocation`.
6. Wiring `internal/agents/module.go` + consumer resolver + reaper.
7. Testes unit/integração/real-LLM + gate.

### Dependências Técnicas
- Kernel de workflow e primitivos de agent/memory já disponíveis (nenhuma infra nova).

## Monitoramento e Observabilidade

- Métrica `agents_budget_edit_total` com label `outcome` (`replied|completed|cancelled|expired|error`) e `operation` (`edit_total|adjust_category|redistribute`) — cardinalidade controlada; **proibido** `user_id`/`category_id` (R-TXN-004/R-AGENT-WF-001.5).
- Run auditável por execução: `thread_id`, `run_id`, `workflow=budget-edit`, `operation`, `status`, `duration_ms`, `error` (RF-38).
- Log de falha de escrita com `wamid`/`run_id`; `StepStatusFailed` pesquisável (sem swallow) — RF-35.

## Considerações Técnicas

### Decisões Chave (ADRs)
- [ADR-001](adr-001-nao-aplicar-padrao-workflow-direto.md) — Não aplicar padrão GoF; workflow direto espelhando `budget-creation` (selector `reject`).
- [ADR-002](adr-002-workflow-unificado-operacao-fechada.md) — Workflow unificado `budget-edit` com `Operation` fechado; uma tool `edit_budget`; remoção da `adjust_allocation` imediata (fecha G-03).
- [ADR-003](adr-003-updateplan-editar-draft-ativo.md) — `UpdatePlan` no repositório + `ChangeTotal`/`ReplaceDistribution` no agregado, editando Draft e Active sem re-stamp de `activated_at`.
- [ADR-004](adr-004-chave-resource-e-exclusividade-fluxo.md) — Chave por `resourceID` + guarda de exclusividade "um fluxo de orçamento por recurso" (R6) + pré-checagem existência/auto-draft (R2/RF-10/RF-13).
- [ADR-005](adr-005-robustez-no-false-success-idempotencia.md) — No-false-success (`StepStatusFailed`), idempotência por `wamid`, alertas no ciclo do job.

### Riscos Conhecidos
- R-1: `EditCategoryPercentage` atual re-stampa `activated_at` a cada edição (usa `repo.Activate`). Mitigação: migrar para `UpdatePlan`; teste de integração assertando `activated_at` inalterado.
- R-2: dois fluxos de orçamento (create+edit) coexistirem para o mesmo recurso. Verificação adversarial (ADR-004): impossível — a `tryResumeChain` curto-circuita no primeiro fluxo suspenso, que intercepta o inbound antes do agente; corrida de duplo-start coberta por `ErrRunAlreadyExists`. Nenhuma guarda extra necessária.
- R-3: editar Draft com `ReplaceDistribution` exige remover a guarda `Active`; risco de afrouxar invariante em Active. Mitigação: manter invariante soma=10000/total>0 em ambos; testes cobrindo Active e Draft.
- R-4: brittleness de roteamento LLM entre 3 operações. Mitigação: gate real-LLM ≥0,90 por categoria; golden cases; instrução por exemplo se necessário.
- R-5: total abaixo do gasto (E1) — permitir e sinalizar; sem bloqueio no domínio; alertas mostram >100% no ciclo do job.

### Conformidade com Padrões
- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): tool fina, roteamento por registry, estados fechados, LLM só nas call-sites sancionadas, Run auditável, pending state antes de confirmar.
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001): kernel intocado; sem domínio/SQL/LLM no kernel.
- `.claude/rules/go-adapters.md` (R-ADAPTER-001): zero comentários; adapter fino; SQL só no postgres.
- `.claude/rules/transactions-workflows.md` (R-TXN-004): cardinalidade de métricas.
- `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001): `Validate()` nos DTOs novos.
- `.claude/rules/go-testing.md` (R-TESTING-001): testify/suite table-driven.

### Arquivos Relevantes e Dependentes
- Moldes: `internal/agents/application/workflows/budget_creation_workflow.go`, `budget_creation_state.go`, `budget_creation_decisions.go`; `application/usecases/budget_creation_continuer.go`; `application/tools/create_budget.go`; `module.go`; `infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`.
- Domínio: `internal/budgets/domain/entities/budget.go` (Activate:132-150, RebalanceAllocations:116-130), `domain/services/allocation_distributor.go`, `domain/services/category_percentage_workflow.go`, `application/usecases/edit_category_percentage.go`, `infrastructure/repositories/postgres/budget_repository.go` (Activate:96-132, GetByUserCompetence:31-61), `module.go`.

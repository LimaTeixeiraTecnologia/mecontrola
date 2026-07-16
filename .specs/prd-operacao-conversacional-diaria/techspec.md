<!-- spec-hash-prd: 490527377919d28e532eb4d17fb26628b01b3c86a1dd2824772e4b2d90bf5087 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Operação Conversacional Diária do MeControla

## Resumo Executivo

Esta especificação descreve a reescrita da camada conversacional do dia a dia em `internal/agents`, preservando os primitivos de plataforma (`internal/platform/{agent,workflow,memory,llm,scorer,tool}`) e alterando os módulos de domínio apenas de forma aditiva. O agente `mecontrola-agent` continua único, resolvendo ferramentas por tool-calling do LLM (OpenRouter), mas a orquestração durável passa a ser organizada em **poucos workflows por forma de interação** (escrita com slot-filling e confirmação, confirmação destrutiva, edição de orçamento, cartão), a retomada de workflows suspensos passa a um **lookup único do run suspenso por (resource, thread) com despacho por `WorkflowRegistry`** (elimina a cadeia hardcoded de 5 continuers), e todas as mensagens ao usuário passam a vir de um **catálogo central determinístico de tom de voz** com frase motivacional rotacionada, repassadas verbatim pelo guard `verbatim_relay`.

As lacunas de produto são fechadas com adições cirúrgicas: novo usecase `EditBudgetTotal` em `internal/budgets` (reescala proporcional reutilizando o `AllocationDistributor` existente); extensão aditiva do enum `PaymentMethod` em `internal/transactions` (carteiras digitais, cheque, DOC e transferência); nova query de busca de candidatos de edição (por valor e/ou descrição, mês vigente, recência); extensão da ferramenta de edição para categoria e forma de pagamento; enriquecimento do evento `TransactionUpdated` com a subcategoria e um novo consumer `transactions.transaction.updated.v1` no budgets para reconciliar o resumo por categoria após edição; ferramentas de leitura estáticas para cancelamento e suporte; ferramenta de detalhe por categoria; e alteração do objetivo via `WorkingMemory`. A liberação é condicionada à suíte de jornada com LLM real cobrindo os 13 fluxos com score maior ou igual a 0,90 por fluxo e zero falso-sucesso, além dos gates de governança. O corte do legado drena runs suspensos por janela de graça.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes NOVOS (todos em `internal/agents`, salvo indicação de módulo):

- **Catálogo central de mensagens** (`internal/agents/application/messages/`): funções puras que produzem os blocos verbatim do documento (confirmação de despesa/receita, sucesso, esclarecimento, resumo por categoria e geral, informacionais) e sorteiam a frase motivacional por cenário a partir de listas fixas. Fonte única; substitui `buildConfirmSummary`/`buildWriteSuccessText`/`buildConfirmQuestion` espalhados. O tom é ancorado em DUAS fontes combinadas (ADR-003): a seção de Identidade/Tom já implementada e validada em produção em `mecontrola_agent.go` (base) e os blocos verbatim do documento fornecido (sobreposição normativa); onde houver conflito, o texto verbatim do documento prevalece.
- **Dispatcher de resume** (`resume_dispatcher` no consumer + `SuspendedRunIndex`): resolve, por `(resourceID, threadID)`, qual workflow durável está suspenso e despacha a retomada via `agent.WorkflowRegistry`. Substitui `tryResumeChain` hardcoded.
- **Workflows consolidados** (`internal/agents/application/workflows/`): `transaction-write` (registro de despesa/receita/recorrência e edição, com slot-filling e confirmação), `destructive-confirm` (excluir cartão/recorrência), `budget-manage` (criação retroativa, alterar total e alterar distribuição do orçamento), `card-manage` (cadastrar/editar cartão), `goal-edit` (alterar objetivo). Cada um é uma `workflow.Definition[S]` com estado `S` fechado próprio.
- **Ferramentas novas** (`internal/agents/application/tools/`): `edit_budget_total`, `category_detail`, `cancel_plan_info`, `support_info`, `edit_goal`; e extensão da `edit_entry` (categoria + forma de pagamento) e `search_edit_candidates`.
- **budgets**: usecase `EditBudgetTotal`, entidade `Budget.ChangeTotal`, DTO `EditBudgetTotalInput`, command `NewEditBudgetTotalCommand`, método `BudgetPlanner.EditBudgetTotal` (interface do agente) + adapter; consumer `transaction_updated_consumer`.
- **transactions**: valores novos no enum `PaymentMethod`; enriquecimento de `TransactionUpdated` com subcategoria; repositório `SearchEditCandidates` + usecase; extensão do comando/DTO de edição do agente.
- **scorers**: `verbatim_tone_adherence` (code-based) e, quando aplicável, um scorer LLM-judged de tom; casos golden para os 13 fluxos.

Componentes MODIFICADOS:

- `whatsapp_inbound_consumer.go` (roteamento de resume), `mecontrola_agent.go` (prompt e catálogo de ferramentas reescritos, referenciando o catálogo de mensagens e as tools novas), `module.go` (wiring de engines/defs/tools/continuer único/reapers/registry), `edit_entry.go` + `register_attempt.go` (campos categoria/forma de pagamento).

### Fluxo de Dados

Inbound: `WhatsApp -> buildWhatsAppAgentRoute -> outbox (agents.whatsapp.inbound.v1) -> WhatsAppInboundConsumer.Handle -> resumeDispatcher.Resolve(resourceID, threadID)`. Se houver run suspenso, `Engine.Resume(def, key, mergePatch)` do workflow dono; senão `HandleInbound -> AgentRuntime.Execute -> Agent.Execute (loop tool-calling) -> tool -> binding -> usecase de domínio`. Resposta ao usuário sempre determinística: `ResponseText` do workflow (no resume) ou `verbatim_relay` do texto da tool (no fluxo LLM). Reconciliação de resumo: `transactions -> TransactionCreated/Updated/Deleted (outbox) -> budgets consumers -> budgets_expenses -> GetMonthlySummary`.

## Design de Implementação

### Interfaces Chave

Novo método na porta de orçamento do agente (`internal/agents/application/interfaces/budget_planner.go`):

```go
type BudgetPlanner interface {
    // ...métodos existentes...
    EditBudgetTotal(ctx context.Context, userID uuid.UUID, competence string, totalCents int64) error
    CategoryDetail(ctx context.Context, userID uuid.UUID, competence, rootSlug string) (CategoryDetail, error)
}
```

Novo mutador na entidade de orçamento (`internal/budgets/domain/entities/budget.go`):

```go
func (b *Budget) ChangeTotal(newTotalCents int64, allocations []Allocation, now time.Time) error {
    if !b.IsActive() {
        return ErrBudgetNotActive
    }
    if newTotalCents <= 0 {
        return ErrBudgetTotalMustBePositive
    }
    if sumBasisPoints(allocations) != 10000 {
        return ErrBudgetAllocationSumMustBe10000
    }
    b.totalCents = newTotalCents
    b.allocations = allocations
    b.updatedAt = now
    return nil
}
```

Novo usecase de orçamento (espelha `EditCategoryPercentage`; `internal/budgets/application/usecases/edit_budget_total.go`):

```go
type EditBudgetTotal struct {
    factory interfaces.RepositoryFactory
    uow     uow.UnitOfWork
    o11y    observability.Observability
}

func (uc *EditBudgetTotal) Execute(ctx context.Context, in input.EditBudgetTotalInput) (output.BudgetOutput, error)
// span "budgets.usecase.edit_budget_total"; in.Validate(); NewEditBudgetTotalCommand;
// uow.Do: GetByUserCompetence -> guard IsActive -> AllocationDistributor.Distribute(newTotal, basisPointsAtuais)
//         -> Budget.ChangeTotal -> budgets.Activate(persistência UPDATE total_cents + upsert allocations)
```

Nova porta de busca de candidatos (`internal/agents/application/interfaces/transactions_ledger.go`, aditivo):

```go
type TransactionsLedger interface {
    // ...
    SearchEditCandidates(ctx context.Context, userID uuid.UUID, q EditCandidateQuery) ([]Entry, error)
}

type EditCandidateQuery struct {
    AmountCents int64   // 0 quando não informado
    Term        string  // "" quando não informado
    RefMonth    string  // mês vigente por padrão
    Limit       int     // top-N pequeno (ex.: 5)
}
```

Novo método de repositório (`internal/transactions/application/interfaces/transaction_repository.go`, aditivo):

```go
SearchEditCandidates(ctx context.Context, userID uuid.UUID, amountCents int64, term string, refMonth valueobjects.RefMonth, limit int) ([]entities.Transaction, error)
// WHERE user_id AND deleted_at IS NULL AND ref_month = $ AND (amount_cents = $amount OR description ILIKE '%'||$term||'%')
// ORDER BY created_at DESC LIMIT $ ; amount/term opcionais (só um pode vir).
```

Catálogo de mensagens (funções puras, sem IO; `internal/agents/application/messages/catalog.go`):

```go
func ExpenseConfirmationBlock(v ConfirmationView) string   // "✅ Encontrei este lançamento:\n\n💰 Valor: ...\n💳 Pagamento: ...\n📂 Categoria: ...\n\nPosso registrar?"
func IncomeConfirmationBlock(v ConfirmationView) string     // "✅ Encontrei esta entrada:\n\n💰 Valor: ...\n📥 Origem: ...\n\nPosso registrar?"
func WriteSuccess(kind WriteKind, seed MotivationSeed) string // "Prontinho! ✅\n\n...\n\n<frase motivacional rotacionada> 💚"
func CategorySummaryBlock(v CategoryView) string
func GeneralSummaryBlock(v GeneralView) string
func CancelPlanInfo() string
func SupportInfo() string
```

A rotação da frase motivacional é determinística e testável: `MotivationSeed` deriva de um valor estável do estado (ex.: `messageID`) para evitar `Math.random`/`time.Now` no caminho de decisão, respeitando a proibição de fontes não determinísticas em `Decide*`.

### Modelos de Dados

- `PaymentMethod` (`internal/transactions/domain/valueobjects/payment_method.go`): adicionar `PaymentMethodTransferencia`, `PaymentMethodApplePay`, `PaymentMethodGooglePay`, `PaymentMethodPicPay`, `PaymentMethodMercadoPago`, `PaymentMethodCheque` (novos `iota`), atualizar `ParsePaymentMethod`, `String()`, e o limite superior de `PaymentMethodFromInt`. Desbloquear `doc` em `ParsePaymentMethodForCreate` (remoção do `ErrPaymentMethodDocReadOnly` no caminho de criação). Todos os novos valores são não-cartão: seguem o ramo sem fatura em `DecideCreate`/`DecideUpdate` automaticamente.
- `TransactionUpdated` (`internal/transactions/domain/entities/events.go`): adicionar `SubcategoryID uuid.UUID` (e manter `AmountCents`, `RefMonth`). O producer e o payload JSON do outbox passam a serializar a subcategoria.
- `budgets_expenses`: sem mudança de schema; o novo consumer de updated reutiliza `UpsertExpense` (que resolve `RootSlug` a partir da subcategoria e move valor/competência via `existing.Edit`).
- Estados de workflow (`S`) fechados por workflow: `TransactionWriteState`, `DestructiveConfirmState`, `BudgetManageState`, `CardManageState`, `GoalEditState`, cada um com enums fechados de `Awaiting*`/`OperationKind` (DMMF state-as-type), sem string livre. `BudgetManageState.OperationKind` inclui `create_retroactive`, `edit_total`, `edit_distribution`; `CardManageState.OperationKind` inclui `create_card`, `edit_card`; `TransactionWriteState.OperationKind` inclui `register_expense`, `register_income`, `edit_entry`, `create_recurrence`.

### Endpoints de API

Nenhum endpoint HTTP novo. Todas as operações do dia a dia entram pelo canal WhatsApp (evento `agents.whatsapp.inbound.v1`) e são servidas por tools/workflows do agente, seguindo o precedente de que edições de plano não têm rota HTTP.

## Pontos de Integração

- **OpenRouter** (`internal/platform/llm`): único provider; usado no loop de tool-calling do agente e no scorer LLM-judged de tom. Sem fallback chain nem chamada HTTP direta.
- **Outbox `transactions -> budgets`**: contrato de evento estendido aditivamente (`TransactionUpdated` com subcategoria) e novo handler `transactions.transaction.updated.v1` no budgets. Idempotência por `event_id` conforme o padrão de outbox do repositório.
- **Kiwify**: a ferramenta `cancel_plan_info` apenas retorna texto informacional; não chama a API da Kiwify. O cancelamento efetivo permanece pelo webhook de billing já existente.

## Abordagem de Testes

### Testes Unitários

- **Domínio puro**: `Budget.ChangeTotal` (guardas Active/positivo/soma 10000); reescala via `AllocationDistributor` (soma de centavos == total, banker's rounding); `ParsePaymentMethod`/`String()` para os novos valores e `doc` desbloqueado; enriquecimento de `TransactionUpdated`. Sem mocks.
- **Usecases** (padrão canônico testify/suite, whitebox, `fake.NewProvider()`, mocks mockery, IIFE por mock — R-TESTING-001): `EditBudgetTotal` (sucesso, orçamento inativo, competência inexistente, falha de infra); `SearchEditCandidates`; `UpsertExpense` no caminho de update por evento updated; edição estendida em `RegisterAttempt.EditEntry` (categoria/forma de pagamento seed).
- **Workflows** (funções `Decide*` puras): transições de `transaction-write`, `budget-manage`, `card-manage`, `goal-edit`; TTL, reprompt, expiração, replay, guarda anti-falso-sucesso (`DecidePostWrite`).
- **Catálogo de mensagens**: cada bloco verbatim comparado ao texto do documento; rotação motivacional determinística por seed.
- **Guards/roteamento**: `resumeDispatcher` resolve o workflow correto por `(resource, thread)`; `verbatim_relay` repassa o campo `message` das tools informacionais.

### Testes de Integração

Critérios do template: o projeto tem fronteiras de IO críticas (Postgres: workflow store, budgets_expenses, outbox) e há histórico de falso-verde conversacional — logo, integration tests são recomendados. Usar testcontainers-go com build tag `//go:build integration`.

- Reconciliação `transactions.updated -> budgets`: criar transação, editar categoria/valor pelo fluxo, verificar `budgets_expenses` e `GetMonthlySummary` movendo o valor entre categorias raiz.
- `EditBudgetTotal`: persistência do novo total + allocations reescaladas somando o novo total.
- Resume durável: suspender um `transaction-write`, reenviar mensagem, confirmar persistência idempotente (`agents_write_ledger`).

### Testes E2E

- **Suíte de jornada golden** (`internal/agents/application/golden/`): novos casos cobrindo os 13 fluxos, cada um com `ResponseProperty`/`ResponseDescribe`. Gate `TestGoldenSetGate` mantém o piso 0,90 por categoria (repetição 3x por caso), agora estendido para as categorias novas (orçamento total, objetivo, cancelamento, suporte, detalhe por categoria, resumo geral).
- **Gate real-LLM** (`//go:build integration`, `RUN_REAL_LLM=1`, `OPENROUTER_API_KEY` e `AGENT_HARNESS_MODEL` do ambiente): executa os 13 fluxos com o provider real; release exige score maior ou igual a 0,90 por fluxo e zero falso-sucesso. As variáveis vêm do ambiente (o repositório não carrega `.env`).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Domínio aditivo** (sem dependência de agente): enum `PaymentMethod` estendido + `doc` desbloqueado; `TransactionUpdated` enriquecido; `Budget.ChangeTotal` + `EditBudgetTotal` usecase + DTO/command; `SearchEditCandidates` repo+usecase; consumer `transaction_updated` no budgets. Testes unitários e de integração destes primeiro.
2. **Catálogo central de mensagens**: funções puras + testes verbatim.
3. **Workflows consolidados** e seus estados fechados, consumindo o catálogo de mensagens e os usecases; `Decide*` puros com testes.
4. **Roteamento de resume**: `SuspendedRunIndex` + `resumeDispatcher` + `WorkflowRegistry`; substituição do `tryResumeChain`.
5. **Ferramentas** (edit_budget_total, category_detail, cancel_plan_info, support_info, edit_goal, edit_entry estendida, search_edit_candidates) + bindings + wiring em `module.go` (engines, defs, registry, reapers, `WithWriteToolSet`).
6. **Prompt** do `mecontrola-agent` reescrito referenciando o catálogo e as tools novas.
7. **Scorers** de aderência verbatim/tom + casos golden dos 13 fluxos.
8. **Gate real-LLM** e ajustes até 0,90 por fluxo; **cutover** com drenagem de runs suspensos.

### Dependências Técnicas

- Postgres (workflow store, budgets_expenses, outbox) para integração. OpenRouter (`OPENROUTER_API_KEY`) para o gate real-LLM. Nenhuma nova infraestrutura.

## Remoção Total do Legado (executada no cutover)

A reescrita elimina completamente os workflows e continuers do dia a dia. A remoção física ocorre no passo de cutover (após o novo substituir e a janela de graça do ADR-005 drenar os runs suspensos), garantindo build e produção íntegros durante a transição. Inventário exato (confrontado com o codebase), em três classes:

### Remover totalmente (arquivos deletados)

Substrato substituído pelos novos workflows/dispatcher; a lógica de negócio relevante é reencarnada (não copiada) nos novos artefatos:

- `internal/agents/application/workflows/pending_entry_workflow.go`, `pending_entry_state.go`, `pending_entry_decisions.go` -> reencarnados em `transaction-write` + catálogo de mensagens. Nota de execução: `category_resolution.go` e `pending_category_candidate.go` foram **preservados e reaproveitados** pelo `transaction-write` novo (apesar do nome `PendingCategoryCandidate`), portanto NÃO foram removidos — ver `deployment/scripts/deadcode-agent-allowlist.txt`.
- `internal/agents/application/workflows/destructive_confirm_workflow.go`, `confirm_state.go` -> reencarnados em `destructive-confirm` (novo).
- `internal/agents/application/workflows/card_create_confirm_workflow.go`, `card_create_state.go`, `card_create_decisions.go` -> reencarnados em `card-manage`.
- `internal/agents/application/workflows/budget_creation_workflow.go`, `budget_creation_state.go`, `budget_creation_decisions.go` -> reencarnados em `budget-manage` (operação `create_retroactive`).
- `internal/agents/application/usecases/pending_entry_continuer.go`, `destructive_confirm_continuer.go`, `card_create_confirm_continuer.go`, `budget_creation_continuer.go` -> substituídos pelo resume dispatcher único (ADR-002).
- `internal/agents/application/usecases/register_attempt.go` -> a iniciação de escrita migra para os tools do `transaction-write`.

### Modificar (não deletar)

- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`: remover `tryResumeChain` e os 5 wrappers de continuer; ligar o resume dispatcher único.
- `internal/agents/module.go`: rewire de engines/defs/tools/registry/reapers/`WithWriteToolSet`; remover o wiring dos continuers legados e das defs antigas; registrar os novos workflows e o `SuspendedRunIndex`.
- `internal/agents/application/agents/mecontrola_agent.go`: prompt reescrito consumindo o catálogo de mensagens e as tools novas.
- Tools reapontadas aos novos workflows/estados: `register_expense.go`, `register_income.go`, `create_recurrence.go`, `edit_entry.go` (estendida com categoria/forma de pagamento) -> `transaction-write`; `delete_entry.go`, `delete_recurrence.go`, `update_recurrence.go` -> `destructive-confirm`; `create_card.go`, `update_card.go` -> `card-manage`; `create_budget.go` -> `budget-manage`; `adjust_allocation.go` + novo `edit_budget_total` -> `budget-manage`.

### Preservar (não é legado do dia a dia ou infra compartilhada reutilizada)

- Onboarding: `internal/agents/application/workflows/onboarding_workflow.go`, `internal/agents/application/usecases/resolve_onboarding_or_agent.go` (independentes dos workflows do dia a dia; confirmado sem referência cruzada).
- Idempotência e utilitários: `internal/agents/infrastructure/persistence/write_ledger_repository.go`, `internal/agents/application/usecases/{write_ledger.go,idempotent_write.go,register_entry.go,purge_ledger.go}`, `internal/agents/application/workflows/transient_error.go` (`IsTransient`/backoff reutilizados pelos novos writes).
- Runtime: `internal/agents/application/usecases/{handle_inbound.go,run_close.go}`.
- Tools de leitura: `query_month.go`, `query_plan.go`, `query_card_invoice.go`, `get_transaction.go`, `search_transactions.go`, `list_cards.go`, `get_card.go`, `resolve_card.go`, `count_cards.go`, `best_purchase_day.go`, `list_recurrences.go`, `list_categories.go`, `classify_category.go`, `suggest_allocation.go` (novas de leitura `category_detail`, `cancel_plan_info`, `support_info` adicionadas).
- Guards: `verbatim_relay.go`, `empty_answer.go`, `internal_terms.go`, `success_without_tool.go`, `card_provenance.go`, `multi_item.go`, `onboarding_initial.go`, `create_card_shortcut.go`, `list_cards_shortcut.go`, `register_income_shortcut.go`.

Critério de conclusão do cutover: `grep` sem resultados por qualquer referência aos símbolos removidos (`PendingEntry*`, `CardCreate*`, `BudgetCreation*`, `DestructiveConfirm*` antigos, `register_attempt`) fora dos testes, e build/vet/test/lint verdes.

## Monitoramento e Observabilidade

- Métricas existentes preservadas: `agents_pending_entry_false_success_total` (generalizada para os workflows de escrita), `agent_tool_invocations_total`, `agent_guard_decisions_total`, `agent_llm_*`, `scorer_runs_total`. Labels restritos a enums fechados (`agent_id`, `channel`, `workflow`, `status`, `tool`, `outcome`); proibido `user_id`/`correlation_key`/`category_id`.
- Novo consumer de budgets segue o padrão dos consumers existentes: counters `..._decode_failed_total`, `..._skipped_total`, spans por handle. `EditBudgetTotal` segue os irmãos: apenas span + log de falha (sem métrica nova, sem evento).
- Reapers de runs suspensos por workflow (janela stale) para não vazar estado; incluídos no wiring de jobs.
- Scorers de aderência verbatim/tom observados de forma amostrada (`AlwaysSample`) via `ScoringHooks`.

## Considerações Técnicas

### Decisões Chave

Cada decisão material possui ADR dedicada nesta pasta:

- ADR-001 — Topologia de workflows por forma de interação (`adr-001-workflow-topology.md`).
- ADR-002 — Roteamento de resume por lookup único + `WorkflowRegistry` (`adr-002-resume-routing-registry.md`).
- ADR-003 — Catálogo central de mensagens determinísticas de tom de voz (`adr-003-message-catalog.md`).
- ADR-004 — Reconciliação de edição: enriquecer `TransactionUpdated` + consumer `updated` no budgets (`adr-004-edit-reconciliation.md`).
- ADR-005 — Cutover com drenagem de runs suspensos por janela de graça (`adr-005-cutover-drain.md`).
- ADR-006 — Extensão aditiva do enum `PaymentMethod` e desbloqueio de `doc` (`adr-006-payment-method-extension.md`).
- ADR-007 — Busca de candidatos de edição por valor e/ou descrição (`adr-007-edit-candidate-search.md`).

Decisões adicionais registradas inline (seguem padrão existente, sem alternativa arquitetural relevante): `EditBudgetTotal` reutiliza `AllocationDistributor` (arredondamento banker's + distribuição de resto garante soma == total) e persiste via `budgets.Activate` (mecanismo já usado por `EditCategoryPercentage`); alteração de objetivo é leitura-modificação-escrita da seção `## Objetivo Financeiro` na `WorkingMemory` via usecases de memory, preservando as demais seções; edição não altera cartão/parcelas (o guard `guardPaymentMethodMigration` bloqueia cruzar a fronteira de crédito, e a mensagem de bloqueio vem do catálogo).

### Mapeamento Requisito -> Decisão -> Teste

| RF | Decisão técnica | Teste |
|----|-----------------|-------|
| RF-01 | LLM extrai; validação determinística em smart constructors/DTO | golden 13 fluxos; unit DTO Validate |
| RF-02 | Conversão a centavos pelo LLM; `NewMoney`/smart constructor rejeita <=0 | unit command; golden valores por extenso |
| RF-03 | ADR-006 enum aditivo + `doc` liberado; cartão nomeado -> apelido | unit `ParsePaymentMethod`/`String`; golden formas de pagamento |
| RF-04 | Confirmação universal nos workflows `*-confirm` antes de gravar | unit `Decide*`; integração resume; golden |
| RF-05 | ADR-003 catálogo central + `verbatim_relay` | unit catálogo verbatim; scorer `verbatim_tone_adherence` |
| RF-06 | Idempotência `wamid+item_seq` via `agents_write_ledger` (preservado) | integração write duplicado |
| RF-07 | `DecidePostWrite` + `StepStatusFailed` + métrica falso-sucesso | unit `DecidePostWrite`; scorer `no_hallucination` |
| RF-08 | ADR-002 registry; estados fechados por workflow | unit resumeDispatcher; gate sem `switch intent.Kind` |
| RF-09 | `ErrRunAlreadyExists` -> mensagem de pendência do catálogo | unit; integração pendência ativa |
| RF-10 | Estado de espera fechado no `Snapshot`; resume por merge-patch; TTL/reaper/reprompt | unit `Decide*`; integração TTL |
| RF-11 | Slot-filling pede só o slot ausente | unit slot; golden |
| RF-12 | Guard `multi_item` mantido | unit invariante `TestInvariantNoFalseMultiItem` |
| RF-13 | Workflow `transaction-write` (despesa) + bloco de confirmação de despesa | unit; golden despesa |
| RF-14 | `transaction-write` (receita) + bloco de entrada com origem | unit; golden receita |
| RF-15 | ADR-007 busca de candidatos; edit_entry estendida (categoria/forma) | unit `SearchEditCandidates`; integração edição |
| RF-16 | Recorrência como operação do `transaction-write` | unit; golden recorrência |
| RF-17 | `EditBudgetTotal` + `Budget.ChangeTotal` + reescala | unit; integração persistência |
| RF-18 | `budget-manage` distribuição via `EditCategoryPercentage` | unit; golden distribuição |
| RF-19 | `card-manage` (cadastro) | unit; golden cartão |
| RF-20 | `card-manage` (edição) com confirmação universal | unit; golden edição cartão |
| RF-21 | `destructive-confirm` (excluir cartão) + aviso de impacto | unit `BuildImpactNote`; golden |
| RF-22 | `goal-edit` sobre `WorkingMemory` (read-modify-write da seção) | unit parse/rewrite; golden objetivo |
| RF-23 | Tool `cancel_plan_info` verbatim (sem billing) | unit catálogo; golden cancelamento |
| RF-24 | Tool `support_info` verbatim (e-mail/24h) | unit catálogo; golden suporte |
| RF-25 | Tool `category_detail` (subcategoria->raiz, lançamentos + planejado/gasto/disponível) | unit; integração; golden |
| RF-26 | Resumo geral via `query_plan`/`GetMonthlySummary` + bloco geral | unit; golden resumo geral |
| RF-27 | Substituição da camada conversacional; domínio aditivo | gates governança; build |
| RF-28 | ADR-005 drenagem de runs suspensos no cutover | runbook de cutover; reaper |
| RF-29 | Gate real-LLM 0,90/fluxo + 0 falso-sucesso + gates governança | `TestGoldenSetGate`; gate real-LLM |
| RF-30 | KPIs por scorers/métricas | scorers; painel |
| RF-31 | `auth.Principal` obrigatório na escrita; onboarding pré-condição | unit binding; integração sem principal |

### Riscos Conhecidos

- **Regressão de invariantes ao reescrever workflows**: mitigação — reencarnar (não copiar) idempotência, guarda de falso-sucesso, reclassificação por kind, TTL/reaper, reprompt; cobertura por testes unitários de `Decide*` e integração de write duplicado antes do cutover.
- **Drift do evento `TransactionUpdated`**: enriquecimento aditivo com versão de payload; consumer novo tolera ausência de subcategoria (skip como no consumer de created) para eventos antigos em trânsito no cutover.
- **Bloqueio de migração de forma de pagamento em edição** (crédito<->não-crédito): comportamento esperado do domínio; a mensagem de bloqueio do catálogo evita falso-sucesso e orienta o usuário.
- **Flakiness do gate real-LLM**: piso 0,90 (não 1,00) por fluxo, repetição 3x por caso e temperatura 0 reduzem variância; invariantes binários (confirmação/falso-sucesso) avaliados à parte, de forma determinística.
- **Desambiguação de resume**: o lookup por `(resource, thread)` assume no máximo um run suspenso por thread; o wiring garante que a abertura de um novo workflow não coexista com outro suspenso na mesma thread (a pendência ativa bloqueia — RF-09).

### Conformidade com Padrões

- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): roteamento por registry, tool fina, estados fechados, LLM só nas call-sites sancionadas, Run auditável, HITL com estado de espera persistido antes da confirmação e resume por merge-patch.
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001): kernel `internal/platform/workflow` intocado; sem domínio no kernel.
- `.claude/rules/go-adapters.md` (R-ADAPTER-001): adaptadores finos e zero comentários; tools delegam a usecase.
- `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001): `Validate()` em cada input DTO novo, logo após abertura do span.
- `.claude/rules/transactions-workflows.md` (R-TXN-WORKFLOWS-001): regra de domínio só em `Decide*`; producers só mapeiam evento; cardinalidade controlada.
- `.claude/rules/go-testing.md` (R-TESTING-001): suíte canônica testify.

### Arquivos Relevantes e Dependentes

- Agentes/plataforma: `internal/agents/module.go`, `internal/agents/application/agents/mecontrola_agent.go`, `internal/agents/application/agents/guard_chain.go`, `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`, `internal/platform/{agent,workflow,memory,llm,scorer,tool}`.
- Workflows/tools/usecases atuais a substituir/estender: `internal/agents/application/workflows/*`, `internal/agents/application/tools/*`, `internal/agents/application/usecases/register_attempt.go`, `internal/agents/application/interfaces/*`, `internal/agents/infrastructure/binding/*`, `internal/agents/infrastructure/persistence/write_ledger_repository.go` (preservado).
- budgets: `internal/budgets/domain/entities/budget.go`, `internal/budgets/domain/services/allocation_distributor.go`, `internal/budgets/application/usecases/{edit_category_percentage.go,activate_budget.go,get_monthly_summary.go,upsert_expense.go}`, `internal/budgets/infrastructure/repositories/postgres/budget_repository.go`, `internal/budgets/infrastructure/messaging/database/consumers/transaction_created_consumer.go`, `internal/budgets/module.go`.
- transactions: `internal/transactions/domain/valueobjects/payment_method.go`, `internal/transactions/domain/entities/events.go`, `internal/transactions/domain/services/transaction_workflow.go`, `internal/transactions/application/usecases/{update_transaction.go,search_transactions.go,helpers.go}`, `internal/transactions/application/interfaces/transaction_repository.go`, `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go`, `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher.go`.
- Testes/gate: `internal/agents/application/golden/*`, `internal/agents/application/scorers/*`.

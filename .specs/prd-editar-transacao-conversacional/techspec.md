<!-- spec-hash-prd: 9c29a68725558f4e41de01882ffa1a8dc11da2e4a8350505f5761bd9964adcbd -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Editar Transação pela Conversa (paridade total)

- PRD: `.specs/prd-editar-transacao-conversacional/prd.md` (spec-version 1, 32 RFs)
- ADRs: `adr-001-budget-reconcile-updated-event.md`, `adr-002-edit-canonical-pending-entry.md`, `adr-003-optimistic-conflict-and-diff.md`, `adr-004-target-resolution-last-entry.md`
- Skills obrigatórias aplicadas: `go-implementation`, `mastra`, `domain-modeling-production`, `design-patterns-mandatory`

## Resumo Executivo

A edição conversacional de transação já é **parcialmente viva**: o `edit_entry` tool roteia por `RegisterAttempt.EditEntry`, que monta `PendingEntryState{OperationKind: PendingOpEditEntry, TargetTransactionID, TargetVersion}` e inicia o `pending-entry` workflow, suspendendo em `AwaitingSlotConfirmation` e gravando via `ledger.UpdateTransaction(buildRawUpdate(state))` com idempotência `operation="edit_entry"`. O trabalho desta feature é **fechar a paridade e a correção** sobre esse caminho vivo, sem introduzir novo padrão estrutural: (1) alargar o schema da tool `edit_entry` e propagar os campos novos (forma de pagamento, cartão, parcelas, categoria/subcategoria, direção) por `EditEntryCommand → PendingEntryState → buildRawUpdate`; (2) resolver o alvo de forma determinística com uma read tool nova; (3) enriquecer o evento `TransactionUpdated` (aditivo) com categoria e o conjunto de parcelas antigo/novo; (4) adicionar um consumidor `transactions.transaction.updated.v1` em `internal/budgets` que delega a um novo usecase de reconciliação (delete-antigos + upsert-atuais), fechando o consumo fantasma de parcelas.

A estratégia é **reuso máximo**: todos os padrões necessários (registry/Strategy-map, Factory Function, Adapter, Facade) já estão inline no repositório; nenhum novo GoF é introduzido (selector = reject). Regra de negócio permanece exclusivamente em `TransactionWorkflow.DecideUpdate` (puro); validação em smart constructors e DTO `Validate()`; adapters finos e sem comentários; estados como tipos fechados (DMMF state-as-type). O caminho legado `destructive_confirm.OpEditEntry` é declarado não-canônico para edição e permanece intocado (ver ADR-002).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Modificados — `internal/agents`:**
- `application/tools/edit_entry.go` — alargar `EditEntryInput`/schema (novos campos) e o `exec`.
- `application/usecases/register_attempt.go` — `EditEntryCommand` + `EditEntry` passam a receber e propagar os campos novos; re-resolução de categoria quando categoria ou direção mudam.
- `application/workflows/pending_entry_state.go` — novos campos no `PendingEntryState` (valores atuais para o diff antes→depois; campos-alvo de edição; flags de campo alterado).
- `application/workflows/pending_entry_workflow.go` — `buildRawUpdate` mapeia os campos novos; `buildConfirmSummary`/novo `buildEditConfirmSummary` produz o diff antes→depois; tratamento de conflito de `version` no `executeWithIdempotency`/resume.
- `module.go` — registrar a nova read tool; incluir `edit_entry` em `WithWriteToolSet`.
- `infrastructure/binding/transactions_ledger_adapter.go` — novo método de leitura para o "último lançamento"/recentes (delegando ao ledger).

**Novos — `internal/agents`:**
- `application/tools/get_last_entry.go` (ou `list_recent_entries.go`) — read tool fina para resolução determinística do alvo (ADR-004).

**Modificados — `internal/transactions`:**
- `domain/entities/events.go` — `TransactionUpdated` recebe `CategoryID`, `SubcategoryID`, `Installments []CardPurchaseInstallment`, `PreviousItemIDs []uuid.UUID` (aditivo, ADR-001).
- `domain/services/transaction_workflow.go` — `DecideUpdate` (puro) popula os campos novos do evento a partir de `cmd` e de `currentItems`.
- `infrastructure/messaging/database/producers/transaction_event_publisher.go` — sem lógica nova; apenas serializa o evento já decidido (mantém R-TXN-003).

**Novos — `internal/budgets`:**
- `infrastructure/messaging/database/consumers/transaction_updated_consumer.go` — consumidor fino (molde: `transaction_created_consumer.go`).
- `application/usecases/reconcile_transaction_update.go` — usecase de reconciliação (delete-antigos + upsert-atuais), orquestra `DeleteExpense` + `UpsertExpense` numa UoW.
- `module.go` — construir e registrar o consumidor para `transactions.transaction.updated.v1`.

### Fluxo de Dados

```
WhatsApp → AgentRuntime.Execute → Thread → Run → Agent (tool-calling)
  ├─ get_last_entry / search_transactions / get_transaction  (resolve alvo: id+version+valores atuais)
  └─ edit_entry(entryId, version, campos alterados)
        → RegisterAttempt.EditEntry (re-resolve categoria se categoria/direção mudam)
        → pending-entry workflow (slot-filling p/ campo faltante) → AwaitingSlotConfirmation (diff antes→depois)
        → resume "sim" → executeWithIdempotency(operation="edit_entry")
        → ledger.UpdateTransaction(buildRawUpdate(state))  [optimistic lock por version]
              → txusecases.UpdateTransaction → DecideUpdate (puro) → repo.UpdateWithVersion + ReplaceItems
              → publisher.PublishUpdated(TransactionUpdated enriquecido)  [outbox]
                    → budgets: transaction_updated_consumer → ReconcileTransactionUpdate
                          → delete parcelas antigas (PreviousItemIDs) / update in-place não-cartão
                          → upsert representação atual → budgets_expenses coerente + ExpenseCommitted
```

Conflito de `version` (RF-21, ADR-003): `UpdateTransaction` retorna erro de versão → o step de escrita re-lê a transação atual (novo `version` + valores), reconstrói o resumo e **re-suspende** no `AwaitingSlotConfirmation` com o diff atualizado; nunca sobrescreve silenciosamente.

## Design de Implementação

### Interfaces Chave

Evento de domínio enriquecido (aditivo, compatível — ADR-001):

```go
type TransactionUpdated struct {
	EventID           uuid.UUID
	AggregateID       uuid.UUID
	UserID            uuid.UUID
	OccurredAt        time.Time
	Direction         valueobjects.Direction
	PaymentMethod     valueobjects.PaymentMethod
	AmountCents       int64
	RefMonth          valueobjects.RefMonth
	CategoryID        uuid.UUID                 // novo
	SubcategoryID     uuid.UUID                 // novo
	RefMonthsAffected []valueobjects.RefMonth
	Installments      []CardPurchaseInstallment // novo: conjunto atual (vazio p/ não-cartão)
	PreviousItemIDs   []uuid.UUID               // novo: itens antigos a remover no consumidor
}
```

Read tool de resolução do alvo (ADR-004) e sua porta no ledger (adapter fino):

```go
type recentEntriesReader interface {
	ListRecentEntries(ctx context.Context, limit int) ([]agentsifaces.MonthlyEntry, error)
}
// tool.NewTool[ListRecentEntriesInput, ListRecentEntriesOutput]("get_last_entry", desc, in, out, exec)
// exec delega a recentEntriesReader; retorna id, version, descrição, valor, categoria, data (sem regra).
```

Usecase de reconciliação no orçamento (ADR-001) — consumidor fino delega a ele:

```go
type ReconcileTransactionUpdate struct { /* uow, factory, categories, publisher, o11y */ }

func (uc *ReconcileTransactionUpdate) Execute(ctx context.Context, in input.ReconcileTransactionUpdateInput) error
// in carrega: userID, aggregateID, direction, isCard, subcategoryID, competences+amounts atuais
//             (não-cartão ou lista de parcelas), previousItemIDs.
// Numa UoW: remove representação anterior (não-cartão: por aggregateID; cartão: por previousItemIDs)
//           e aplica a atual (não-cartão: update in-place por version; cartão: insere parcelas atuais).
```

Consumidor novo (molde `transaction_created_consumer.go`, R-ADAPTER-001):

```go
type TransactionUpdatedConsumer struct { reconcile reconcileUseCase; o11y observability.Observability; /* counters */ }
func (c *TransactionUpdatedConsumer) Handle(ctx context.Context, event platformevents.Event) error
// decode outbox.Envelope → transactionUpdatedPayload → filtra direction=outcome → chama reconcile.Execute
```

Comando de edição do agente (propagação dos campos novos — ADR-002):

```go
type EditEntryCommand struct {
	UserID              uuid.UUID
	ThreadID            string
	WAMID               string
	ItemSeq             int
	TargetTransactionID uuid.UUID
	TargetVersion       int64          // novo: version lida no alvo (optimistic lock)
	AmountCents         *int64         // ponteiros: só sobrescrevem quando presentes
	Description         *string
	OccurredAt          *string
	Direction           *string        // novo
	PaymentMethod       *string        // novo
	CardNickname        *string        // novo (resolve_card antes de gravar, se crédito)
	Installments        *int           // novo
	CategoryTerm        *string        // novo: dispara re-resolução via classify_category
}
```

### Modelos de Dados

- Sem alteração de schema no Postgres: `TransactionUpdated` trafega como JSON no payload do outbox (coluna JSONB); enriquecer o struct é compatível. Última migração é `000008`; **nenhuma migração nova** é necessária (confirmado).
- `budgets_expenses` mantém identidade `(user_id, source, external_transaction_id)`; não-cartão `source=transactions, external=aggregateID` (estável); cartão `source=transactions_card, external=ItemID` (recriado a cada edição, por isso o delete-antigos + insert-atuais).
- `PendingEntryState` ganha campos aditivos (json tags novos): `PrevAmountCents`, `PrevDescription`, `PrevCategoryPath`, `PrevOccurredAt`, `PrevPaymentMethod` (para o diff antes→depois) e os campos-alvo de edição já existentes (`TargetTransactionID`, `TargetVersion`) reutilizados.

### Endpoints de API

Nenhum endpoint HTTP novo. A capacidade é 100% conversacional (WhatsApp) via tools do agente. O `UpdateTransactionHandler` HTTP existente permanece inalterado e fora de escopo.

## Pontos de Integração

- LLM: exclusivamente nas call-sites sancionadas (loop de tool-calling do agente; scorer LLM-judged). OpenRouter único provider; sem fallback chain. Nenhum LLM no kernel de workflow nem dentro do `exec` das tools (RNF-01, R-AGENT-WF-001.4).
- Outbox: `PublishUpdated` já existe; o dispatcher entrega o `Envelope` ao novo consumidor com timeout por handler, retry com backoff e dead-letter por esgotamento (semântica atual preservada). Reprocessamento é idempotente pelo desenho do `ReconcileTransactionUpdate` (delete-por-identidade + upsert-por-identidade).

## Abordagem de Testes

### Testes Unitários

Padrão canônico testify/suite (R-TESTING-001): whitebox package, `s.obs = fake.NewProvider()`, um mock tipado por dependência via mockery, tabela com `args`/`dependencies` + IIFE por mock, SUT dentro de `s.Run`.

- `DecideUpdate` (domínio, puro, zero mock): popula `CategoryID`/`SubcategoryID`/`Installments`/`PreviousItemIDs` corretamente para não-cartão, cartão, e migração pix↔crédito; `RefMonthsAffected` = antigos ∪ novos; no-op (valores idênticos) detectado.
- `ReconcileTransactionUpdate` (budgets usecase): não-cartão update in-place; cartão delete(PreviousItemIDs)+upsert(atuais); migração não-cartão→cartão e cartão→não-cartão; idempotência em reprocessamento; tombstone/conflict tratados.
- `TransactionUpdatedConsumer`: decode do envelope, filtro `direction=outcome`, delegação ao usecase, contadores de decode/skip.
- `edit_entry` tool + `RegisterAttempt.EditEntry`: propagação dos campos novos; re-resolução de categoria em mudança de categoria/direção; `TargetVersion` carregada; diff antes→depois montado; conflito de version → re-suspend.
- `pending_entry` decisions: aceite/cancelamento/reprompt único/expiração já cobertos; adicionar casos de edição multi-campo e no-op.

Interfaces novas adicionadas ao `.mockery.yml` (ex.: `reconcileUseCase`, `recentEntriesReader`) e `task mocks` executado.

### Testes de Integração

Sim — critérios atendidos (fronteira de IO crítica: outbox Postgres transactions→budgets; risco de consumo fantasma comprovado). Usar testcontainers com `//go:build integration`.

- Cadeia edição: criar transação (evento created) → editar valor/categoria/data (evento updated) → asserir `budgets_expenses`, `GetMonthlySummary` e ausência de linha fantasma; editar compra parcelada (3x→2x) e asserir remoção da parcela extinta e recomputo por competência; migração pix→crédito e crédito→pix.
- Reprocessamento do evento updated (redelivery) mantém o read model idêntico (idempotência).

### Testes E2E

Suíte golden real-LLM (`RUN_REAL_LLM=1`, OpenRouter) cobrindo os cenários de edição de ponta a ponta (categoria golden `expense_income`/`pending`/`confirmation`/`tool_error`), com razão de acerto ≥ 0,90 por categoria (D-05), scorer `write_persistence_accuracy` verde e verificação de consistência transação↔orçamento. O harness dirige até o estado/invariante semântico sem baixar a régua.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. Domínio transactions: enriquecer `TransactionUpdated` + `DecideUpdate` (puro) + testes unitários (nenhuma dependência; base para o resto).
2. Producer: garantir serialização do evento enriquecido (sem lógica nova) + teste.
3. Budgets: `ReconcileTransactionUpdate` usecase + mocks + testes unitários.
4. Budgets: `TransactionUpdatedConsumer` fino + registro em `module.go` + testes.
5. Integração transactions→budgets (testcontainers).
6. Agente: read tool de resolução do alvo + porta no ledger.
7. Agente: alargar `edit_entry` schema + `EditEntryCommand`/`PendingEntryState`/`buildRawUpdate` + re-resolução de categoria + diff antes→depois + conflito de version; incluir `edit_entry` no `WithWriteToolSet`.
8. Golden real-LLM + gates de governança + validação de risco proporcional.

### Dependências Técnicas

- Nenhuma infraestrutura nova. Reusa outbox, workflow kernel, agent runtime, testcontainers e OpenRouter existentes.

## Monitoramento e Observabilidade

- Métricas (cardinalidade controlada — R-TXN-004/R-AGENT-WF-001.5; proibido `user_id`/`category_id` como label):
  - `budgets_transaction_updated_consumer_decode_failed_total`, `budgets_transaction_updated_consumer_skipped_total{reason}` (molde dos consumers existentes).
  - `budgets_reconcile_transaction_update_total{outcome}` (outcome ∈ enum fechado: `updated`, `recreated`, `noop`, `conflict`).
  - Métrica de escrita do agente já existente por `operation="edit_entry"` (idempotência).
- Run auditável (RF-32): `thread_id`, `run_id`, `workflow`, `tool`, `status`, `duration_ms`, `error`, `decision_id`.
- Spans: `budgets.consumer.transaction_updated.handle`, `budgets.usecase.reconcile_transaction_update`, e os spans já existentes do fluxo de edição.
- Logs de aviso em skip (income/missing_subcategory/tombstone) e em conflito de version.

## Considerações Técnicas

### Decisões Chave

- ADR-001 — Reflexo no orçamento por evento enriquecido + usecase de reconciliação (delete-antigos + upsert-atuais), sem migração de dados.
- ADR-002 — Caminho canônico de edição = `pending-entry` (`PendingOpEditEntry`); alargar `edit_entry` e incluí-la no `WithWriteToolSet`; `destructive_confirm.OpEditEntry` declarado não-canônico para edição.
- ADR-003 — Conflito de controle otimista → re-ler + re-apresentar confirmação fresca (sem last-writer-wins); resumo de confirmação em diff antes→depois.
- ADR-004 — Resolução determinística do alvo via read tool `get_last_entry`/`list_recent_entries` + desambiguação por atributos.

### Riscos Conhecidos

- R-01 (contrato de evento): consumidores atuais de `updated.v1` — não há nenhum hoje; enriquecimento é aditivo/compatível. Mitigação: campos aditivos, testes de decode tolerante.
- R-02 (parcelas fantasma): endereçado por `PreviousItemIDs` + reconcile. Mitigação: teste de integração 3x→2x e migração de forma de pagamento.
- R-03 (semântica de alerta): cartão emite Delete+Create por competência; correto por mês, porém aumenta volume de `ExpenseCommitted`. Mitigação: métrica `outcome` e verificação de thresholds no teste de integração.
- R-04 (brittleness de teste real-LLM mascarar defeito): dirigir ao estado/invariante semântico, jamais baixar a régua 0,90 (lição registrada em reviews anteriores).
- R-05 (dead path destructive_confirm.OpEditEntry): manter intocado evita regressão; documentado como dívida técnica em ADR-002.

### Conformidade com Padrões

- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.1–.8): registry, tool fina, estados fechados, LLM sancionado, Run auditável, pending step antes da confirmação, resume por merge-patch, WorkingMemory no prompt.
- `.claude/rules/go-adapters.md` (R-ADAPTER-001): consumidor/tool finos, sem SQL, zero comentários.
- `.claude/rules/transactions-workflows.md` (R-TXN-001..004): regra só em `DecideUpdate` puro; producer só mapeia; cardinalidade controlada.
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001): kernel sem domínio; resume por merge-patch.
- `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001): `Validate()` nos DTOs de input novos (ex.: `ReconcileTransactionUpdateInput`).
- `.claude/rules/go-testing.md` (R-TESTING-001): suíte canônica; `.mockery.yml` atualizado.
- Design patterns: nenhum GoF novo (selector = reject); reusar registry/Factory/Adapter/Facade inline; proibido carregar `patterns-structural.md`.

### Arquivos Relevantes e Dependentes

- `internal/transactions/domain/entities/events.go` (26-52), `internal/transactions/domain/services/transaction_workflow.go` (136-253), `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher.go` (69-94), `internal/transactions/application/usecases/update_transaction.go` (124-182).
- `internal/budgets/module.go` (132-146), `internal/budgets/infrastructure/messaging/database/consumers/transaction_created_consumer.go` (molde), `internal/budgets/application/usecases/upsert_expense.go` (125-219), `internal/budgets/application/usecases/delete_expense.go` (88-125).
- `internal/agents/application/tools/edit_entry.go`, `internal/agents/application/usecases/register_attempt.go` (202-263), `internal/agents/application/workflows/pending_entry_state.go` (67-200), `internal/agents/application/workflows/pending_entry_workflow.go` (544-730), `internal/agents/infrastructure/binding/transactions_ledger_adapter.go` (57-146), `internal/agents/module.go` (193-347).
- Substrato: `internal/platform/{workflow,tool,agent,outbox,events}` (consumido, não modificado).

## Mapeamento Requisito → Decisão → Teste

| RF | Decisão/Componente | Teste |
|----|--------------------|-------|
| RF-01..04 | ADR-004 read tool + desambiguação; `principalCtx` (ownership) | unit tool + golden `pending`; integração ownership |
| RF-05..08 | ADR-002 alargar `edit_entry`/`EditEntryCommand`/`buildRawUpdate`; guarda multi-transação | unit RegisterAttempt; golden `expense_income` |
| RF-09..15 | pending-entry slot-filling; `classify_category`/`resolve_card`; guard de migração | unit decisions; golden `pending`/`card` |
| RF-16..19 | ADR-003 diff antes→depois; TTL 35/5min; merge-patch resume | unit confirm; golden `confirmation` |
| RF-20..23 | ADR-003 optimistic lock + re-confirm; idempotência `operation=edit_entry`; no-op | unit + integração conflito/replay |
| RF-24 | `DecideUpdate` recompõe compra inteira | unit DecideUpdate cartão |
| RF-25..26 | anti-simulação + formatação WhatsApp | golden `tool_error`/`whatsapp_format`/`no_internal_terms` |
| RF-27..30 | ADR-001 evento enriquecido + reconcile; competência passada respeita cutoff | unit reconcile; integração cadeia |
| RF-31..32 | registry routing; Run auditável + métricas | gates grep; verificação de labels |

## Itens em Aberto

Nenhum. As 15 decisões de produto (PRD) e as 4 decisões técnicas (ADR-001..004) estão fechadas; todos os forks materiais foram resolvidos por múltipla escolha com recomendação.

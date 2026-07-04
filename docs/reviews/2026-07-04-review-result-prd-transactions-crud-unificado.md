# Resultado da Revisão — CRUD Unificado de Transações

- Data: 2026-07-04
- PRD: `.specs/prd-transactions-crud-unificado/` (prd spec-hash `4a96b713`, techspec `ff7547e7`)
- Branch: `feat/transactions-crud-unificado`
- Método: 6 subagentes especializados por trilha + gates objetivos + ciclo `review → bugfix → review`

## verdict: APPROVED

100% dos RF-01..RF-24a atendidos e evidenciados. 0 findings abertos, 0 gaps, 0 ressalvas
bloqueantes. Ciclo fechado após 1 rodada de bugfix (2 defeitos reais + 1 decisão de escopo do dono).

## findings (todos resolvidos)

| # | Sev | Origem | Descrição | Resolução |
|---|-----|--------|-----------|-----------|
| 1 | high | R-TXN-001 | `uuid.New()` dentro de `DecideCreate`/`DecideUpdate` — geração de ID aleatório em `Decide*` puro (regra HARD proíbe explicitamente) | `itemIDs []uuid.UUID` passado como parâmetro pelo use case (mesmo padrão de `txID`/`eventID`); helper puro `newInvoiceItemIDs`; teste de determinismo fortalecido para exigir igualdade total de item IDs |
| 2 | low | RF-24 | Branch morto `case "card_purchase":` em `destructive_confirm_workflow.go:344` | removido |
| 3 | low | techspec | `output.Transaction` + OpenAPI declaravam `ref_months_affected` e `items` nunca preenchidos por `TransactionFrom` | **decisão do dono**: remover campos mortos do DTO + schema OpenAPI (schema órfão `CardInvoiceItem` também removido); itens permanecem expostos via `GET /cards/{id}/invoices/{ref_month}` |

## Não-findings (avaliados e descartados com evidência)

- `CardPurchaseInstallment` (events.go) e `AsCardPurchase` (recurring_workflow.go): vocabulário de
  domínio **retido** (parcela/evento da transação de crédito), não a superfície removida. RF-24/24a
  removem endpoints/handlers/rotas/tabela — não exigem renomear conceitos de parcela retidos.
- `resolveBilling` antes do check de idempotência (create): lookup read-only redundante em replay;
  otimização opcional, sem impacto de correção. Não é defeito.
- `EditEntryInput.InstallmentsTotal` sem mapeamento no agente: limitação pré-existente do consumidor;
  edição de parcelas via agente não é RF deste PRD (escopo do agente = RF-11a/11b/24). Fora de escopo.
- OpenAPI `installments.minimum: 1` incondicional: `default: 1` neutraliza; sem impacto runtime.

## validations_run

- `go build ./...` = 0; `go vet ./...` = 0.
- `go test ./...` = **3817 passed / 232 packages / 0 FAIL**.
- Gates: R-TXN-001 (Decide* puro — `uuid.New`/`time.Now`/`rand`/`ctx` ausentes), R-TXN-004 (sem
  user_id/category_id/card_id em label de métrica), R-ADAPTER-001.1 (zero comentários), R-ADAPTER-001.2
  (sem SQL em adapter), R-AGENT-WF-001.1 (sem `switch intent.Kind`), R-DTO-VALIDATE-001, R-TESTING-001
  — todos vazios/pass.
- Sem resíduo de superfície card-purchase em produção (`transactions_card_purchases`, handlers, rotas,
  use cases, tools, consumer de budgets) — confirmado por grep e pelas 6 trilhas.

## Matriz de rastreabilidade (RF → status → evidência)

| RF | Status | Evidência (file:line) |
|----|--------|-----------------------|
| RF-01 CRUD único income+outcome | atendido | `application/usecases/{create,update,delete,get,list_transactions,search_transactions}.go` |
| RF-02 soft-delete + version | atendido | `delete_transaction.go:86` `repo.SoftDelete(...,version,...)` |
| RF-03 campos mínimos | atendido | `raw_create_transaction.go:24-56` Validate |
| RF-04 idempotente + outbox | atendido | `create_transaction.go:155-170` (ON CONFLICT origin) + publish no UoW |
| RF-05 enum fechado, rejeita fora | atendido | `payment_method.go:11,26-51` |
| RF-06 exatamente 9 métodos p/ criação | atendido | `payment_method.go:53-58` (`ParsePaymentMethodForCreate`) |
| RF-07 `doc` legado read-only | atendido | `payment_method.go:54-56`; update usa `ParsePaymentMethodForCreate` |
| RF-08 vale_refeicao(9)/vale_alimentacao(10) simples | atendido | `payment_method.go:22-23`; branch simples `transaction_workflow.go:63` |
| RF-09 exclusões com racional | atendido | prd §RF-09; enum não os inclui |
| RF-10 não-credit = simples | atendido | `transaction_workflow.go:62-78,143-171` |
| RF-11 só credit aciona cartão | atendido | `transaction_workflow.go:80-131` |
| RF-11a credit ⇒ outcome | atendido | `commands/create_transaction.go:105`, `update_transaction.go:114` |
| RF-11b credit ⇒ card_id | atendido | `commands/create_transaction.go:101`, `update_transaction.go:110` |
| RF-12 resolve/abre fatura, não bloqueia | atendido | `transaction_workflow.go:84` + use case UoW upsert; `BillingCycleResolver` reusado |
| RF-13 parcelas 1..24 | atendido | `valueobjects/installment_count.go:14-19` |
| RF-14 opcional, default 1 | atendido | `card_invoice_orchestration.go:16-22` |
| RF-15 split determinístico, soma=total | atendido | `installment_splitter.go:9-23`; teste 12x soma=total |
| RF-16 edição recompõe deltas atômico | atendido | `transaction_workflow.go` DecideUpdate + `update_transaction.go:151-175` UoW |
| RF-16a delete reverte deltas atômico | atendido | `transaction_workflow.go` DecideDelete + `delete_transaction.go:90-99` UoW |
| RF-17 raiz sem parent_id | atendido | `categories_cache.go:116-120` |
| RF-18 subcategoria filha direta | atendido | `validate_subcategory.go:35,52` (expectedParentID) |
| RF-19 outcome ⇒ subcategoria obrigatória | atendido | `helpers.go:90-96`, guard em create+update |
| RF-20 income ⇒ opcional | atendido | `categories_cache.go:122-124` |
| RF-21 kind ↔ direction | atendido | `helpers.go:105-111`; `CategorySnapshot{Kind,ParentID}` |
| RF-22 recorrência via CRUD unificado | atendido | `materialize_recurring_for_day.go:164,189` → `CreateTransaction` |
| RF-23 resumo mensal fonte única | atendido | `recompute_monthly_summary.go:50-56`; `SumByMonthExcludingCredit` (pm<>7) + invoice |
| RF-24 remoção da superfície card-purchase | atendido | handlers/rotas/use cases/DTOs/tools/consumer removidos; `router_test.go:167` 404 |
| RF-24a drop de dados + gate pré-release | atendido | `migrations/000003_...up.sql:95` DROP; gate em `docs/runbooks/transactions.md:58-71` |

## Cobertura ADR

- ADR-001 (despacho por PaymentMethod): orquestração no use case seleciona path de `Decide*` puro — confirmado; sem Strategy de classe, sem switch de domínio.
- ADR-002 (corte + drop card-purchase): superfície removida + drop guardado + tools do agente refatoradas.
- ADR-003 (schema unificado + fonte única): migration 000003 idempotente; credit fora de `SumByMonth`/`ListEntries` do ramo transactions (sem double-count).

## Validação com LLM real (OpenRouter, `.env`)

Executado com `RUN_REAL_LLM=1` + `OPENROUTER_BASE_URL`/`OPENROUTER_API_KEY` do `.env`
(gpt-4o-mini via OpenRouter). Todos PASS:

| Teste | Resultado | Evidência |
|-------|-----------|-----------|
| `TestRealLLM_CardPurchaseChain_ResolveClassifyRegister` | PASS | "parcelada 3x" → `register_expense` credit_card installments=3; "à vista" → installments=1 (RF-11a/11b/24) |
| `TestRealLLM_ToolCalling_RegisterExpense` | PASS | despesa simples roteia p/ `register_expense` |
| `TestRealLLM_ToolCoverage_All22Tools` (scorers) | PASS | **M-04 = 1.00 (22/22)**, 0 tools não exercidas |
| `TestRealLLM_ToolError_ProducesHonestResponse` | PASS | tool em falha → resposta honesta, sem sucesso alucinado |
| `TestRealLLM_EP01_AntiSimulation_RegisterExpenseDoesNotFakeSuccess` | PASS | não simula sucesso |
| `TestRealLLM_ToolCalling_QueryMonth` / `TestRealLLM_Scorer_CategorizationLLMJudged` | PASS | consulta e categorização |

### Finding 4 (resolvido nesta rodada) — real-LLM scorer desalinhado com RF-24

- Sev: high (produção). `mecontrola_tools_realllm_test.go` ainda definia 3 tools sintéticas
  removidas (`register_card_purchase`, `get_card_purchase`, `list_card_purchases`) e asseverava
  roteamento a elas + `Len==25` — validava contrato que RF-24 apagou.
- Resolução: harness alinhado à superfície unificada (22 tools = catálogo real
  `mecontrola_scorers.go`); 3 tools e 3 cenários removidos; `Len` 25→22; teste renomeado
  `TestRealLLM_ToolCoverage_All22Tools`. Re-executado com LLM real: M-04 = 1.00 (22/22).

## Arquivos alterados nesta rodada de bugfix

- `internal/transactions/domain/services/transaction_workflow.go` (itemIDs param, sem `uuid.New` em Decide*)
- `internal/transactions/domain/services/transaction_workflow_test.go` (itemIDs + determinismo fortalecido)
- `internal/transactions/application/usecases/card_invoice_orchestration.go` (`newInvoiceItemIDs`)
- `internal/transactions/application/usecases/create_transaction.go`, `update_transaction.go` (passam itemIDs)
- `internal/transactions/application/dtos/output/transaction.go` (remove campos mortos)
- `internal/transactions/openapi.yaml` (remove `ref_months_affected`/`items` + schema órfão)
- `internal/agents/application/workflows/destructive_confirm_workflow.go` (remove branch morto)
- `internal/agents/application/scorers/mecontrola_tools_realllm_test.go` (harness real-LLM alinhado a 22 tools)

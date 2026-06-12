# Tarefa 3.0: Domain services — splitter, cycle resolver (clamp), workflows `Decide*`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa as funções puras de domínio: `InstallmentSplitter` (divisão determinística de centavos), `BillingCycleResolver` (com clamp `min(day, last_day_of_target_month)` — audit fix #3), `RefMonthResolver`, e os 3 workflows DMMF puros (`TransactionWorkflow`, `CardPurchaseWorkflow`, `RecurringWorkflow`) que calculam `Decision` com `RefMonthsAffected` e `InvoiceDeltas`. Tudo sem mocks (ADR-006 §2).

<requirements>
- `InstallmentSplitter.Split(Money, InstallmentCount) []Money` determinístico: centavos residuais distribuídos nas primeiras parcelas em ordem; soma exata = total.
- `RefMonthResolver.From(t time.Time, loc *time.Location) RefMonth` retorna `YYYY-MM` em fuso BR.
- `BillingCycleResolver.Resolve(purchasedAt time.Time, snapshot CardBillingSnapshot, n InstallmentCount) (refMonths []RefMonth, closings, dues []time.Time)`:
  - Regra RF-14: se `purchasedAt.day <= snapshot.ClosingDay` → primeira fatura é mês corrente; senão próxima.
  - **Clamp obrigatório**: `effective_day = min(day, last_day_of_target_month)` para `closing_at` e `due_at` quando o dia natural inexiste no mês destino.
- `TransactionWorkflow.DecideCreate(cmd commands.CreateTransaction, ids, now) Decision`: produz `entities.Transaction` + `entities.TransactionCreated{}`.
- `TransactionWorkflow.DecideUpdate(current entities.Transaction, cmd commands.UpdateTransaction, ids, now) Decision`: produz update + event com `RefMonthsAffected = {old.RefMonth, new.RefMonth}` deduped (apenas se mudou de mês).
- `CardPurchaseWorkflow.DecideCreate/Update/Delete` consomem `snapshot CardBillingSnapshot` (audit fix #2) e calculam **`RefMonthsAffected` + `InvoiceDeltas`** (audit fix #4): `delta = sum(new[ref]) − sum(old[ref])` por `ref` afetado.
- `RecurringWorkflow.DecideMaterializeForDay(template, today, ids) MaterializeDecision`: decide se materializa `Transaction` ou `CardPurchase` conforme `payment_method` e produz inputs prontos para os use cases respectivos.
- Sem `ctx`, sem repo, sem `time.Now()` interno em nenhum método (instante chega como `now time.Time`).
</requirements>

## Subtarefas

- [ ] 3.1 `domain/services/installment_splitter.go` + table-driven test (1, 2, 3, 12, 24 parcelas; total ímpar; total par).
- [ ] 3.2 `domain/services/ref_month_resolver.go` + test cobrindo borda de mês em fuso BR (01:00 UTC = 22:00 BR do dia anterior).
- [ ] 3.3 `domain/services/billing_cycle_resolver.go` com clamp + test cobrindo fev/30 dias, abril/31 dias, dia 28/29 fev bissexto.
- [ ] 3.4 `domain/services/transaction_workflow.go` (`DecideCreate`, `DecideUpdate`) + test.
- [ ] 3.5 `domain/services/card_purchase_workflow.go` (`DecideCreate`, `DecideUpdate`, `DecideDelete`) + test cobrindo PATCH 12→3 parcelas com `InvoiceDeltas` em todas as 12 competências.
- [ ] 3.6 `domain/services/recurring_workflow.go` (`DecideMaterializeForDay`) + test cobrindo credit_card vs débito.

## Detalhes de Implementação

Referência: techspec "Visão Geral dos Componentes" / `domain/services/` + ADR-006 §2 (Decide* puro). RF-14 (regra de competência), RF-15 (split determinístico), RF-16 (upsert idempotente — chamada vem do use case, não do workflow), RF-25 (mudança de `ref_month` → recompute de duas competências), RF-36 (single event com `ref_months_affected`).

## Critérios de Sucesso

- `go test -race -count=1 ./internal/transactions/domain/services/...` passa com cobertura ≥ 95%.
- Teste do `BillingCycleResolver` com `due_day=30` em fevereiro retorna dia 28 (ou 29 em bissexto), não 02/março.
- Teste do `CardPurchaseWorkflow.DecideUpdate` para 12→3 parcelas retorna `RefMonthsAffected` com 12 competências e `InvoiceDeltas` negativo nas 9 competências removidas.
- Zero mocks em testes deste pacote.
- `golangci-lint run ./internal/transactions/domain/services/...` limpo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários table-driven por serviço e workflow.
- [ ] Sem integration tests (pacote puro).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/domain/services/installment_splitter.go` (novo)
- `internal/transactions/domain/services/ref_month_resolver.go` (novo)
- `internal/transactions/domain/services/billing_cycle_resolver.go` (novo)
- `internal/transactions/domain/services/transaction_workflow.go` (novo)
- `internal/transactions/domain/services/card_purchase_workflow.go` (novo)
- `internal/transactions/domain/services/recurring_workflow.go` (novo)
- Testes `*_test.go` correspondentes.

# Tarefa 8.0: Wiring, integração/e2e, observabilidade e gates

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechamento da unificação: fazer o wiring final dos use cases unificados, remover
todo o wiring de card-purchase, integrar recorrência e resumo mensal à porta única,
ajustar observabilidade (spans/métricas/dashboards/alertas) e provar o conjunto por
testes de integração (testcontainers) + e2e (HTTP real + agente real-LLM). É a tarefa
de validação de encerramento; assume que as tarefas 1–7 (VOs, workflow, migration,
use cases, categorias, HTTP, agente) já estão implementadas. Cobre RF-01, RF-22,
RF-23, RF-24, RF-24a. Ver techspec `## Sequenciamento de Desenvolvimento` (passo 8) e
`## Monitoramento e Observabilidade`; ADR-002 (corte/descarte) e ADR-003 (fonte única
do resumo mensal).

<requirements>
- Atualizar `internal/transactions/module.go`, `cmd/server/server.go` e
  `cmd/worker/worker.go`: injetar `cardLookup`, `cardInvoiceRepo` e `publisher` nos use
  cases unificados; remover a construção de use cases/handlers/rotas de card-purchase e
  ajustar o registro de `EventHandlers` do consumer (não subscrever mais
  `transactions.card_purchase.*`).
- `MonthlySummaryRecomputeConsumer` deve processar `transactions.transaction.*` com
  `ref_months_affected` multi-mês, disparando recompute em cada mês afetado (RF-22/23).
- Materialização de recorrência `credit_card` passa a chamar o `CreateTransaction`
  unificado (mesma regra de cartão/categoria), sem reimplementar recorrência (RF-22).
- Observabilidade: spans com atributos `installments_total` e
  `ref_months_affected_count` (sem PII); métricas Prometheus com cardinalidade
  controlada (R-TXN-004): labels `operation`, `payment_method`, `installments_bucket`;
  proibido `user_id`/`category_id`/`card_id` como label. Atualizar
  `docs/dashboards/transactions-overview.json` e `docs/alerts/transactions.yaml`
  removendo séries de card_purchase.
- Gate pré-release (ADR-002): `SELECT count(*) FROM mecontrola.transactions_card_purchases`
  deve ser 0 antes do drop; abortar o corte se > 0.
- Gates de conformidade R-TXN-001..004, R-ADAPTER-001, R-DTO-VALIDATE-001,
  R-TESTING-001 executados e vazios.
- Zero comentários em Go de produção (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 8.1 Wiring: injetar `cardLookup`/`cardInvoiceRepo`/`publisher` nos use cases
  unificados em `module.go`; propagar em `cmd/server/server.go` e `cmd/worker/worker.go`.
- [ ] 8.2 Remover wiring de card-purchase: use cases, handlers, rotas e o registro de
  `EventHandlers` de `transactions.card_purchase.*` no consumer.
- [ ] 8.3 `MonthlySummaryRecomputeConsumer`: processar `transactions.transaction.*`
  iterando `ref_months_affected` (recompute por mês, sem double-count — ADR-003).
- [ ] 8.4 Recorrência: `materialize_recurring_for_day` materializa `credit_card` via
  `CreateTransaction` unificado (RF-22).
- [ ] 8.5 Observabilidade: atributos de span (`installments_total`,
  `ref_months_affected_count`) e labels de métrica controlados (R-TXN-004).
- [ ] 8.6 Atualizar `docs/dashboards/transactions-overview.json`,
  `docs/alerts/transactions.yaml` e `docs/runbooks/transactions.md` removendo
  card_purchase e cobrindo credit_card unificado.
- [ ] 8.7 Gate pré-release ADR-002: rotina/check de contagem 0 em
  `transactions_card_purchases` antes do drop, abortando se > 0.
- [ ] 8.8 Testes de integração (testcontainers, `//go:build integration`).
- [ ] 8.9 Testes e2e (jornada HTTP + 404 em `/card-purchases` + tool `register_expense`
  real-LLM).
- [ ] 8.10 Executar os greps dos gates de conformidade e confirmar saída vazia.

## Detalhes de Implementação

Ver techspec.md — `## Sequenciamento de Desenvolvimento` (passo 8: "Wiring `module.go`;
integração + e2e; gates de conformidade"), `## Monitoramento e Observabilidade`,
`## Abordagem de Testes` (Integração + E2E) e `## Pontos de Integração`. Decisões:
ADR-002 (corte imediato + gate de contagem 0) e ADR-003 (tabela única + fonte única do
resumo mensal / `SumByMonthExcludingCredit` + `ListEntries` filtrando `payment_method<>7`).

Regra de fonte única (ADR-003): transação `credit_card` é excluída do ramo
`transactions` na agregação mensal; o valor de crédito entra apenas via
`CardInvoiceRepository.SumByMonth`. O consumer deve iterar `ref_months_affected` do
evento — nunca re-derivar meses no adapter (herda R-ADAPTER-001.2 e R-TXN-003).

Cardinalidade (techspec `## Monitoramento`): reusar `transactions_idempotency_replay_total`
com label `operation` (`create_transaction`|`update_transaction`|`delete_transaction`);
labels permitidos por R-TXN-004: `operation`, `payment_method`, `installments_bucket`.

## Critérios de Sucesso

- Suíte de integração + e2e verde.
- Resumo mensal bate com a soma independente das faturas em cenário parcelado — 0
  double-count (ADR-003).
- 404 confirmado nas rotas `/api/v1/card-purchases` removidas (RF-24).
- Gate pré-release: contagem de `transactions_card_purchases` = 0 confirmada antes do
  drop; release aborta se > 0 (RF-24a / ADR-002).
- Gates de conformidade (R-TXN-001..004, R-ADAPTER-001, R-DTO-VALIDATE-001,
  R-TESTING-001) retornam vazio.
- `docs/dashboards/transactions-overview.json` e `docs/alerts/transactions.yaml` sem
  séries de card_purchase.
- Zero comentários em Go de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes de integração (testcontainers, `//go:build integration`): create
  credit_card 12x → 12 itens / 12 faturas com soma = total e 1 evento com
  `ref_months_affected`; update 12→6 com deltas atômicos + falha no meio → rollback
  total; delete revertendo os deltas; resumo mensal SEM double-count; migration up/down.
- [ ] Testes e2e: jornada HTTP (despesa pix; compra credit_card parcelada; list; edit;
  delete; 404 em `/card-purchases`) + tool do agente `register_expense` com credit_card
  em validação real-LLM.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/transactions/module.go` — wiring dos use cases unificados; remoção do
  wiring de card-purchase e do registro de `EventHandlers` `transactions.card_purchase.*`.
- `cmd/server/server.go` — propagação das dependências (cardLookup/cardInvoiceRepo/publisher).
- `cmd/worker/worker.go` — propagação das dependências no worker (consumers/jobs).
- `internal/transactions/infrastructure/messaging/database/consumers/monthly_summary_recompute_consumer.go`
  — processar `transactions.transaction.*` com `ref_months_affected` multi-mês (RF-22/23).
- `internal/transactions/infrastructure/jobs/handlers/recurring_materializer_job.go` — job
  de materialização de recorrência.
- `internal/transactions/infrastructure/jobs/handlers/monthly_summary_reconciler_job.go` —
  reconciler alinhado à fonte única (ADR-003).
- `internal/transactions/application/usecases/materialize_recurring_for_day.go` —
  materialização credit_card via `CreateTransaction` unificado (RF-22).
- `docs/dashboards/transactions-overview.json` — remover séries card_purchase.
- `docs/alerts/transactions.yaml` — remover alertas card_purchase.
- `docs/runbooks/transactions.md` — atualizar fonte única e gate pré-release.
- `.specs/prd-transactions-crud-unificado/techspec.md`,
  `.specs/prd-transactions-crud-unificado/adr-002-card-purchase-cutover-drop.md`,
  `.specs/prd-transactions-crud-unificado/adr-003-unified-transactions-schema.md` — referências.

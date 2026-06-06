# Tarefa 5.0: Use cases billing (5 webhook + reconcile)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os 5 use cases que respondem aos triggers Kiwify (`ProcessSaleApproved`, `ProcessSubscriptionRenewed`, `ProcessSubscriptionLate`, `ProcessSubscriptionCanceled`, `ProcessRefundOrChargeback`) e o use case `ReconcileSubscriptions` consumido pelo job. Toda a lógica de idempotência (`event_key`), ordering (`last_event_at`), grace 3d e transições passa por aqui, dentro de UoW transacional que abriga repo + outbox publisher na mesma fronteira. Erros tipados conforme R5.10. Mocks gerados para cada interface em `usecases/mocks/`.

<requirements>
- Cada use case é método de struct (R1) com construtor explícito.
- Cada use case abre `uow.New[...]` antes de mutar estado.
- Insere em `processed_events` antes da mutação; conflito → no-op (`ErrEventAlreadyProcessed`).
- Aplica regra de ordering (techspec §5.3 / §7.3): evento staled em transição "regression" marca `superseded` e retorna sem mutar.
- Erros sentinel: `ErrFunnelTokenMissing`, `ErrPlanNotFound`, `ErrEventAlreadyProcessed`, `ErrEventSuperseded`, `ErrConcurrentActiveSub`, `ErrUnknownTrigger`.
- `extractFunnelToken(payload)` é uma única função isolada (L-03); troca de campo é diff cirúrgico.
- `ProcessRefundOrChargeback` usa mesmo `event_key` para refund e chargeback (`refund:{sale.id}`) — RF-09.
- Mocks gerados em `usecases/mocks/` para todas as interfaces consumidas; nunca mockar Postgres real.
</requirements>

## Subtarefas

- [ ] 5.1 `usecases/process_sale_approved.go`: resolve plano por `kiwify_product_id`, extrai funnel_token (`tracking.s1` por padrão), upsert subscription `ACTIVE`, publica `SubscriptionActivated`.
- [ ] 5.2 `usecases/process_subscription_renewed.go`: estende período; se sub não existe, cria placeholder ACTIVE com período derivado (ADR-005).
- [ ] 5.3 `usecases/process_subscription_late.go`: transição → `PAST_DUE` com `grace_end = late_at + 3*24h` (RF-06, Q-02 travada).
- [ ] 5.4 `usecases/process_subscription_canceled.go`: transição → `CANCELED_PENDING` mantendo `period_end` (RF-07).
- [ ] 5.5 `usecases/process_refund_or_chargeback.go`: força `REFUNDED` (terminal); mesmo `event_key` para refund e chargeback (RF-09, Q-03 travada).
- [ ] 5.6 `usecases/reconcile_subscriptions.go`: invoca `KiwifyClient.ListSalesUpdatedSince`, traduz sale em pseudo-evento e roteia para o use case correspondente; checkpoint avançado só em sucesso completo.
- [ ] 5.7 Função `extractFunnelToken(payload)` isolada (L-03); padrão atual `tracking.s1`.
- [ ] 5.8 Detecção de evento staled (`occurred_at <= last_event_at && transition é regressão`) via `domain/services/transitions.IsRegression`.
- [ ] 5.9 Mocks em `usecases/mocks/` para todas as interfaces consumidas + `uow`.
- [ ] 5.10 Unit tests por use case cobrindo: idempotência, token vazio, plano inexistente, sucesso, staled→superseded, refund após cancel→REFUNDED, sub já refunded recebe novo refund→no-op idempotente.

## Detalhes de Implementação

- Fluxo canônico em techspec §5.1; variações em §5.2; ordering em §5.3 e §7.3.
- `event_key` por trigger em techspec §7.1.
- Use case publica evento via interface `SubscriptionEventPublisher` (declarada em 3.0); a implementação concreta vive em 6.0.
- `time.Now().UTC()` inline; nada de clock abstrato (R6.7).
- `ProcessSaleApproved` retorna `ErrFunnelTokenMissing` **sem** persistir `processed_events` nem `subscriptions`; ainda persiste em `kiwify_events` para auditoria (proativo pelo handler antes do use case).

## Critérios de Sucesso

- `go build ./internal/billing/application/usecases/...` verde.
- `go test -race -count=1 ./internal/billing/application/usecases/...` cobre todos os casos críticos da matriz §13 da techspec.
- Idempotência: replay 5× via mock retorna 1 transição + 4 `ErrEventAlreadyProcessed`.
- Ordering: sequência staled→fresh resulta em estado correto.
- `extractFunnelToken` cobre payload sem `tracking.s1` e com `tracking.src` (fallback caso L-03 mude).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests por use case (`process_sale_approved_test.go`, `process_subscription_renewed_test.go`, `process_subscription_late_test.go`, `process_subscription_canceled_test.go`, `process_refund_or_chargeback_test.go`, `reconcile_subscriptions_test.go`).
- [ ] Test de `extractFunnelToken` cobrindo presença/ausência de `tracking.s1`.
- [ ] Test de ordering: staled→fresh.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/application/usecases/{process_sale_approved,process_subscription_renewed,process_subscription_late,process_subscription_canceled,process_refund_or_chargeback,reconcile_subscriptions}.go` + `_test.go`
- `internal/billing/application/usecases/mocks/*.go`
- `internal/billing/application/usecases/funnel_token.go` (ou inline em `process_sale_approved.go`)
- Referência: `internal/identity/application/usecases/upsert_user_by_whatsapp.go` (padrão de UoW + mocks).
- Referência: techspec §5, §7, §9.3.

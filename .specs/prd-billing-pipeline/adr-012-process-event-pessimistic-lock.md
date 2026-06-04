# ADR-012 — Concorrência no `ProcessBillingEventUseCase` via `SELECT ... FOR UPDATE`

## Metadados

- **Título:** Pessimistic locking de Subscription durante mutação
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de plataforma + dados
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-21, RF-22, F-4), `techspec.md` §process_billing_event, `internal/platform/outbox/storage_pgx.go:106` (`FOR UPDATE SKIP LOCKED`), ADR-009

## Contexto

`outbox.Dispatcher` claim batch de 50 deliveries por tick (default). Dois eventos referentes ao mesmo `subscription_id` (e.g., `subscription_late` seguido de `subscription_renewed` no mesmo ciclo) podem ser entregues a workers concorrentes do mesmo processo OU a workers em processos diferentes (multi-pod). Sem coordenação:

```
T0: Worker A lê Subscription{status=ACTIVE, period_end=2026-07-01}
T0: Worker B lê Subscription{status=ACTIVE, period_end=2026-07-01}
T1: Worker A aplica subscription_late → {status=PAST_DUE}
T1: Worker B aplica subscription_renewed → {status=ACTIVE, period_end=2026-08-01}
T2: Worker A commit
T2: Worker B commit  → lost update: B aplicou em snapshot stale
```

Resultado final: `ACTIVE` (do Worker B), mas a transição `PAST_DUE → ACTIVE` é legal só por accident — perdemos o evento de inadimplência.

Confronto com codebase: `internal/platform/outbox/storage_pgx.go:89-110` já usa `FOR UPDATE SKIP LOCKED` em `ClaimReady` exatamente para resolver concorrência em deliveries. Padrão estabelecido.

## Decisão

Em `ProcessBillingEventUseCase`, ao buscar a `Subscription` corrente dentro da UoW, usar `SELECT ... FOR UPDATE` (sem `SKIP LOCKED` — diferente do outbox).

```sql
-- internal/billing/infrastructure/repositories/postgres/queries.go
const findActiveByUserIDForUpdate = `
    SELECT id, user_id, provider, external_subscription_id, plan_code, status,
           period_start, period_end, grace_period_end, refund_amount_cents,
           last_event_at, last_webhook_event_id, created_at, updated_at, deleted_at
      FROM subscriptions
     WHERE user_id = $1
       AND status IN ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING')
       AND deleted_at IS NULL
     LIMIT 1
       FOR UPDATE
`
```

Diferenças em relação ao outbox:
- **Sem `SKIP LOCKED`**: queremos que workers concorrentes bloqueiem (e serializem), não saltem.
- **Scope da fila lockada**: somente a row da Subscription atual. Outros user_ids não bloqueiam.

Para o caso de **primeira ativação** (nenhuma Subscription ainda existe para o user), a UoW depende do índice único parcial `uq_subscriptions_one_active_per_user` para forçar serialização: dois workers tentando criar simultaneamente colidem em UNIQUE; o perdedor retorna `pgerrcode.UniqueViolation`, mapeado para `ErrDuplicateActiveSubscription`, classificado como **transitório** (Dispatcher retenta — vencedor já fez UPSERT; retry lê o estado correto).

## Alternativas Consideradas

### OCC via coluna `version INT` (Optimistic Concurrency Control)

- Vantagem: sem lock no banco; maior throughput em baixa contenda.
- Desvantagem: complexidade no UPDATE (sempre checa `WHERE version = $expectedVersion`); conflito devolve erro → retry pelo Dispatcher (backoff); pode haver starvation; +1 coluna no schema.
- Rejeitada porque batch=50 do Dispatcher tem contenda baixa o suficiente para não justificar OCC. Pessimist é mais simples e correto.

### Idempotência por `billing_event_applications` é suficiente

- Vantagem: zero locking.
- Desvantagem: previne reprocessamento do mesmo `event_id`, mas NÃO previne lost-update entre eventos DIFERENTES do mesmo subscription_id chegando concorrentes. Cenário T0..T2 acima continua quebrando.
- Rejeitada por insuficiência.

### Particionamento por `subscription_id` no Dispatcher (1 worker por hash)

- Vantagem: serialização natural por subscription.
- Desvantagem: requer mudança estrutural no `outbox.Dispatcher` (não suporta partitioning hoje); aumenta escopo da techspec billing significativamente; design futuro.
- Rejeitada para MVP. Possível evolução em E4.

### Advisory lock (`pg_advisory_xact_lock(hashtext(subscription_id))`)

- Vantagem: serialização sem precisar row exist (cobre caso de primeira ativação).
- Desvantagem: pg_advisory_lock IDs colidem por hash; correto mas menos idiomático que `FOR UPDATE`.
- Rejeitada por não trazer benefício sobre `FOR UPDATE` + índice único.

## Consequências

### Benefícios Esperados

- Garantia formal de "1 evento por vez por subscription".
- Padrão alinhado com `outbox.ClaimReady`.
- Cobre tanto reprocessamento (via `billing_event_applications`) quanto concorrência (via `FOR UPDATE`).

### Trade-offs e Custos

- Lock durante toda a UoW (~50ms p99). Worker B aguarda Worker A. Aceitável.
- Em alta contenda extrema (e.g., dezenas de eventos para mesmo subscription_id), workers serializam. Sem lock starvation em PostgreSQL FIFO.

### Riscos e Mitigações

- **Risco:** deadlock se UoW pegar lock em 2 subscriptions na ordem inversa de outro worker. **Mitigação:** processor lê UMA subscription por evento — sem deadlock possível.
- **Risco:** lock timeout (default Postgres = sem timeout) se UoW pendurar. **Mitigação:** `database.UnitOfWork[T]` já aplica `context.WithTimeout(5s)` pelo padrão do projeto — lock liberado em 5s no pior caso.
- **Risco:** primeira ativação com UNIQUE violation gera retry mesmo em sucesso real. **Mitigação:** mapeamento `pgerrcode.UniqueViolation → ErrDuplicateActiveSubscription` é transitório, Dispatcher retenta uma vez, lê estado correto, segue normal.

## Plano de Implementação

1. `WebhookEventRepository`-irmão: adicionar `SubscriptionRepository.FindActiveByUserIDForUpdate(ctx, userID)` na interface.
2. Impl Postgres: query com `FOR UPDATE`.
3. `ProcessBillingEventUseCase` usa `FindActiveByUserIDForUpdate` em vez de `FindActiveByUserID` dentro da UoW.
4. Test integração: spawnar 2 goroutines processando eventos para mesmo user_id; verificar serialização (segundo só commita após primeiro).
5. Test integração: primeira ativação concorrente — duas goroutines tentando criar; verificar UNIQUE violation e retry idempotente.

## Monitoramento e Validação

- Métrica `billing_event_lock_wait_seconds` (histogram) — tempo aguardando lock.
- Métrica `billing_event_processed_total{outcome="unique_violation_retry"}` em primeira ativação concorrente.
- Span OTel `billing.event.process` inclui atributo `lock_acquired_after_ms`.

## Impacto em Documentação e Operação

- AGENTS.md billing documenta padrão de locking.
- Runbook: investigação de `lock_wait_seconds > 1s` sustentado orienta verificar se Dispatcher está com batch oversized para a contenda real.

## Revisão Futura

- Se contenda crítica observada (lock_wait_seconds_p99 > 500ms sustentado), considerar particionamento no Dispatcher (rejected alternative acima).

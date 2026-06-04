# Tarefa 6.0: Use cases finos + mocks via mockery + unit tests table-driven

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar 5 use cases em `internal/billing/application/usecases/` orquestrando ports (sem detalhes de IO): `IngestKiwifyWebhookUseCase`, `ProcessBillingEventUseCase` (impl `outbox.Handler`), `CheckEntitlementUseCase`, `ReconcileSubscriptionsUseCase`, `AnonymizeWebhookEventsUseCase`. Estender `.mockery.yml` para gerar mocks das 6 interfaces declaradas na task 5.0. Escrever unit tests table-driven com mockery v2 (R3, R4).

<requirements>
- 5 use cases como structs com construtor `New*UseCase(deps...)` e método único de execução (`Execute` ou `Handle`)
- Idempotência por `webhookEventID` no `ProcessBillingEventUseCase` via `RecordApplication` (RF-22, ADR-009)
- Ordem das operações na UoW respeitando ADR-009 (1.UpsertUser → 2.FindActiveForUpdate → 3.Apply → 4.RecordApplication → 5.Upsert → 6.MarkProcessed)
- Eventos stale (`OccurredAt < LastEventAt`) retornam no-op (RF-25)
- Erros de parse/event_type desconhecido → `outbox.ErrPermanent` (RF-26)
- `IngestKiwifyWebhookUseCase` publica via `outbox.Publisher.Publish` com payload pointer `{webhook_event_id, provider}` (ADR-001)
- `CheckEntitlementUseCase` cache-first; miss → Postgres → `IsEntitled` → cache set com TTL fixo 5min (RF-33)
- `ReconcileSubscriptionsUseCase` itera batches, compara `local` vs `remote.FetchSubscription`, publica evento sintético em divergência (RF-39, RF-41)
- `AnonymizeWebhookEventsUseCase` usa `PIIRedactor` (entregue na task 7.0; aqui apenas consome o port — defini-la como part do `application/interfaces/pii_redactor.go` se precisar)
- `.mockery.yml` declara as 6 interfaces; `mockery --config .mockery.yml` gera mocks em `application/interfaces/mocks/`
- Cobertura ≥ 90% em cada use case (CA-04)
</requirements>

## Subtarefas

- [ ] 6.1 Estender `.mockery.yml` adicionando entrada `github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces` com 6 interfaces (`SubscriptionRepository`, `WebhookEventRepository`, `BillingProvider`, `EntitlementCache`, `IDGenerator`, `UserResolver`); executar `mockery --config .mockery.yml` e verificar mocks em `internal/billing/application/interfaces/mocks/`.
- [ ] 6.2 `usecases/ingest_kiwify_webhook.go` conforme techspec §IngestKiwifyWebhookUseCase: verifica assinatura → extrai `ExternalEventID` cascade → abre UoW → `InsertIfNew` → publica via `outbox.Publisher.Publish` → return result.
- [ ] 6.3 `usecases/process_billing_event.go` (impl `outbox.Handler`): decode pointer → lookup raw payload → parse canonical → UoW (resolve user → FindActiveForUpdate → stateMachine.Apply → check noop/stale → RecordApplication → Upsert → MarkProcessed) → pós-commit invalidate cache + publish events.Bus.
- [ ] 6.4 `usecases/check_entitlement.go`: cache hit fast-path → cache miss → repo lookup → `identity.EntitlementChecker.IsEntitled` → cache set TTL=5min → return Decision.
- [ ] 6.5 `usecases/reconcile_subscriptions.go`: cursor pagination via `ListByStatusInBatch` → para cada sub: `FetchSubscription` remote → comparar status + period_end → divergência: `InsertIfNew` webhook_event sintético + `Publish` outbox event → rate limit local 100 req/min (RF-38).
- [ ] 6.6 `usecases/anonymize_webhook_events.go`: `ListPendingAnonymization` → para cada row: `redactor.Strip(payload)` → `Anonymize(id, redacted, now)` → batch counters.
- [ ] 6.7 Definir `application/interfaces/pii_redactor.go` com interface `PIIRedactor { Strip(json.RawMessage) (json.RawMessage, error) }` (implementação concreta na task 7.0).
- [ ] 6.8 Suites table-driven para cada use case: caminho feliz + falhas de cada port + idempotência (duplicate, recorded=false) + stale event ignorado + permanent vs transient error classification.

## Detalhes de Implementação

Ver techspec §Design de Implementação para cada use case (especialmente §process_billing_event.go com ordem da UoW). ADR-001 (outbox payload), ADR-009 (idempotência), ADR-011 (trust provider para period_end), ADR-012 (lock pessimista), ADR-013 (PII redactor).

## Critérios de Sucesso

- `mockery --config .mockery.yml --dry-run` confirma 6 mocks gerados sem diff.
- `go test ./internal/billing/application/usecases/... -cover` retorna ≥ 90% por arquivo.
- Test "5x replay do mesmo `webhookEventID` produz 1 row em `billing_event_applications` e 1 estado final em `subscription`" passa (mocks contam chamadas).
- Test "evento com `OccurredAt < LastEventAt` retorna nil sem `RecordApplication`/`Upsert`" passa.
- Test "`ParseEvent` retorna `ErrUnknownKiwifyEventType` → handler retorna erro wrapped com `outbox.ErrPermanent`" passa.
- Test "cache hit em `CheckEntitlement` retorna sem chamar repo" passa.
- Test "divergência detectada em reconciliation publica 1 outbox event com event_type `billing.kiwify.received` apontando para webhook sintético" passa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Suite `IngestKiwifyWebhookSuite` table-driven: signature_ok, signature_failed, payload_invalid, duplicate, publish_failed.
- [ ] Suite `ProcessBillingEventSuite` table-driven: happy path (purchase_approved → Active), happy path renewal, stale event ignored, parse failure → ErrPermanent, illegal transition → ErrPermanent, db transient error, idempotent replay.
- [ ] Suite `CheckEntitlementSuite`: cache hit, cache miss + active subscription → granted, cache miss + no subscription → denied, cache miss + past_due with grace → granted.
- [ ] Suite `ReconcileSubscriptionsSuite`: clean batch (no divergence), divergence detected → synthetic event published, rate limit honored, fetch_failed → erro propagado.
- [ ] Suite `AnonymizeWebhookEventsSuite`: batch normal, batch vazio, erro de redactor em 1 row não interrompe demais.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `.mockery.yml` (alterado — adicionar billing)
- `internal/billing/application/interfaces/pii_redactor.go` (novo)
- `internal/billing/application/usecases/ingest_kiwify_webhook.go` (novo)
- `internal/billing/application/usecases/process_billing_event.go` (novo)
- `internal/billing/application/usecases/check_entitlement.go` (novo)
- `internal/billing/application/usecases/reconcile_subscriptions.go` (novo)
- `internal/billing/application/usecases/anonymize_webhook_events.go` (novo)
- `internal/billing/application/usecases/*_test.go` (novos: 5 arquivos)
- `internal/billing/application/interfaces/mocks/*.go` (gerados por mockery)
- Depende de: task 5.0 (ports), task 3.0 (services), task 4.0 (entities), task 2.0 (VOs)
- Importa: `internal/platform/outbox`, `internal/platform/database`, `internal/platform/clock`, `internal/platform/events`, `internal/identity/...`

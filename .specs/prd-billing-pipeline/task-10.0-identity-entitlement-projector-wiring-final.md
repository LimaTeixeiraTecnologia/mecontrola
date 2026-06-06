# Tarefa 10.0: Identity entitlement (read model, projector, DecideUserEntitlement) e wiring final em cmd/server, cmd/worker e billing.NewBillingModule

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o pipeline ponta a ponta:

1. **Identity entitlement (read model):** declarar `EntitlementReader`/`EntitlementRepository` em `internal/identity/application/interfaces/`, implementar repositório Postgres consumindo `identity_entitlements` (e `identity_entitlements_pending` enquanto E3 não fecha), criar use case `DecideUserEntitlement` que delega para `domain.IsEntitled`, expor DTO `EntitlementDecision`.
2. **Projector cross-module:** consumer `SubscriptionEventProjector` em `internal/identity/infrastructure/messaging/database/consumers/` que recebe `billing.subscription.*` da `events.Dispatcher` e atualiza `identity_entitlements` (ou `identity_entitlements_pending` se ainda não há `user_id`).
3. **Notification handler best-effort:** consumer secundário em billing (ou identity, conforme padrão) que escuta `past_due`/`refunded`/`expired_after_grace` e chama `NotificationSender` (stub no-op no MVP). Falha de envio é log, não erro propagado (RF-20).
4. **`internal/identity/module.go`:** adicionar campos `EntitlementReader`, `SubscriptionProjector`, `EventHandlers []EventHandlerRegistration` (com os 5 tipos billing).
5. **`internal/billing/module.go`:** implementar `NewBillingModule(cfg, o11y, mgr) BillingModule` substituindo o placeholder atual, no estilo `IdentityModule`, expondo `RepositoryFactory`, `WebhookRouter`, `ReconciliationJob`, `KiwifyEventsHousekeeper`, `SubscriptionEventPublisher` e `EventHandlers` (vazio no MVP — billing só produz).
6. **`cmd/server/server.go`:** após `identityModule := identity.NewIdentityModule(...)`, construir `billingModule := billing.NewBillingModule(...)` e registrar `billingModule.WebhookRouter` quando não-nil.
7. **`cmd/worker/worker.go`:** construir `identityModule` e `billingModule`; registrar `identityModule.EventHandlers` em `events.Dispatcher` **antes** de iniciar `outbox.DispatcherJob`; adicionar `billingModule.ReconciliationJob` e `billingModule.KiwifyEventsHousekeeper` ao slice `jobs`.

<requirements>
- Reconhecer RF-19 (sweep 90d full + dashboard MRR/churn) e RF-21 (whitelist comandos administrativos) como **fora do MVP por design** — esta tarefa documenta a não-implementação; nenhum código produzido para satisfazer formalmente esses RFs.
- Manter `NotificationSender` como stub no-op (RF-20 best-effort; Q-01 isolada como decisão operacional).
- Wiring no worker deve registrar handlers identity ANTES de iniciar o dispatcher (evita race com publish billing).
- `internal/billing/module.go` segue padrão de DI manual de `IdentityModule` (AGENTS.md §"Padrao Obrigatorio de Modulo").
- `WebhookRouter.Register(chi.Router)` só é registrado se houver rotas reais (precondição da Tarefa 8.0).
- Cache LRU opcional para `EntitlementReader` aceito (configurável via `BillingConfig.EntitlementCacheCapacity/TTL`); pode ser entregue como no-op TTL=0 se time decidir adiar.
</requirements>

## Subtarefas

- [ ] 10.1 `internal/identity/application/interfaces/entitlement_repository.go`: declarar `EntitlementRepository`/`EntitlementReader` (consumer-defined).
- [ ] 10.2 `internal/identity/application/dtos/output/entitlement_decision.go`: DTO `EntitlementDecision{Entitled bool, Reason string, Subscription domain.Subscription}`.
- [ ] 10.3 `internal/identity/application/usecases/decide_user_entitlement.go`: use case que lê read model e delega para `domain.IsEntitled` (RF-13, RF-14, RF-15).
- [ ] 10.4 `internal/identity/infrastructure/repositories/postgres/entitlement_repository.go`: UPSERT em `identity_entitlements`; lookup por `user_id`; gravação em `identity_entitlements_pending` quando `user_id` é nulo. + integ test.
- [ ] 10.5 `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go`: implementa `events.Handler` para os 5 tipos `billing.subscription.*`; idempotente por `subscription_id`. + unit test.
- [ ] 10.6 `internal/identity/module.go`: estender struct com `EntitlementReader`, `SubscriptionProjector`, `EventHandlers []EventHandlerRegistration`; preencher slice com `{EventType: "billing.subscription.activated", Handler: projector}` e 4 demais.
- [ ] 10.7 Notification handler best-effort consumindo `past_due`/`refunded`/`expired_after_grace`; chama `NotificationSender.NotifyTransition`; erro é log + métrica, nunca propagado.
- [ ] 10.8 `internal/billing/module.go`: substituir placeholder por `NewBillingModule(cfg, o11y, mgr) BillingModule` com DI manual; reusa todos os artefatos das tarefas 4.0/5.0/6.0/7.0/8.0/9.0.
- [ ] 10.9 `cmd/server/server.go`: instanciar `billingModule` após `identityModule`; registrar `billingModule.WebhookRouter` quando não-nil; log `billing module wired`.
- [ ] 10.10 `cmd/worker/worker.go`: instanciar `identityModule` e `billingModule`; loop `for _, reg := range identityModule.EventHandlers { eventsDispatcher.Register(reg.EventType, reg.Handler) }` ANTES do append do `outbox.DispatcherJob`; adicionar `billingModule.ReconciliationJob` e `billingModule.KiwifyEventsHousekeeper` ao slice `jobs`.
- [ ] 10.11 Integration test end-to-end (testcontainers Postgres): POST webhook → 202 → outbox dispatcher despacha → projector grava `identity_entitlements_pending` (ou `identity_entitlements` se `user_id` resolvido por test fixture).
- [ ] 10.12 Integration test confirmando `DecideUserEntitlement` retorna decisão correta para cada um dos 5 estados ativos.

## Detalhes de Implementação

- Wiring esperado em techspec §6.4 e §6.5; preserva exatamente o estilo de `cmd/server` e `cmd/worker` atuais.
- Read model duplicado em identity é decisão ADR-004 (evita query cross-module no hot path; RF-15).
- `identity_entitlements_pending`: enquanto E3 não emite bind por token, o projector grava aqui. Quando E3 fechar (futuro), `user_id` será preenchido por outro consumer e o pending pode ser limpo. Esta tarefa **não** implementa o bind.
- Cache LRU opcional para `EntitlementReader` (`BillingConfig.EntitlementCacheCapacity/TTL`). Pode ser entregue como interface satisfeita por implementação trivial (sem cache) sem bloquear o MVP — registrar trade-off em comentário inline curto.
- Notificação best-effort: em caso de falha do `NotificationSender`, emitir log `billing.notification.failed` e incrementar `billing_notification_failures_total{trigger}`; **não** propagar erro.
- **RF-19 e RF-21 fora do MVP:** validar que o diff não inclui implementação de sweep 90d full, dashboard MRR/churn nem whitelist de comandos administrativos.

## Critérios de Sucesso

- `go build ./...` verde.
- `go vet ./...` sem warnings.
- `go test -race -count=1 ./...` verde.
- `go test -race -count=1 -tags=integration ./...` verde com testcontainers.
- Smoke test manual: subir `server` + `worker` localmente, postar webhook fixture válido, observar 202, row em `billing_subscriptions`, row em `platform_outbox_events`, dispatch para projector, row em `identity_entitlements_pending`.
- `DecideUserEntitlement` retorna `Entitled=true` para `ACTIVE`, `Entitled=true` durante grace de `PAST_DUE`, `Entitled=false` após grace, `Entitled=true` em `CANCELED_PENDING` até `period_end`, `Entitled=false` em `EXPIRED` e `REFUNDED`.
- Diff não introduz código para RF-19 ou RF-21 (validação por revisão).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests de `DecideUserEntitlement` cobrindo 5 estados.
- [ ] Unit test do `SubscriptionEventProjector` (idempotência, with/without user_id).
- [ ] Integ test e2e webhook → outbox → projector → `identity_entitlements_pending`.
- [ ] Integ test do `entitlement_repository` Postgres.
- [ ] Smoke test manual de `cmd/server` + `cmd/worker` co-locados.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/application/interfaces/entitlement_repository.go`
- `internal/identity/application/usecases/decide_user_entitlement.go` + `_test.go`
- `internal/identity/application/dtos/output/entitlement_decision.go`
- `internal/identity/infrastructure/repositories/postgres/entitlement_repository.go` + `_integration_test.go`
- `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go` + `_test.go`
- `internal/identity/module.go` (modificação)
- `internal/billing/module.go` (substituição do placeholder)
- `cmd/server/server.go` (modificação)
- `cmd/worker/worker.go` (modificação)
- Referência: techspec §4 (arquitetura), §6.1, §6.2, §6.4, §6.5, §11.3 (out-of-scope), §11.4 (condicionado a E3/E4).
- Referência: `internal/identity/domain/entitlement.go` (função `IsEntitled` já implementada em E1).

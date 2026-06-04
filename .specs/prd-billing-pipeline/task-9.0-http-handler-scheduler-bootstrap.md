# Tarefa 9.0: HTTP handler + chiserver.Router + scheduler + outbox registrar + bootstrap

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar a camada de entrada e wiring: `KiwifyWebhookHandler` (POST /webhooks/kiwify) com `chiserver.Router` reusando contrato do devkit-go (ADR-002); `WithRouteTimeout("/webhooks/kiwify", 2*time.Second)` em `internal/platform/http/server.go`; `BillingScheduler` Subsystem com `robfig/cron/v3` para reconciliation + anonymization (ADR-003); registro de `BillingEventProcessor` no `outbox.Registry`; `KiwifyConfig` + `BillingConfig` em `configs/config.go` com defaults e validation; `runtime/billing_subsystem.go` orquestrando tudo no bootstrap.

<requirements>
- `internal/platform/http/server.go` adiciona `Registrars []chiserver.Router` em `Deps` e chama `srv.RegisterRouters(deps.Registrars...)` — preserva comportamento atual quando slice é nil
- `chiserver.WithRouteTimeout("/webhooks/kiwify", 2*time.Second)` em `buildOptions` (RF-06 ack p99 < 2s)
- `KiwifyWebhookHandler` traduz: `kiwify.ErrInvalidSignature/ErrMissingSignature` → 401; `kiwify.ErrPayloadDecode` → 400; `IngestWebhookResult{Duplicate:true}` → 204; sucesso → 200 com `{"received":true,"duplicate":false}`; demais erros → 500 com correlation_id no log (sem expor causa) (RF-01, RF-04, RF-06, RF-09)
- Headers JSONB salvos em `webhook_events.headers` excluem `Authorization`/`Cookie` (RF-08)
- `KiwifyRouteRegistrar` implementa `chiserver.Router` (`Register(r chi.Router)`)
- `BillingScheduler` registra reconciliation `@hourly` (configurável) e anonymization `@daily` (configurável); semáforo `TryLock` para evitar overlap (ADR-003)
- `BillingEventProcessor` registrado em `outbox.Registry` com event_type `billing.kiwify.received`, subscription_name `billing-event-processor`, **antes** do `Dispatcher.Start` (RF-21, RF-47)
- `KiwifyConfig` + `BillingConfig` em `configs/config.go` com env vars `KIWIFY_*` e `BILLING_*`; defaults conforme techspec §Configuração; `SafeKiwifyConfig()` redacta secrets (RF-44)
- `runtime/billing_subsystem.go` constrói: HTTP client, OAuth client, signature verifier, payload mapper, adapter Kiwify, repos Postgres, cache LRU, redactor, use cases, scheduler, route registrar; injeta no http subsystem e em `outbox.Registry`
- Sem `init()`; subsystem lifecycle (`Start`/`Stop`) compatível com `runtime.Subsystem`
- Logs sempre via `mask.WhatsApp`/`mask.Email` em campos PII (RF-42)
- Spans OTel: `billing.webhook.ingress`, `billing.event.process`, `billing.entitlement.check`, `billing.reconciliation.tick`, `billing.anonymization.tick`, `kiwify.oauth.fetch_token`, `kiwify.api.fetch_subscription` (RF-43)
</requirements>

## Subtarefas

- [ ] 9.1 Estender `internal/platform/http/Deps` com `Registrars []chiserver.Router`. Em `NewServer`, chamar `srv.RegisterRouters(deps.Registrars...)` após criação. Adicionar `chiserver.WithRouteTimeout("/webhooks/kiwify", 2*time.Second)` em `buildOptions`. Teste com router fake confirma que rota responde.
- [ ] 9.2 `internal/platform/runtime/http_subsystem.go` propaga `Registrars` via `infrahttp.Deps`. Aceita slice via novo parâmetro em `newServerSubsystem`.
- [ ] 9.3 `internal/billing/infrastructure/http/server/kiwify_webhook_handler.go` com `KiwifyWebhookHandler struct{}`, construtor recebendo `*usecases.IngestKiwifyWebhookUseCase` + `*slog.Logger` + `clock.Clock`. Método `ServeHTTP`: ler body com limit 1 MiB, copiar headers (excluindo `Authorization`/`Cookie`), montar `IngestWebhookInput`, chamar use case, mapear erro → status, escrever JSON resposta. Span OTel `billing.webhook.ingress`.
- [ ] 9.4 `internal/billing/infrastructure/http/server/route_registrar.go` com `KiwifyRouteRegistrar struct{ handler *KiwifyWebhookHandler }` implementando `chiserver.Router`. `Register(r chi.Router)` chama `r.Post("/webhooks/kiwify", h.handler.ServeHTTP)`.
- [ ] 9.5 `internal/billing/infrastructure/scheduler/subsystem.go` com `BillingScheduler` + `Deps` + lifecycle `Start`/`Stop`. `robfigcron.New(robfigcron.WithSeconds())`. AddFunc para reconciliation + anonymization, cada um envolvido em `TryLock` semáforo + log de skip se overlap.
- [ ] 9.6 `internal/billing/infrastructure/scheduler/reconciliation_job.go` e `anonymization_job.go` wrappers chamando o use case correspondente com timeout via context.WithTimeout.
- [ ] 9.7 `internal/billing/infrastructure/outbox/event_payload.go` com `ReceivedPayload struct{WebhookEventID; Provider string}` + `EncodeReceivedPayload(id) (json.RawMessage, error)` + `DecodeReceivedPayload(raw) (ReceivedPayload, error)` (ADR-001).
- [ ] 9.8 `internal/billing/infrastructure/outbox/handler.go` com `RegisterHandlers(registry *outbox.Registry, processor *usecases.ProcessBillingEventUseCase) error` chamando `registry.Register(outbox.Subscription{Name:"billing-event-processor", EventType:"billing.kiwify.received", Handler: processor.Handle})`.
- [ ] 9.9 Estender `configs/config.go` com `KiwifyConfig` + `BillingConfig` (campos da techspec §Configuração) + defaults em `setDefaults()` + `Safe*` methods + validation em `Validate()` (rejeita boot prod sem secrets).
- [ ] 9.10 `internal/platform/runtime/billing_subsystem.go` (lazy subsystem) constrói toda a cadeia: db manager → identity user resolver wrapper → repos → LRU cache → kiwify http client → oauth → adapter → use cases → scheduler → registrar → registra outbox handler. Implementa `Subsystem` (`Start`, `Stop`, `Name`).
- [ ] 9.11 `cmd/server/server.go` (e `cmd/worker` se aplicável) injeta `runtime.BootstrapWithBilling(...)` ou estende `runtime.Bootstrap` para incluir billing subsystem. Garantir registro de handler **antes** do `Dispatcher.Start`.

## Detalhes de Implementação

Ver techspec §HTTP routing pattern (ADR-002 revisado), §Scheduler Subsystem (ADR-003), §Configuração, §Adapter Kiwify. Atenção à ordem de bootstrap: `outbox.Registry` precisa ter handler registrado antes do `Dispatcher.Start` — orquestrar via `Application.Subsystems` order ou hook explícito.

## Critérios de Sucesso

- `curl -X POST http://localhost:8080/webhooks/kiwify -H 'X-Kiwify-Webhook-Token: wrong'` retorna `401`.
- `curl -X POST http://localhost:8080/webhooks/kiwify -H 'X-Kiwify-Webhook-Token: $SECRET' -d '{"id":"1",...}'` retorna `200` na primeira chamada e `204` na segunda (idempotência).
- `curl ... -d 'not-json'` retorna `400`.
- Boot log inclui `billing reconciliation agendada` e `billing anonymization agendada`.
- Boot log inclui `outbox subscription registered: billing-event-processor`.
- `go run ./cmd server` sobe sem erro com env vars `KIWIFY_*` setadas; falha de boot clara se secrets ausentes em prod.
- Lint verde; `depguard` aprova: `internal/billing/infrastructure/` é único que importa `chi`, `pgx`, `golang-lru`, `robfigcron`.
- Spans OTel emitidos: verificar via traces exporter em ambiente local (collector mock).
- Logs com WhatsApp/email em claro retornam zero entradas em grep (`grep -E '"whatsapp_number":"[+0-9]' logs.json` vazio).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Suite `KiwifyWebhookHandlerSuite` table-driven: 200 sucesso, 204 duplicata, 400 payload inválido, 401 assinatura, 500 erro DB, header `Authorization` não persistido em headers JSONB.
- [ ] Suite `RouteRegistrarSuite`: registrar é chamado e rota responde via httptest.
- [ ] Suite `BillingSchedulerSuite`: Start agenda 2 jobs; semáforo TryLock pula execução overlap (mocka job longo).
- [ ] Suite `EventPayloadSuite`: encode/decode roundtrip, decode com JSON inválido → erro.
- [ ] Suite `ConfigSuite`: defaults aplicados, validation falha em prod sem secrets, `SafeKiwifyConfig` redacta campos.
- [ ] Teste integração `platform/http/server_test.go` com fakeRouter confirmando que `RegisterRouters` é chamado.
- [ ] Smoke local manual: `task task:smoke-billing` (script criado nesta task ou adicionado em Taskfile) sobe servidor + envia webhook fake + valida 200.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/platform/http/server.go` (alterado — Deps.Registrars + WithRouteTimeout)
- `internal/platform/http/server_test.go` (alterado — caso com fakeRouter)
- `internal/platform/runtime/http_subsystem.go` (alterado — propaga Registrars)
- `internal/platform/runtime/billing_subsystem.go` (novo)
- `internal/billing/infrastructure/http/server/kiwify_webhook_handler.go` (novo)
- `internal/billing/infrastructure/http/server/route_registrar.go` (novo)
- `internal/billing/infrastructure/scheduler/subsystem.go` (novo)
- `internal/billing/infrastructure/scheduler/reconciliation_job.go` (novo)
- `internal/billing/infrastructure/scheduler/anonymization_job.go` (novo)
- `internal/billing/infrastructure/outbox/event_payload.go` (novo)
- `internal/billing/infrastructure/outbox/handler.go` (novo)
- `configs/config.go` (alterado — KiwifyConfig + BillingConfig)
- `cmd/server/server.go` (alterado — bootstrap billing subsystem)
- Depende de: task 6.0 (use cases), task 7.0 (adapter Kiwify), task 8.0 (repos + cache + UUID)
- Importa: `internal/platform/outbox`, `internal/platform/http`, `internal/platform/runtime`, `internal/identity/...`

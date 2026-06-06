# ADR-003 — Cross-module billing→identity via Outbox + events.Dispatcher in-process

## Metadados

- **Título:** Entrega cross-module por outbox transacional + events.Dispatcher in-process
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-billing-pipeline/techspec.md` §4, §5.1, `internal/platform/outbox/dispatcher.go`, `internal/platform/events/dispatcher.go`, AGENTS.md "Worker, HTTP Outbound e Outbox"

## Contexto

`internal/identity` precisa reagir a transições de assinatura para manter seu read model (`identity.entitlements`). `internal/billing` é dono do agregado `Subscription`. As duas regras de governança aplicáveis:

- Comunicação cross-module deve usar **interface declarada pelo consumidor**, **domain event** ou **outbox** (AGENTS.md).
- Toda escrita que precisa sobreviver a crash/deploy/restart deve usar `outbox.Publisher` (AGENTS.md "Outbox"); entrega é at-least-once; handlers devem ser idempotentes por `event.ID`.

O `cmd/worker` já instancia `events.Dispatcher` (in-process) e o registra no `outbox.DispatcherJob`. Identity ainda não registra nenhum handler nele (gap real do codebase).

## Decisão

Billing usa `outbox.Publisher` (na mesma transação dos use cases) para todo evento `billing.subscription.*`. O worker (`cmd/worker`) constrói `identity.NewIdentityModule` e registra cada `events.Handler` exposto via `IdentityModule.EventHandlers` no `events.Dispatcher` global. O `outbox.DispatcherJob` reclama rows, invoca `events.Dispatcher.Dispatch`, e o handler de identity (`SubscriptionEventProjector`) atualiza `identity.entitlements`/`identity.entitlements_pending`.

Idempotência cross-module garantida por:
- `subscription_id` como aggregate;
- UPSERT em `identity.entitlements(user_id PK)` ou `identity.entitlements_pending(subscription_id PK)`;
- `event.ID` (UUID) reportado em log para forensics.

## Alternativas Consideradas

1. **`consumer.Runner` + `consumer.Source` lendo billing.outbox diretamente.** Recusada — duplica o caminho que `outbox.DispatcherJob` já implementa; introduziria mais um runner e novo registry sem ganho operacional no MVP.
2. **Chamada direta billing → identity em-processo (use case → use case).** Recusada — quebra a fronteira de módulo (billing passa a importar identity), perde durabilidade (em crash entre commit e chamada, evento se perde) e quebra at-least-once.
3. **Broker externo (NATS/Kafka).** Recusada — over-engineering para monolito modular; nenhum benefício no MVP.
4. **HTTP entre módulos (call interno).** Recusada — viola fronteira e sobrecarrega rede para nada.

## Consequências

### Benefícios Esperados

- Durabilidade garantida (commit do evento na mesma transação do agregado).
- At-least-once → projector idempotente cobre duplicatas.
- Caminho já provado em produção pelo outbox.
- Sem broker externo.

### Trade-offs e Custos

- Cross-module same-binary: precisa que o **worker** suba ambos os módulos (billing + identity). Server (`cmd/server`) só precisa de billing para o endpoint webhook; não registra handlers (não é o consumidor).
- Falha do projector cascateia retry exponencial no outbox; após `RetryMaxAttempts` vira `failed` e exige reset manual.

### Riscos e Mitigações

- **R:** Projector identity falha persistente bloqueia entitlements. **M:** Alerta operacional em `outbox` rows `failed` para tipos `billing.subscription.*`; runbook §9.4.
- **R:** Ordering entre eventos (renewed antes de activated). **M:** Projector idempotente; estado final converge (ADR-005 cobre ordering no produtor; projector aceita qualquer ordem).
- **R:** Acoplamento via nome de evento. **M:** Type names estáveis e versionados implicitamente por payload backward-compatible; mudança de schema exige novo type (`billing.subscription.activated.v2`).

## Plano de Implementação

1. `internal/billing/infrastructure/messaging/database/producers/{events,subscription_event_publisher}.go` — define `events.Event` concretos + `Publisher` que serializa e chama `outbox.Publisher`.
2. `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go` — implementa `events.Handler` para cada um dos 5 tipos.
3. `internal/identity/module.go` — expor `EventHandlers []EventHandlerRegistration`.
4. `cmd/worker/worker.go` — após criar `eventsDispatcher`, iterar `identityModule.EventHandlers` e registrar; depois iniciar `outbox.DispatcherJob` (que já consome o dispatcher).
5. Testes de integração `outbox_to_projector_test.go`.

## Monitoramento e Validação

- Métrica `outbox` (já existente) para `billing.subscription.*`: rows `pending|claimed|published|failed`.
- Log `identity.entitlement.projected` em cada UPSERT.
- Alerta: `outbox{event_type=~"billing.subscription.*",status="failed"} > 0` sustained por 5min.

## Impacto em Documentação e Operação

- README operacional do worker deve documentar que ele precisa rodar para entitlements funcionarem.
- Runbook §9.4 cobre falha persistente.

## Revisão Futura

- Reabrir se a aplicação for dividida em múltiplos binários (monolito → micro), quando broker externo se torna preferencial.
- Reabrir se o volume cross-module exceder a capacidade do `events.Dispatcher` in-process (improvável no MVP).

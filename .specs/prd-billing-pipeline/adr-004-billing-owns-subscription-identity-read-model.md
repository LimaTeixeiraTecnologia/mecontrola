# ADR-004 — Billing é dono do agregado Subscription; Identity mantém read model materializado

## Metadados

- **Título:** Billing owns Subscription; identity holds materialized read model
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-billing-pipeline/techspec.md` §4, §6, [ADR-003](./adr-003-outbox-to-events-dispatcher-cross-module.md), `internal/identity/domain/entitlement.go`

## Contexto

`internal/identity/domain/entitlement.go` (do E1) define a interface `Subscription` (`Status() / PeriodEnd() / GracePeriodEnd()`) e a função pura `IsEntitled(sub, now) (bool, Reason)`. O PRD E2 exige a decisão única de direito de acesso (RF-13/14/15) com latência baixa e sem replicar a regra de negócio.

Trade-off arquitetural: **onde mora o agregado Subscription** (billing — coerente com o gateway que o domina, ou identity — coerente com quem decide entitlement) e **como identity acessa o estado** (cross-module query vs read model materializado).

A regra de fronteira de módulo (AGENTS.md) exige interface no consumidor e proíbe acoplamento em schemas alheios; latência no hot path de decisão de entitlement exige read local.

## Decisão

- **Billing é dono canônico** do agregado `Subscription` e da tabela `billing_subscriptions` (e tabelas correlatas: `billing_plans`, `billing_processed_events`, `billing_kiwify_events`, `billing_reconciliation_checkpoints`).
- **Identity mantém read model** `identity_entitlements(user_id PK, subscription_id, status, period_end, grace_end, updated_at)` materializado por `SubscriptionEventProjector` (ADR-003).
- Para usuários ainda não vinculados (sub criada com `funnel_token` mas sem `user_id` — pré-E3), o projector grava em `identity_entitlements_pending(subscription_id PK, funnel_token, payload JSONB)`. E3 (futuro) move da pending para a tabela final ao fechar bind user↔token.
- A função pura `domain.IsEntitled` consome um `Subscription` construído sobre uma linha de `identity_entitlements` (via adapter local em `internal/identity/infrastructure/repositories/postgres/entitlement_repository.go`).
- **Identity nunca consulta `billing.*` diretamente.**

## Alternativas Consideradas

1. **Billing dono; identity faz query cross-module via `interfaces.SubscriptionReader` (implementada por billing).** Recusada — acopla identity ao schema/latência de billing; viola fronteira; identity passaria a depender de billing em tempo de chamada, não só em tempo de evento.
2. **Tabela compartilhada `subscriptions` lida por ambos.** Recusada — quebra DDD (cada módulo é dono do seu schema) e impede evoluir cada módulo independentemente.
3. **Identity como dono e billing apenas como adapter.** Recusada — billing tem regra de domínio rica (state machine, idempotência, ordering, reconciliação) que vive naturalmente onde o gateway é processado. Identity ficaria com regra de billing forçada.

## Consequências

### Benefícios Esperados

- **Latência baixa no hot path:** `IsEntitled` lê uma linha local (`identity_entitlements`) sem cross-module.
- **Fronteira limpa:** billing pode evoluir schema, regras de transição e idempotência sem afetar identity (compatibilidade preservada pelo contrato dos eventos).
- **Cache simples:** opcional LRU em `BillingConfig.EntitlementCacheCapacity/TTL` sobre `identity_entitlements`.

### Trade-offs e Custos

- **Duplicação de estado:** mesma assinatura existe em `billing_subscriptions` + `identity_entitlements`. Aceito pelo ganho de latência e fronteira.
- **Janela de inconsistência eventual:** entre commit em billing e projection em identity há latência (definida pelo tick do `outbox.DispatcherJob`, default sub-segundo). Aceitável para o caso de uso.

### Riscos e Mitigações

- **R:** Read model desincronizado de `billing_subscriptions`. **M:** Job de auditoria opcional (fora do MVP) compara as duas tabelas e reporta diff; correção manual via re-emissão de evento.
- **R:** Falha do projector deixa `identity_entitlements` stale. **M:** Outbox retry + alerta (ADR-003).
- **R:** Subscription com `user_id` NULL fica em `entitlements_pending` indefinidamente. **M:** E3 implementa o bind; até lá, métricas indicam volume pending e identificam abandono de funil.

## Plano de Implementação

1. Migration `0005_create_billing_subscriptions.up.sql`.
2. Migration `0009_create_identity_entitlements.up.sql` (cria `identity_entitlements` + `identity_entitlements_pending`).
3. `internal/billing/domain/entities/subscription.go` — agregado com state machine.
4. `internal/billing/infrastructure/repositories/postgres/subscription_repository.go`.
5. `internal/identity/application/interfaces/entitlement_repository.go` — `Get(ctx, userID) (Subscription, error)` (retorna interface do domain).
6. `internal/identity/infrastructure/repositories/postgres/entitlement_repository.go` — implementa, retorna adapter `entitlementRow` que implementa `domain.Subscription`.
7. `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go` — handler de cada um dos 5 tipos.
8. `internal/identity/application/usecases/decide_user_entitlement.go` — wraps `domain.IsEntitled` e expõe DTO.

## Monitoramento e Validação

- Métrica `billing_subscription_transitions_total{from,to,trigger}` (canônico em billing).
- Métrica `identity_entitlements_projected_total{status,from_pending}` (em identity).
- Auditoria periódica (futura, fora do MVP): comparar contagens por status entre `billing_subscriptions` e `identity_entitlements`.

## Impacto em Documentação e Operação

- Diagrama de arquitetura: billing como dono; identity como consumer.
- Runbook de incidente: documentação clara de que `IsEntitled` lê de identity (não de billing).

## Revisão Futura

- Reabrir se a regra de entitlement evoluir para depender de dados externos a `(status, period_end, grace_end)` (ex.: limites de uso, planos com features); pode exigir read model mais rico ou caminho diferente.
- Reabrir se a janela de eventual consistency se tornar inaceitável (cenários de fraude em tempo real, p. ex.).

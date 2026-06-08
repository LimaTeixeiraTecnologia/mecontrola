# ADR-003 — Contrato E2 → E3 via outbox `billing.subscription.activated` enriquecido + novo type `subscription.activated_without_token`

## Metadados

- **Título:** Acoplamento E2 → E3 por eventos outbox enriquecidos
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §6.6, RF-03, RF-18; `.specs/prd-billing-pipeline/adr-003-outbox-to-events-dispatcher-cross-module.md`; [ADR-004](./adr-004-adopt-tracking-sck-as-magic-token-carrier.md), [ADR-006](./adr-006-support-signals-single-table.md)

## Contexto

O PRD (RF-03 e RF-18) exige que o webhook `compra_aprovada` de E2 alimente E3 com:
1. `funnel_token` (quando presente) para marcar `onboarding_tokens.status=PAID`.
2. `customer_mobile_e164` (digitado no checkout) — necessário para outreach (RF-09) e fallback E.164 (RF-10).
3. `customer_email` — usado para suporte (RF-18) e para gravar no user (E1 reanimação).
4. `external_sale_id` — para correlação com Kiwify e sinal RF-18.
5. `paid_at` — para janela do outreach e métrica `paid_to_consumed_seconds`.

Quando `funnel_token` está ausente (S-14), o PRD (RF-18) exige que o pagamento **não** seja descartado e que um sinal seja emitido para suporte.

E2 atual:
- Publica `billing.subscription.activated` com `subscription_id, funnel_token, plan_id, status, period_end` apenas.
- Falha com `ErrFunnelTokenMissing` (HTTP 422 ao webhook Kiwify) quando token vazio — comportamento incompatível com RF-18.

## Decisão

### 1. Enriquecer payload de `billing.subscription.activated`
Adicionar ao JSON payload:
```json
{
  "subscription_id": "...",
  "funnel_token": "...",
  "plan_id": "...",
  "external_sale_id": "...",
  "customer_mobile_e164": "+5511...",
  "customer_email": "fulano@exemplo.com",
  "paid_at": "2026-06-06T12:34:56Z",
  "period_end": "2026-07-06T12:34:56Z"
}
```
Campos preexistentes mantêm contrato; novos campos são aditivos (consumers antigos toleram).

### 2. Introduzir novo type `billing.subscription.activated_without_token`
Publicado quando `funnel_token` vazio:
```json
{
  "external_sale_id": "...",
  "customer_mobile_e164": "+5511...",
  "customer_email": "fulano@exemplo.com",
  "paid_at": "2026-06-06T12:34:56Z",
  "plan_id": "..."
}
```
Sem `subscription_id` no caminho consumido por onboarding? **Não** — a sub **é** criada normalmente em E2 (PRD RF-18 §"E2 segue criando a assinatura conforme suas próprias regras"); o `subscription_id` está no payload mas é irrelevante para onboarding (não há token para linkar). Mantemos o campo para auditoria.

### 3. Remover bloqueio por token ausente em E2
`ProcessSaleApproved` deixa de falhar com `ErrFunnelTokenMissing`. Em vez disso, persiste a sub com `funnel_token=NULL` (coluna passa a ser nullable) e publica `subscription.activated_without_token`. Comportamento downstream:
- `EntitlementProjector` (E1) processa normalmente — sub sem user vinculado ainda é projetável quando user for criado por suporte.
- `OnboardingPaidWithoutTokenConsumer` insere `support_signals(kind='paid_without_token')`.

### 4. Onboarding publica `onboarding.subscription_bound`
Após `ConsumeMagicToken`, E3 publica:
```json
{
  "user_id": "...",
  "subscription_id": "...",
  "funnel_token_hash_prefix": "ab12cd34",
  "bound_at": "2026-06-06T12:36:00Z",
  "activation_path": "direct"
}
```
`EntitlementProjector` (E1) registra novo handler para esse type e dispara reprojeção do entitlement do user.

## Alternativas Consideradas

1. **Interface hook síncrona `OnPaid` injetada em E2.** Recusada — acopla módulos via interface, falha de E3 bloqueia commit de E2, força retry de Kiwify por motivo errado.
2. **Chamada HTTP de E2 para E3 (`POST /internal/...`).** Recusada — rede no caminho crítico, sem garantia transacional, viola padrão outbox.
3. **Tabela compartilhada `billing.subscriptions` lida diretamente por E3.** Recusada — fere fronteira E2↔E3, força E3 a conhecer schema de E2.
4. **Manter falha em token ausente.** Recusada — viola RF-18 explicitamente; pagamento desapareceria.

## Consequências

### Benefícios
- Acoplamento assíncrono respeitando padrão outbox existente.
- Idempotência herdada do dispatcher (`event_id` único).
- E2 não conhece E3 (publisher cego); E3 conhece type `billing.*` (consumer registrado).
- RF-18 atendido sem retry de Kiwify.

### Trade-offs
- Payload outbox cresce (`customer_email` em JSON). **Mitigação:** mascarar em logs do dispatcher (`MaskedEmail`).
- E2 precisa de alteração comportamental (deixar de falhar com `ErrFunnelTokenMissing`). **Mitigação:** mudança documentada aqui; teste de regressão obrigatório em E2.

### Riscos e Mitigações
- **R:** Consumer onboarding falha persistente → outbox cresce indefinidamente. **M:** `outbox.DispatcherJob` já tem retry com backoff + dead letter por tentativas máx. Métrica em platform indica saúde.
- **R:** Email em payload outbox viola LGPD. **M:** Acesso ao DB de outbox restrito por role; logs do dispatcher mascaram via VO.
- **R:** Type novo `created_without_token` não registrado em consumer → eventos órfãos. **M:** Test integração de wiring valida registro no `events.Dispatcher`.

## Plano de Implementação
1. Alterar `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go` — payload enriquecido + novo type.
2. Alterar `internal/billing/application/usecases/process_sale_approved.go` — não falhar com token ausente.
3. Alterar `internal/billing/application/usecases/process_kiwify_webhook.go` — extrair `Customer.mobile`, `Customer.email`, `order_ref` (external sale id).
4. Adicionar `funnel_token NULL` em `billing.subscriptions` (migration idempotente — pode já ser).
5. Registrar consumers em `cmd/worker` (E3).
6. Registrar handler `onboarding.subscription_bound` em `EntitlementProjector` (E1).
7. Test integração cross-module: simular webhook Kiwify → outbox → E3 consumer → estado consistente.

## Monitoramento
- Métrica `billing_paid_without_token_total{provider="kiwify"}` (já em RF-18).
- Métrica platform `outbox_consumer_failures_total{type, consumer}` (já existe).

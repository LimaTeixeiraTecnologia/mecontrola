# Billing

Pipeline de faturamento do MeControla: ingestão de webhooks, processamento de eventos, verificação de entitlement e anonimização.

## Como Adicionar uma Nova Rota HTTP

1. Crie o handler em `infrastructure/http/handlers/`.
2. Registre a rota via `chiserver.Router` em `module.go`:
   ```go
   router.Post("/billing/minha-rota", handler.Handle)
   ```
3. Adicione o use case correspondente em `application/usecases/` e declare sua interface em `application/interfaces/`.

## Como Ajustar o Schedule dos Jobs

Os jobs periódicos são ligados em `module.go` e executados por `infrastructure/scheduler/`:

- **Reconciliação**: ajuste o intervalo em `ReconcileSubscriptions` scheduler.
- **Anonimização**: ajuste o intervalo e `OlderThan` em `AnonymizeWebhookEvents` scheduler.

## Runbook Básico

### Webhook duplicado não detectado

Verificar a constraint UNIQUE `webhook_events(provider, external_event_id)`. Se estiver ausente, rodar migration pendente.

### Subscription em estado inválido

Executar `ReconcileSubscriptionsUseCase` manualmente via endpoint de admin ou trigger do scheduler. O use case compara estado local com o provedor e publica evento sintético de correção.

### PII não anonizado após 365 dias

Verificar se o job `AnonymizeWebhookEvents` está em execução. Checar `anonymized_at IS NULL AND received_at < now() - interval '365 days'` na tabela `webhook_events`.

### Entitlement retornando `denied` incorretamente

1. Confirmar `period_end` na tabela `subscriptions`.
2. Confirmar que o clock do servidor está sincronizado (NTP).
3. Se `period_end` estiver desatualizado, disparar reconciliação.

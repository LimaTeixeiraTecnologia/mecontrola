// Package domain contém a lógica de negócio pura do módulo billing.
//
// Responsabilidades:
//   - entities/: agregados (Subscription, WebhookEvent) com invariantes e máquina de estados
//   - valueobjects/: tipos com identidade por valor (ExternalEventID, SubscriptionStatus, PlanCode)
//   - services/: serviços de domínio sem efeitos colaterais (StateMachine, PIIRedactor, CanonicalEvent)
//
// Invariantes principais:
//   - Subscription só aceita transições legais definidas em StateMachine
//   - ExternalEventID é construído apenas via NewExternalEventIDCascade — nunca de string direta
//   - grace period de cancelamento: 7 dias após period_end (CANCELED_PENDING → denied após expirar)
//   - period_end é confiado ao provedor externo sem ajuste local (trust provider)
//
// Restrição absoluta: este pacote não pode importar infrastructure, application, platform ou configs.
package domain

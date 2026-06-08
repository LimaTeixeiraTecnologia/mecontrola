// Package onboarding implementa o modulo E3 — Onboarding via Magic Token.
//
// Contratos exportados via NewOnboardingModule (a ser criado em tasks seguintes):
//
//   - domain: MagicToken (aggregate root), SupportSignal, VOs de token/status/path/kind.
//   - application/interfaces: RepositoryFactory, WhatsAppGateway, CheckoutURLBuilder, IdentityGateway.
//   - application/usecases: CreateCheckoutSession, MarkTokenPaid, ConsumeMagicToken,
//     TryFallbackActivation, SendOutreach, ExpireTokens, GetTokenState, HandlePaidWithoutToken.
//   - infrastructure/repositories: factories Postgres para MagicTokenRepository e SupportSignalRepository.
//
// Migrations: 0010 (schema onboarding + tokens), 0011 (support_signals),
// 0012 (extend billing_subscriptions), 0013 (meta_processed_messages).
package onboarding

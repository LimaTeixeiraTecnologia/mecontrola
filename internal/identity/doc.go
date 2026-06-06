// Package identity expoe o agregado User, o port RepositoryFactory e as
// abstracoes de domain necessarias para que outros modulos (billing, etc.)
// consumam identidade sem acoplar-se a infraestrutura.
//
// Contratos exportados:
//
//   - NewIdentityModule(cfg, o11y, mgr): bootstrap canonico seguindo o
//     Padrao Obrigatorio de Modulo (AGENTS.md). Recebe configuracao,
//     observabilidade e database manager; instancia repositorios, casos de
//     uso, handler HTTP de upsert e o projector de eventos.
//   - IdentityModule.RepositoryFactory: ponto de entrada para criar
//     UserRepository e EntitlementRepository amarrados a uma database.DBTX
//     (pool ou transacao). Consumido por billing para upsert e leitura
//     transacional.
//   - IdentityModule.UserRouter: placeholder para registro de rotas HTTP
//     (POST /api/v1/identity/users). Bootstrap em cmd/server so registra
//     quando != nil.
//   - IdentityModule.UpsertUserUseCase / FindUserByIDUseCase /
//     FindUserByWhatsApp / MarkUserDeleted: casos de uso para gestao de
//     usuarios via WhatsApp. Reanimacao dentro da janela de
//     domain.ReanimationWindow preserva UUID; fora da janela cria novo.
//   - IdentityModule.EntitlementReader: leitura somente do estado de
//     entitlement projetado a partir de eventos de billing.
//   - IdentityModule.SubscriptionProjector / EventHandlers: registros de
//     handlers para eventos billing.subscription.* consumidos pelo worker.
//
// Sentinels em internal/identity/application/errors.go (ADR-004):
// ErrUserNotFound, ErrWhatsAppNumberInUse, ErrEmailInUse,
// ErrEntitlementNotFound. Usar errors.Is para checagem.
//
// Garantias de dominio: PII mascarada em logs (valueobjects.Masked,
// pii.MaskDisplayName); IDs gerados por entities.NewID (UUID v4) sem
// injecao; janela de reanimacao constante via domain.ReanimationWindow.
package identity

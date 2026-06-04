package interfaces

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

// UserRepository é o port canônico de persistência do agregado User.
// Implementações concretas vivem em infrastructure/repositories/postgres.
//
// Contrato de soft delete: toda leitura filtra deleted_at IS NULL implicitamente.
// FindByID e FindByWhatsAppNumber retornam entities.ErrUserNotFound quando o registro
// não existe ou está soft-deleted.
//
// Transacionalidade encapsulada: LinkNewNumber e SoftDelete abrem sua própria UnitOfWork
// internamente — nenhum tipo de transação (tx, pgx.Tx, sql.Conn) é exposto na assinatura
// (ADR-010).
type UserRepository interface {
	// UpsertByWhatsAppNumber retorna o User ativo com o número, criando-o se não existir.
	// Idempotente: chamadas repetidas com o mesmo número retornam o mesmo UserID.
	UpsertByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber, now time.Time) (*entities.User, error)

	// FindByID localiza um User ativo pelo seu ID.
	// Retorna ErrUserNotFound quando o registro não existe ou está soft-deleted.
	FindByID(ctx context.Context, id entities.UserID) (*entities.User, error)

	// FindByWhatsAppNumber localiza um User ativo pelo número WhatsApp normalizado.
	// Retorna ErrUserNotFound quando o registro não existe ou está soft-deleted.
	FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (*entities.User, error)

	// SoftDelete marca o User como deletado e desativa todos os registros em
	// user_whatsapp_history para o mesmo user_id (ADR-009).
	// Retorna ErrUserNotFound se o User não existe ou já está deletado.
	SoftDelete(ctx context.Context, id entities.UserID, now time.Time) error

	// LinkNewNumber desativa o número atual, registra o histórico e atualiza
	// users.whatsapp_number de forma atômica (ADR-010).
	// Retorna ErrUserNotFound se o User não existe ou está soft-deleted.
	LinkNewNumber(ctx context.Context, id entities.UserID, number valueobjects.WhatsAppNumber, reason string, now time.Time) error
}

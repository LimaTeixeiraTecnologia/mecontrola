package interfaces

import (
	"context"

	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

// UserResolver é o port de resolução cross-module para o agregado User.
// Wrapper segregado de identity.UserRepository.UpsertByWhatsAppNumber — billing
// não depende da interface completa de identity (princípio de segregação de interface).
type UserResolver interface {
	UpsertByWhatsAppNumber(ctx context.Context, number identityvo.WhatsAppNumber) (*identityentities.User, error)
}

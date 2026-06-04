package infrastructure

import (
	"context"
	"time"

	identityinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type IdentityUserResolverAdapter struct {
	repo identityinterfaces.UserRepository
}

func NewIdentityUserResolverAdapter(
	repo identityinterfaces.UserRepository,
) *IdentityUserResolverAdapter {
	return &IdentityUserResolverAdapter{
		repo: repo,
	}
}

func (a *IdentityUserResolverAdapter) UpsertByWhatsAppNumber(
	ctx context.Context,
	number identityvo.WhatsAppNumber,
) (*identityentities.User, error) {
	return a.repo.UpsertByWhatsAppNumber(ctx, number, time.Now().UTC())
}

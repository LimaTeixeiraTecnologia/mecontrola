package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type ResolvedChannel struct {
	Channel    string
	ExternalID string
}

type ResolvePreferredChannel struct {
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewResolvePreferredChannel(mgr manager.Manager, factory interfaces.RepositoryFactory, o11y observability.Observability) *ResolvePreferredChannel {
	return &ResolvePreferredChannel{mgr: mgr, factory: factory, o11y: o11y}
}

func (uc *ResolvePreferredChannel) Execute(ctx context.Context, userID uuid.UUID) (ResolvedChannel, bool, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "identity.usecase.resolve_preferred_channel")
	defer span.End()

	repo := uc.factory.UserIdentityRepository(uc.mgr.DBTX(ctx))
	identities, err := repo.ListByUser(ctx, userID)
	if err != nil {
		span.RecordError(err)
		return ResolvedChannel{}, false, fmt.Errorf("identity: resolve preferred channel: %w", err)
	}

	selected, ok := pickPreferred(identities)
	if !ok {
		return ResolvedChannel{}, false, nil
	}
	return ResolvedChannel{
		Channel:    selected.Channel().String(),
		ExternalID: selected.ExternalID().String(),
	}, true, nil
}

func pickPreferred(identities []entities.UserIdentity) (entities.UserIdentity, bool) {
	var (
		whatsapp entities.UserIdentity
		telegram entities.UserIdentity
		hasWa    bool
		hasTg    bool
	)
	for _, identity := range identities {
		if !identity.UnlinkedAt().IsZero() {
			continue
		}
		channel := identity.Channel()
		switch {
		case channel.IsWhatsApp():
			if !hasWa || identity.VerifiedAt().After(whatsapp.VerifiedAt()) {
				whatsapp = identity
				hasWa = true
			}
		case channel.IsTelegram():
			if !hasTg || identity.VerifiedAt().After(telegram.VerifiedAt()) {
				telegram = identity
				hasTg = true
			}
		}
	}
	if hasWa {
		return whatsapp, true
	}
	if hasTg {
		return telegram, true
	}
	return entities.UserIdentity{}, false
}

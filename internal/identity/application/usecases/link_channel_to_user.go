package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

const prefixLinkChannelToUser = "identity.usecase.link_channel_to_user:"

type LinkChannelResult struct {
	IdentityID    uuid.UUID
	AlreadyLinked bool
}

type LinkChannelToUser struct {
	uow         uow.UnitOfWork[LinkChannelResult]
	factory     interfaces.RepositoryFactory
	o11y        observability.Observability
	linkedTotal observability.Counter
}

func NewLinkChannelToUser(
	u uow.UnitOfWork[LinkChannelResult],
	factory interfaces.RepositoryFactory,
	o11y observability.Observability,
) *LinkChannelToUser {
	linkedTotal := o11y.Metrics().Counter(
		"identity_link_channel_total",
		"Total de vinculacoes de canal a user por outcome",
		"1",
	)
	return &LinkChannelToUser{
		uow:         u,
		factory:     factory,
		o11y:        o11y,
		linkedTotal: linkedTotal,
	}
}

func (uc *LinkChannelToUser) Execute(ctx context.Context, in input.LinkChannelToUser) (LinkChannelResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "identity.usecase.link_channel_to_user")
	defer span.End()

	if in.UserID == uuid.Nil {
		return LinkChannelResult{}, fmt.Errorf("%s %w", prefixLinkChannelToUser, application.ErrUserNotFound)
	}

	channel, err := valueobjects.NewChannel(in.Channel)
	if err != nil {
		return LinkChannelResult{}, fmt.Errorf("%s parse channel: %w", prefixLinkChannelToUser, err)
	}
	externalID, err := valueobjects.NewExternalID(channel, in.ExternalID)
	if err != nil {
		return LinkChannelResult{}, fmt.Errorf("%s parse external_id: %w", prefixLinkChannelToUser, err)
	}

	identityID, err := uuid.NewV7()
	if err != nil {
		return LinkChannelResult{}, fmt.Errorf("%s generate identity id: %w", prefixLinkChannelToUser, err)
	}
	now := time.Now().UTC()
	identity, err := entities.NewUserIdentity(identityID, in.UserID, channel, externalID, now)
	if err != nil {
		return LinkChannelResult{}, fmt.Errorf("%s build user_identity: %w", prefixLinkChannelToUser, err)
	}

	res, err := uc.linkWithinTransaction(ctx, in.UserID, channel, externalID, identity)
	if err != nil {
		span.RecordError(err)
		uc.linkedTotal.Add(ctx, 1, observability.String("outcome", "error"), observability.String("channel", channel.String()))
		return LinkChannelResult{}, fmt.Errorf("%s %w", prefixLinkChannelToUser, err)
	}
	outcome := "linked"
	if res.AlreadyLinked {
		outcome = "already_linked"
	}
	uc.linkedTotal.Add(ctx, 1, observability.String("outcome", outcome), observability.String("channel", channel.String()))
	return res, nil
}

func (uc *LinkChannelToUser) linkWithinTransaction(
	ctx context.Context,
	userID uuid.UUID,
	channel valueobjects.Channel,
	externalID valueobjects.ExternalID,
	identity entities.UserIdentity,
) (LinkChannelResult, error) {
	return uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (LinkChannelResult, error) {
		repo := uc.factory.UserIdentityRepository(tx)

		existing, found, err := repo.TryFindActive(ctx, channel, externalID)
		if err != nil {
			return LinkChannelResult{}, fmt.Errorf("lookup existing: %w", err)
		}
		if found {
			return uc.resolveExistingLink(existing, userID)
		}
		if err := repo.Insert(ctx, identity); err != nil {
			return uc.resolveInsertConflict(ctx, repo, channel, externalID, userID, err)
		}
		return LinkChannelResult{IdentityID: identity.ID()}, nil
	})
}

func (uc *LinkChannelToUser) resolveExistingLink(existing entities.UserIdentity, userID uuid.UUID) (LinkChannelResult, error) {
	if existing.UserID() == userID {
		return LinkChannelResult{IdentityID: existing.ID(), AlreadyLinked: true}, nil
	}
	return LinkChannelResult{}, fmt.Errorf("%s %w", prefixLinkChannelToUser, application.ErrUserIdentityAlreadyLinked)
}

func (uc *LinkChannelToUser) resolveInsertConflict(
	ctx context.Context,
	repo interfaces.UserIdentityRepository,
	channel valueobjects.Channel,
	externalID valueobjects.ExternalID,
	userID uuid.UUID,
	err error,
) (LinkChannelResult, error) {
	if !errors.Is(err, application.ErrUserIdentityAlreadyLinked) {
		return LinkChannelResult{}, fmt.Errorf("insert: %w", err)
	}
	winner, found, reReadErr := repo.TryFindActive(ctx, channel, externalID)
	if reReadErr != nil {
		return LinkChannelResult{}, fmt.Errorf("post-conflict re-read: %w", reReadErr)
	}
	if !found {
		return LinkChannelResult{}, fmt.Errorf("%s %w", prefixLinkChannelToUser, application.ErrUserIdentityAlreadyLinked)
	}
	return uc.resolveExistingLink(winner, userID)
}

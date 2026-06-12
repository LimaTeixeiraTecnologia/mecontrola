package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type GetCardForUser struct {
	factory interfaces.RepositoryFactory
	mgr     manager.Manager
	o11y    observability.Observability
}

func NewGetCardForUser(
	factory interfaces.RepositoryFactory,
	mgr manager.Manager,
	o11y observability.Observability,
) *GetCardForUser {
	return &GetCardForUser{factory: factory, mgr: mgr, o11y: o11y}
}

func (u *GetCardForUser) Execute(ctx context.Context, cardID, userID uuid.UUID) (valueobjects.BillingCycle, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.get_card_for_user",
		observability.WithAttributes(
			observability.String("card_id", cardID.String()),
			observability.String("user_id", userID.String()),
		),
	)
	defer span.End()

	repo := u.factory.CardRepository(u.mgr.DBTX(ctx))
	card, err := repo.GetByIDForUser(ctx, cardID.String(), userID.String())
	if err != nil {
		outcome := classifyCardOutcome(err)
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", outcome))
		return valueobjects.BillingCycle{}, fmt.Errorf("card/get_card_for_user: %w", err)
	}

	if card.IsDeleted() {
		span.SetAttributes(observability.String("outcome", "not_found"))
		return valueobjects.BillingCycle{}, fmt.Errorf("card/get_card_for_user: %w", domain.ErrCardNotFound)
	}

	span.SetAttributes(observability.String("outcome", "success"))
	u.o11y.Logger().Info(ctx, "card.get_card_for_user.success",
		observability.String("card_id", cardID.String()),
		observability.String("user_id", userID.String()),
	)

	return card.Cycle, nil
}

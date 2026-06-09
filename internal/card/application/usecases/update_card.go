package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
)

type UpdateCard struct {
	uow     uow.UnitOfWork[entities.Card]
	factory interfaces.RepositoryFactory
	idem    idempotency.Storage
	o11y    observability.Observability
}

func NewUpdateCard(
	u uow.UnitOfWork[entities.Card],
	factory interfaces.RepositoryFactory,
	idem idempotency.Storage,
	o11y observability.Observability,
) *UpdateCard {
	return &UpdateCard{uow: u, factory: factory, idem: idem, o11y: o11y}
}

func (u *UpdateCard) Execute(ctx context.Context, in input.UpdateCard) (output.Card, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.update",
		observability.WithAttributes(
			observability.String("card_id", in.ID.String()),
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	name, err := valueobjects.NewCardName(in.Name)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return output.Card{}, err
	}

	nickname, err := valueobjects.NewNickname(in.Nickname)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return output.Card{}, err
	}

	cycle, err := valueobjects.NewBillingCycle(in.ClosingDay, in.DueDay)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return output.Card{}, err
	}

	ic, hasIdem := idempotency.FromContext(ctx)

	card, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Card, error) {
		repo := u.factory.CardRepository(tx)

		existing, getErr := repo.GetByIDForUser(ctx, in.ID.String(), in.UserID.String())
		if getErr != nil {
			return entities.Card{}, getErr
		}
		if existing.IsDeleted() {
			return entities.Card{}, domain.ErrCardNotFound
		}

		updated := entities.HydrateCard(
			existing.ID,
			existing.UserID,
			name,
			nickname,
			cycle,
			existing.CreatedAt,
			existing.UpdatedAt,
			existing.DeletedAt,
		)

		persisted, updateErr := repo.UpdateByIDForUser(ctx, updated)
		if updateErr != nil {
			return entities.Card{}, updateErr
		}

		if hasIdem {
			body, marshalErr := json.Marshal(toCardOutput(persisted))
			if marshalErr != nil {
				return entities.Card{}, fmt.Errorf("update_card: marshal output: %w", marshalErr)
			}
			rec := idempotency.Record{
				Scope:          ic.Scope,
				Key:            ic.Key,
				UserID:         ic.UserID,
				RequestHash:    ic.RequestHash,
				ResponseStatus: http.StatusOK,
				ResponseBody:   body,
				ExpiresAt:      ic.ExpiresAt,
			}
			if putErr := u.idem.Put(ctx, rec); putErr != nil {
				return entities.Card{}, fmt.Errorf("update_card: idempotency put: %w", putErr)
			}
		}

		return persisted, nil
	})

	if err != nil {
		outcome := classifyCardOutcome(err)
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", outcome))
		u.o11y.Logger().Error(ctx, "card.update.failed",
			observability.String("card_id", in.ID.String()),
			observability.String("user_id", in.UserID.String()),
			observability.Error(err),
		)
		return output.Card{}, err
	}

	span.SetAttributes(observability.String("outcome", "success"))
	u.o11y.Logger().Info(ctx, "card.update.completed",
		observability.String("card_id", card.ID.String()),
		observability.String("user_id", card.UserID.String()),
	)
	return toCardOutput(card), nil
}

package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/mappers"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
)

type UpdateCard struct {
	uow     uow.UnitOfWork
	factory interfaces.RepositoryFactory
	idem    idempotency.Storage
	decider services.UpdateCardDecider
	o11y    observability.Observability
}

func NewUpdateCard(
	u uow.UnitOfWork,
	factory interfaces.RepositoryFactory,
	idem idempotency.Storage,
	o11y observability.Observability,
) *UpdateCard {
	return &UpdateCard{
		uow:     u,
		factory: factory,
		idem:    idem,
		decider: services.NewUpdateCardDecider(),
		o11y:    o11y,
	}
}

func (u *UpdateCard) Execute(ctx context.Context, in input.UpdateCard) (output.Card, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.update",
		observability.WithAttributes(
			observability.String("card_id", in.ID.String()),
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.Card{}, err
	}

	ic, hasIdem := idempotency.FromContext(ctx)

	var card entities.Card
	err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		repo := u.factory.CardRepository(tx)

		existing, getErr := repo.GetByIDForUser(ctx, in.ID.String(), in.UserID.String())
		if getErr != nil {
			return getErr
		}
		if existing.IsDeleted() {
			return fmt.Errorf("card/update: %w", domain.ErrCardNotFound)
		}

		decided, decErr := u.decider.Decide(existing, services.UpdateCardCommand{
			Name:       in.Name,
			Nickname:   in.Nickname,
			ClosingDay: in.ClosingDay,
			DueDay:     in.DueDay,
		}, time.Now().UTC())
		if decErr != nil {
			span.SetAttributes(observability.String("outcome", "invalid"))
			return decErr
		}

		persisted, updateErr := repo.UpdateByIDForUser(ctx, decided)
		if updateErr != nil {
			return updateErr
		}

		if hasIdem {
			body, marshalErr := json.Marshal(mappers.M.ToCardOutput(persisted))
			if marshalErr != nil {
				return fmt.Errorf("update_card: marshal output: %w", marshalErr)
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
				return fmt.Errorf("update_card: idempotency put: %w", putErr)
			}
		}

		card = persisted
		return nil
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
	return mappers.M.ToCardOutput(card), nil
}

package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/mappers"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
)

type UpdateCardLimit struct {
	uow     uow.UnitOfWork[entities.Card]
	factory interfaces.RepositoryFactory
	idem    idempotency.Storage
	o11y    observability.Observability
	updated observability.Counter
}

func NewUpdateCardLimit(
	u uow.UnitOfWork[entities.Card],
	factory interfaces.RepositoryFactory,
	idem idempotency.Storage,
	o11y observability.Observability,
) *UpdateCardLimit {
	counter := o11y.Metrics().Counter(
		"card_limit_updated_total",
		"Total de atualizações de limite de cartão por resultado",
		"1",
	)
	return &UpdateCardLimit{
		uow:     u,
		factory: factory,
		idem:    idem,
		o11y:    o11y,
		updated: counter,
	}
}

func (u *UpdateCardLimit) Execute(ctx context.Context, in input.UpdateCardLimit) (output.Card, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.update_limit",
		observability.WithAttributes(
			observability.String("card_id", in.CardID.String()),
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	limit, err := valueobjects.NewCardLimit(in.LimitCents)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		u.updated.Add(ctx, 1, observability.String("result", "invalid"))
		return output.Card{}, err
	}

	ic, hasIdem := idempotency.FromContext(ctx)

	card, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Card, error) {
		repo := u.factory.CardRepository(tx)

		existing, getErr := repo.GetByIDForUser(ctx, in.CardID.String(), in.UserID.String())
		if getErr != nil {
			return entities.Card{}, getErr
		}
		if existing.IsDeleted() {
			return entities.Card{}, fmt.Errorf("card/update_limit: %w", domain.ErrCardNotFound)
		}

		if in.ExpectedVersion != nil && *in.ExpectedVersion != existing.Version {
			return entities.Card{}, fmt.Errorf("card/update_limit: %w", domain.ErrCardLimitConflict)
		}

		updated := existing.UpdateLimit(limit, time.Now().UTC())

		persisted, updateErr := repo.UpdateLimitByIDForUser(ctx, updated, existing.Version)
		if updateErr != nil {
			return entities.Card{}, updateErr
		}

		if hasIdem {
			body, marshalErr := json.Marshal(mappers.M.ToCardOutput(persisted))
			if marshalErr != nil {
				return entities.Card{}, fmt.Errorf("update_card_limit: marshal output: %w", marshalErr)
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
				return entities.Card{}, fmt.Errorf("update_card_limit: idempotency put: %w", putErr)
			}
		}

		return persisted, nil
	})

	if err != nil {
		outcome := classifyCardOutcome(err)
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", outcome))
		u.updated.Add(ctx, 1, observability.String("result", outcome))
		u.o11y.Logger().Error(ctx, "card.update_limit.failed",
			observability.String("card_id", in.CardID.String()),
			observability.String("user_id", in.UserID.String()),
			observability.Error(err),
		)
		return output.Card{}, err
	}

	span.SetAttributes(observability.String("outcome", "success"))
	u.updated.Add(ctx, 1, observability.String("result", "success"))
	u.o11y.Logger().Info(ctx, "card.update_limit.completed",
		observability.String("card_id", card.ID.String()),
		observability.String("user_id", card.UserID.String()),
	)
	return mappers.M.ToCardOutput(card), nil
}

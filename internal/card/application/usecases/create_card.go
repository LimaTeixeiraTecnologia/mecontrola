package usecases

import (
	"context"
	"encoding/json"
	"errors"
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

type CreateCard struct {
	uow     uow.UnitOfWork[entities.Card]
	factory interfaces.RepositoryFactory
	idem    idempotency.Storage
	o11y    observability.Observability
}

func NewCreateCard(
	u uow.UnitOfWork[entities.Card],
	factory interfaces.RepositoryFactory,
	idem idempotency.Storage,
	o11y observability.Observability,
) *CreateCard {
	return &CreateCard{uow: u, factory: factory, idem: idem, o11y: o11y}
}

func (u *CreateCard) Execute(ctx context.Context, in input.CreateCard) (output.Card, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.create",
		observability.WithAttributes(
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	u.o11y.Logger().Info(ctx, "card.create.started",
		observability.String("user_id", in.UserID.String()),
	)

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
		c := entities.NewCard(entities.NewCardInput{
			UserID:   in.UserID,
			Name:     name,
			Nickname: nickname,
			Cycle:    cycle,
		})
		if insertErr := repo.Insert(ctx, c); insertErr != nil {
			return entities.Card{}, insertErr
		}
		if hasIdem {
			body, marshalErr := json.Marshal(toCardOutput(c))
			if marshalErr != nil {
				return entities.Card{}, fmt.Errorf("create_card: marshal output: %w", marshalErr)
			}
			rec := idempotency.Record{
				Scope:          ic.Scope,
				Key:            ic.Key,
				UserID:         ic.UserID,
				RequestHash:    ic.RequestHash,
				ResponseStatus: http.StatusCreated,
				ResponseBody:   body,
				ExpiresAt:      ic.ExpiresAt,
			}
			if putErr := u.idem.Put(ctx, rec); putErr != nil {
				return entities.Card{}, fmt.Errorf("create_card: idempotency put: %w", putErr)
			}
		}
		return c, nil
	})

	if err != nil {
		outcome := classifyCardOutcome(err)
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", outcome))
		u.o11y.Logger().Error(ctx, "card.create.failed",
			observability.String("user_id", in.UserID.String()),
			observability.Error(err),
		)
		return output.Card{}, err
	}

	span.SetAttributes(
		observability.String("card_id", card.ID.String()),
		observability.String("outcome", "success"),
	)
	u.o11y.Logger().Info(ctx, "card.create.completed",
		observability.String("card_id", card.ID.String()),
		observability.String("user_id", card.UserID.String()),
	)
	return toCardOutput(card), nil
}

func toCardOutput(c entities.Card) output.Card {
	return output.Card{
		ID:         c.ID.String(),
		UserID:     c.UserID.String(),
		Name:       c.Name.String(),
		Nickname:   c.Nickname.String(),
		ClosingDay: c.Cycle.ClosingDay,
		DueDay:     c.Cycle.DueDay,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
		DeletedAt:  c.DeletedAt,
	}
}

func classifyCardOutcome(err error) string {
	switch {
	case errors.Is(err, domain.ErrCardNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrNicknameConflict):
		return "conflict"
	case isCardValidationError(err):
		return "invalid"
	default:
		return "internal_error"
	}
}

func isCardValidationError(err error) bool {
	return errors.Is(err, domain.ErrInvalidCardName) ||
		errors.Is(err, domain.ErrInvalidNickname) ||
		errors.Is(err, domain.ErrInvalidClosingDay) ||
		errors.Is(err, domain.ErrInvalidDueDay) ||
		errors.Is(err, domain.ErrInvalidPurchaseDate) ||
		errors.Is(err, domain.ErrInvalidCursor)
}

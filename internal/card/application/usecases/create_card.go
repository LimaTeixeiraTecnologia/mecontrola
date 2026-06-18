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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
)

type CreateCard struct {
	uow     uow.UnitOfWork
	factory interfaces.RepositoryFactory
	idem    idempotency.Storage
	decider services.CreateCardDecider
	o11y    observability.Observability
}

func NewCreateCard(
	u uow.UnitOfWork,
	factory interfaces.RepositoryFactory,
	idem idempotency.Storage,
	o11y observability.Observability,
) *CreateCard {
	return &CreateCard{
		uow:     u,
		factory: factory,
		idem:    idem,
		decider: services.NewCreateCardDecider(),
		o11y:    o11y,
	}
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

	limit, err := valueobjects.NewCardLimit(in.LimitCents)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return output.Card{}, err
	}

	ic, hasIdem := idempotency.FromContext(ctx)

	cardID := entities.NewCardID()
	now := time.Now().UTC()
	cmd := services.CreateCardCommand{
		UserID:     in.UserID,
		Name:       name,
		Nickname:   nickname,
		Cycle:      cycle,
		LimitCents: limit.Cents(),
	}

	var card entities.Card
	err = u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		repo := u.factory.CardRepository(tx)
		c := u.decider.Decide(cmd, cardID, now)
		if insertErr := repo.Insert(ctx, c); insertErr != nil {
			return insertErr
		}
		if hasIdem {
			body, marshalErr := json.Marshal(mappers.M.ToCardOutput(c))
			if marshalErr != nil {
				return fmt.Errorf("create_card: marshal output: %w", marshalErr)
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
				return fmt.Errorf("create_card: idempotency put: %w", putErr)
			}
		}
		card = c
		return nil
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
	return mappers.M.ToCardOutput(card), nil
}

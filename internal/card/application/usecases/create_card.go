package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/mappers"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
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
	svc     services.PurchaseDayService
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
		svc:     services.PurchaseDayService{},
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

	if err := in.Validate(); err != nil {
		return output.Card{}, err
	}

	nickname, err := valueobjects.NewNickname(in.Nickname)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return output.Card{}, err
	}

	bank, err := valueobjects.NewBankCode(in.Bank)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return output.Card{}, err
	}

	tz, err := services.NewSaoPauloLocation()
	if err != nil {
		return output.Card{}, fmt.Errorf("card/create: timezone: %w", err)
	}

	now := time.Now().UTC()
	cardID := entities.NewCardID()
	ic, hasIdem := idempotency.FromContext(ctx)

	u.o11y.Logger().Info(ctx, "card.create.started",
		observability.String("user_id", in.UserID.String()),
	)

	var card entities.Card
	err = u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		bankReader := u.factory.BankDaysReader(tx)
		repo := u.factory.CardRepository(tx)

		days, readErr := bankReader.DaysBeforeDue(ctx, bank)
		if readErr != nil {
			return fmt.Errorf("card/create: bank_days: %w", readErr)
		}

		pd := u.svc.Decide(in.DueDay, days, now, tz)

		cycle, cycleErr := valueobjects.NewBillingCycle(pd.ClosingDay, in.DueDay)
		if cycleErr != nil {
			return fmt.Errorf("card/create: cycle: %w", cycleErr)
		}

		cmd := services.CreateCardCommand{
			UserID:   in.UserID,
			Nickname: nickname,
			Bank:     bank,
			Cycle:    cycle,
		}

		c := u.decider.Decide(cmd, cardID, now)
		if insertErr := repo.Insert(ctx, c); insertErr != nil {
			if !errors.Is(insertErr, domain.ErrNicknameConflict) {
				return insertErr
			}
			existing, findErr := u.findExistingByNickname(ctx, repo, in.UserID.String(), nickname)
			if findErr != nil {
				return findErr
			}
			card = existing
			return nil
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

func (u *CreateCard) findExistingByNickname(ctx context.Context, repo interfaces.CardRepository, userID string, nickname valueobjects.Nickname) (entities.Card, error) {
	cards, _, err := repo.ListByUser(ctx, userID, "", resolveCardByNicknameLimit)
	if err != nil {
		return entities.Card{}, fmt.Errorf("card/create: lookup existing: %w", err)
	}

	target := strings.TrimSpace(nickname.String())
	for _, c := range cards {
		if c.IsDeleted() {
			continue
		}
		if strings.EqualFold(c.Nickname.String(), target) {
			return c, nil
		}
	}

	return entities.Card{}, fmt.Errorf("card/create: %w", domain.ErrNicknameConflict)
}

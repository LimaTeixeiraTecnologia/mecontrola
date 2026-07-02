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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
)

type UpdateCard struct {
	uow     uow.UnitOfWork
	factory interfaces.RepositoryFactory
	idem    idempotency.Storage
	decider services.UpdateCardDecider
	svc     services.PurchaseDayService
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
		svc:     services.PurchaseDayService{},
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

	tz, err := services.NewSaoPauloLocation()
	if err != nil {
		return output.Card{}, fmt.Errorf("card/update: timezone: %w", err)
	}

	now := time.Now().UTC()

	var card entities.Card
	err = u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		persisted, applyErr := u.applyUpdate(ctx, tx, in, now, tz, span)
		if applyErr != nil {
			return applyErr
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

func (u *UpdateCard) applyUpdate(ctx context.Context, tx database.DBTX, in input.UpdateCard, now time.Time, tz *time.Location, span observability.Span) (entities.Card, error) {
	repo := u.factory.CardRepository(tx)

	existing, getErr := repo.GetByIDForUser(ctx, in.ID.String(), in.UserID.String())
	if getErr != nil {
		return entities.Card{}, getErr
	}
	if existing.IsDeleted() {
		return entities.Card{}, fmt.Errorf("card/update: %w", domain.ErrCardNotFound)
	}

	newCycle, newBank, resolveErr := u.resolveUpdate(ctx, tx, existing, in, now, tz)
	if resolveErr != nil {
		return entities.Card{}, resolveErr
	}

	decided, decErr := u.decider.Decide(existing, services.UpdateCardCommand{
		Nickname: in.Nickname,
		Bank:     newBank,
		Cycle:    newCycle,
	}, now)
	if decErr != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return entities.Card{}, decErr
	}

	persisted, updateErr := repo.UpdateByIDForUser(ctx, decided)
	if updateErr != nil {
		return entities.Card{}, updateErr
	}

	if err := u.persistIdempotency(ctx, persisted); err != nil {
		return entities.Card{}, err
	}

	return persisted, nil
}

func (u *UpdateCard) resolveUpdate(ctx context.Context, tx database.DBTX, existing entities.Card, in input.UpdateCard, now time.Time, tz *time.Location) (*valueobjects.BillingCycle, *valueobjects.BankCode, error) {
	var newBank *valueobjects.BankCode
	if in.Bank != nil {
		bc, bankErr := valueobjects.NewBankCode(*in.Bank)
		if bankErr != nil {
			return nil, nil, fmt.Errorf("card/update: bank: %w", bankErr)
		}
		newBank = &bc
	}

	if in.Bank == nil && in.DueDay == nil {
		return nil, newBank, nil
	}

	bankCode := existing.Bank
	if newBank != nil {
		bankCode = *newBank
	}

	dueDay := existing.Cycle.DueDay
	if in.DueDay != nil {
		dueDay = *in.DueDay
	}

	days, readErr := u.factory.BankDaysReader(tx).DaysBeforeDue(ctx, bankCode)
	if readErr != nil {
		return nil, nil, fmt.Errorf("card/update: bank_days: %w", readErr)
	}

	pd := u.svc.Decide(dueDay, days, now, tz)

	cycle, cycleErr := valueobjects.NewBillingCycle(pd.ClosingDay, dueDay)
	if cycleErr != nil {
		return nil, nil, fmt.Errorf("card/update: cycle: %w", cycleErr)
	}

	return &cycle, newBank, nil
}

func (u *UpdateCard) persistIdempotency(ctx context.Context, persisted entities.Card) error {
	ic, hasIdem := idempotency.FromContext(ctx)
	if !hasIdem {
		return nil
	}

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

	return nil
}

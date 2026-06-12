package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	cardvos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrCardLookupFailed = errors.New("transactions: card lookup falhou")

type cardForUserExecutor interface {
	Execute(ctx context.Context, cardID, userID uuid.UUID) (cardvos.BillingCycle, error)
}

type cardLookupAdapter struct {
	executor cardForUserExecutor
	o11y     observability.Observability
}

func NewCardLookupAdapter(
	executor cardForUserExecutor,
	o11y observability.Observability,
) interfaces.CardLookup {
	return &cardLookupAdapter{
		executor: executor,
		o11y:     o11y,
	}
}

func (a *cardLookupAdapter) GetForUser(ctx context.Context, cardID, userID uuid.UUID) (valueobjects.CardBillingSnapshot, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "transactions.card_lookup.get_for_user",
		observability.WithAttributes(
			observability.String("card_id", cardID.String()),
			observability.String("user_id", userID.String()),
		),
	)
	defer span.End()

	cycle, err := a.executor.Execute(ctx, cardID, userID)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, carddomain.ErrCardNotFound) {
			return valueobjects.CardBillingSnapshot{}, fmt.Errorf("transactions/card_lookup: %w", interfaces.ErrCardNotFound)
		}
		return valueobjects.CardBillingSnapshot{}, fmt.Errorf("transactions/card_lookup: %w", ErrCardLookupFailed)
	}

	snapshot, err := valueobjects.NewCardBillingSnapshot(cycle.ClosingDay, cycle.DueDay)
	if err != nil {
		span.RecordError(err)
		return valueobjects.CardBillingSnapshot{}, fmt.Errorf("transactions/card_lookup: converter snapshot: %w", ErrCardLookupFailed)
	}

	span.SetAttributes(observability.String("outcome", "success"))
	return snapshot, nil
}

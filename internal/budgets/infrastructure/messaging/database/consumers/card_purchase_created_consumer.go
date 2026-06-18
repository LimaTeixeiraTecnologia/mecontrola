package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const cardPurchaseExpenseSource = "transactions_card"

type cardPurchaseInstallmentPayload struct {
	ItemID      string `json:"item_id"`
	RefMonth    string `json:"ref_month"`
	AmountCents int64  `json:"amount_cents"`
	Index       int    `json:"index"`
}

type cardPurchaseCreatedPayload struct {
	AggregateID   string                           `json:"aggregate_id"`
	UserID        string                           `json:"user_id"`
	OccurredAt    time.Time                        `json:"occurred_at"`
	SubcategoryID string                           `json:"subcategory_id"`
	Installments  []cardPurchaseInstallmentPayload `json:"installments"`
}

type CardPurchaseCreatedConsumer struct {
	upsert      upsertExpenseUseCase
	o11y        observability.Observability
	decodeFails observability.Counter
	skipped     observability.Counter
}

func NewCardPurchaseCreatedConsumer(
	upsert upsertExpenseUseCase,
	o11y observability.Observability,
) *CardPurchaseCreatedConsumer {
	decodeFails := o11y.Metrics().Counter(
		"budgets_card_purchase_created_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de compras de cartão criadas",
		"1",
	)
	skipped := o11y.Metrics().Counter(
		"budgets_card_purchase_created_consumer_skipped_total",
		"Total de compras de cartão ignoradas pelo consumer de orçamento",
		"1",
	)
	return &CardPurchaseCreatedConsumer{
		upsert:      upsert,
		o11y:        o11y,
		decodeFails: decodeFails,
		skipped:     skipped,
	}
}

func (c *CardPurchaseCreatedConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "budgets.consumer.card_purchase_created.handle")
	defer span.End()

	rawPayload := event.GetPayload()
	env, ok := rawPayload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("budgets.consumer.card_purchase_created: unexpected payload type %T", rawPayload)
	}

	var p cardPurchaseCreatedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.card_purchase_created: deserializar payload: %w", err)
	}

	if p.SubcategoryID == "" || p.SubcategoryID == uuid.Nil.String() {
		c.skipped.Add(ctx, 1, observability.String("reason", "missing_subcategory"))
		c.o11y.Logger().Warn(ctx, "budgets.consumer.card_purchase_created.skipped_missing_subcategory",
			observability.String("aggregate_id", p.AggregateID),
		)
		return nil
	}

	var errs []error
	for _, inst := range p.Installments {
		_, err := c.upsert.Execute(ctx, input.UpsertExpenseInput{
			UserID:                p.UserID,
			Source:                cardPurchaseExpenseSource,
			ExternalTransactionID: inst.ItemID,
			SubcategoryID:         p.SubcategoryID,
			Competence:            inst.RefMonth,
			AmountCents:           inst.AmountCents,
			OccurredAt:            p.OccurredAt,
		})
		if err != nil {
			if errors.Is(err, appinterfaces.ErrExpenseTombstoneConflict) {
				c.skipped.Add(ctx, 1, observability.String("reason", "tombstone"))
				continue
			}
			errs = append(errs, fmt.Errorf("budgets.consumer.card_purchase_created: upsert expense %s: %w", inst.ItemID, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

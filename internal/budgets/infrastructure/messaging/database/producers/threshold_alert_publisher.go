package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeThresholdAlertTriggered = "budgets.threshold_alert_triggered.v1"

const aggregateTypeBudgetAlert = "budgets.threshold_alert"

type thresholdAlertPayload struct {
	UserID               string `json:"user_id"`
	BudgetID             string `json:"budget_id"`
	Kind                 string `json:"kind"`
	CategoryID           string `json:"category_id,omitempty"`
	CardID               string `json:"card_id,omitempty"`
	RootSlug             string `json:"root_slug,omitempty"`
	PercentUsedBps       int32  `json:"percent_used_bps"`
	AmountRemainingCents int64  `json:"amount_remaining_cents"`
	RefDay               string `json:"ref_day"`
}

type ThresholdAlertPublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	idGen         id.Generator
	o11y          observability.Observability
}

func NewThresholdAlertPublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	idGen id.Generator,
	o11y observability.Observability,
) *ThresholdAlertPublisher {
	return &ThresholdAlertPublisher{
		outboxFactory: outboxFactory,
		cfg:           cfg,
		idGen:         idGen,
		o11y:          o11y,
	}
}

func (p *ThresholdAlertPublisher) Publish(ctx context.Context, db database.DBTX, alert services.DomainAlert, occurredAt time.Time) error {
	payload := thresholdAlertPayload{
		UserID:               alert.UserID.String(),
		BudgetID:             alert.BudgetID.String(),
		Kind:                 alert.Kind.String(),
		RootSlug:             alert.RootSlug.String(),
		PercentUsedBps:       alert.PercentUsedBps,
		AmountRemainingCents: alert.AmountRemainingCents,
		RefDay:               alert.RefDay.UTC().Format("2006-01-02"),
	}
	if alert.CategoryID.String() != "00000000-0000-0000-0000-000000000000" {
		payload.CategoryID = alert.CategoryID.String()
	}
	if alert.CardID.String() != "00000000-0000-0000-0000-000000000000" {
		payload.CardID = alert.CardID.String()
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("budgets/producer: marshal threshold_alert payload: %w", err)
	}

	outboxEvt, err := outbox.NewEvent(outbox.EventInput{
		ID:              p.idGen.NewID(),
		Type:            eventTypeThresholdAlertTriggered,
		AggregateType:   aggregateTypeBudgetAlert,
		AggregateID:     alert.BudgetID.String(),
		AggregateUserID: alert.UserID.String(),
		Payload:         raw,
		OccurredAt:      occurredAt,
	})
	if err != nil {
		return fmt.Errorf("budgets/producer: new threshold_alert event: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	if err := publisher.Publish(ctx, outboxEvt); err != nil {
		return fmt.Errorf("budgets/producer: publish threshold_alert: %w", err)
	}
	return nil
}

package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeThresholdAlertTriggered = "budgets.threshold_alert_triggered.v1"

type notifyThresholdAlertUseCase interface {
	Execute(ctx context.Context, in usecases.NotifyThresholdAlertInput) (usecases.NotifyThresholdAlertResult, error)
}

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

type ThresholdAlertNotifier struct {
	notifyAlert notifyThresholdAlertUseCase
	o11y        observability.Observability
	decodeFails observability.Counter
}

func NewThresholdAlertNotifier(notifyAlert notifyThresholdAlertUseCase, o11y observability.Observability) *ThresholdAlertNotifier {
	decodeFails := o11y.Metrics().Counter(
		"budgets_threshold_alert_notifier_decode_failed_total",
		"Total de falhas de decode do consumer threshold_alert_notifier",
		"1",
	)
	return &ThresholdAlertNotifier{
		notifyAlert: notifyAlert,
		o11y:        o11y,
		decodeFails: decodeFails,
	}
}

func (c *ThresholdAlertNotifier) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "budgets.consumer.threshold_alert_notifier.handle")
	defer span.End()

	if event.GetEventType() != eventTypeThresholdAlertTriggered {
		return fmt.Errorf("budgets.consumer.threshold_alert_notifier: unhandled event type %q", event.GetEventType())
	}

	payload := event.GetPayload()
	env, ok := payload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("budgets.consumer.threshold_alert_notifier: unexpected payload type %T", payload)
	}

	var p thresholdAlertPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.threshold_alert_notifier: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.threshold_alert_notifier: user_id invalido: %w", err)
	}
	budgetID, err := uuid.Parse(p.BudgetID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.threshold_alert_notifier: budget_id invalido: %w", err)
	}
	kind, err := parseAlertKindLabel(p.Kind)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.threshold_alert_notifier: kind invalido: %w", err)
	}
	refDay, err := time.Parse("2006-01-02", p.RefDay)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.threshold_alert_notifier: ref_day invalido: %w", err)
	}

	in := usecases.NotifyThresholdAlertInput{
		UserID:               userID,
		BudgetID:             budgetID,
		Kind:                 kind,
		RootSlug:             p.RootSlug,
		PercentUsedBps:       p.PercentUsedBps,
		AmountRemainingCents: p.AmountRemainingCents,
		RefDay:               refDay.UTC(),
	}

	if _, err := c.notifyAlert.Execute(ctx, in); err != nil {
		return fmt.Errorf("budgets.consumer.threshold_alert_notifier: notificar: %w", err)
	}
	return nil
}

func parseAlertKindLabel(s string) (services.ThresholdAlertKind, error) {
	switch s {
	case "category_threshold":
		return services.ThresholdAlertCategory, nil
	case "goal_achieved":
		return services.ThresholdAlertGoal, nil
	case "card_limit_near":
		return services.ThresholdAlertCardLimit, nil
	default:
		return 0, fmt.Errorf("budgets.consumer.threshold_alert_notifier: kind desconhecido %q", s)
	}
}

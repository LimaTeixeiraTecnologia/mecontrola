package output

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
)

type StateChangedEvent struct {
	SubscriptionID   string
	UserID           string
	WhatsAppNumber   string
	PlanCode         string
	PreviousState    string
	NewState         string
	TransitionReason string
	PeriodEnd        time.Time
	GracePeriodEnd   *time.Time
	OccurredAtValue  time.Time
}

func (e StateChangedEvent) Name() events.EventName {
	name, _ := events.NewEventName("billing.subscription.state_changed")
	return name
}

func (e StateChangedEvent) OccurredAt() time.Time {
	return e.OccurredAtValue
}

func (e StateChangedEvent) AggregateID() string {
	return e.SubscriptionID
}

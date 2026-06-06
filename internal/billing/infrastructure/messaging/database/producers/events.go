package producers

import "time"

const (
	EventTypeSubscriptionActivated = "billing.subscription.activated"
	EventTypeSubscriptionRenewed   = "billing.subscription.renewed"
	EventTypeSubscriptionPastDue   = "billing.subscription.past_due"
	EventTypeSubscriptionCanceled  = "billing.subscription.canceled"
	EventTypeSubscriptionRefunded  = "billing.subscription.refunded"
	EventTypeSubscriptionExpired   = "billing.subscription.expired_after_grace"
)

type SubscriptionActivatedPayload struct {
	SubscriptionID string    `json:"subscription_id"`
	FunnelToken    string    `json:"funnel_token"`
	PlanCode       string    `json:"plan_code"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	OccurredAt     time.Time `json:"occurred_at"`
}

func (e SubscriptionActivatedPayload) GetEventType() string { return EventTypeSubscriptionActivated }
func (e SubscriptionActivatedPayload) GetPayload() any      { return e }

type SubscriptionRenewedPayload struct {
	SubscriptionID    string    `json:"subscription_id"`
	PlanCode          string    `json:"plan_code"`
	PreviousPeriodEnd time.Time `json:"previous_period_end"`
	PeriodEnd         time.Time `json:"period_end"`
	OccurredAt        time.Time `json:"occurred_at"`
}

func (e SubscriptionRenewedPayload) GetEventType() string { return EventTypeSubscriptionRenewed }
func (e SubscriptionRenewedPayload) GetPayload() any      { return e }

type SubscriptionPastDuePayload struct {
	SubscriptionID string    `json:"subscription_id"`
	PeriodEnd      time.Time `json:"period_end"`
	GraceEnd       time.Time `json:"grace_end"`
	OccurredAt     time.Time `json:"occurred_at"`
}

func (e SubscriptionPastDuePayload) GetEventType() string { return EventTypeSubscriptionPastDue }
func (e SubscriptionPastDuePayload) GetPayload() any      { return e }

type SubscriptionCanceledPayload struct {
	SubscriptionID string    `json:"subscription_id"`
	PeriodEnd      time.Time `json:"period_end"`
	OccurredAt     time.Time `json:"occurred_at"`
}

func (e SubscriptionCanceledPayload) GetEventType() string { return EventTypeSubscriptionCanceled }
func (e SubscriptionCanceledPayload) GetPayload() any      { return e }

type SubscriptionRefundedPayload struct {
	SubscriptionID string    `json:"subscription_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

func (e SubscriptionRefundedPayload) GetEventType() string { return EventTypeSubscriptionRefunded }
func (e SubscriptionRefundedPayload) GetPayload() any      { return e }

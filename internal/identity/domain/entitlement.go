package domain

import "time"

type SubscriptionStatus string

const (
	SubscriptionTrialing        SubscriptionStatus = "TRIALING"
	SubscriptionActive          SubscriptionStatus = "ACTIVE"
	SubscriptionPastDue         SubscriptionStatus = "PAST_DUE"
	SubscriptionCanceledPending SubscriptionStatus = "CANCELED_PENDING"
	SubscriptionExpired         SubscriptionStatus = "EXPIRED"
	SubscriptionRefunded        SubscriptionStatus = "REFUNDED"
)

type Subscription interface {
	Status() SubscriptionStatus
	PeriodEnd() time.Time
	GracePeriodEnd() time.Time
}

type Reason string

const (
	ReasonNoSubscription  Reason = "no_subscription"
	ReasonActive          Reason = "active"
	ReasonTrialing        Reason = "trialing"
	ReasonCanceledPending Reason = "canceled_pending"
	ReasonPastDueGrace    Reason = "past_due_grace"
	ReasonExpired         Reason = "expired"
	ReasonRefunded        Reason = "refunded"
	ReasonPastDueNoGrace  Reason = "past_due_no_grace"
)

func IsEntitled(sub Subscription, now time.Time) (bool, Reason) {
	if sub == nil {
		return false, ReasonNoSubscription
	}
	switch sub.Status() {
	case SubscriptionActive:
		if sub.PeriodEnd().After(now) {
			return true, ReasonActive
		}
		return false, ReasonExpired
	case SubscriptionTrialing:
		if sub.PeriodEnd().After(now) {
			return true, ReasonTrialing
		}
		return false, ReasonExpired
	case SubscriptionPastDue:
		grace := sub.GracePeriodEnd()
		if !grace.IsZero() && grace.After(now) {
			return true, ReasonPastDueGrace
		}
		return false, ReasonPastDueNoGrace
	case SubscriptionCanceledPending:
		if sub.PeriodEnd().After(now) {
			return true, ReasonCanceledPending
		}
		return false, ReasonExpired
	case SubscriptionExpired:
		return false, ReasonExpired
	case SubscriptionRefunded:
		return false, ReasonRefunded
	default:
		return false, ReasonExpired
	}
}

package services

import "time"

type EntitlementChecker struct{}

func NewEntitlementChecker() EntitlementChecker { return EntitlementChecker{} }

func (EntitlementChecker) IsEntitled(subscription Subscription, now time.Time) bool {
	if subscription == nil {
		return false
	}
	switch subscription.Status() {
	case StatusTrialing, StatusActive:
		return now.Before(subscription.CurrentPeriodEnd())
	case StatusPastDue, StatusCanceledPending:
		return now.Before(subscription.GracePeriodEnd())
	default:
		return false
	}
}

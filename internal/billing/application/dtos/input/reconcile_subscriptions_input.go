package input

import "time"

type ReconcileSubscriptionsInput struct {
	WindowStart time.Time
	WindowEnd   time.Time
}

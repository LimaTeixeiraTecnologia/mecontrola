package output

import "time"

// EntitlementDecision é o resultado da verificação de entitlement para um usuário.
type EntitlementDecision struct {
	Status         string
	Reason         string
	SubscriptionID string
	ExpiresAt      time.Time
}

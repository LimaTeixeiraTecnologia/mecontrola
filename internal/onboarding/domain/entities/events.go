package entities

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type SubscriptionBound struct {
	EventID         string                      `json:"event_id"`
	TokenID         string                      `json:"-"`
	UserID          string                      `json:"user_id"`
	SubscriptionID  string                      `json:"subscription_id"`
	TokenHashPrefix string                      `json:"token_hash_prefix"`
	ActivationPath  valueobjects.ActivationPath `json:"-"`
	PeerE164        string                      `json:"-"`
	BoundAt         time.Time                   `json:"bound_at"`
}

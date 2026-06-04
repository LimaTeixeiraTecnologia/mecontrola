package interfaces

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

// EntitlementCache é o port para o cache in-memory de decisões de entitlement (ADR-004).
type EntitlementCache interface {
	Get(userID identityentities.UserID) (output.EntitlementDecision, bool)
	Set(userID identityentities.UserID, decision output.EntitlementDecision, ttl time.Duration)
	Invalidate(userID identityentities.UserID)
}

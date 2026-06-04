package input

import identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"

// CheckEntitlementInput identifica o usuário para consulta de entitlement.
type CheckEntitlementInput struct {
	UserID identityentities.UserID
}

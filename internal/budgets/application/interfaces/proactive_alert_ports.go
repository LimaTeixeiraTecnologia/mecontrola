package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
)

type UserTimezoneResolver interface {
	Resolve(ctx context.Context, userID uuid.UUID) (*time.Location, error)
}

type MarketingConsentReader interface {
	HasConsent(ctx context.Context, userID uuid.UUID) (bool, error)
}

type AlertTemplate struct {
	Name         string
	LanguageCode string
	Category     services.TemplateCategory
	Status       services.TemplateStatus
}

type AlertTemplateCatalog interface {
	Lookup(kind services.ThresholdAlertKind) (AlertTemplate, bool)
}

type AlertContextRecorder interface {
	Record(ctx context.Context, resourceID, threadID, kind, followUpTopic string, sentAt time.Time) error
}

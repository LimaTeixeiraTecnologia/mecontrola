package interfaces

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type RecurringTemplateEventPublisher interface {
	PublishCreated(ctx context.Context, db database.DBTX, evt entities.RecurringTemplateCreated) error
	PublishUpdated(ctx context.Context, db database.DBTX, evt entities.RecurringTemplateUpdated) error
	PublishDeleted(ctx context.Context, db database.DBTX, evt entities.RecurringTemplateDeleted) error
}

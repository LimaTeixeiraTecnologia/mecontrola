package interfaces

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type TransactionEventPublisher interface {
	PublishCreated(ctx context.Context, db database.DBTX, evt entities.TransactionCreated) error
	PublishUpdated(ctx context.Context, db database.DBTX, evt entities.TransactionUpdated) error
	PublishDeleted(ctx context.Context, db database.DBTX, evt entities.TransactionDeleted) error
}

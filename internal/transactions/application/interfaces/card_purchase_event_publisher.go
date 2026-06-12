package interfaces

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type CardPurchaseEventPublisher interface {
	PublishCreated(ctx context.Context, db database.DBTX, evt entities.CardPurchaseCreated) error
	PublishUpdated(ctx context.Context, db database.DBTX, evt entities.CardPurchaseUpdated) error
	PublishDeleted(ctx context.Context, db database.DBTX, evt entities.CardPurchaseDeleted) error
}

package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type RecurringMaterializationRepository interface {
	InsertIfAbsent(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth, materializedTransactionID, materializedPurchaseID *uuid.UUID, now time.Time) (inserted bool, err error)
	TryAdvisoryLock(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth) (acquired bool, release func(), err error)
	IsCompleted(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth) (bool, error)
	MarkCompleted(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth, transactionID, purchaseID *uuid.UUID) error
}

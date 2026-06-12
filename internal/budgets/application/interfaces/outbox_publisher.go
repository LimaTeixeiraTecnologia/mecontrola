package interfaces

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/events"
)

type ExpenseCommittedPublisher interface {
	Publish(ctx context.Context, db database.DBTX, evt events.ExpenseCommitted) error
}

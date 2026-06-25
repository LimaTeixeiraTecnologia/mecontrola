package interfaces

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
)

type ExpenseCommittedPublisher interface {
	Publish(ctx context.Context, db database.DBTX, evt events.ExpenseCommitted) error
}

type ThresholdAlertPublisher interface {
	Publish(ctx context.Context, db database.DBTX, alert services.DomainAlert, occurredAt time.Time) error
}

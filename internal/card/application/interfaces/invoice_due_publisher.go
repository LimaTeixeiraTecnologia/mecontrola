package interfaces

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type InvoiceDuePublisher interface {
	Publish(ctx context.Context, db database.DBTX, alert services.InvoiceDueAlert, occurredAt time.Time) error
}

package interfaces

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
)

type InvoiceDuePublisher interface {
	Publish(ctx context.Context, db database.DBTX, alert services.InvoiceDueAlert, occurredAt time.Time) error
}

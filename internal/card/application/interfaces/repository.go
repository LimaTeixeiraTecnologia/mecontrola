package interfaces

import (
	"context"
	"errors"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
)

var ErrInvoiceDueAlertRecordMissing = errors.New("card: registro de alerta de vencimento nao encontrado")

type CardRepository interface {
	Insert(ctx context.Context, c entities.Card) error
	GetByIDForUser(ctx context.Context, cardID, userID string) (entities.Card, error)
	ListByUser(ctx context.Context, userID, cursor string, limit int) ([]entities.Card, string, error)
	UpdateByIDForUser(ctx context.Context, c entities.Card) (entities.Card, error)
	UpdateLimitByIDForUser(ctx context.Context, c entities.Card, expectedVersion int64) (entities.Card, error)
	SoftDeleteByIDForUser(ctx context.Context, cardID, userID string, now time.Time) error
	FindCardsWithInvoiceDueWithin(ctx context.Context, windowDays, limit int) ([]entities.Card, error)
}

type InvoiceDueAlertSentRecord struct {
	UserID     uuid.UUID
	CardID     uuid.UUID
	RefDueDate time.Time
	NotifiedAt time.Time
}

type InvoiceDueAlertSentRepository interface {
	ListSentForDueDates(ctx context.Context, dueDates []time.Time) ([]InvoiceDueAlertSentRecord, error)
	InsertSent(ctx context.Context, userID, cardID uuid.UUID, refDueDate time.Time) error
	IsNotified(ctx context.Context, userID, cardID uuid.UUID, refDueDate time.Time) (bool, error)
	MarkNotified(ctx context.Context, userID, cardID uuid.UUID, refDueDate time.Time, channel string, notifiedAt time.Time) (bool, error)
}

type RepositoryFactory interface {
	CardRepository(db database.DBTX) CardRepository
	InvoiceDueAlertSentRepository(db database.DBTX) InvoiceDueAlertSentRepository
}

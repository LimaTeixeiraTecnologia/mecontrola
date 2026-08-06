package interfaces

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrAlertRecordMissing = errors.New("budgets: registro de alerta nao encontrado")

type ActiveBudgetForScan struct {
	UserID       uuid.UUID
	BudgetID     uuid.UUID
	Competence   valueobjects.Competence
	RootSlug     valueobjects.RootSlug
	PlannedCents int64
	SpentCents   int64
}

type ThresholdAlertSentRecord struct {
	UserID   uuid.UUID
	BudgetID uuid.UUID
	Kind     services.ThresholdAlertKind
	RefDay   time.Time
	SentAt   time.Time
}

type ThresholdAlertSentRepository interface {
	ListActiveForThresholdScan(ctx context.Context, refMonth valueobjects.Competence, limit int) ([]ActiveBudgetForScan, error)
	ListSentForDay(ctx context.Context, refDay time.Time) ([]ThresholdAlertSentRecord, error)
	InsertSent(ctx context.Context, rec ThresholdAlertSentRecord) error
	IsNotified(ctx context.Context, userID, budgetID uuid.UUID, kind services.ThresholdAlertKind, refDay time.Time) (bool, error)
	MarkNotified(ctx context.Context, userID, budgetID uuid.UUID, kind services.ThresholdAlertKind, refDay time.Time, channel string, notifiedAt time.Time) (bool, error)
}

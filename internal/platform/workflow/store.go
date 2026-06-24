package workflow

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrVersionConflict = errors.New("workflow: version conflict")

type Snapshot struct {
	RunID          uuid.UUID
	Workflow       string
	CorrelationKey string
	Status         RunStatus
	SuspendReason  SuspendReason
	Cursor         int
	State          []byte
	Attempts       int
	MaxAttempts    int
	Version        int64
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	EndedAt        *time.Time
}

type StepRecord struct {
	ID         uuid.UUID
	RunID      uuid.UUID
	StepID     string
	Seq        int
	Status     StepStatus
	Attempt    int
	DurationMs int64
	Error      string
	StartedAt  time.Time
	EndedAt    *time.Time
}

type Store interface {
	Insert(ctx context.Context, snap Snapshot) error
	Load(ctx context.Context, workflow, key string) (Snapshot, bool, error)
	Save(ctx context.Context, snap Snapshot, expectedVersion int64) error
	AppendStep(ctx context.Context, rec StepRecord) error
	DeleteCompleted(ctx context.Context, retention time.Duration, limit int) (int64, error)
}

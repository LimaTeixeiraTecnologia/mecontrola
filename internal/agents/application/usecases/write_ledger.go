package usecases

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrLedgerEntryNotFound = errors.New("agents.persistence: ledger entry not found")

type WriteLedgerEntry struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	WAMID        string
	ItemSeq      int
	Operation    string
	ResourceID   uuid.UUID
	ResourceKind string
	CreatedAt    time.Time
}

type WriteLedgerRepository interface {
	FindByKey(ctx context.Context, wamid string, itemSeq int, operation string) (WriteLedgerEntry, error)
	Insert(ctx context.Context, entry WriteLedgerEntry) error
	DeleteBefore(ctx context.Context, before time.Time, batchSize int) (int64, error)
}

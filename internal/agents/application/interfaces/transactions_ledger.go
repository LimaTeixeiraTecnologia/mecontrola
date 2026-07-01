package interfaces

import (
	"context"

	"github.com/google/uuid"
)

type TransactionsLedger interface {
	CreateTransaction(ctx context.Context, in RawTransaction) (EntryRef, error)
	CreateCardPurchase(ctx context.Context, in RawCardPurchase) (EntryRef, error)
	UpdateTransaction(ctx context.Context, in RawUpdateTransaction) (EntryRef, error)
	DeleteTransaction(ctx context.Context, ref EntryRef, version int64) error
	UpdateCardPurchase(ctx context.Context, in RawUpdateCardPurchase) (EntryRef, error)
	DeleteCardPurchase(ctx context.Context, ref EntryRef, version int64) error
	ListMonthlyEntries(ctx context.Context, userID uuid.UUID, refMonth string, cursor string, limit int) ([]MonthlyEntry, error)
	GetMonthlySummary(ctx context.Context, userID uuid.UUID, refMonth string) (MonthlySummary, error)
}

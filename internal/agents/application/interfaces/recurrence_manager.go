package interfaces

import "context"

type RecurrenceManager interface {
	CreateRecurrence(ctx context.Context, in RawRecurrence) (EntryRef, error)
	UpdateRecurrence(ctx context.Context, templateID string, in RawUpdateRecurrence) (EntryRef, error)
	DeleteRecurrence(ctx context.Context, templateID string, version int64) error
	ListRecurrences(ctx context.Context, activeOnly bool, cursor string, limit int) ([]Recurrence, error)
}

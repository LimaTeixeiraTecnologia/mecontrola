package interfaces

import (
	"context"
	"errors"
	"time"
)

var ErrEventAlreadyProcessed = errors.New("billing: event already processed")

type ProcessedEventRepository interface {
	MarkApplied(ctx context.Context, eventKey string, trigger string, recursoID string, occurredAt time.Time) error
	MarkSuperseded(ctx context.Context, eventKey string) error
}

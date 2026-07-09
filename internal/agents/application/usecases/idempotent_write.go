package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type WriteFn func(ctx context.Context) (uuid.UUID, bool, error)

type IdempotentWriteResult struct {
	ResourceID uuid.UUID
	Outcome    agent.ToolOutcome
}

type idempotentWriteLedger interface {
	FindByKey(ctx context.Context, wamid string, itemSeq int, operation string) (WriteLedgerEntry, error)
	Insert(ctx context.Context, entry WriteLedgerEntry) error
}

type IdempotentWrite struct {
	ledger idempotentWriteLedger
	o11y   observability.Observability
	total  observability.Counter
}

func NewIdempotentWrite(ledger idempotentWriteLedger, o11y observability.Observability) *IdempotentWrite {
	total := o11y.Metrics().Counter(
		"agents_write_total",
		"Total de escritas do agente por operação e resultado",
		"1",
	)
	return &IdempotentWrite{ledger: ledger, o11y: o11y, total: total}
}

func (uc *IdempotentWrite) Execute(
	ctx context.Context,
	userID uuid.UUID,
	wamid string,
	itemSeq int,
	operation string,
	resourceKind string,
	write WriteFn,
	isDomainErr workflows.DomainErrorClassifier,
) (IdempotentWriteResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.idempotent_write")
	defer span.End()

	return uc.executeLocked(ctx, span, userID, wamid, itemSeq, operation, resourceKind, write, isDomainErr)
}

func (uc *IdempotentWrite) executeLocked(
	ctx context.Context,
	span observability.Span,
	userID uuid.UUID,
	wamid string,
	itemSeq int,
	operation string,
	resourceKind string,
	write WriteFn,
	isDomainErr workflows.DomainErrorClassifier,
) (IdempotentWriteResult, error) {
	existing, err := uc.ledger.FindByKey(ctx, wamid, itemSeq, operation)
	if err != nil && !errors.Is(err, ErrLedgerEntryNotFound) {
		span.RecordError(err)
		return IdempotentWriteResult{}, fmt.Errorf("agents.usecase.idempotent_write: ledger lookup: %w", err)
	}

	if err == nil {
		uc.total.Add(ctx, 1,
			observability.String("operation", operation),
			observability.String("outcome", "replay"),
		)
		return IdempotentWriteResult{
			ResourceID: existing.ResourceID,
			Outcome:    agent.ToolOutcomeReplay,
		}, nil
	}

	resourceID, reconciled, writeErr := write(ctx)
	if writeErr != nil {
		span.RecordError(writeErr)
		writeOutcome := "usecase_error"
		if isDomainErr != nil && isDomainErr(writeErr) {
			writeOutcome = "domain_rejected"
		}
		uc.total.Add(ctx, 1,
			observability.String("operation", operation),
			observability.String("outcome", writeOutcome),
		)
		return IdempotentWriteResult{}, fmt.Errorf("agents.usecase.idempotent_write: write: %w", writeErr)
	}

	id, err := uuid.NewV7()
	if err != nil {
		span.RecordError(err)
		return IdempotentWriteResult{}, fmt.Errorf("agents.usecase.idempotent_write: generate id: %w", err)
	}

	insertErr := uc.ledger.Insert(ctx, WriteLedgerEntry{
		ID:           id,
		UserID:       userID,
		WAMID:        wamid,
		ItemSeq:      itemSeq,
		Operation:    operation,
		ResourceID:   resourceID,
		ResourceKind: resourceKind,
		CreatedAt:    time.Now().UTC(),
	})
	if insertErr != nil {
		span.RecordError(insertErr)
		uc.o11y.Logger().Error(ctx, "agents.usecase.idempotent_write.ledger_insert_failed",
			observability.String("wamid", wamid),
			observability.Int("item_seq", itemSeq),
			observability.String("operation", operation),
			observability.String("user_id", userID.String()),
			observability.String("resource_id", resourceID.String()),
			observability.String("resource_kind", resourceKind),
			observability.Error(insertErr),
		)
		return IdempotentWriteResult{}, fmt.Errorf("agents.usecase.idempotent_write: persist replay ledger: %w", insertErr)
	}

	outcome := agent.ToolOutcomeRouted
	label := "created"
	if reconciled {
		outcome = agent.ToolOutcomeReconciled
		label = "reconciled"
	}
	uc.total.Add(ctx, 1,
		observability.String("operation", operation),
		observability.String("outcome", label),
	)
	return IdempotentWriteResult{
		ResourceID: resourceID,
		Outcome:    outcome,
	}, nil
}

package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
)

type PendingEventOutcome uint8

const (
	PendingEventOutcomeApplied PendingEventOutcome = iota + 1
	PendingEventOutcomeObsoleteIdempotent
	PendingEventOutcomeStillPending
	PendingEventOutcomeExpired
)

type pendingEventPayload struct {
	SubcategoryID string    `json:"subcategory_id"`
	Competence    string    `json:"competence"`
	AmountCents   int64     `json:"amount_cents"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type ApplyPendingEvent struct {
	factory    interfaces.RepositoryFactory
	upsert     *UpsertExpense
	delete     *DeleteExpense
	resolver   *services.PendingEventOutcomeResolver
	pendingTTL time.Duration
	o11y       observability.Observability
}

func NewApplyPendingEvent(
	factory interfaces.RepositoryFactory,
	upsert *UpsertExpense,
	del *DeleteExpense,
	pendingTTL time.Duration,
	o11y observability.Observability,
) *ApplyPendingEvent {
	return &ApplyPendingEvent{
		factory:    factory,
		upsert:     upsert,
		delete:     del,
		resolver:   services.NewPendingEventOutcomeResolver(),
		pendingTTL: pendingTTL,
		o11y:       o11y,
	}
}

func (uc *ApplyPendingEvent) Execute(ctx context.Context, db database.DBTX, evt entities.PendingEvent) (PendingEventOutcome, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.apply_pending_event")
	defer span.End()

	if evt.IsExpired(uc.pendingTTL, time.Now().UTC()) {
		return PendingEventOutcomeExpired, nil
	}

	current, loadErr := uc.loadCurrent(ctx, db, evt)
	if loadErr != nil {
		return 0, loadErr
	}

	decision := uc.resolver.Decide(evt, current)

	switch decision.Kind {
	case services.OutcomeNoop:
		return PendingEventOutcomeObsoleteIdempotent, nil
	case services.OutcomeDefer:
		return PendingEventOutcomeStillPending, nil
	case services.OutcomeCreate:
		return uc.applyCreate(ctx, db, evt)
	case services.OutcomeUpdate:
		if err := uc.applyUpdate(ctx, db, evt, decision.ExpectedVersion); err != nil {
			return 0, err
		}
		return PendingEventOutcomeApplied, nil
	case services.OutcomeDelete:
		return uc.applyDelete(ctx, db, evt, decision.ExpectedVersion)
	default:
		return PendingEventOutcomeStillPending, nil
	}
}

func (uc *ApplyPendingEvent) loadCurrent(ctx context.Context, db database.DBTX, evt entities.PendingEvent) (*entities.Expense, error) {
	expenses := uc.factory.ExpenseRepository(db)
	identity := entities.ExpenseIdentity{
		UserID:                evt.UserID(),
		Source:                evt.Source(),
		ExternalTransactionID: evt.ExternalTransactionID(),
	}
	existing, _, getErr := expenses.GetByIdentity(ctx, identity)
	if getErr != nil {
		if errors.Is(getErr, interfaces.ErrExpenseNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("budgets.usecase.apply_pending_event: ler despesa: %w", getErr)
	}
	return &existing, nil
}

func (uc *ApplyPendingEvent) applyCreate(ctx context.Context, db database.DBTX, evt entities.PendingEvent) (PendingEventOutcome, error) {
	payload, err := uc.decodePayload(evt)
	if err != nil {
		return 0, err
	}

	if _, err := uc.upsert.ExecuteWithTx(ctx, db, input.UpsertExpenseInput{
		UserID:                evt.UserID().String(),
		Source:                evt.Source().String(),
		ExternalTransactionID: evt.ExternalTransactionID().String(),
		SubcategoryID:         payload.SubcategoryID,
		Competence:            payload.Competence,
		AmountCents:           payload.AmountCents,
		OccurredAt:            payload.OccurredAt,
	}); err != nil {
		return 0, fmt.Errorf("budgets.usecase.apply_pending_event: upsert_expense criar: %w", err)
	}

	return PendingEventOutcomeApplied, nil
}

func (uc *ApplyPendingEvent) applyDelete(ctx context.Context, db database.DBTX, evt entities.PendingEvent, expectedVersion int64) (PendingEventOutcome, error) {
	if err := uc.delete.ExecuteWithTx(ctx, db, input.DeleteExpenseInput{
		UserID:                evt.UserID().String(),
		Source:                evt.Source().String(),
		ExternalTransactionID: evt.ExternalTransactionID().String(),
		ExpectedVersion:       expectedVersion,
	}); err != nil {
		return 0, fmt.Errorf("budgets.usecase.apply_pending_event: delete_expense: %w", err)
	}
	return PendingEventOutcomeApplied, nil
}

func (uc *ApplyPendingEvent) applyUpdate(ctx context.Context, db database.DBTX, evt entities.PendingEvent, expectedVersion int64) error {
	payload, err := uc.decodePayload(evt)
	if err != nil {
		return err
	}

	ev := expectedVersion
	if _, err := uc.upsert.ExecuteWithTx(ctx, db, input.UpsertExpenseInput{
		UserID:                evt.UserID().String(),
		Source:                evt.Source().String(),
		ExternalTransactionID: evt.ExternalTransactionID().String(),
		SubcategoryID:         payload.SubcategoryID,
		Competence:            payload.Competence,
		AmountCents:           payload.AmountCents,
		OccurredAt:            payload.OccurredAt,
		ExpectedVersion:       &ev,
	}); err != nil {
		return fmt.Errorf("budgets.usecase.apply_pending_event: upsert_expense atualizar: %w", err)
	}
	return nil
}

func (uc *ApplyPendingEvent) decodePayload(evt entities.PendingEvent) (pendingEventPayload, error) {
	var payload pendingEventPayload
	if err := json.Unmarshal(evt.Payload(), &payload); err != nil {
		return pendingEventPayload{}, fmt.Errorf("budgets.usecase.apply_pending_event: deserializar payload: %w", err)
	}
	return payload, nil
}

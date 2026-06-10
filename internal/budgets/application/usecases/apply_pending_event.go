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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
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
	expenses   interfaces.ExpenseRepository
	upsert     *UpsertExpense
	delete     *DeleteExpense
	pendingTTL time.Duration
	o11y       observability.Observability
}

func NewApplyPendingEvent(
	expenses interfaces.ExpenseRepository,
	upsert *UpsertExpense,
	del *DeleteExpense,
	pendingTTL time.Duration,
	o11y observability.Observability,
) *ApplyPendingEvent {
	return &ApplyPendingEvent{
		expenses:   expenses,
		upsert:     upsert,
		delete:     del,
		pendingTTL: pendingTTL,
		o11y:       o11y,
	}
}

func (uc *ApplyPendingEvent) Execute(ctx context.Context, db database.DBTX, evt entities.PendingEvent) (PendingEventOutcome, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.apply_pending_event")
	defer span.End()

	now := time.Now().UTC()
	if evt.IsExpired(uc.pendingTTL, now) {
		return PendingEventOutcomeExpired, nil
	}

	identity := entities.ExpenseIdentity{
		UserID:                evt.UserID(),
		Source:                evt.Source(),
		ExternalTransactionID: evt.ExternalTransactionID(),
	}

	existing, _, getErr := uc.expenses.GetByIdentity(ctx, db, identity)

	if evt.MutationKind() == valueobjects.MutationKindCreate {
		if getErr != nil && !errors.Is(getErr, interfaces.ErrExpenseNotFound) {
			return 0, fmt.Errorf("budgets.usecase.apply_pending_event: ler despesa: %w", getErr)
		}

		if getErr == nil {
			return PendingEventOutcomeObsoleteIdempotent, nil
		}

		if evt.ExpectedVersion() != 1 {
			return PendingEventOutcomeObsoleteIdempotent, nil
		}

		var p pendingEventPayload
		if err := json.Unmarshal(evt.Payload(), &p); err != nil {
			return 0, fmt.Errorf("budgets.usecase.apply_pending_event: deserializar payload: %w", err)
		}

		_, execErr := uc.upsert.ExecuteWithTx(ctx, db, input.UpsertExpenseInput{
			UserID:                evt.UserID().String(),
			Source:                evt.Source().String(),
			ExternalTransactionID: evt.ExternalTransactionID().String(),
			SubcategoryID:         p.SubcategoryID,
			Competence:            p.Competence,
			AmountCents:           p.AmountCents,
			OccurredAt:            p.OccurredAt,
		})
		if execErr != nil {
			return 0, fmt.Errorf("budgets.usecase.apply_pending_event: upsert_expense criar: %w", execErr)
		}

		return PendingEventOutcomeApplied, nil
	}

	if getErr != nil {
		if errors.Is(getErr, interfaces.ErrExpenseNotFound) {
			return PendingEventOutcomeStillPending, nil
		}
		return 0, fmt.Errorf("budgets.usecase.apply_pending_event: ler despesa: %w", getErr)
	}

	currentVersion := existing.Version()

	if evt.ExpectedVersion() <= currentVersion {
		return PendingEventOutcomeObsoleteIdempotent, nil
	}

	if evt.ExpectedVersion() > currentVersion+1 {
		return PendingEventOutcomeStillPending, nil
	}

	if evt.MutationKind() == valueobjects.MutationKindDelete {
		execErr := uc.delete.ExecuteWithTx(ctx, db, input.DeleteExpenseInput{
			UserID:                evt.UserID().String(),
			Source:                evt.Source().String(),
			ExternalTransactionID: evt.ExternalTransactionID().String(),
			ExpectedVersion:       currentVersion,
		})
		if execErr != nil {
			return 0, fmt.Errorf("budgets.usecase.apply_pending_event: delete_expense: %w", execErr)
		}
		return PendingEventOutcomeApplied, nil
	}

	var p pendingEventPayload
	if err := json.Unmarshal(evt.Payload(), &p); err != nil {
		return 0, fmt.Errorf("budgets.usecase.apply_pending_event: deserializar payload: %w", err)
	}

	expectedVersion := currentVersion
	_, execErr := uc.upsert.ExecuteWithTx(ctx, db, input.UpsertExpenseInput{
		UserID:                evt.UserID().String(),
		Source:                evt.Source().String(),
		ExternalTransactionID: evt.ExternalTransactionID().String(),
		SubcategoryID:         p.SubcategoryID,
		Competence:            p.Competence,
		AmountCents:           p.AmountCents,
		OccurredAt:            p.OccurredAt,
		ExpectedVersion:       &expectedVersion,
	})
	if execErr != nil {
		return 0, fmt.Errorf("budgets.usecase.apply_pending_event: upsert_expense atualizar: %w", execErr)
	}

	return PendingEventOutcomeApplied, nil
}

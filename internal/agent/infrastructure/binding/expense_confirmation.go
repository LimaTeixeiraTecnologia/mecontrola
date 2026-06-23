package binding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

const pendingExpensePrefix = "pending_expense:"

type PendingExpenseConfirmationAdapter struct {
	repo appinterfaces.AgentSessionRepository
	unit uow.UnitOfWork
}

func NewPendingExpenseConfirmationAdapter(repo appinterfaces.AgentSessionRepository, unit uow.UnitOfWork) *PendingExpenseConfirmationAdapter {
	return &PendingExpenseConfirmationAdapter{repo: repo, unit: unit}
}

func (a *PendingExpenseConfirmationAdapter) Load(ctx context.Context, userID uuid.UUID, channel string) (pendingexpense.Draft, bool, error) {
	record, err := a.repo.GetByUserAndChannel(ctx, userID, channel)
	if err != nil {
		if errors.Is(err, appinterfaces.ErrAgentSessionNotFound) {
			return pendingexpense.Draft{}, false, nil
		}
		return pendingexpense.Draft{}, false, fmt.Errorf("agent: pending expense load: %w", err)
	}
	raw := record.PendingAction
	prefix := []byte(pendingExpensePrefix)
	if len(raw) <= len(prefix) || string(raw[:len(prefix)]) != pendingExpensePrefix {
		return pendingexpense.Draft{}, false, nil
	}
	payload := raw[len(prefix):]
	draft, err := pendingexpense.Decode(payload)
	if err != nil {
		return pendingexpense.Draft{}, false, fmt.Errorf("agent: pending expense decode: %w", err)
	}
	return draft, true, nil
}

func (a *PendingExpenseConfirmationAdapter) Save(ctx context.Context, userID uuid.UUID, channel string, draft pendingexpense.Draft) error {
	encoded, err := pendingexpense.Encode(draft)
	if err != nil {
		return fmt.Errorf("agent: pending expense encode: %w", err)
	}
	prefix := []byte(pendingExpensePrefix)
	pending := append(prefix, encoded...)
	now := time.Now().UTC()
	record := appinterfaces.AgentSessionRecord{
		ID:            uuid.New(),
		UserID:        userID,
		Channel:       channel,
		PendingAction: pending,
		RecentTurns:   []byte("[]"),
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(sessionTTL),
	}
	persist := func(ctx context.Context, db database.DBTX) error {
		return a.repo.Upsert(ctx, record)
	}
	if a.unit == nil {
		if err := persist(ctx, nil); err != nil {
			return fmt.Errorf("agent: pending expense save: %w", err)
		}
		return nil
	}
	if err := a.unit.Do(ctx, persist); err != nil {
		return fmt.Errorf("agent: pending expense save: %w", err)
	}
	return nil
}

func (a *PendingExpenseConfirmationAdapter) Clear(ctx context.Context, userID uuid.UUID, channel string) error {
	now := time.Now().UTC()
	record := appinterfaces.AgentSessionRecord{
		ID:            uuid.New(),
		UserID:        userID,
		Channel:       channel,
		PendingAction: []byte("{}"),
		RecentTurns:   []byte("[]"),
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(-time.Minute),
	}
	persist := func(ctx context.Context, db database.DBTX) error {
		return a.repo.Upsert(ctx, record)
	}
	if a.unit == nil {
		if err := persist(ctx, nil); err != nil {
			return fmt.Errorf("agent: pending expense clear: %w", err)
		}
		return nil
	}
	if err := a.unit.Do(ctx, persist); err != nil {
		return fmt.Errorf("agent: pending expense clear: %w", err)
	}
	return nil
}

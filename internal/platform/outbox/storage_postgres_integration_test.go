//go:build integration

package outbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type StoragePostgresIntegrationSuite struct {
	suite.Suite
}

func TestStoragePostgresIntegrationSuite(t *testing.T) {
	suite.Run(t, new(StoragePostgresIntegrationSuite))
}

func (s *StoragePostgresIntegrationSuite) SetupTest() {}

func (s *StoragePostgresIntegrationSuite) setupOutboxDB() manager.Manager {
	mgr, _ := testcontainer.Postgres(s.T())
	return mgr
}

func (s *StoragePostgresIntegrationSuite) newEvent(id string) outbox.Event {
	if id == "" {
		id = uuid.NewString()
	}
	event, err := outbox.NewEvent(outbox.EventInput{
		ID:            id,
		Type:          "billing.subscription.activated",
		AggregateType: "subscription",
		AggregateID:   uuid.NewString(),
		Payload:       []byte(`{"foo":"bar"}`),
		Metadata:      map[string]string{"source": "test"},
		OccurredAt:    time.Now().UTC().Add(-time.Minute),
	})
	s.Require().NoError(err)
	return event
}

func (s *StoragePostgresIntegrationSuite) newEventWithUserID(userID string) outbox.Event {
	event, err := outbox.NewEvent(outbox.EventInput{
		Type:            "billing.subscription.activated",
		AggregateType:   "subscription",
		AggregateID:     uuid.NewString(),
		AggregateUserID: userID,
		Payload:         []byte(`{"foo":"bar"}`),
		Metadata:        map[string]string{"source": "test"},
		OccurredAt:      time.Now().UTC().Add(-time.Minute),
	})
	s.Require().NoError(err)
	return event
}

func (s *StoragePostgresIntegrationSuite) countStatus(mgr manager.Manager, id string) int {
	ctx := context.Background()
	var status int
	err := mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT status FROM outbox_events WHERE id = $1`, id).Scan(&status)
	s.Require().NoError(err)
	return status
}

func (s *StoragePostgresIntegrationSuite) countRows(mgr manager.Manager, id string) int {
	ctx := context.Background()
	var total int
	err := mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE id = $1`, id).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *StoragePostgresIntegrationSuite) TestStoragePostgres() {
	type observed struct {
		rows []outbox.Row
	}

	scenarios := []struct {
		name   string
		setup  func(manager.Manager, outbox.OutboxRepository, *observed)
		act    func(manager.Manager, outbox.OutboxRepository, *observed)
		expect func(manager.Manager, *observed)
	}{
		{
			name:  "deve manter insert idempotente por id",
			setup: func(manager.Manager, outbox.OutboxRepository, *observed) {},
			act: func(mgr manager.Manager, storage outbox.OutboxRepository, _ *observed) {
				ctx := context.Background()
				event := s.newEvent("")
				s.NoError(storage.Insert(ctx, event, 10))
				s.NoError(storage.Insert(ctx, event, 10))
				s.NoError(storage.Insert(ctx, event, 10))
				s.Equal(1, s.countRows(mgr, event.ID))
				s.Equal(int(outbox.StatusPending), s.countStatus(mgr, event.ID))
			},
			expect: func(manager.Manager, *observed) {},
		},
		{
			name:  "deve transicionar para published",
			setup: func(manager.Manager, outbox.OutboxRepository, *observed) {},
			act: func(mgr manager.Manager, storage outbox.OutboxRepository, _ *observed) {
				ctx := context.Background()
				event := s.newEvent("")
				s.NoError(storage.Insert(ctx, event, 5))
				s.NoError(storage.MarkPublished(ctx, event.ID))
				s.Equal(int(outbox.StatusPublished), s.countStatus(mgr, event.ID))
			},
			expect: func(manager.Manager, *observed) {},
		},
		{
			name:  "deve transicionar para failed",
			setup: func(manager.Manager, outbox.OutboxRepository, *observed) {},
			act: func(mgr manager.Manager, storage outbox.OutboxRepository, _ *observed) {
				ctx := context.Background()
				event := s.newEvent("")
				s.NoError(storage.Insert(ctx, event, 3))
				s.NoError(storage.MarkFailed(ctx, event.ID, "exhausted"))
				s.Equal(int(outbox.StatusFailed), s.countStatus(mgr, event.ID))
			},
			expect: func(manager.Manager, *observed) {},
		},
		{
			name:  "deve incrementar attempts ao marcar retry pendente",
			setup: func(manager.Manager, outbox.OutboxRepository, *observed) {},
			act: func(mgr manager.Manager, storage outbox.OutboxRepository, _ *observed) {
				ctx := context.Background()
				event := s.newEvent("")
				s.NoError(storage.Insert(ctx, event, 5))
				next := time.Now().UTC().Add(30 * time.Second)
				s.NoError(storage.MarkPendingRetry(ctx, event.ID, "transient", next))
				var attempts int
				var lastError string
				err := mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT attempts, last_error FROM outbox_events WHERE id = $1`, event.ID).Scan(&attempts, &lastError)
				s.Require().NoError(err)
				s.Equal(1, attempts)
				s.Equal("transient", lastError)
				s.Equal(int(outbox.StatusPending), s.countStatus(mgr, event.ID))
			},
			expect: func(manager.Manager, *observed) {},
		},
		{
			name:  "deve claimar evento imediatamente mesmo com occurred_at futuro",
			setup: func(manager.Manager, outbox.OutboxRepository, *observed) {},
			act: func(mgr manager.Manager, storage outbox.OutboxRepository, state *observed) {
				ctx := context.Background()
				event, err := outbox.NewEvent(outbox.EventInput{
					Type:          "billing.subscription.activated",
					AggregateType: "subscription",
					AggregateID:   uuid.NewString(),
					Payload:       []byte(`{"foo":"bar"}`),
					Metadata:      map[string]string{"source": "future"},
					OccurredAt:    time.Now().UTC().Add(3 * time.Hour),
				})
				s.Require().NoError(err)
				s.NoError(storage.Insert(ctx, event, 5))
				rows, claimErr := storage.ClaimBatch(ctx, "worker-future", 10)
				s.Require().NoError(claimErr)
				state.rows = rows
			},
			expect: func(_ manager.Manager, state *observed) {
				s.Len(state.rows, 1)
				s.Equal("billing.subscription.activated", state.rows[0].Type)
			},
		},
		{
			name:  "deve claimar lote e evitar reclaim antes do reset",
			setup: func(manager.Manager, outbox.OutboxRepository, *observed) {},
			act: func(mgr manager.Manager, storage outbox.OutboxRepository, state *observed) {
				ctx := context.Background()
				for range 3 {
					s.NoError(storage.Insert(ctx, s.newEvent(""), 5))
				}
				rows, err := storage.ClaimBatch(ctx, "worker-1", 10)
				s.Require().NoError(err)
				state.rows = rows
				s.Len(rows, 3)
				for _, row := range rows {
					s.Equal(int(outbox.StatusProcessing), s.countStatus(mgr, row.ID))
				}
				again, err := storage.ClaimBatch(ctx, "worker-2", 10)
				s.Require().NoError(err)
				s.Empty(again)
			},
			expect: func(_ manager.Manager, state *observed) {
				s.Len(state.rows, 3)
			},
		},
		{
			name:  "deve respeitar retention ao deletar publicados",
			setup: func(manager.Manager, outbox.OutboxRepository, *observed) {},
			act: func(mgr manager.Manager, storage outbox.OutboxRepository, _ *observed) {
				ctx := context.Background()
				oldEvent := s.newEvent("")
				s.NoError(storage.Insert(ctx, oldEvent, 5))
				s.NoError(storage.MarkPublished(ctx, oldEvent.ID))
				_, err := mgr.DBTX(ctx).ExecContext(ctx, `UPDATE outbox_events SET published_at = now() - interval '2 hours' WHERE id = $1`, oldEvent.ID)
				s.Require().NoError(err)

				recentEvent := s.newEvent("")
				s.NoError(storage.Insert(ctx, recentEvent, 5))
				s.NoError(storage.MarkPublished(ctx, recentEvent.ID))

				deleted, err := storage.DeletePublishedBatch(ctx, time.Hour, 100)
				s.Require().NoError(err)
				s.Equal(int64(1), deleted)
				s.Equal(0, s.countRows(mgr, oldEvent.ID))
				s.Equal(1, s.countRows(mgr, recentEvent.ID))
			},
			expect: func(manager.Manager, *observed) {},
		},
		{
			name:  "deve round-trip aggregate_user_id em Insert e ClaimBatch",
			setup: func(manager.Manager, outbox.OutboxRepository, *observed) {},
			act: func(mgr manager.Manager, storage outbox.OutboxRepository, state *observed) {
				ctx := context.Background()
				userID := uuid.NewString()
				eventWithUser := s.newEventWithUserID(userID)
				eventNoUser := s.newEvent("")

				s.NoError(storage.Insert(ctx, eventWithUser, 5))
				s.NoError(storage.Insert(ctx, eventNoUser, 5))

				rows, err := storage.ClaimBatch(ctx, "worker-rt", 10)
				s.Require().NoError(err)
				state.rows = rows
			},
			expect: func(_ manager.Manager, state *observed) {
				s.Len(state.rows, 2)
				withUser := 0
				withoutUser := 0
				for _, r := range state.rows {
					if r.AggregateUserID != "" {
						withUser++
					} else {
						withoutUser++
					}
				}
				s.Equal(1, withUser, "deve haver exatamente 1 evento com AggregateUserID")
				s.Equal(1, withoutUser, "deve haver exatamente 1 evento sem AggregateUserID")
			},
		},
		{
			name:  "deve resetar eventos stuck para pending",
			setup: func(manager.Manager, outbox.OutboxRepository, *observed) {},
			act: func(mgr manager.Manager, storage outbox.OutboxRepository, _ *observed) {
				ctx := context.Background()
				event := s.newEvent("")
				s.NoError(storage.Insert(ctx, event, 5))
				rows, err := storage.ClaimBatch(ctx, "worker-A", 10)
				s.Require().NoError(err)
				s.Len(rows, 1)
				_, err = mgr.DBTX(ctx).ExecContext(ctx, `UPDATE outbox_events SET locked_at = now() - interval '10 minutes' WHERE id = $1`, event.ID)
				s.Require().NoError(err)
				resetCount, err := storage.ResetStuck(ctx, 5*time.Minute)
				s.Require().NoError(err)
				s.Equal(int64(1), resetCount)
				s.Equal(int(outbox.StatusPending), s.countStatus(mgr, event.ID))
			},
			expect: func(manager.Manager, *observed) {},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := &observed{}
			mgr := s.setupOutboxDB()
			ctx := context.Background()
			sut := outbox.NewPostgresStorage(mgr.DBTX(ctx))
			scenario.setup(mgr, sut, state)
			scenario.act(mgr, sut, state)
			scenario.expect(mgr, state)
		})
	}
}

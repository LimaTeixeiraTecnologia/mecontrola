//go:build integration

package outbox_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type alwaysFailHandler struct{}

func (alwaysFailHandler) Handle(context.Context, events.Event) error {
	return errors.New("permanent handler error")
}

type failingRegistry struct{}

func (failingRegistry) HandlersOf(string) []events.Handler {
	return []events.Handler{alwaysFailHandler{}}
}

type ClaimPartitionedIntegrationSuite struct {
	suite.Suite
}

func TestClaimPartitionedIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ClaimPartitionedIntegrationSuite))
}

func (s *ClaimPartitionedIntegrationSuite) SetupTest() {}

func (s *ClaimPartitionedIntegrationSuite) newDB() *sqlx.DB {
	db, _ := testcontainer.Postgres(s.T())
	return db
}

func (s *ClaimPartitionedIntegrationSuite) insertUserEvt(ctx context.Context, repo outbox.OutboxRepository, userID string, occurredAt time.Time, maxAttempts int) outbox.Event {
	evt, err := outbox.NewEvent(outbox.EventInput{
		Type:            "agents.whatsapp.inbound.v1",
		AggregateType:   "user",
		AggregateID:     uuid.NewString(),
		AggregateUserID: userID,
		Payload:         []byte(`{"user_id":"` + userID + `"}`),
		OccurredAt:      occurredAt,
	})
	s.Require().NoError(err)
	s.Require().NoError(repo.Insert(ctx, evt, maxAttempts))
	return evt
}

func (s *ClaimPartitionedIntegrationSuite) TestCA01_TwoWorkersNoDoubleProcessingPerUser() {
	ctx := context.Background()
	repo := outbox.NewPostgresStorage(s.newDB())
	userID := uuid.NewString()
	otherUserID := uuid.NewString()

	now := time.Now().UTC()
	s.insertUserEvt(ctx, repo, userID, now.Add(-3*time.Minute), 5)
	s.insertUserEvt(ctx, repo, userID, now.Add(-2*time.Minute), 5)
	s.insertUserEvt(ctx, repo, userID, now.Add(-1*time.Minute), 5)
	s.insertUserEvt(ctx, repo, otherUserID, now.Add(-30*time.Second), 5)

	var (
		mu  sync.Mutex
		all []outbox.Row
		wg  sync.WaitGroup
	)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			rows, err := repo.ClaimBatch(ctx, name, 10)
			s.Require().NoError(err)
			mu.Lock()
			all = append(all, rows...)
			mu.Unlock()
		}(fmt.Sprintf("worker-%d", i))
	}
	wg.Wait()

	userInflight := 0
	for _, r := range all {
		if r.AggregateUserID == userID {
			userInflight++
		}
	}
	s.LessOrEqual(userInflight, 1, "CA-01: at most 1 event per user in flight concurrently")
}

func (s *ClaimPartitionedIntegrationSuite) TestCA07_MultiMessageOrderedByMetaTimestamp() {
	ctx := context.Background()
	repo := outbox.NewPostgresStorage(s.newDB())
	userID := uuid.NewString()

	now := time.Now().UTC()
	e1 := s.insertUserEvt(ctx, repo, userID, now.Add(-3*time.Minute), 5)
	e2 := s.insertUserEvt(ctx, repo, userID, now.Add(-2*time.Minute), 5)
	e3 := s.insertUserEvt(ctx, repo, userID, now.Add(-1*time.Minute), 5)

	r1, err := repo.ClaimBatch(ctx, "w", 10)
	s.Require().NoError(err)
	s.Require().Len(r1, 1)
	s.Equal(e1.ID, r1[0].ID, "CA-07: first claimed must be earliest Meta timestamp")
	s.Require().NoError(repo.MarkPublished(ctx, r1[0].ID))

	r2, err := repo.ClaimBatch(ctx, "w", 10)
	s.Require().NoError(err)
	s.Require().Len(r2, 1)
	s.Equal(e2.ID, r2[0].ID, "CA-07: second claimed must be second Meta timestamp")
	s.Require().NoError(repo.MarkPublished(ctx, r2[0].ID))

	r3, err := repo.ClaimBatch(ctx, "w", 10)
	s.Require().NoError(err)
	s.Require().Len(r3, 1)
	s.Equal(e3.ID, r3[0].ID, "CA-07: third claimed must be third Meta timestamp")
}

func (s *ClaimPartitionedIntegrationSuite) TestC1_SameSecondNoLivelockDeterministicFIFO() {
	ctx := context.Background()
	db := s.newDB()
	repo := outbox.NewPostgresStorage(db)
	userID := uuid.NewString()
	otherUserID := uuid.NewString()

	sameSecond := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Minute)
	first := s.insertUserEvt(ctx, repo, userID, sameSecond, 5)
	second := s.insertUserEvt(ctx, repo, userID, sameSecond, 5)
	s.insertUserEvt(ctx, repo, otherUserID, sameSecond, 5)

	r1, err := repo.ClaimBatch(ctx, "w", 50)
	s.Require().NoError(err, "C1: same-second events must not trigger 23505 livelock")

	userClaimed := 0
	otherClaimed := false
	var firstClaimedID string
	for _, r := range r1 {
		switch r.AggregateUserID {
		case userID:
			userClaimed++
			firstClaimedID = r.ID
		case otherUserID:
			otherClaimed = true
		}
	}
	s.Equal(1, userClaimed, "C1: exactly 1 event per user in flight despite identical occurred_at")
	s.True(otherClaimed, "C1: distinct user must not be starved by the tied pair")
	s.Equal(first.ID, firstClaimedID, "C1: created_at/id tiebreak yields deterministic FIFO (first insert first)")

	s.Require().NoError(repo.MarkPublished(ctx, firstClaimedID))

	r2, err := repo.ClaimBatch(ctx, "w", 50)
	s.Require().NoError(err)
	remaining := 0
	for _, r := range r2 {
		if r.AggregateUserID == userID {
			remaining++
			s.Equal(second.ID, r.ID, "C1: second same-second event claimed after the first completes")
		}
	}
	s.Equal(1, remaining, "C1: no livelock — remaining same-second event becomes claimable")
}

func (s *ClaimPartitionedIntegrationSuite) TestCA10_PoisonDeadLetterNoBlockFIFO() {
	ctx := context.Background()
	db := s.newDB()
	repo := outbox.NewPostgresStorage(db)
	userID := uuid.NewString()

	now := time.Now().UTC()
	poison := s.insertUserEvt(ctx, repo, userID, now.Add(-3*time.Minute), 1)
	normal1 := s.insertUserEvt(ctx, repo, userID, now.Add(-2*time.Minute), 5)
	normal2 := s.insertUserEvt(ctx, repo, userID, now.Add(-1*time.Minute), 5)

	cfg := configs.OutboxConfig{
		DispatcherBatchSize:      50,
		DispatcherHandlerTimeout: 5 * time.Second,
		DispatcherTickInterval:   500 * time.Millisecond,
		RetryBaseBackoff:         10 * time.Millisecond,
		RetryMaxBackoff:          100 * time.Millisecond,
		RetryMaxAttempts:         1,
	}
	o11y := fake.NewProvider()
	job := outbox.NewObservableDispatcherJob(
		uow.NewUnitOfWork(db),
		outbox.NewRepositoryFactory(o11y),
		failingRegistry{},
		cfg,
		o11y,
		rand.New(rand.NewSource(1)),
	)

	s.Require().NoError(job.Run(ctx), "CA-10: dispatcher tick must not fail the batch on a poison event")

	var poisonStatus int
	s.Require().NoError(db.GetContext(ctx, &poisonStatus,
		"SELECT status FROM mecontrola.outbox_events WHERE id = $1", poison.ID))
	s.Equal(4, poisonStatus, "CA-10: poison event must reach dead-letter (status=4) within max_attempts budget")

	r1, err := repo.ClaimBatch(ctx, "w2", 50)
	s.Require().NoError(err)
	s.Require().Len(r1, 1, "CA-10: after poison dead-letter, next event must be claimable (no head-of-line block)")
	s.Equal(normal1.ID, r1[0].ID, "CA-10: FIFO preserved — normal1 before normal2")
	s.Require().NoError(repo.MarkPublished(ctx, normal1.ID))

	r2, err := repo.ClaimBatch(ctx, "w2", 50)
	s.Require().NoError(err)
	s.Require().Len(r2, 1)
	s.Equal(normal2.ID, r2[0].ID, "CA-10: normal2 processed after normal1, FIFO preserved")
}

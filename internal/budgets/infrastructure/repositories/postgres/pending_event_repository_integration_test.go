//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type PendingEventRepositorySuite struct {
	suite.Suite
}

func TestPendingEventRepositorySuite(t *testing.T) {
	suite.Run(t, new(PendingEventRepositorySuite))
}

func (s *PendingEventRepositorySuite) newPendingEvent() entities.PendingEvent {
	return entities.NewPendingEvent(
		uuid.New(),
		mustProducerSource(s.T(), "api"),
		uuid.New(),
		mustExternalTransactionID(s.T(), newUUIDv4()),
		1,
		valueobjects.MutationKindCreate,
		[]byte(`{"amount":100}`),
		time.Now().UTC(),
	)
}

func (s *PendingEventRepositorySuite) TestInsertAndListReady() {
	mgr := setupTestDB(s.T())
	repo := newPendingEventRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	p := s.newPendingEvent()
	s.Require().NoError(repo.Insert(ctx, db, p))

	events, err := repo.ListReady(ctx, db, 10)
	s.Require().NoError(err)
	s.Require().NotEmpty(events)

	found := false
	for _, e := range events {
		if e.ID() == p.ID() {
			found = true
			s.Assert().Equal(entities.PendingStatePending, e.State())
		}
	}
	s.Assert().True(found)
}

func (s *PendingEventRepositorySuite) TestInsertDuplicateEventID() {
	mgr := setupTestDB(s.T())
	repo := newPendingEventRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	eventID := uuid.New()
	p1 := entities.NewPendingEvent(
		eventID,
		mustProducerSource(s.T(), "api"),
		uuid.New(),
		mustExternalTransactionID(s.T(), newUUIDv4()),
		1, valueobjects.MutationKindCreate, []byte(`{}`), time.Now().UTC(),
	)
	p2 := entities.NewPendingEvent(
		eventID,
		mustProducerSource(s.T(), "api"),
		uuid.New(),
		mustExternalTransactionID(s.T(), newUUIDv4()),
		1, valueobjects.MutationKindCreate, []byte(`{}`), time.Now().UTC(),
	)

	s.Require().NoError(repo.Insert(ctx, db, p1))
	err := repo.Insert(ctx, db, p2)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrPendingEventDuplicate))
}

func (s *PendingEventRepositorySuite) TestTransitionToApplied() {
	mgr := setupTestDB(s.T())
	repo := newPendingEventRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	p := s.newPendingEvent()
	s.Require().NoError(repo.Insert(ctx, db, p))

	s.Require().NoError(repo.Transition(ctx, db, p.ID(), entities.PendingStateApplied, "applied successfully"))

	events, err := repo.ListReady(ctx, db, 10)
	s.Require().NoError(err)

	for _, e := range events {
		s.Assert().NotEqual(p.ID(), e.ID(), "evento applied não deve aparecer em ListReady")
	}
}

func (s *PendingEventRepositorySuite) TestTransitionToFailed() {
	mgr := setupTestDB(s.T())
	repo := newPendingEventRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	p := s.newPendingEvent()
	s.Require().NoError(repo.Insert(ctx, db, p))

	s.Require().NoError(repo.Transition(ctx, db, p.ID(), entities.PendingStateFailed, "version mismatch"))

	events, err := repo.ListReady(ctx, db, 10)
	s.Require().NoError(err)

	for _, e := range events {
		s.Assert().NotEqual(p.ID(), e.ID(), "evento failed não deve aparecer em ListReady")
	}
}

func (s *PendingEventRepositorySuite) TestTransitionNotFound() {
	mgr := setupTestDB(s.T())
	repo := newPendingEventRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	err := repo.Transition(ctx, db, uuid.New(), entities.PendingStateApplied, "")
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrPendingEventNotFound))
}

func (s *PendingEventRepositorySuite) TestListReadyOrderByReceivedAt() {
	mgr := setupTestDB(s.T())
	repo := newPendingEventRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		p := entities.NewPendingEvent(
			uuid.New(),
			mustProducerSource(s.T(), "api"),
			userID,
			mustExternalTransactionID(s.T(), newUUIDv4()),
			int64(i+1), valueobjects.MutationKindCreate, []byte(`{}`),
			now.Add(time.Duration(i)*time.Second),
		)
		s.Require().NoError(repo.Insert(ctx, db, p))
	}

	events, err := repo.ListReady(ctx, db, 10)
	s.Require().NoError(err)

	found := make([]entities.PendingEvent, 0)
	for _, e := range events {
		if e.UserID() == userID {
			found = append(found, e)
		}
	}
	s.Require().Len(found, 3)

	for i := 1; i < len(found); i++ {
		s.Assert().True(
			!found[i].ReceivedAt().Before(found[i-1].ReceivedAt()),
			"eventos devem estar ordenados por received_at ASC",
		)
	}
}

func (s *PendingEventRepositorySuite) TestListReadyLimitRespected() {
	mgr := setupTestDB(s.T())
	repo := newPendingEventRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	for i := 0; i < 5; i++ {
		p := s.newPendingEvent()
		s.Require().NoError(repo.Insert(ctx, db, p))
	}

	events, err := repo.ListReady(ctx, db, 2)
	s.Require().NoError(err)
	s.Assert().LessOrEqual(len(events), 2)
}

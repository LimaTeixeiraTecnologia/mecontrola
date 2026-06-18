//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type AlertRepositorySuite struct {
	suite.Suite
}

func TestAlertRepositorySuite(t *testing.T) {
	suite.Run(t, new(AlertRepositorySuite))
}

func (s *AlertRepositorySuite) newAlert(userID uuid.UUID) entities.Alert {
	competence := mustCompetence(s.T(), "2025-01")
	rootSlug := mustRootSlug(s.T(), "expense.custo_fixo")
	threshold := mustThreshold(s.T(), 80)
	return entities.NewAlert(
		userID,
		competence,
		rootSlug,
		threshold,
		entities.AlertStatePendingDelivery,
		time.Now().UTC(),
		80000,
		100000,
		time.Now().UTC(),
	)
}

func (s *AlertRepositorySuite) TestInsertAlert() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newAlertRepo(testO11y(), mgr)

	userID := uuid.New()
	alert := s.newAlert(userID)

	s.Require().NoError(repo.Insert(ctx, alert))
}

func (s *AlertRepositorySuite) TestCountDelivered() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newAlertRepo(testO11y(), mgr)

	userID := uuid.New()
	competence := mustCompetence(s.T(), "2025-01")
	rootSlug := mustRootSlug(s.T(), "expense.custo_fixo")
	threshold := mustThreshold(s.T(), 80)

	key := entities.ThresholdKey{
		UserID:     userID,
		Competence: competence,
		RootSlug:   rootSlug,
		Threshold:  threshold,
	}

	count, err := repo.CountDelivered(ctx, key)
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), count)

	alert := entities.NewAlert(userID, competence, rootSlug, threshold,
		entities.AlertStatePendingDelivery, time.Now().UTC(), 80000, 100000, time.Now().UTC())
	s.Require().NoError(repo.Insert(ctx, alert))

	count, err = repo.CountDelivered(ctx, key)
	s.Require().NoError(err)
	s.Assert().Equal(int64(1), count)
}

func (s *AlertRepositorySuite) TestListForUser() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newAlertRepo(testO11y(), mgr)

	userID := uuid.New()

	for i := 0; i < 3; i++ {
		alert := s.newAlert(userID)
		s.Require().NoError(repo.Insert(ctx, alert))
	}

	q := input.AlertQuery{Limit: 10}
	alerts, cursor, err := repo.ListForUser(ctx, userID, q)
	s.Require().NoError(err)
	s.Assert().Len(alerts, 3)
	s.Assert().Empty(cursor)
}

func (s *AlertRepositorySuite) TestListForUserCursorPagination() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newAlertRepo(testO11y(), mgr)

	userID := uuid.New()

	for i := 0; i < 5; i++ {
		alert := s.newAlert(userID)
		s.Require().NoError(repo.Insert(ctx, alert))
	}

	q := input.AlertQuery{Limit: 2}
	page1, cursor1, err := repo.ListForUser(ctx, userID, q)
	s.Require().NoError(err)
	s.Assert().Len(page1, 2)
	s.Assert().NotEmpty(cursor1)

	q2 := input.AlertQuery{Limit: 2, Cursor: cursor1}
	page2, cursor2, err := repo.ListForUser(ctx, userID, q2)
	s.Require().NoError(err)
	s.Assert().Len(page2, 2)
	s.Assert().NotEmpty(cursor2)

	q3 := input.AlertQuery{Limit: 2, Cursor: cursor2}
	page3, cursor3, err := repo.ListForUser(ctx, userID, q3)
	s.Require().NoError(err)
	s.Assert().Len(page3, 1)
	s.Assert().Empty(cursor3)
}

func (s *AlertRepositorySuite) TestCountDeliveredExcludesNonVisible() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newAlertRepo(testO11y(), mgr)

	userID := uuid.New()
	competence := mustCompetence(s.T(), "2025-01")
	rootSlug := mustRootSlug(s.T(), "expense.custo_fixo")
	threshold := mustThreshold(s.T(), 80)

	key := entities.ThresholdKey{UserID: userID, Competence: competence, RootSlug: rootSlug, Threshold: threshold}

	suppressed := entities.NewAlert(userID, competence, rootSlug, threshold,
		entities.AlertStateSuppressedStale, time.Now().UTC(), 80000, 100000, time.Now().UTC())
	s.Require().NoError(repo.Insert(ctx, suppressed))

	count, err := repo.CountDelivered(ctx, key)
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), count, "suppressed_stale não deve ser contado")
}

//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type ThresholdStateRepositorySuite struct {
	suite.Suite
}

func TestThresholdStateRepositorySuite(t *testing.T) {
	suite.Run(t, new(ThresholdStateRepositorySuite))
}

func (s *ThresholdStateRepositorySuite) newKey() entities.ThresholdKey {
	return entities.ThresholdKey{
		UserID:     uuid.New(),
		Competence: mustCompetence(s.T(), "2025-01"),
		RootSlug:   mustRootSlug(s.T(), "expense.custo_fixo"),
		Threshold:  mustThreshold(s.T(), 80),
	}
}

func (s *ThresholdStateRepositorySuite) TestInsertNewRow_Transitioned() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newThresholdStateRepo(testO11y(), mgr.DBTX(ctx))

	key := s.newKey()
	committedAt := time.Now().UTC()

	transitioned, err := repo.UpsertIfTransition(ctx, key, true, committedAt)
	s.Require().NoError(err)
	s.Assert().True(transitioned, "nova linha sem estado anterior deve ser transição")
}

func (s *ThresholdStateRepositorySuite) TestFalseToTrue_Transitioned() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newThresholdStateRepo(testO11y(), mgr.DBTX(ctx))

	key := s.newKey()
	t1 := time.Now().UTC()

	trans1, err := repo.UpsertIfTransition(ctx, key, false, t1)
	s.Require().NoError(err)
	s.Assert().True(trans1)

	t2 := t1.Add(time.Second)
	trans2, err := repo.UpsertIfTransition(ctx, key, true, t2)
	s.Require().NoError(err)
	s.Assert().True(trans2, "false→true deve ser transição")
}

func (s *ThresholdStateRepositorySuite) TestTrueToFalse_Transitioned() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newThresholdStateRepo(testO11y(), mgr.DBTX(ctx))

	key := s.newKey()
	t1 := time.Now().UTC()

	_, err := repo.UpsertIfTransition(ctx, key, true, t1)
	s.Require().NoError(err)

	t2 := t1.Add(time.Second)
	transitioned, err := repo.UpsertIfTransition(ctx, key, false, t2)
	s.Require().NoError(err)
	s.Assert().True(transitioned, "true→false deve ser transição")
}

func (s *ThresholdStateRepositorySuite) TestSameState_NotTransitioned() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newThresholdStateRepo(testO11y(), mgr.DBTX(ctx))

	key := s.newKey()
	t1 := time.Now().UTC()

	_, err := repo.UpsertIfTransition(ctx, key, true, t1)
	s.Require().NoError(err)

	t2 := t1.Add(time.Second)
	transitioned, err := repo.UpsertIfTransition(ctx, key, true, t2)
	s.Require().NoError(err)
	s.Assert().False(transitioned, "true→true não deve ser transição")
}

func (s *ThresholdStateRepositorySuite) TestOutOfOrder_Ignored() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newThresholdStateRepo(testO11y(), mgr.DBTX(ctx))

	key := s.newKey()
	t2 := time.Now().UTC()
	t1 := t2.Add(-time.Minute)

	_, err := repo.UpsertIfTransition(ctx, key, true, t2)
	s.Require().NoError(err)

	transitioned, err := repo.UpsertIfTransition(ctx, key, false, t1)
	s.Require().NoError(err)
	s.Assert().False(transitioned, "evento out-of-order não deve transitar")
}

func (s *ThresholdStateRepositorySuite) TestIdempotent_SameTimestamp() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newThresholdStateRepo(testO11y(), mgr.DBTX(ctx))

	key := s.newKey()
	committedAt := time.Now().UTC()

	trans1, err := repo.UpsertIfTransition(ctx, key, true, committedAt)
	s.Require().NoError(err)

	trans2, err := repo.UpsertIfTransition(ctx, key, false, committedAt)
	s.Require().NoError(err)

	s.Assert().True(trans1)
	s.Assert().False(trans2, "mesmo timestamp <= last deve ser ignorado")
}

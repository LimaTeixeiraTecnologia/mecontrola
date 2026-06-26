//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	agentrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
)

type ProcessedEventRepositorySuite struct {
	suite.Suite
	db   *sqlx.DB
	repo interfaces.ProcessedEventRepository
}

func TestProcessedEventRepositorySuite(t *testing.T) {
	suite.Run(t, new(ProcessedEventRepositorySuite))
}

func (s *ProcessedEventRepositorySuite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.repo = agentrepos.NewProcessedEventRepositoryFactory(noop.NewProvider()).ProcessedEventRepository(s.db)
}

func (s *ProcessedEventRepositorySuite) SetupTest() {
	_, err := s.db.ExecContext(context.Background(), `TRUNCATE TABLE mecontrola.agent_processed_events`)
	s.Require().NoError(err)
}

func (s *ProcessedEventRepositorySuite) TestIsProcessed_ReturnsFalseWhenEventDoesNotExist() {
	ctx := context.Background()
	eventID := uuid.New()

	processed, err := s.repo.IsProcessed(ctx, eventID)

	s.Require().NoError(err)
	s.False(processed)
}

func (s *ProcessedEventRepositorySuite) TestIsProcessed_ReturnsTrueWhenEventExists() {
	ctx := context.Background()
	eventID := uuid.New()

	err := s.repo.MarkProcessed(ctx, eventID, "onboarding.completed", uuid.Nil, time.Now().UTC())
	s.Require().NoError(err)

	processed, err := s.repo.IsProcessed(ctx, eventID)

	s.Require().NoError(err)
	s.True(processed)
}

func (s *ProcessedEventRepositorySuite) TestMarkProcessed_ReturnsAlreadyExistsOnDuplicate() {
	ctx := context.Background()
	eventID := uuid.New()

	err := s.repo.MarkProcessed(ctx, eventID, "onboarding.completed", uuid.Nil, time.Now().UTC())
	s.Require().NoError(err)

	err = s.repo.MarkProcessed(ctx, eventID, "onboarding.completed", uuid.Nil, time.Now().UTC())
	s.ErrorIs(err, interfaces.ErrProcessedEventAlreadyExists)
}

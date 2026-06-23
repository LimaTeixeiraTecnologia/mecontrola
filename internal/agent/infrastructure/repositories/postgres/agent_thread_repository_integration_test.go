//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	agentrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
)

type AgentThreadRepositorySuite struct {
	suite.Suite
	db      *sqlx.DB
	factory interfaces.AgentThreadRepositoryFactory
}

func TestAgentThreadRepositorySuite(t *testing.T) {
	suite.Run(t, new(AgentThreadRepositorySuite))
}

func (s *AgentThreadRepositorySuite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.factory = agentrepos.NewThreadRepositoryFactory(noop.NewProvider())
}

func (s *AgentThreadRepositorySuite) SetupTest() {}

func (s *AgentThreadRepositorySuite) repo() interfaces.AgentThreadRepository {
	return s.factory.AgentThreadRepository(s.db)
}

func (s *AgentThreadRepositorySuite) TestUpsertAndGet() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	repo := s.repo()

	thread, err := entities.NewThread(userID, "whatsapp")
	s.Require().NoError(err)

	persisted, err := repo.Upsert(ctx, thread)
	s.Require().NoError(err)
	s.Equal(thread.ID(), persisted.ID())
	s.Equal(userID, persisted.UserID())
	s.Equal("whatsapp", persisted.Channel())

	got, found, err := repo.GetByUserAndChannel(ctx, userID, "whatsapp")
	s.Require().NoError(err)
	s.True(found)
	s.Equal(persisted.ID(), got.ID())
}

func (s *AgentThreadRepositorySuite) TestUpsertIsIdempotentByUserChannel() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	repo := s.repo()

	first, err := entities.NewThread(userID, "whatsapp")
	s.Require().NoError(err)
	persistedFirst, err := repo.Upsert(ctx, first)
	s.Require().NoError(err)

	second, err := entities.NewThread(userID, "whatsapp")
	s.Require().NoError(err)
	persistedSecond, err := repo.Upsert(ctx, second)
	s.Require().NoError(err)

	s.Equal(persistedFirst.ID(), persistedSecond.ID())
}

func (s *AgentThreadRepositorySuite) TestGetNotFound() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)

	_, found, err := s.repo().GetByUserAndChannel(ctx, userID, "telegram")
	s.Require().NoError(err)
	s.False(found)
}

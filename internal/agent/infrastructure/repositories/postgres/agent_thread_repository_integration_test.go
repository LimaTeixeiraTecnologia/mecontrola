//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	agentrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
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

func (s *AgentThreadRepositorySuite) TestGetOrCreateIsAtomicUnderConcurrency() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	gateway := binding.NewThreadGatewayAdapter(s.factory, uow.NewUnitOfWork(s.db))

	const goroutines = 20
	var (
		start    = make(chan struct{})
		wg       sync.WaitGroup
		mu       sync.Mutex
		ids      = make([]uuid.UUID, 0, goroutines)
		failures = make([]error, 0)
	)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			thread, err := gateway.GetOrCreate(ctx, userID, "whatsapp")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures = append(failures, err)
				return
			}
			ids = append(ids, thread.ID())
		}()
	}

	close(start)
	wg.Wait()

	s.Require().Empty(failures, "GetOrCreate deve ser livre de erro sob concorrência")
	s.Require().Len(ids, goroutines)

	first := ids[0]
	for _, got := range ids {
		s.Equal(first, got, "todas as chamadas concorrentes devem retornar o mesmo thread id")
	}

	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM mecontrola.agent_threads WHERE user_id = $1 AND channel = $2`,
		userID, "whatsapp",
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count, "deve existir exatamente uma linha para (user_id, channel)")
}

func (s *AgentThreadRepositorySuite) TestGetNotFound() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)

	_, found, err := s.repo().GetByUserAndChannel(ctx, userID, "telegram")
	s.Require().NoError(err)
	s.False(found)
}

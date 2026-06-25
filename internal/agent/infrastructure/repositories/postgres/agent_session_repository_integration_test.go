//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	agentrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type AgentSessionRepositorySuite struct {
	suite.Suite
	db      *sqlx.DB
	factory interfaces.AgentSessionRepositoryFactory
}

func TestAgentSessionRepositorySuite(t *testing.T) {
	suite.Run(t, new(AgentSessionRepositorySuite))
}

func (s *AgentSessionRepositorySuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.factory = agentrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *AgentSessionRepositorySuite) newRepo() interfaces.AgentSessionRepository {
	return s.factory.AgentSessionRepository(s.db)
}

func (s *AgentSessionRepositorySuite) insertTestUser(ctx context.Context) uuid.UUID {
	userID := uuid.New()
	number := fmt.Sprintf("+5511%09d", time.Now().UnixNano()%1000000000)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, number,
	)
	s.Require().NoError(err)
	return userID
}

func (s *AgentSessionRepositorySuite) makeRecord(userID uuid.UUID, channel string, expiresAt time.Time) interfaces.AgentSessionRecord {
	now := time.Now().UTC()
	return interfaces.AgentSessionRecord{
		ID:            uuid.New(),
		UserID:        userID,
		Channel:       channel,
		PendingAction: []byte(`{"action":"awaiting_amount"}`),
		RecentTurns:   []byte(`[{"role":"user","text":"oi"}]`),
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     expiresAt,
	}
}

func (s *AgentSessionRepositorySuite) TestCreateThenGet() {
	ctx := context.Background()
	repo := s.newRepo()
	userID := s.insertTestUser(ctx)

	record := s.makeRecord(userID, "whatsapp", time.Now().UTC().Add(time.Hour))
	s.Require().NoError(repo.Create(ctx, record))

	got, err := repo.GetByUserAndChannel(ctx, userID, "whatsapp")
	s.Require().NoError(err)
	s.Assert().Equal(record.ID, got.ID)
	s.Assert().Equal(record.UserID, got.UserID)
	s.Assert().Equal("whatsapp", got.Channel)
	s.Assert().JSONEq(string(record.PendingAction), string(got.PendingAction))
	s.Assert().JSONEq(string(record.RecentTurns), string(got.RecentTurns))
}

func (s *AgentSessionRepositorySuite) TestGetNotFound() {
	ctx := context.Background()
	repo := s.newRepo()
	userID := s.insertTestUser(ctx)

	_, err := repo.GetByUserAndChannel(ctx, userID, "whatsapp")
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrAgentSessionNotFound))
}

func (s *AgentSessionRepositorySuite) TestCreateConflictSameUserChannel() {
	ctx := context.Background()
	repo := s.newRepo()
	userID := s.insertTestUser(ctx)

	first := s.makeRecord(userID, "whatsapp", time.Now().UTC().Add(time.Hour))
	s.Require().NoError(repo.Create(ctx, first))

	second := s.makeRecord(userID, "whatsapp", time.Now().UTC().Add(time.Hour))
	err := repo.Create(ctx, second)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrAgentSessionConflict))
}

func (s *AgentSessionRepositorySuite) TestUpdate() {
	ctx := context.Background()
	repo := s.newRepo()
	userID := s.insertTestUser(ctx)

	record := s.makeRecord(userID, "whatsapp", time.Now().UTC().Add(time.Hour))
	s.Require().NoError(repo.Create(ctx, record))

	record.PendingAction = []byte(`{"action":"awaiting_category"}`)
	record.RecentTurns = []byte(`[{"role":"assistant","text":"qual categoria?"}]`)
	record.UpdatedAt = time.Now().UTC()
	record.ExpiresAt = time.Now().UTC().Add(2 * time.Hour)
	s.Require().NoError(repo.Update(ctx, record))

	got, err := repo.GetByUserAndChannel(ctx, userID, "whatsapp")
	s.Require().NoError(err)
	s.Assert().JSONEq(`{"action":"awaiting_category"}`, string(got.PendingAction))
	s.Assert().JSONEq(`[{"role":"assistant","text":"qual categoria?"}]`, string(got.RecentTurns))
}

func (s *AgentSessionRepositorySuite) TestUpdateMissingReturnsNotFound() {
	ctx := context.Background()
	repo := s.newRepo()
	userID := s.insertTestUser(ctx)

	record := s.makeRecord(userID, "whatsapp", time.Now().UTC().Add(time.Hour))
	err := repo.Update(ctx, record)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrAgentSessionNotFound))
}

func (s *AgentSessionRepositorySuite) TestUserIsolation() {
	ctx := context.Background()
	repo := s.newRepo()
	userA := s.insertTestUser(ctx)
	userB := s.insertTestUser(ctx)

	s.Require().NoError(repo.Create(ctx, s.makeRecord(userA, "whatsapp", time.Now().UTC().Add(time.Hour))))

	_, err := repo.GetByUserAndChannel(ctx, userB, "whatsapp")
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrAgentSessionNotFound))
}

func (s *AgentSessionRepositorySuite) TestGetSkipsExpired() {
	ctx := context.Background()
	repo := s.newRepo()
	userID := s.insertTestUser(ctx)

	expired := s.makeRecord(userID, "whatsapp", time.Now().UTC().Add(-time.Minute))
	s.Require().NoError(repo.Create(ctx, expired))

	_, err := repo.GetByUserAndChannel(ctx, userID, "whatsapp")
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrAgentSessionNotFound))
}

func (s *AgentSessionRepositorySuite) TestUpsertInsertsWhenAbsent() {
	ctx := context.Background()
	repo := s.newRepo()
	userID := s.insertTestUser(ctx)

	record := s.makeRecord(userID, "whatsapp", time.Now().UTC().Add(time.Hour))
	record.PendingAction = []byte(`{"kind":"budget_config","total_cents":500000,"allocations":{},"competence":"2026-06"}`)
	s.Require().NoError(repo.Upsert(ctx, record))

	got, err := repo.GetByUserAndChannel(ctx, userID, "whatsapp")
	s.Require().NoError(err)
	s.Assert().JSONEq(string(record.PendingAction), string(got.PendingAction))
}

func (s *AgentSessionRepositorySuite) TestUpsertUpdatesOnConflict() {
	ctx := context.Background()
	repo := s.newRepo()
	userID := s.insertTestUser(ctx)

	first := s.makeRecord(userID, "whatsapp", time.Now().UTC().Add(time.Hour))
	s.Require().NoError(repo.Create(ctx, first))

	second := s.makeRecord(userID, "whatsapp", time.Now().UTC().Add(2*time.Hour))
	second.PendingAction = []byte(`{"kind":"budget_config","total_cents":800000,"allocations":{"expense.metas":2000},"competence":"2026-06"}`)
	s.Require().NoError(repo.Upsert(ctx, second))

	got, err := repo.GetByUserAndChannel(ctx, userID, "whatsapp")
	s.Require().NoError(err)
	s.Assert().Equal(first.ID, got.ID)
	s.Assert().JSONEq(string(second.PendingAction), string(got.PendingAction))
}

func (s *AgentSessionRepositorySuite) TestDeleteExpired() {
	ctx := context.Background()
	repo := s.newRepo()
	userActive := s.insertTestUser(ctx)
	userExpired := s.insertTestUser(ctx)

	s.Require().NoError(repo.Create(ctx, s.makeRecord(userActive, "whatsapp", time.Now().UTC().Add(time.Hour))))
	s.Require().NoError(repo.Create(ctx, s.makeRecord(userExpired, "whatsapp", time.Now().UTC().Add(-time.Hour))))

	deleted, err := repo.DeleteExpired(ctx, time.Now().UTC())
	s.Require().NoError(err)
	s.Assert().GreaterOrEqual(deleted, int64(1))

	_, err = repo.GetByUserAndChannel(ctx, userActive, "whatsapp")
	s.Require().NoError(err)
}

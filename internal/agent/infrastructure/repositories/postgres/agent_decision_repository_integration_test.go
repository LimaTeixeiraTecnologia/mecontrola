//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	agentrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
)

type AgentDecisionRepositorySuite struct {
	suite.Suite
	db      *sqlx.DB
	factory interfaces.AgentDecisionRepositoryFactory
}

func TestAgentDecisionRepositorySuite(t *testing.T) {
	suite.Run(t, new(AgentDecisionRepositorySuite))
}

func (s *AgentDecisionRepositorySuite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.factory = agentrepos.NewDecisionRepositoryFactory(noop.NewProvider())
}

func (s *AgentDecisionRepositorySuite) SetupTest() {}

func (s *AgentDecisionRepositorySuite) repo() interfaces.AgentDecisionRepository {
	return s.factory.AgentDecisionRepository(s.db)
}

func (s *AgentDecisionRepositorySuite) pending(userID uuid.UUID, messageID string, stepIndex int) entities.AgentDecision {
	decision, err := entities.NewPendingDecision(entities.AgentDecisionParams{
		ID:               uuid.New(),
		UserID:           userID,
		Channel:          "whatsapp",
		MessageID:        messageID,
		IntentKind:       "record_expense",
		PromptSHA256:     "a3f1e9b2c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1",
		LLMModel:         "google/gemini-2.5-flash-lite",
		RedactedResponse: json.RawMessage(`{"redacted":"Lancei R$ 58,00 no iFood"}`),
		TraceID:          "trace-123",
		DecidedAction:    "record_expense",
		CreatedAt:        time.Now().UTC(),
		StepIndex:        stepIndex,
	})
	s.Require().NoError(err)
	return decision
}

func (s *AgentDecisionRepositorySuite) TestInsertFindAndSettle() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	repo := s.repo()
	decision := s.pending(userID, "wamid.find-1", 0)

	s.Require().NoError(repo.Insert(ctx, decision))

	snapshot, found, err := repo.FindByMessage(ctx, userID, "whatsapp", "wamid.find-1", 0)
	s.Require().NoError(err)
	s.True(found)
	s.Equal("pending", snapshot.Status)
	s.Contains(string(snapshot.RedactedResponse), "Lancei R$ 58,00")

	settled, err := decision.Execute(uuid.New(), time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateSettlement(ctx, settled))

	after, found, err := repo.FindByMessage(ctx, userID, "whatsapp", "wamid.find-1", 0)
	s.Require().NoError(err)
	s.True(found)
	s.Equal("executed", after.Status)
}

func (s *AgentDecisionRepositorySuite) TestFindByMessageNotFound() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)

	_, found, err := s.repo().FindByMessage(ctx, userID, "whatsapp", "wamid.absent", 0)
	s.Require().NoError(err)
	s.False(found)
}

func (s *AgentDecisionRepositorySuite) TestDuplicateMessageReturnsConflict() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	repo := s.repo()

	s.Require().NoError(repo.Insert(ctx, s.pending(userID, "wamid.dup", 0)))

	err := repo.Insert(ctx, s.pending(userID, "wamid.dup", 0))
	s.Require().Error(err)
	s.ErrorIs(err, interfaces.ErrAgentDecisionConflict)
}

func (s *AgentDecisionRepositorySuite) TestSameMessageDifferentStepIndexCoexist() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	repo := s.repo()

	dec0 := s.pending(userID, "wamid.plan", 0)
	s.Require().NoError(repo.Insert(ctx, dec0))
	s.Require().NoError(repo.Insert(ctx, s.pending(userID, "wamid.plan", 1)))

	step0, found0, err := repo.FindByMessage(ctx, userID, "whatsapp", "wamid.plan", 0)
	s.Require().NoError(err)
	s.True(found0)
	s.Equal("pending", step0.Status)

	step1, found1, err := repo.FindByMessage(ctx, userID, "whatsapp", "wamid.plan", 1)
	s.Require().NoError(err)
	s.True(found1)
	s.Equal("pending", step1.Status)

	settled, err := dec0.Execute(uuid.New(), time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateSettlement(ctx, settled))

	after0, _, err := repo.FindByMessage(ctx, userID, "whatsapp", "wamid.plan", 0)
	s.Require().NoError(err)
	s.Equal("executed", after0.Status)

	after1, _, err := repo.FindByMessage(ctx, userID, "whatsapp", "wamid.plan", 1)
	s.Require().NoError(err)
	s.Equal("pending", after1.Status)
}

func (s *AgentDecisionRepositorySuite) TestSameMessageSameStepIndexConflicts() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	repo := s.repo()

	s.Require().NoError(repo.Insert(ctx, s.pending(userID, "wamid.plan-dup", 2)))

	err := repo.Insert(ctx, s.pending(userID, "wamid.plan-dup", 2))
	s.Require().Error(err)
	s.ErrorIs(err, interfaces.ErrAgentDecisionConflict)
}

func (s *AgentDecisionRepositorySuite) TestUpdateSettlementNotFound() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	decision := s.pending(userID, "wamid.never-inserted", 0)

	settled, err := decision.Execute(uuid.New(), time.Now().UTC())
	s.Require().NoError(err)

	err = s.repo().UpdateSettlement(ctx, settled)
	s.ErrorIs(err, interfaces.ErrAgentDecisionNotFound)
}

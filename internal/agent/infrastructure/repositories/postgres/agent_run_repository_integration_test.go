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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	agentrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
)

type AgentRunRepositorySuite struct {
	suite.Suite
	db            *sqlx.DB
	threadFactory interfaces.AgentThreadRepositoryFactory
	runFactory    interfaces.AgentRunRepositoryFactory
}

func TestAgentRunRepositorySuite(t *testing.T) {
	suite.Run(t, new(AgentRunRepositorySuite))
}

func (s *AgentRunRepositorySuite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.threadFactory = agentrepos.NewThreadRepositoryFactory(noop.NewProvider())
	s.runFactory = agentrepos.NewRunRepositoryFactory(noop.NewProvider())
}

func (s *AgentRunRepositorySuite) SetupTest() {}

func (s *AgentRunRepositorySuite) runRepo() interfaces.AgentRunRepository {
	return s.runFactory.AgentRunRepository(s.db)
}

func (s *AgentRunRepositorySuite) seedThread(userID uuid.UUID) entities.Thread {
	thread, err := entities.NewThread(userID, "whatsapp")
	s.Require().NoError(err)
	persisted, err := s.threadFactory.AgentThreadRepository(s.db).Upsert(context.Background(), thread)
	s.Require().NoError(err)
	return persisted
}

func (s *AgentRunRepositorySuite) seedDecision(userID uuid.UUID, messageID string) uuid.UUID {
	decisionID := uuid.New()
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO mecontrola.agent_decisions
		        (id, user_id, channel, message_id, intent_kind, prompt_sha256, llm_model,
		         decided_action, status, created_at)
		 VALUES ($1, $2, 'whatsapp', $3, 'create_card',
		         'a3f1e9b2c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1',
		         'google/gemini-2.5-flash-lite', 'create_card', 'pending', now())`,
		decisionID, userID, messageID,
	)
	s.Require().NoError(err)
	return decisionID
}

func (s *AgentRunRepositorySuite) startRun(threadID, userID uuid.UUID, decisionID uuid.UUID) entities.Run {
	run, err := entities.StartRun(entities.StartRunParams{
		ThreadID:   threadID,
		UserID:     userID,
		Channel:    "whatsapp",
		MessageID:  "wamid.run-1",
		AgentID:    "daily",
		Workflow:   "cards",
		ToolName:   "createCardTool",
		IntentKind: "create_card",
		DecisionID: decisionID,
	})
	s.Require().NoError(err)
	return run
}

func (s *AgentRunRepositorySuite) TestInsertAndFinishSucceeded() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	thread := s.seedThread(userID)
	decisionID := s.seedDecision(userID, "wamid.run-1")
	repo := s.runRepo()

	run := s.startRun(thread.ID(), userID, decisionID)
	s.Require().NoError(repo.Insert(ctx, run))

	time.Sleep(2 * time.Millisecond)
	finished := run.Finish("routed", true, "")
	s.Require().NoError(repo.UpdateOnFinish(ctx, finished))

	var (
		status   string
		outcome  string
		duration int64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT status, outcome, duration_ms FROM mecontrola.agent_runs WHERE id = $1`, run.ID(),
	).Scan(&status, &outcome, &duration)
	s.Require().NoError(err)
	s.Equal("succeeded", status)
	s.Equal("routed", outcome)
	s.GreaterOrEqual(duration, int64(0))
}

func (s *AgentRunRepositorySuite) TestInsertAndFinishFailed() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	thread := s.seedThread(userID)
	repo := s.runRepo()

	run := s.startRun(thread.ID(), userID, uuid.Nil)
	s.Require().NoError(repo.Insert(ctx, run))

	finished := run.Finish("usecase_error", false, "boom")
	s.Require().NoError(repo.UpdateOnFinish(ctx, finished))

	var (
		status  string
		errText string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT status, error FROM mecontrola.agent_runs WHERE id = $1`, run.ID(),
	).Scan(&status, &errText)
	s.Require().NoError(err)
	s.Equal("failed", status)
	s.Equal("boom", errText)
}

func (s *AgentRunRepositorySuite) TestUpdateOnFinishNotFound() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	thread := s.seedThread(userID)

	run := s.startRun(thread.ID(), userID, uuid.Nil)
	finished := run.Finish("routed", true, "")

	err := s.runRepo().UpdateOnFinish(ctx, finished)
	s.ErrorIs(err, interfaces.ErrAgentRunNotFound)
}

func (s *AgentRunRepositorySuite) TestDecisionSetNullOnDecisionDelete() {
	ctx := context.Background()
	userID := insertTestUser(s.T(), s.db)
	thread := s.seedThread(userID)
	decisionID := s.seedDecision(userID, "wamid.run-cascade")
	repo := s.runRepo()

	run := s.startRun(thread.ID(), userID, decisionID)
	s.Require().NoError(repo.Insert(ctx, run))

	_, err := s.db.ExecContext(ctx, `DELETE FROM mecontrola.agent_decisions WHERE id = $1`, decisionID)
	s.Require().NoError(err)

	var stored uuid.NullUUID
	err = s.db.QueryRowContext(ctx,
		`SELECT decision_id FROM mecontrola.agent_runs WHERE id = $1`, run.ID(),
	).Scan(&stored)
	s.Require().NoError(err)
	s.False(stored.Valid)
}

//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type RunStoreIntegrationSuite struct {
	suite.Suite
	ctx   context.Context
	db    *sqlx.DB
	store agent.RunStore
}

func TestRunStoreIntegrationSuite(t *testing.T) {
	suite.Run(t, new(RunStoreIntegrationSuite))
}

func (s *RunStoreIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
	s.store = agentpostgres.NewRunStore(s.db)
}

func (s *RunStoreIntegrationSuite) insertThread(resourceID, threadID string) uuid.UUID {
	id := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_threads (id, resource_id, thread_id, title)
		VALUES ($1, $2, $3, $4)`,
		id, resourceID, threadID, "test thread",
	)
	s.Require().NoError(err)
	return id
}

func (s *RunStoreIntegrationSuite) TestRunAuditRoundTrip() {
	resourceID := "res-run-" + uuid.NewString()
	threadID := "thr-run"
	threadPK := s.insertThread(resourceID, threadID)

	runID := uuid.New()
	started := time.Now().UTC()
	running := agent.Run{
		ID:               runID,
		PlatformThreadID: threadPK,
		ResourceID:       resourceID,
		ThreadID:         threadID,
		AgentID:          "weather-agent",
		Workflow:         "weather",
		CorrelationKey:   "corr-" + runID.String(),
		Status:           agent.RunStatusRunning,
		StartedAt:        started,
	}
	s.Require().NoError(s.store.Insert(s.ctx, running))

	loadedRunning, err := s.store.Load(s.ctx, runID)
	s.Require().NoError(err)
	s.Equal(agent.RunStatusRunning, loadedRunning.Status)
	s.Nil(loadedRunning.EndedAt)

	ended := started.Add(250 * time.Millisecond)
	succeeded := running
	succeeded.Status = agent.RunStatusSucceeded
	succeeded.Outcome = agent.ToolOutcomeRouted
	succeeded.EndedAt = &ended
	succeeded.DurationMs = 250
	s.Require().NoError(s.store.Update(s.ctx, succeeded))

	loaded, err := s.store.Load(s.ctx, runID)
	s.Require().NoError(err)
	s.Equal(agent.RunStatusSucceeded, loaded.Status)
	s.Equal(agent.ToolOutcomeRouted, loaded.Outcome)
	s.Equal(int64(250), loaded.DurationMs)
	s.Require().NotNil(loaded.EndedAt)
	s.WithinDuration(ended, *loaded.EndedAt, time.Second)
	s.Equal("weather-agent", loaded.AgentID)
	s.Equal("weather", loaded.Workflow)
}

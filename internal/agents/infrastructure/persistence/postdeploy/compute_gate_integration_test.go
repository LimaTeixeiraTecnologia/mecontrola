//go:build integration

package postdeploy_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	postdeployapp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/postdeploy"
	postdeploypostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence/postdeploy"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type ComputeGateIntegrationSuite struct {
	suite.Suite
	ctx    context.Context
	db     *sqlx.DB
	reader postdeployapp.AggregateReader
}

func TestComputeGateIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ComputeGateIntegrationSuite))
}

func (s *ComputeGateIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
	s.reader = postdeploypostgres.NewAggregateReader(s.db)
}

func (s *ComputeGateIntegrationSuite) insertThread(resourceID, threadID string) uuid.UUID {
	id := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_threads (id, resource_id, thread_id, title)
		VALUES ($1, $2, $3, $4)`,
		id, resourceID, threadID, "gate compute thread",
	)
	s.Require().NoError(err)
	return id
}

func (s *ComputeGateIntegrationSuite) insertRun(agentID, resourceID, threadID string, platformThreadID uuid.UUID, status agent.RunStatus, outcome agent.ToolOutcome, startedAt time.Time) uuid.UUID {
	runID := uuid.New()
	outcomeStr := ""
	if outcome.IsValid() {
		outcomeStr = outcome.String()
	}
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_runs
			(id, platform_thread_id, resource_id, thread_id, agent_id, workflow, correlation_key, status, outcome, error, started_at, ended_at, duration_ms)
		VALUES ($1,$2,$3,$4,$5,'',$6,$7,$8,'',$9,$9,10)`,
		runID, platformThreadID, resourceID, threadID, agentID, "corr-"+runID.String(),
		status.String(), outcomeStr, startedAt,
	)
	s.Require().NoError(err)
	return runID
}

func (s *ComputeGateIntegrationSuite) insertScorerResult(runID uuid.UUID, scorerID string, score float64) {
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_scorer_results
			(id, run_id, scorer_id, kind, score, reason, metadata, sampled, created_at)
		VALUES ($1,$2,$3,'code_based',$4,'',$5,true,now())`,
		uuid.New(), runID, scorerID, score, []byte(`{}`),
	)
	s.Require().NoError(err)
}

func (s *ComputeGateIntegrationSuite) TestComputeGatePromotesWhenSampleSufficientAndAboveMargin() {
	agentID := "gate-compute-promote-" + uuid.NewString()
	resourceID := "res-" + uuid.NewString()
	threadID := "thr-compute"
	platformThreadID := s.insertThread(resourceID, threadID)
	base := time.Now().UTC()

	const total = 120
	for i := 0; i < total; i++ {
		startedAt := base.Add(time.Duration(i) * time.Second)
		runID := s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, startedAt)
		s.insertScorerResult(runID, "completeness", 0.3)
		s.insertScorerResult(runID, "categorization", 0.7)
		s.insertScorerResult(runID, postdeployapp.ScorerIDNoDuplicateWrite, 1.0)
	}

	since := base.Add(-time.Minute)
	verdict, err := postdeployapp.ComputeGate(s.ctx, s.reader, agentID, since, postdeployapp.PrometheusCounters{})
	s.Require().NoError(err)

	s.True(verdict.SampleSufficient)
	s.True(verdict.FailureRatePassed)
	s.True(verdict.NoRegressionOperational)
	s.True(verdict.Promote, "verdict.Reasons=%v", verdict.Reasons)
}

func (s *ComputeGateIntegrationSuite) TestComputeGateBlocksOnInsufficientSample() {
	agentID := "gate-compute-insufficient-" + uuid.NewString()
	resourceID := "res-" + uuid.NewString()
	threadID := "thr-compute-insufficient"
	platformThreadID := s.insertThread(resourceID, threadID)
	base := time.Now().UTC()

	runID := s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, base)
	s.insertScorerResult(runID, "completeness", 0.9)
	s.insertScorerResult(runID, "categorization", 0.9)

	since := base.Add(-time.Minute)
	verdict, err := postdeployapp.ComputeGate(s.ctx, s.reader, agentID, since, postdeployapp.PrometheusCounters{})
	s.Require().NoError(err)

	s.False(verdict.SampleSufficient)
	s.False(verdict.Promote)
}

func (s *ComputeGateIntegrationSuite) TestComputeGateBlocksOnTruncationRegression() {
	agentID := "gate-compute-truncated-" + uuid.NewString()
	resourceID := "res-" + uuid.NewString()
	threadID := "thr-compute-truncated"
	platformThreadID := s.insertThread(resourceID, threadID)
	base := time.Now().UTC()

	const total = 120
	for i := 0; i < total; i++ {
		startedAt := base.Add(time.Duration(i) * time.Second)
		outcome := agent.ToolOutcomeRouted
		if i == 0 {
			outcome = agent.ToolOutcomeTruncated
		}
		runID := s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, outcome, startedAt)
		s.insertScorerResult(runID, "completeness", 0.9)
		s.insertScorerResult(runID, "categorization", 0.9)
	}

	since := base.Add(-time.Minute)
	verdict, err := postdeployapp.ComputeGate(s.ctx, s.reader, agentID, since, postdeployapp.PrometheusCounters{})
	s.Require().NoError(err)

	s.True(verdict.SampleSufficient)
	s.False(verdict.NoRegressionOperational)
	s.False(verdict.Promote)
}

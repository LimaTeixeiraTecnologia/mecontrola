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

type AggregateReaderIntegrationSuite struct {
	suite.Suite
	ctx    context.Context
	db     *sqlx.DB
	reader postdeployapp.AggregateReader
}

func TestAggregateReaderIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AggregateReaderIntegrationSuite))
}

func (s *AggregateReaderIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
	s.reader = postdeploypostgres.NewAggregateReader(s.db)
}

func (s *AggregateReaderIntegrationSuite) insertThread(resourceID, threadID string) uuid.UUID {
	id := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_threads (id, resource_id, thread_id, title)
		VALUES ($1, $2, $3, $4)`,
		id, resourceID, threadID, "gate test thread",
	)
	s.Require().NoError(err)
	return id
}

func (s *AggregateReaderIntegrationSuite) insertRun(agentID, resourceID, threadID string, platformThreadID uuid.UUID, status agent.RunStatus, outcome agent.ToolOutcome, startedAt time.Time) uuid.UUID {
	runID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_runs
			(id, platform_thread_id, resource_id, thread_id, agent_id, workflow, correlation_key, status, outcome, error, started_at, ended_at, duration_ms)
		VALUES ($1,$2,$3,$4,$5,'',$6,$7,$8,'',$9,$9,10)`,
		runID, platformThreadID, resourceID, threadID, agentID, "corr-"+runID.String(),
		status.String(), outcomeText(outcome), startedAt,
	)
	s.Require().NoError(err)
	return runID
}

func outcomeText(o agent.ToolOutcome) string {
	if !o.IsValid() {
		return ""
	}
	return o.String()
}

func (s *AggregateReaderIntegrationSuite) insertScorerResult(runID uuid.UUID, scorerID string, score float64) {
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_scorer_results
			(id, run_id, scorer_id, kind, score, reason, metadata, sampled, created_at)
		VALUES ($1,$2,$3,'code_based',$4,'',$5,true,now())`,
		uuid.New(), runID, scorerID, score, []byte(`{}`),
	)
	s.Require().NoError(err)
}

func (s *AggregateReaderIntegrationSuite) TestRunAggregateComputesTotalsAndWindow() {
	agentID := "gate-agent-" + uuid.NewString()
	resourceID := "res-gate-" + uuid.NewString()
	threadID := "thr-gate"
	platformThreadID := s.insertThread(resourceID, threadID)

	since := time.Now().UTC().Add(-1 * time.Hour)
	base := time.Now().UTC()

	s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, base)
	s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeClarify, base.Add(time.Minute))
	s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusFailed, agent.ToolOutcomeUsecaseError, base.Add(2*time.Minute))
	s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeTruncated, base.Add(3*time.Minute))

	got, err := s.reader.RunAggregate(s.ctx, agentID, since)
	s.Require().NoError(err)

	s.Equal(agentID, got.AgentID)
	s.Equal(4, got.TotalRuns)
	s.Equal(3, got.SucceededRuns)
	s.Equal(1, got.FailedRuns)
	s.Equal(1, got.TruncatedRuns)
	s.Equal(3, got.ExpectedToolRuns)
	s.WithinDuration(base, got.WindowStart, time.Second)
	s.WithinDuration(base.Add(3*time.Minute), got.WindowEnd, time.Second)
}

func (s *AggregateReaderIntegrationSuite) TestRunAggregateEmptyWindow() {
	agentID := "gate-agent-empty-" + uuid.NewString()
	got, err := s.reader.RunAggregate(s.ctx, agentID, time.Now().UTC())
	s.Require().NoError(err)
	s.Equal(0, got.TotalRuns)
	s.True(got.WindowStart.IsZero())
	s.True(got.WindowEnd.IsZero())
}

func (s *AggregateReaderIntegrationSuite) TestExpectedToolHitsExcludesClarifyAndReplay() {
	agentID := "gate-agent-hits-" + uuid.NewString()
	resourceID := "res-gate-hits-" + uuid.NewString()
	threadID := "thr-gate-hits"
	platformThreadID := s.insertThread(resourceID, threadID)
	since := time.Now().UTC().Add(-1 * time.Hour)
	base := time.Now().UTC()

	s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, base)
	s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeReconciled, base.Add(time.Minute))
	s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeClarify, base.Add(2*time.Minute))
	s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeReplay, base.Add(3*time.Minute))
	s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusFailed, agent.ToolOutcomeRouted, base.Add(4*time.Minute))

	hits, err := s.reader.ExpectedToolHits(s.ctx, agentID, since)
	s.Require().NoError(err)
	s.Equal(2, hits)
}

func (s *AggregateReaderIntegrationSuite) TestScorerAggregatesGroupsByScorerID() {
	agentID := "gate-agent-scorers-" + uuid.NewString()
	resourceID := "res-gate-scorers-" + uuid.NewString()
	threadID := "thr-gate-scorers"
	platformThreadID := s.insertThread(resourceID, threadID)
	since := time.Now().UTC().Add(-1 * time.Hour)
	base := time.Now().UTC()

	run1 := s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, base)
	run2 := s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, base.Add(time.Minute))

	s.insertScorerResult(run1, "completeness", 0.2)
	s.insertScorerResult(run2, "completeness", 0.4)
	s.insertScorerResult(run1, "categorization", 0.6)

	got, err := s.reader.ScorerAggregates(s.ctx, agentID, since)
	s.Require().NoError(err)

	s.Require().Contains(got, "completeness")
	s.Equal(2, got["completeness"].SampleN)
	s.InDelta(0.3, got["completeness"].MeanScore, 0.0001)

	s.Require().Contains(got, "categorization")
	s.Equal(1, got["categorization"].SampleN)
	s.InDelta(0.6, got["categorization"].MeanScore, 0.0001)
}

func (s *AggregateReaderIntegrationSuite) TestDuplicateWriteViolationsCountsScoreBelowOne() {
	agentID := "gate-agent-dup-" + uuid.NewString()
	resourceID := "res-gate-dup-" + uuid.NewString()
	threadID := "thr-gate-dup"
	platformThreadID := s.insertThread(resourceID, threadID)
	since := time.Now().UTC().Add(-1 * time.Hour)
	base := time.Now().UTC()

	run1 := s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, base)
	run2 := s.insertRun(agentID, resourceID, threadID, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, base.Add(time.Minute))

	s.insertScorerResult(run1, postdeployapp.ScorerIDNoDuplicateWrite, 1.0)
	s.insertScorerResult(run2, postdeployapp.ScorerIDNoDuplicateWrite, 0.0)

	violations, err := s.reader.DuplicateWriteViolations(s.ctx, agentID, since)
	s.Require().NoError(err)
	s.Equal(int64(1), violations)
}

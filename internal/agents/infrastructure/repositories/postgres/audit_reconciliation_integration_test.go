//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/reconciliation"
	agentspostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type AuditReconciliationIntegrationSuite struct {
	suite.Suite
	ctx    context.Context
	db     *sqlx.DB
	uc     *reconciliation.ReconcileRunConsistency
	reader reconciliation.RunConsistencyReader
}

func TestAuditReconciliationIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AuditReconciliationIntegrationSuite))
}

func (s *AuditReconciliationIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
	s.reader = agentspostgres.NewAuditReconciliationReader(s.db)
	s.uc = reconciliation.NewReconcileRunConsistency(s.reader, fake.NewProvider())
}

func (s *AuditReconciliationIntegrationSuite) insertThread(resourceID, threadID string) uuid.UUID {
	id := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_threads (id, resource_id, thread_id, title)
		VALUES ($1, $2, $3, $4)`,
		id, resourceID, threadID, "reconciliation test thread",
	)
	s.Require().NoError(err)
	return id
}

func (s *AuditReconciliationIntegrationSuite) insertRun(agentID, resourceID, threadID, correlationKey string, platformThreadID uuid.UUID, status agent.RunStatus, outcome agent.ToolOutcome, startedAt time.Time) uuid.UUID {
	runID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_runs
			(id, platform_thread_id, resource_id, thread_id, agent_id, workflow, correlation_key, status, outcome, error, started_at, ended_at, duration_ms)
		VALUES ($1,$2,$3,$4,$5,'pending-entry',$6,$7,$8,'',$9,$9,10)`,
		runID, platformThreadID, resourceID, threadID, agentID, correlationKey,
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

func (s *AuditReconciliationIntegrationSuite) insertWorkflowRun(workflowID, correlationKey string, status workflow.RunStatus, stateStatus string) {
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.workflow_runs
			(id, workflow, correlation_key, status, state, max_attempts)
		VALUES ($1,$2,$3,$4,$5,3)`,
		uuid.New(), workflowID, correlationKey, status.String(),
		[]byte(`{"status":"`+stateStatus+`"}`),
	)
	s.Require().NoError(err)
}

func (s *AuditReconciliationIntegrationSuite) insertLedger(wamid string, resourceID uuid.UUID) {
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.agents_write_ledger
			(id, user_id, wamid, item_seq, operation, resource_id, resource_kind, created_at)
		VALUES ($1,$2,$3,0,'create_transaction',$4,'transaction',now())`,
		uuid.New(), uuid.New(), wamid, resourceID,
	)
	s.Require().NoError(err)
}

func (s *AuditReconciliationIntegrationSuite) insertScorerResult(runID uuid.UUID, scorerID string, score float64) {
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_scorer_results
			(id, run_id, scorer_id, kind, score, reason, metadata, sampled, created_at)
		VALUES ($1,$2,$3,'code_based',$4,'',$5,true,now())`,
		uuid.New(), runID, scorerID, score, []byte(`{}`),
	)
	s.Require().NoError(err)
}

func (s *AuditReconciliationIntegrationSuite) TestHealthyFlowHasZeroViolations() {
	agentID := "recon-ok-" + uuid.NewString()
	resourceID := "res-ok-" + uuid.NewString()
	threadID := "thr-ok"
	wamid := "wamid-ok-" + uuid.NewString()
	platformThreadID := s.insertThread(resourceID, threadID)
	since := time.Now().UTC().Add(-1 * time.Hour)

	resourceUUID := uuid.New()
	s.insertLedger(wamid, resourceUUID)
	runID := s.insertRun(agentID, resourceID, threadID, wamid, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, time.Now().UTC())
	s.insertWorkflowRun("pending-entry", wamid, workflow.RunStatusSucceeded, "succeeded")
	s.insertScorerResult(runID, "write_persistence_accuracy", 1.0)

	violations, err := s.uc.Execute(s.ctx, agentID, since)
	s.Require().NoError(err)
	s.Empty(violations)
}

func (s *AuditReconciliationIntegrationSuite) TestEmptyCorrelationKeyRejectedByConstraint() {
	resourceID := "res-empty-" + uuid.NewString()
	threadID := "thr-empty"
	platformThreadID := s.insertThread(resourceID, threadID)

	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_runs
			(id, platform_thread_id, resource_id, thread_id, agent_id, workflow, correlation_key, status, outcome, error, started_at, ended_at, duration_ms)
		VALUES ($1,$2,$3,$4,$5,'pending-entry','','succeeded','replay','',now(),now(),10)`,
		uuid.New(), platformThreadID, resourceID, threadID, "recon-empty-"+uuid.NewString(),
	)
	s.Require().Error(err)
	s.Contains(err.Error(), "platform_runs_correlation_len_chk")
}

func (s *AuditReconciliationIntegrationSuite) TestFailedRunWithPreexistingWriteDetected() {
	agentID := "recon-orphan-" + uuid.NewString()
	resourceID := "res-orphan-" + uuid.NewString()
	threadID := "thr-orphan"
	wamid := "wamid-orphan-" + uuid.NewString()
	platformThreadID := s.insertThread(resourceID, threadID)
	since := time.Now().UTC().Add(-1 * time.Hour)

	s.insertLedger(wamid, uuid.New())
	s.insertRun(agentID, resourceID, threadID, wamid, platformThreadID, agent.RunStatusFailed, agent.ToolOutcomeUsecaseError, time.Now().UTC())
	s.insertWorkflowRun("pending-entry", wamid, workflow.RunStatusFailed, "failed")

	violations, err := s.uc.Execute(s.ctx, agentID, since)
	s.Require().NoError(err)
	s.Require().Len(violations, 1)
	s.Equal(reconciliation.ViolationFailedOrphanWrite, violations[0].Kind)
}

func (s *AuditReconciliationIntegrationSuite) TestStatusDivergenceDetected() {
	agentID := "recon-diverge-" + uuid.NewString()
	resourceID := "res-diverge-" + uuid.NewString()
	threadID := "thr-diverge"
	wamid := "wamid-diverge-" + uuid.NewString()
	platformThreadID := s.insertThread(resourceID, threadID)
	since := time.Now().UTC().Add(-1 * time.Hour)

	resourceUUID := uuid.New()
	s.insertLedger(wamid, resourceUUID)
	s.insertRun(agentID, resourceID, threadID, wamid, platformThreadID, agent.RunStatusSucceeded, agent.ToolOutcomeRouted, time.Now().UTC())
	s.insertWorkflowRun("pending-entry", wamid, workflow.RunStatusFailed, "failed")

	violations, err := s.uc.Execute(s.ctx, agentID, since)
	s.Require().NoError(err)
	s.Require().Len(violations, 1)
	s.Equal(reconciliation.ViolationStatusDivergence, violations[0].Kind)
}

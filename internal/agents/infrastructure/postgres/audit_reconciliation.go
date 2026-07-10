package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/reconciliation"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type auditReconciliationReader struct {
	db database.DBTX
}

func NewAuditReconciliationReader(db database.DBTX) reconciliation.RunConsistencyReader {
	return &auditReconciliationReader{db: db}
}

func (r *auditReconciliationReader) RunConsistencyRows(ctx context.Context, agentID string, since time.Time) ([]reconciliation.RunConsistencyRow, error) {
	const q = `
		SELECT
			pr.id,
			pr.correlation_key,
			pr.status,
			pr.outcome,
			COALESCE(l.ledger_writes, 0)     AS ledger_writes,
			COALESCE(l.transaction_count, 0) AS transaction_count,
			wf.status                        AS workflow_status,
			wf.state_status                  AS workflow_state_status
		FROM mecontrola.platform_runs pr
		LEFT JOIN LATERAL (
			SELECT
				count(*)                                  AS ledger_writes,
				count(t.id) FILTER (WHERE t.id IS NOT NULL) AS transaction_count
			FROM mecontrola.agents_write_ledger wl
			LEFT JOIN mecontrola.transactions t ON t.id = wl.resource_id
			WHERE wl.wamid = pr.correlation_key
		) l ON true
		LEFT JOIN LATERAL (
			SELECT wr.status, wr.state->>'status' AS state_status
			FROM mecontrola.workflow_runs wr
			WHERE wr.correlation_key = pr.correlation_key
			ORDER BY wr.updated_at DESC
			LIMIT 1
		) wf ON true
		WHERE pr.agent_id = $1 AND pr.started_at >= $2`

	rows, err := r.db.QueryContext(ctx, q, agentID, since)
	if err != nil {
		return nil, fmt.Errorf("agents.postgres.audit_reconciliation.run_consistency_rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []reconciliation.RunConsistencyRow
	for rows.Next() {
		var (
			runID               string
			correlationKey      string
			statusText          string
			outcomeText         string
			ledgerWrites        int
			txCount             int
			workflowStatus      sql.NullString
			workflowStateStatus sql.NullString
		)
		if err := rows.Scan(&runID, &correlationKey, &statusText, &outcomeText, &ledgerWrites, &txCount, &workflowStatus, &workflowStateStatus); err != nil {
			return nil, fmt.Errorf("agents.postgres.audit_reconciliation.run_consistency_rows: scan: %w", err)
		}

		runStatus, err := agent.ParseRunStatus(statusText)
		if err != nil {
			return nil, fmt.Errorf("agents.postgres.audit_reconciliation.run_consistency_rows: run_status: %w", err)
		}

		row := reconciliation.RunConsistencyRow{
			RunID:            runID,
			CorrelationKey:   correlationKey,
			RunStatus:        runStatus,
			LedgerWrites:     ledgerWrites,
			TransactionCount: txCount,
		}

		if outcomeText != "" {
			outcome, err := agent.ParseToolOutcome(outcomeText)
			if err != nil {
				return nil, fmt.Errorf("agents.postgres.audit_reconciliation.run_consistency_rows: outcome: %w", err)
			}
			row.Outcome = outcome
		}

		if workflowStatus.Valid {
			wfStatus, err := workflow.ParseRunStatus(workflowStatus.String)
			if err != nil {
				return nil, fmt.Errorf("agents.postgres.audit_reconciliation.run_consistency_rows: workflow_status: %w", err)
			}
			row.WorkflowStatus = wfStatus
			row.WorkflowFound = true
		}

		if workflowStateStatus.Valid {
			row.WorkflowStateSet = true
			row.WorkflowStateText = workflowStateStatus.String
		}

		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("agents.postgres.audit_reconciliation.run_consistency_rows: rows: %w", err)
	}
	return out, nil
}

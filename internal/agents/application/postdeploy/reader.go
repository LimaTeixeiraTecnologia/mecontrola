package postdeploy

import (
	"context"
	"time"
)

type AggregateReader interface {
	RunAggregate(ctx context.Context, agentID string, since time.Time) (RunAggregate, error)
	ExpectedToolHits(ctx context.Context, agentID string, since time.Time) (int, error)
	ScorerAggregates(ctx context.Context, agentID string, since time.Time) (map[string]ScorerAggregate, error)
	DuplicateWriteViolations(ctx context.Context, agentID string, since time.Time) (int64, error)
	WritePersistenceViolations(ctx context.Context, agentID string, since time.Time) (int64, error)
}

type PrometheusCounters struct {
	RunUpdateErrors     int64
	MessageAppendErrors int64
}

func ComputeGate(ctx context.Context, reader AggregateReader, agentID string, since time.Time, prom PrometheusCounters) (GateVerdict, error) {
	runs, err := reader.RunAggregate(ctx, agentID, since)
	if err != nil {
		return GateVerdict{}, err
	}

	hits, err := reader.ExpectedToolHits(ctx, agentID, since)
	if err != nil {
		return GateVerdict{}, err
	}

	scorers, err := reader.ScorerAggregates(ctx, agentID, since)
	if err != nil {
		return GateVerdict{}, err
	}

	duplicateWrites, err := reader.DuplicateWriteViolations(ctx, agentID, since)
	if err != nil {
		return GateVerdict{}, err
	}

	writePersistenceViolations, err := reader.WritePersistenceViolations(ctx, agentID, since)
	if err != nil {
		return GateVerdict{}, err
	}

	ops := OperationalCounters{
		AgentID:                    agentID,
		RunUpdateErrors:            prom.RunUpdateErrors,
		MessageAppendErrors:        prom.MessageAppendErrors,
		DuplicateWriteViolations:   duplicateWrites,
		WritePersistenceViolations: writePersistenceViolations,
	}

	return EvaluateGate(runs, hits, scorers, ops), nil
}

package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
)

type Definition[S any] struct {
	ID          string
	Root        Step[S]
	Durable     bool
	MaxAttempts int
}

type RunResult[S any] struct {
	RunID   uuid.UUID
	Status  RunStatus
	State   S
	Suspend *Suspension
}

type Engine[S any] interface {
	Start(ctx context.Context, def Definition[S], key string, initial S) (RunResult[S], error)
	Resume(ctx context.Context, def Definition[S], key string, resume []byte) (RunResult[S], error)
}

type engine[S any] struct {
	store   Store
	codec   Codec[S]
	o11y    observability.Observability
	metrics engineMetrics
}

type engineMetrics struct {
	runsTotal       observability.Counter
	runDuration     observability.Histogram
	stepsTotal      observability.Counter
	stepDuration    observability.Histogram
	suspendTotal    observability.Counter
	resumeTotal     observability.Counter
	versionConflict observability.Counter
}

func NewEngine[S any](store Store, o11y observability.Observability) Engine[S] {
	m := engineMetrics{
		runsTotal:       o11y.Metrics().Counter("workflow_runs_total", "Total workflow runs", "1"),
		runDuration:     o11y.Metrics().Histogram("workflow_run_duration_seconds", "Workflow run duration", "s"),
		stepsTotal:      o11y.Metrics().Counter("workflow_steps_total", "Total workflow steps executed", "1"),
		stepDuration:    o11y.Metrics().Histogram("workflow_step_duration_seconds", "Workflow step duration", "s"),
		suspendTotal:    o11y.Metrics().Counter("workflow_suspend_total", "Total workflow suspensions", "1"),
		resumeTotal:     o11y.Metrics().Counter("workflow_resume_total", "Total workflow resumes", "1"),
		versionConflict: o11y.Metrics().Counter("workflow_version_conflict_total", "Total CAS version conflicts", "1"),
	}
	return &engine[S]{
		store:   store,
		codec:   NewCodec[S](),
		o11y:    o11y,
		metrics: m,
	}
}

func (e *engine[S]) Start(ctx context.Context, def Definition[S], key string, initial S) (RunResult[S], error) {
	ctx, span := e.o11y.Tracer().Start(ctx, "workflow.engine.start",
		observability.WithAttributes(
			observability.String("workflow", def.ID),
		),
	)
	defer span.End()

	runStart := time.Now()
	runID := uuid.New()

	maxAttempts := def.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	snap := Snapshot{
		RunID:          runID,
		Workflow:       def.ID,
		CorrelationKey: key,
		Status:         RunStatusRunning,
		Cursor:         0,
		Attempts:       0,
		MaxAttempts:    maxAttempts,
		Version:        1,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if def.Durable {
		stateBytes, err := e.codec.Encode(initial)
		if err != nil {
			span.RecordError(err)
			return RunResult[S]{}, fmt.Errorf("workflow.engine.start: encode initial state: %w", err)
		}
		snap.State = stateBytes

		if err := e.store.Insert(ctx, snap); err != nil {
			span.RecordError(err)
			return RunResult[S]{}, fmt.Errorf("workflow.engine.start: insert snapshot: %w", err)
		}
	}

	result, err := e.execute(ctx, def, snap, initial, 0)
	if err != nil {
		span.RecordError(err)
		e.metrics.runsTotal.Add(ctx, 1,
			observability.String("workflow", def.ID),
			observability.String("status", RunStatusFailed.String()),
		)
		e.metrics.runDuration.Record(ctx, time.Since(runStart).Seconds(),
			observability.String("workflow", def.ID),
		)
		return result, err
	}

	span.SetAttributes(observability.String("status", result.Status.String()))
	e.metrics.runsTotal.Add(ctx, 1,
		observability.String("workflow", def.ID),
		observability.String("status", result.Status.String()),
	)
	e.metrics.runDuration.Record(ctx, time.Since(runStart).Seconds(),
		observability.String("workflow", def.ID),
	)

	if result.Status == RunStatusSuspended && result.Suspend != nil {
		e.metrics.suspendTotal.Add(ctx, 1,
			observability.String("workflow", def.ID),
			observability.String("reason", result.Suspend.Reason.String()),
		)
	}

	return result, nil
}

func (e *engine[S]) Resume(ctx context.Context, def Definition[S], key string, resume []byte) (RunResult[S], error) {
	ctx, span := e.o11y.Tracer().Start(ctx, "workflow.engine.resume",
		observability.WithAttributes(
			observability.String("workflow", def.ID),
		),
	)
	defer span.End()

	runStart := time.Now()

	if !def.Durable {
		span.SetAttributes(observability.String("outcome", "no_run_found"))
		return RunResult[S]{}, nil
	}

	snap, found, err := e.store.Load(ctx, def.ID, key)
	if err != nil {
		span.RecordError(err)
		return RunResult[S]{}, fmt.Errorf("workflow.engine.resume: load snapshot: %w", err)
	}
	if !found || snap.Status != RunStatusSuspended {
		span.SetAttributes(observability.String("outcome", "no_run_found"))
		return RunResult[S]{}, nil
	}

	current, err := e.codec.Decode(snap.State)
	if err != nil {
		span.RecordError(err)
		return RunResult[S]{}, fmt.Errorf("workflow.engine.resume: decode state: %w", err)
	}

	if len(resume) > 0 {
		resumeState, decErr := e.codec.Decode(resume)
		if decErr == nil {
			if applier, ok := any(current).(ResumeApplier[S]); ok {
				current = applier.ApplyResume(resumeState)
			} else {
				current = resumeState
			}
		}
	}

	result, err := e.execute(ctx, def, snap, current, snap.Cursor)
	if err != nil {
		span.RecordError(err)
		e.metrics.runsTotal.Add(ctx, 1,
			observability.String("workflow", def.ID),
			observability.String("status", RunStatusFailed.String()),
		)
		e.metrics.runDuration.Record(ctx, time.Since(runStart).Seconds(),
			observability.String("workflow", def.ID),
		)
		return result, err
	}

	span.SetAttributes(observability.String("status", result.Status.String()))
	e.metrics.resumeTotal.Add(ctx, 1,
		observability.String("workflow", def.ID),
		observability.String("result", result.Status.String()),
	)
	e.metrics.runsTotal.Add(ctx, 1,
		observability.String("workflow", def.ID),
		observability.String("status", result.Status.String()),
	)
	e.metrics.runDuration.Record(ctx, time.Since(runStart).Seconds(),
		observability.String("workflow", def.ID),
	)

	return result, nil
}

func (e *engine[S]) execute(ctx context.Context, def Definition[S], snap Snapshot, state S, cursorOffset int) (RunResult[S], error) { //nolint:revive,cyclop // state machine: durable + suspended + failed paths são intrinsecamente acoplados
	seq, isSeq := def.Root.(*sequenceStep[S])
	if !isSeq {
		return e.executeStep(ctx, def, snap, def.Root, state, 0)
	}

	current := state
	stepCount := len(seq.steps)

	for i := cursorOffset; i < stepCount; i++ {
		step := seq.steps[i]
		out, stepErr := e.runStep(ctx, def, snap, step, current, i)
		if stepErr != nil {
			snap.Status = RunStatusFailed
			snap.LastError = stepErr.Error()
			snap.Attempts++
			if def.Durable {
				now := time.Now().UTC()
				snap.EndedAt = &now
				snap.UpdatedAt = now
				if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
					e.o11y.Logger().Error(ctx, "workflow.engine: save snap on step error failed", observability.Error(snapErr))
				}
			}
			return RunResult[S]{RunID: snap.RunID, Status: RunStatusFailed, State: current}, stepErr
		}

		current = out.State

		if out.Status == StepStatusSuspended {
			snap.Status = RunStatusSuspended
			snap.Cursor = i
			if out.Suspend != nil {
				snap.SuspendReason = out.Suspend.Reason
			}
			if def.Durable {
				stateBytes, encErr := e.codec.Encode(current)
				if encErr != nil {
					return RunResult[S]{}, fmt.Errorf("workflow.engine: encode suspended state: %w", encErr)
				}
				snap.State = stateBytes
				snap.UpdatedAt = time.Now().UTC()
				if saveErr := e.saveSnap(ctx, def, snap); saveErr != nil {
					return RunResult[S]{}, saveErr
				}
			}
			return RunResult[S]{RunID: snap.RunID, Status: RunStatusSuspended, State: current, Suspend: out.Suspend}, nil
		}

		if out.Status == StepStatusFailed {
			snap.Status = RunStatusFailed
			snap.LastError = fmt.Sprintf("step %s returned failed", step.ID())
			snap.Attempts++
			if def.Durable {
				now := time.Now().UTC()
				snap.EndedAt = &now
				snap.UpdatedAt = now
				if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
					e.o11y.Logger().Error(ctx, "workflow.engine: save snap on step failed status failed", observability.Error(snapErr))
				}
			}
			return RunResult[S]{RunID: snap.RunID, Status: RunStatusFailed, State: current}, fmt.Errorf("workflow.engine: step %s failed", step.ID())
		}
	}

	snap.Status = RunStatusSucceeded
	if def.Durable {
		now := time.Now().UTC()
		snap.EndedAt = &now
		snap.UpdatedAt = now
		stateBytes, encErr := e.codec.Encode(current)
		if encErr == nil {
			snap.State = stateBytes
		}
		if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
			e.o11y.Logger().Error(ctx, "workflow.engine: save snap on succeeded failed", observability.Error(snapErr))
		}
	}

	return RunResult[S]{RunID: snap.RunID, Status: RunStatusSucceeded, State: current}, nil
}

func (e *engine[S]) executeStep(ctx context.Context, def Definition[S], snap Snapshot, step Step[S], state S, idx int) (RunResult[S], error) {
	out, err := e.runStep(ctx, def, snap, step, state, idx)
	if err != nil {
		snap.Status = RunStatusFailed
		snap.LastError = err.Error()
		if def.Durable {
			now := time.Now().UTC()
			snap.EndedAt = &now
			snap.UpdatedAt = now
			if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
				e.o11y.Logger().Error(ctx, "workflow.engine: save snap on step error failed", observability.Error(snapErr))
			}
		}
		return RunResult[S]{RunID: snap.RunID, Status: RunStatusFailed, State: state}, err
	}

	if out.Status == StepStatusSuspended {
		snap.Status = RunStatusSuspended
		snap.Cursor = idx
		if out.Suspend != nil {
			snap.SuspendReason = out.Suspend.Reason
		}
		if def.Durable {
			stateBytes, encErr := e.codec.Encode(out.State)
			if encErr != nil {
				return RunResult[S]{}, fmt.Errorf("workflow.engine: encode suspended state: %w", encErr)
			}
			snap.State = stateBytes
			snap.UpdatedAt = time.Now().UTC()
			if saveErr := e.saveSnap(ctx, def, snap); saveErr != nil {
				return RunResult[S]{}, saveErr
			}
		}
		return RunResult[S]{RunID: snap.RunID, Status: RunStatusSuspended, State: out.State, Suspend: out.Suspend}, nil
	}

	snap.Status = RunStatusSucceeded
	if def.Durable {
		now := time.Now().UTC()
		snap.EndedAt = &now
		snap.UpdatedAt = now
		stateBytes, encErr := e.codec.Encode(out.State)
		if encErr == nil {
			snap.State = stateBytes
		}
		if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
			e.o11y.Logger().Error(ctx, "workflow.engine: save snap on succeeded failed", observability.Error(snapErr))
		}
	}

	return RunResult[S]{RunID: snap.RunID, Status: RunStatusSucceeded, State: out.State}, nil
}

func (e *engine[S]) runStep(ctx context.Context, def Definition[S], snap Snapshot, step Step[S], state S, seq int) (StepOutput[S], error) {
	ctx, span := e.o11y.Tracer().Start(ctx, "workflow.step.execute",
		observability.WithAttributes(
			observability.String("workflow", def.ID),
			observability.String("step", step.ID()),
		),
	)
	defer span.End()

	stepStart := time.Now()
	out, err := step.Execute(ctx, state)
	elapsed := time.Since(stepStart)
	durationMs := elapsed.Milliseconds()
	durationSec := elapsed.Seconds()

	status := StepStatusCompleted
	errStr := ""
	if err != nil {
		status = StepStatusFailed
		errStr = err.Error()
		span.RecordError(err)
	} else if out.Status != 0 {
		status = out.Status
	}

	span.SetAttributes(
		observability.String("status", status.String()),
		observability.Int64("duration_ms", durationMs),
	)

	e.metrics.stepsTotal.Add(ctx, 1,
		observability.String("workflow", def.ID),
		observability.String("step", step.ID()),
		observability.String("status", status.String()),
	)
	e.metrics.stepDuration.Record(ctx, durationSec,
		observability.String("workflow", def.ID),
		observability.String("step", step.ID()),
	)

	if def.Durable {
		now := time.Now().UTC()
		rec := StepRecord{
			ID:         uuid.New(),
			RunID:      snap.RunID,
			StepID:     step.ID(),
			Seq:        seq,
			Status:     status,
			Attempt:    snap.Attempts + 1,
			DurationMs: durationMs,
			Error:      errStr,
			StartedAt:  stepStart.UTC(),
			EndedAt:    &now,
		}
		_ = e.store.AppendStep(ctx, rec)
	}

	return out, err
}

func (e *engine[S]) saveSnap(ctx context.Context, def Definition[S], snap Snapshot) error {
	expectedVersion := snap.Version
	snap.Version++
	err := e.store.Save(ctx, snap, expectedVersion)
	if err != nil {
		e.metrics.versionConflict.Add(ctx, 1,
			observability.String("workflow", def.ID),
		)
	}
	return err
}

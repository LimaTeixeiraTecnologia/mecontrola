package workflow

import (
	"context"
	"errors"
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

		if existing, found, loadErr := e.store.Load(ctx, def.ID, key); loadErr != nil {
			span.RecordError(loadErr)
			return RunResult[S]{}, fmt.Errorf("workflow.engine.start: check active run: %w", loadErr)
		} else if found && (existing.Status == RunStatusRunning || existing.Status == RunStatusSuspended) {
			span.SetAttributes(observability.String("outcome", "active_run_exists"))
			return RunResult[S]{}, ErrRunAlreadyExists
		}

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
			observability.String("outcome", result.Suspend.Reason.String()),
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
		mergedBytes, mErr := e.codec.MergePatch(snap.State, resume)
		if mErr != nil {
			span.RecordError(mErr)
			return RunResult[S]{}, fmt.Errorf("workflow.engine.resume: merge resume: %w", mErr)
		}
		merged, dErr := e.codec.Decode(mergedBytes)
		if dErr != nil {
			span.RecordError(dErr)
			return RunResult[S]{}, fmt.Errorf("workflow.engine.resume: decode merged: %w", dErr)
		}
		current = merged
	}

	result, err := e.execute(ctx, def, snap, current, snap.Cursor)
	if err != nil {
		span.RecordError(err)
		e.metrics.resumeTotal.Add(ctx, 1,
			observability.String("workflow", def.ID),
			observability.String("status", RunStatusFailed.String()),
		)
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
		observability.String("status", result.Status.String()),
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
	var lastOut StepOutput[S]

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
				stateBytes, encErr := e.codec.Encode(current)
				if encErr == nil {
					snap.State = stateBytes
				}
				if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
					return e.resolveConflictOrFail(ctx, def, snap, snapErr)
				}
			}
			return RunResult[S]{RunID: snap.RunID, Status: RunStatusFailed, State: current}, stepErr
		}

		current = out.State
		lastOut = out

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
					return e.resolveConflictOrFail(ctx, def, snap, saveErr)
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
				stateBytes, encErr := e.codec.Encode(current)
				if encErr == nil {
					snap.State = stateBytes
				}
				if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
					return e.resolveConflictOrFail(ctx, def, snap, snapErr)
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
		if encErr != nil {
			return RunResult[S]{}, fmt.Errorf("workflow.engine: encode succeeded state: %w", encErr)
		}
		snap.State = stateBytes
		if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
			return e.resolveConflictOrFail(ctx, def, snap, snapErr)
		}
	}

	return RunResult[S]{RunID: snap.RunID, Status: RunStatusSucceeded, State: current, Suspend: lastOut.Suspend}, nil
}

func (e *engine[S]) executeStep(ctx context.Context, def Definition[S], snap Snapshot, step Step[S], state S, idx int) (RunResult[S], error) {
	out, err := e.runStep(ctx, def, snap, step, state, idx)
	if err != nil {
		return e.finalizeFailed(ctx, def, snap, state, err)
	}

	if out.Status == StepStatusSuspended {
		return e.finalizeSuspended(ctx, def, snap, idx, out)
	}

	return e.finalizeSucceeded(ctx, def, snap, out)
}

func (e *engine[S]) finalizeFailed(ctx context.Context, def Definition[S], snap Snapshot, state S, runErr error) (RunResult[S], error) {
	snap.Status = RunStatusFailed
	snap.LastError = runErr.Error()
	snap.Attempts++
	if def.Durable {
		snap = e.encodeAndStampFailed(snap, state)
		if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
			return e.resolveConflictOrFail(ctx, def, snap, snapErr)
		}
	}
	return RunResult[S]{RunID: snap.RunID, Status: RunStatusFailed, State: state}, runErr
}

func (e *engine[S]) encodeAndStampFailed(snap Snapshot, state S) Snapshot {
	now := time.Now().UTC()
	snap.EndedAt = &now
	snap.UpdatedAt = now
	stateBytes, encErr := e.codec.Encode(state)
	if encErr == nil {
		snap.State = stateBytes
	}
	return snap
}

func (e *engine[S]) finalizeSuspended(ctx context.Context, def Definition[S], snap Snapshot, idx int, out StepOutput[S]) (RunResult[S], error) {
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
			return e.resolveConflictOrFail(ctx, def, snap, saveErr)
		}
	}
	return RunResult[S]{RunID: snap.RunID, Status: RunStatusSuspended, State: out.State, Suspend: out.Suspend}, nil
}

func (e *engine[S]) finalizeSucceeded(ctx context.Context, def Definition[S], snap Snapshot, out StepOutput[S]) (RunResult[S], error) {
	snap.Status = RunStatusSucceeded
	if def.Durable {
		now := time.Now().UTC()
		snap.EndedAt = &now
		snap.UpdatedAt = now
		stateBytes, encErr := e.codec.Encode(out.State)
		if encErr != nil {
			return RunResult[S]{}, fmt.Errorf("workflow.engine: encode succeeded state: %w", encErr)
		}
		snap.State = stateBytes
		if snapErr := e.saveSnap(ctx, def, snap); snapErr != nil {
			return e.resolveConflictOrFail(ctx, def, snap, snapErr)
		}
	}
	return RunResult[S]{RunID: snap.RunID, Status: RunStatusSucceeded, State: out.State, Suspend: out.Suspend}, nil
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
		if appendErr := e.store.AppendStep(ctx, rec); appendErr != nil {
			return StepOutput[S]{}, fmt.Errorf("workflow.engine: append step record: %w", appendErr)
		}
	}

	return out, err
}

func (e *engine[S]) saveSnap(ctx context.Context, def Definition[S], snap Snapshot) error {
	expectedVersion := snap.Version
	snap.Version++
	err := e.store.Save(ctx, snap, expectedVersion)
	if errors.Is(err, ErrVersionConflict) {
		e.metrics.versionConflict.Add(ctx, 1,
			observability.String("workflow", def.ID),
		)
	}
	return err
}

func (e *engine[S]) resolveConflictOrFail(ctx context.Context, def Definition[S], snap Snapshot, saveErr error) (RunResult[S], error) {
	if !errors.Is(saveErr, ErrVersionConflict) {
		return RunResult[S]{}, fmt.Errorf("workflow.engine: save snapshot: %w", saveErr)
	}
	latest, found, loadErr := e.store.Load(ctx, def.ID, snap.CorrelationKey)
	if loadErr != nil {
		return RunResult[S]{}, fmt.Errorf("workflow.engine: version conflict, reload failed: %w", loadErr)
	}
	if !found {
		return RunResult[S]{}, fmt.Errorf("workflow.engine: version conflict and run not found: %w", saveErr)
	}
	state, decodeErr := e.codec.Decode(latest.State)
	if decodeErr != nil {
		return RunResult[S]{}, fmt.Errorf("workflow.engine: version conflict, decode failed: %w", decodeErr)
	}
	return RunResult[S]{RunID: latest.RunID, Status: latest.Status, State: state}, ErrRunConflict
}

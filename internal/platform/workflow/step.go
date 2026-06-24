package workflow

import (
	"context"
	"errors"
	"fmt"
)

type RunStatus int

const (
	RunStatusRunning RunStatus = iota + 1
	RunStatusSuspended
	RunStatusSucceeded
	RunStatusFailed
)

func (s RunStatus) String() string {
	switch s {
	case RunStatusRunning:
		return "running"
	case RunStatusSuspended:
		return "suspended"
	case RunStatusSucceeded:
		return "succeeded"
	case RunStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

func (s RunStatus) IsValid() bool {
	return s >= RunStatusRunning && s <= RunStatusFailed
}

var errInvalidRunStatus = errors.New("workflow: invalid run status")

func ParseRunStatus(s string) (RunStatus, error) {
	switch s {
	case "running":
		return RunStatusRunning, nil
	case "suspended":
		return RunStatusSuspended, nil
	case "succeeded":
		return RunStatusSucceeded, nil
	case "failed":
		return RunStatusFailed, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidRunStatus, s)
	}
}

type StepStatus int

const (
	StepStatusCompleted StepStatus = iota + 1
	StepStatusSuspended
	StepStatusFailed
	StepStatusSkipped
)

func (s StepStatus) String() string {
	switch s {
	case StepStatusCompleted:
		return "completed"
	case StepStatusSuspended:
		return "suspended"
	case StepStatusFailed:
		return "failed"
	case StepStatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

func (s StepStatus) IsValid() bool {
	return s >= StepStatusCompleted && s <= StepStatusSkipped
}

var errInvalidStepStatus = errors.New("workflow: invalid step status")

func ParseStepStatus(s string) (StepStatus, error) {
	switch s {
	case "completed":
		return StepStatusCompleted, nil
	case "suspended":
		return StepStatusSuspended, nil
	case "failed":
		return StepStatusFailed, nil
	case "skipped":
		return StepStatusSkipped, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidStepStatus, s)
	}
}

type SuspendReason int

const (
	SuspendAwaitingInput SuspendReason = iota + 1
)

func (r SuspendReason) String() string {
	switch r {
	case SuspendAwaitingInput:
		return "awaiting_input"
	default:
		return "unknown"
	}
}

func (r SuspendReason) IsValid() bool {
	return r == SuspendAwaitingInput
}

var errInvalidSuspendReason = errors.New("workflow: invalid suspend reason")

func ParseSuspendReason(s string) (SuspendReason, error) {
	switch s {
	case "awaiting_input":
		return SuspendAwaitingInput, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidSuspendReason, s)
	}
}

type Suspension struct {
	Reason SuspendReason
	Prompt string
}

type StepOutput[S any] struct {
	State   S
	Status  StepStatus
	Suspend *Suspension
}

type Step[S any] interface {
	ID() string
	Execute(ctx context.Context, state S) (StepOutput[S], error)
}

type stepFunc[S any] struct {
	id string
	fn func(context.Context, S) (StepOutput[S], error)
}

func NewStepFunc[S any](id string, fn func(context.Context, S) (StepOutput[S], error)) Step[S] {
	return &stepFunc[S]{id: id, fn: fn}
}

func (s *stepFunc[S]) ID() string { return s.id }

func (s *stepFunc[S]) Execute(ctx context.Context, state S) (StepOutput[S], error) {
	return s.fn(ctx, state)
}

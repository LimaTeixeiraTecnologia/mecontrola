package agent

import (
	"errors"
	"fmt"
)

type RunStatus int

const (
	RunStatusRunning RunStatus = iota + 1
	RunStatusSucceeded
	RunStatusFailed
)

func (s RunStatus) String() string {
	switch s {
	case RunStatusRunning:
		return "running"
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

var errInvalidRunStatus = errors.New("agent: invalid run status")

func ParseRunStatus(s string) (RunStatus, error) {
	switch s {
	case "running":
		return RunStatusRunning, nil
	case "succeeded":
		return RunStatusSucceeded, nil
	case "failed":
		return RunStatusFailed, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidRunStatus, s)
	}
}

type ToolOutcome int

const (
	ToolOutcomeRouted ToolOutcome = iota + 1
	ToolOutcomeClarify
	ToolOutcomeUsecaseError
	ToolOutcomeMissingResolver
	ToolOutcomeReplay
	ToolOutcomeReconciled
)

func (o ToolOutcome) String() string {
	switch o {
	case ToolOutcomeRouted:
		return "routed"
	case ToolOutcomeClarify:
		return "clarify"
	case ToolOutcomeUsecaseError:
		return "usecaseError"
	case ToolOutcomeMissingResolver:
		return "missingResolver"
	case ToolOutcomeReplay:
		return "replay"
	case ToolOutcomeReconciled:
		return "reconciled"
	default:
		return "unknown"
	}
}

func (o ToolOutcome) IsValid() bool {
	return o >= ToolOutcomeRouted && o <= ToolOutcomeReconciled
}

var errInvalidToolOutcome = errors.New("agent: invalid tool outcome")

func ParseToolOutcome(s string) (ToolOutcome, error) {
	switch s {
	case "routed":
		return ToolOutcomeRouted, nil
	case "clarify":
		return ToolOutcomeClarify, nil
	case "usecaseError":
		return ToolOutcomeUsecaseError, nil
	case "missingResolver":
		return ToolOutcomeMissingResolver, nil
	case "replay":
		return ToolOutcomeReplay, nil
	case "reconciled":
		return ToolOutcomeReconciled, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidToolOutcome, s)
	}
}

type AwaitingKind int

const (
	AwaitingKindNone AwaitingKind = iota + 1
	AwaitingKindConfirm
)

func (a AwaitingKind) String() string {
	switch a {
	case AwaitingKindNone:
		return "none"
	case AwaitingKindConfirm:
		return "confirm"
	default:
		return "unknown"
	}
}

func (a AwaitingKind) IsValid() bool {
	return a >= AwaitingKindNone && a <= AwaitingKindConfirm
}

var errInvalidAwaitingKind = errors.New("agent: invalid awaiting kind")

func ParseAwaitingKind(s string) (AwaitingKind, error) {
	switch s {
	case "none":
		return AwaitingKindNone, nil
	case "confirm":
		return AwaitingKindConfirm, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidAwaitingKind, s)
	}
}

type ExecutionMode int

const (
	ExecutionModeSync ExecutionMode = iota + 1
	ExecutionModeStream
)

func (m ExecutionMode) String() string {
	switch m {
	case ExecutionModeSync:
		return "sync"
	case ExecutionModeStream:
		return "stream"
	default:
		return "unknown"
	}
}

func (m ExecutionMode) IsValid() bool {
	return m >= ExecutionModeSync && m <= ExecutionModeStream
}

var errInvalidExecutionMode = errors.New("agent: invalid execution mode")

func ParseExecutionMode(s string) (ExecutionMode, error) {
	switch s {
	case "sync":
		return ExecutionModeSync, nil
	case "stream":
		return ExecutionModeStream, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidExecutionMode, s)
	}
}

package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunStatus_String(t *testing.T) {
	scenarios := []struct {
		status   RunStatus
		expected string
	}{
		{RunStatusRunning, "running"},
		{RunStatusSuspended, "suspended"},
		{RunStatusSucceeded, "succeeded"},
		{RunStatusFailed, "failed"},
		{RunStatus(99), "unknown"},
	}
	for _, s := range scenarios {
		t.Run(s.expected, func(t *testing.T) {
			assert.Equal(t, s.expected, s.status.String())
		})
	}
}

func TestRunStatus_IsValid(t *testing.T) {
	assert.True(t, RunStatusRunning.IsValid())
	assert.True(t, RunStatusSuspended.IsValid())
	assert.True(t, RunStatusSucceeded.IsValid())
	assert.True(t, RunStatusFailed.IsValid())
	assert.False(t, RunStatus(0).IsValid())
	assert.False(t, RunStatus(99).IsValid())
}

func TestParseRunStatus(t *testing.T) {
	scenarios := []struct {
		input    string
		expected RunStatus
		wantErr  bool
	}{
		{"running", RunStatusRunning, false},
		{"suspended", RunStatusSuspended, false},
		{"succeeded", RunStatusSucceeded, false},
		{"failed", RunStatusFailed, false},
		{"invalid", 0, true},
		{"", 0, true},
	}
	for _, s := range scenarios {
		t.Run(s.input, func(t *testing.T) {
			got, err := ParseRunStatus(s.input)
			if s.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, s.expected, got)
		})
	}
}

func TestStepStatus_String(t *testing.T) {
	scenarios := []struct {
		status   StepStatus
		expected string
	}{
		{StepStatusCompleted, "completed"},
		{StepStatusSuspended, "suspended"},
		{StepStatusFailed, "failed"},
		{StepStatusSkipped, "skipped"},
		{StepStatus(99), "unknown"},
	}
	for _, s := range scenarios {
		t.Run(s.expected, func(t *testing.T) {
			assert.Equal(t, s.expected, s.status.String())
		})
	}
}

func TestStepStatus_IsValid(t *testing.T) {
	assert.True(t, StepStatusCompleted.IsValid())
	assert.True(t, StepStatusSuspended.IsValid())
	assert.True(t, StepStatusFailed.IsValid())
	assert.True(t, StepStatusSkipped.IsValid())
	assert.False(t, StepStatus(0).IsValid())
	assert.False(t, StepStatus(99).IsValid())
}

func TestParseStepStatus(t *testing.T) {
	scenarios := []struct {
		input    string
		expected StepStatus
		wantErr  bool
	}{
		{"completed", StepStatusCompleted, false},
		{"suspended", StepStatusSuspended, false},
		{"failed", StepStatusFailed, false},
		{"skipped", StepStatusSkipped, false},
		{"invalid", 0, true},
	}
	for _, s := range scenarios {
		t.Run(s.input, func(t *testing.T) {
			got, err := ParseStepStatus(s.input)
			if s.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, s.expected, got)
		})
	}
}

func TestSuspendReason_String(t *testing.T) {
	assert.Equal(t, "awaiting_input", SuspendAwaitingInput.String())
	assert.Equal(t, "unknown", SuspendReason(99).String())
}

func TestSuspendReason_IsValid(t *testing.T) {
	assert.True(t, SuspendAwaitingInput.IsValid())
	assert.False(t, SuspendReason(0).IsValid())
	assert.False(t, SuspendReason(99).IsValid())
}

func TestParseSuspendReason(t *testing.T) {
	got, err := ParseSuspendReason("awaiting_input")
	assert.NoError(t, err)
	assert.Equal(t, SuspendAwaitingInput, got)

	_, err = ParseSuspendReason("unknown")
	assert.Error(t, err)
}

type intState struct{ value int }

func TestNewStepFunc(t *testing.T) {
	step := NewStepFunc("test-step", func(_ context.Context, s intState) (StepOutput[intState], error) {
		return StepOutput[intState]{State: intState{value: s.value + 1}, Status: StepStatusCompleted}, nil
	})
	assert.Equal(t, "test-step", step.ID())

	out, err := step.Execute(context.Background(), intState{value: 5})
	assert.NoError(t, err)
	assert.Equal(t, 6, out.State.value)
	assert.Equal(t, StepStatusCompleted, out.Status)
}

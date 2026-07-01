package agent

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type TypesTestSuite struct {
	suite.Suite
}

func TestTypesTestSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}

func (s *TypesTestSuite) TestRunStatus_String() {
	s.Equal("running", RunStatusRunning.String())
	s.Equal("succeeded", RunStatusSucceeded.String())
	s.Equal("failed", RunStatusFailed.String())
	s.Equal("unknown", RunStatus(0).String())
}

func (s *TypesTestSuite) TestRunStatus_IsValid() {
	s.True(RunStatusRunning.IsValid())
	s.True(RunStatusSucceeded.IsValid())
	s.True(RunStatusFailed.IsValid())
	s.False(RunStatus(0).IsValid())
	s.False(RunStatus(99).IsValid())
}

func (s *TypesTestSuite) TestParseRunStatus_Valid() {
	cases := []struct {
		input    string
		expected RunStatus
	}{
		{"running", RunStatusRunning},
		{"succeeded", RunStatusSucceeded},
		{"failed", RunStatusFailed},
	}
	for _, c := range cases {
		got, err := ParseRunStatus(c.input)
		s.NoError(err)
		s.Equal(c.expected, got)
	}
}

func (s *TypesTestSuite) TestParseRunStatus_Invalid() {
	_, err := ParseRunStatus("bad")
	s.Error(err)
}

func (s *TypesTestSuite) TestToolOutcome_String() {
	s.Equal("routed", ToolOutcomeRouted.String())
	s.Equal("clarify", ToolOutcomeClarify.String())
	s.Equal("usecaseError", ToolOutcomeUsecaseError.String())
	s.Equal("missingResolver", ToolOutcomeMissingResolver.String())
	s.Equal("replay", ToolOutcomeReplay.String())
	s.Equal("reconciled", ToolOutcomeReconciled.String())
	s.Equal("unknown", ToolOutcome(0).String())
}

func (s *TypesTestSuite) TestToolOutcome_IsValid() {
	s.True(ToolOutcomeRouted.IsValid())
	s.True(ToolOutcomeReplay.IsValid())
	s.True(ToolOutcomeReconciled.IsValid())
	s.False(ToolOutcome(0).IsValid())
}

func (s *TypesTestSuite) TestParseToolOutcome_Valid() {
	cases := []struct {
		input    string
		expected ToolOutcome
	}{
		{"routed", ToolOutcomeRouted},
		{"clarify", ToolOutcomeClarify},
		{"usecaseError", ToolOutcomeUsecaseError},
		{"missingResolver", ToolOutcomeMissingResolver},
		{"replay", ToolOutcomeReplay},
		{"reconciled", ToolOutcomeReconciled},
	}
	for _, c := range cases {
		got, err := ParseToolOutcome(c.input)
		s.NoError(err)
		s.Equal(c.expected, got)
	}
}

func (s *TypesTestSuite) TestParseToolOutcome_Invalid() {
	_, err := ParseToolOutcome("unknown_value")
	s.Error(err)
}

func (s *TypesTestSuite) TestAwaitingKind_String() {
	s.Equal("none", AwaitingKindNone.String())
	s.Equal("confirm", AwaitingKindConfirm.String())
	s.Equal("unknown", AwaitingKind(0).String())
}

func (s *TypesTestSuite) TestAwaitingKind_IsValid() {
	s.True(AwaitingKindNone.IsValid())
	s.True(AwaitingKindConfirm.IsValid())
	s.False(AwaitingKind(0).IsValid())
}

func (s *TypesTestSuite) TestParseAwaitingKind_Valid() {
	got, err := ParseAwaitingKind("none")
	s.NoError(err)
	s.Equal(AwaitingKindNone, got)

	got, err = ParseAwaitingKind("confirm")
	s.NoError(err)
	s.Equal(AwaitingKindConfirm, got)
}

func (s *TypesTestSuite) TestParseAwaitingKind_Invalid() {
	_, err := ParseAwaitingKind("bad")
	s.Error(err)
}

func (s *TypesTestSuite) TestExecutionMode_String() {
	s.Equal("sync", ExecutionModeSync.String())
	s.Equal("stream", ExecutionModeStream.String())
	s.Equal("unknown", ExecutionMode(0).String())
}

func (s *TypesTestSuite) TestExecutionMode_IsValid() {
	s.True(ExecutionModeSync.IsValid())
	s.True(ExecutionModeStream.IsValid())
	s.False(ExecutionMode(0).IsValid())
}

func (s *TypesTestSuite) TestParseExecutionMode_Valid() {
	got, err := ParseExecutionMode("sync")
	s.NoError(err)
	s.Equal(ExecutionModeSync, got)

	got, err = ParseExecutionMode("stream")
	s.NoError(err)
	s.Equal(ExecutionModeStream, got)
}

func (s *TypesTestSuite) TestParseExecutionMode_Invalid() {
	_, err := ParseExecutionMode("bad")
	s.Error(err)
}

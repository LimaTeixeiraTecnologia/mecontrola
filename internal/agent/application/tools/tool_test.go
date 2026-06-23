package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type ToolOutcomeSuite struct {
	suite.Suite
}

func TestToolOutcomeSuite(t *testing.T) {
	suite.Run(t, new(ToolOutcomeSuite))
}

func (s *ToolOutcomeSuite) TestStringRoundTrip() {
	scenarios := []struct {
		name    string
		outcome ToolOutcome
		raw     string
	}{
		{name: "routed", outcome: OutcomeRouted, raw: "routed"},
		{name: "fallback", outcome: OutcomeFallback, raw: "fallback"},
		{name: "parse_error", outcome: OutcomeParseError, raw: "parse_error"},
		{name: "usecase_error", outcome: OutcomeUsecaseError, raw: "usecase_error"},
		{name: "missing_resolver", outcome: OutcomeMissingResolver, raw: "missing_resolver"},
		{name: "reply_failed", outcome: OutcomeReplyFailed, raw: "reply_failed"},
		{name: "empty_text", outcome: OutcomeEmptyText, raw: "empty_text"},
		{name: "authz_denied", outcome: OutcomeAuthzDenied, raw: "authz_denied"},
		{name: "clarify", outcome: OutcomeClarify, raw: "clarify"},
		{name: "policy_blocked", outcome: OutcomePolicyBlocked, raw: "policy_blocked"},
		{name: "replay", outcome: OutcomeReplay, raw: "replay"},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.raw, scenario.outcome.String())
			parsed, err := ParseOutcome(scenario.raw)
			s.NoError(err)
			s.Equal(scenario.outcome, parsed)
		})
	}
}

func (s *ToolOutcomeSuite) TestParseOutcomeRejectsUnknown() {
	parsed, err := ParseOutcome("not_an_outcome")
	s.ErrorIs(err, ErrToolOutcomeUnknown)
	s.Equal(ToolOutcome(0), parsed)
}

type ToolSuite struct {
	suite.Suite
	ctx context.Context
}

func TestToolSuite(t *testing.T) {
	suite.Run(t, new(ToolSuite))
}

func (s *ToolSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ToolSuite) TestExecute() {
	type args struct {
		input ToolInput
	}
	type dependencies struct {
		exec ExecuteFunc
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result ToolResult, err error)
	}{
		{
			name: "routed delega execucao",
			args: args{input: ToolInput{UserID: uuid.New(), Channel: "whatsapp", Intent: intent.NewListCards()}},
			dependencies: dependencies{exec: func(_ context.Context, in ToolInput) (ToolResult, error) {
				return ToolResult{Reply: "ok", Outcome: OutcomeRouted, Kind: in.Intent.Kind()}, nil
			}},
			expect: func(result ToolResult, err error) {
				s.NoError(err)
				s.Equal(OutcomeRouted, result.Outcome)
				s.Equal(intent.KindListCards, result.Kind)
				s.Equal("ok", result.Reply)
			},
		},
		{
			name: "erro propaga",
			args: args{input: ToolInput{UserID: uuid.New(), Channel: "whatsapp", Intent: intent.NewListCards()}},
			dependencies: dependencies{exec: func(_ context.Context, _ ToolInput) (ToolResult, error) {
				return ToolResult{Outcome: OutcomeUsecaseError}, errors.New("boom")
			}},
			expect: func(result ToolResult, err error) {
				s.Error(err)
				s.Equal(OutcomeUsecaseError, result.Outcome)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			tool := NewTool(ToolSpec{Name: "probe", IntentKind: intent.KindListCards, Description: "probe"}, scenario.dependencies.exec)
			s.Equal("probe", tool.Name())
			s.Equal("probe", tool.Descriptor().Name)
			result, err := tool.Execute(s.ctx, scenario.args.input)
			scenario.expect(result, err)
		})
	}
}

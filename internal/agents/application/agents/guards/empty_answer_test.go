package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type EmptyAnswerGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func TestEmptyAnswerGuardSuite(t *testing.T) {
	suite.Run(t, new(EmptyAnswerGuardSuite))
}

func (s *EmptyAnswerGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *EmptyAnswerGuardSuite) TestName() {
	guard := NewEmptyAnswerGuard()
	s.Equal("empty_answer", guard.Name())
}

func (s *EmptyAnswerGuardSuite) TestInspect() {
	type args struct {
		out agent.Result
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(decision GuardDecision)
	}{
		{
			name: "content vazio -> fallback seguro",
			args: args{out: agent.Result{Content: ""}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(emptyAnswerFallbackMessage, decision.Result.Content)
			},
		},
		{
			name: "content apenas espacos -> fallback seguro",
			args: args{out: agent.Result{Content: "   "}},
			expect: func(decision GuardDecision) {
				s.True(decision.Handled)
				s.Equal(emptyAnswerFallbackMessage, decision.Result.Content)
			},
		},
		{
			name: "content valido -> nao trata",
			args: args{out: agent.Result{Content: "Resposta válida"}},
			expect: func(decision GuardDecision) {
				s.False(decision.Handled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			guard := NewEmptyAnswerGuard()
			decision := guard.Inspect(s.ctx, agent.Request{}, scenario.args.out)
			scenario.expect(decision)
		})
	}
}

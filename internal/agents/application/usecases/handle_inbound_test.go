package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
)

type HandleInboundSuite struct {
	suite.Suite
	ctx         context.Context
	obs         observability.Observability
	runtimeMock *agentmocks.AgentRuntime
}

func TestHandleInboundSuite(t *testing.T) {
	suite.Run(t, new(HandleInboundSuite))
}

func (s *HandleInboundSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.runtimeMock = agentmocks.NewAgentRuntime(s.T())
}

func (s *HandleInboundSuite) TestExecute() {
	runID := uuid.New()

	type args struct {
		in input.InboundInput
	}
	type dependencies struct {
		runtimeMock *agentmocks.AgentRuntime
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(outcome agent.Outcome, err error)
	}{
		{
			name: "deve executar com sucesso",
			args: args{
				in: input.InboundInput{
					ResourceID: "user-123",
					ThreadID:   "thread-abc",
					AgentID:    "mecontrola-agent",
					Message:    "Qual o clima em São Paulo?",
					MessageID:  "msg-001",
				},
			},
			dependencies: dependencies{
				runtimeMock: func() *agentmocks.AgentRuntime {
					s.runtimeMock.EXPECT().
						Execute(mock.Anything, agent.InboundRequest{
							ResourceID: "user-123",
							ThreadID:   "thread-abc",
							AgentID:    "mecontrola-agent",
							Message:    "Qual o clima em São Paulo?",
							MessageID:  "msg-001",
						}).
						Return(agent.Outcome{
							RunID:   runID,
							Content: "Em São Paulo está 25°C, parcialmente nublado.",
							Status:  agent.RunStatusSucceeded,
							Mode:    agent.ExecutionModeSync,
						}, nil).
						Once()
					return s.runtimeMock
				}(),
			},
			expect: func(outcome agent.Outcome, err error) {
				s.NoError(err)
				s.Equal(runID, outcome.RunID)
				s.Equal(agent.RunStatusSucceeded, outcome.Status)
				s.NotEmpty(outcome.Content)
			},
		},
		{
			name: "deve retornar erro quando resource_id ausente",
			args: args{
				in: input.InboundInput{
					ThreadID: "thread-abc",
					AgentID:  "mecontrola-agent",
					Message:  "Qual o clima?",
				},
			},
			dependencies: dependencies{runtimeMock: s.runtimeMock},
			expect: func(outcome agent.Outcome, err error) {
				s.Error(err)
				s.Equal(agent.Outcome{}, outcome)
			},
		},
		{
			name: "deve retornar erro quando thread_id ausente",
			args: args{
				in: input.InboundInput{
					ResourceID: "user-123",
					AgentID:    "mecontrola-agent",
					Message:    "Qual o clima?",
				},
			},
			dependencies: dependencies{runtimeMock: s.runtimeMock},
			expect: func(outcome agent.Outcome, err error) {
				s.Error(err)
				s.Equal(agent.Outcome{}, outcome)
			},
		},
		{
			name: "deve retornar erro quando agent_id ausente",
			args: args{
				in: input.InboundInput{
					ResourceID: "user-123",
					ThreadID:   "thread-abc",
					Message:    "Qual o clima?",
				},
			},
			dependencies: dependencies{runtimeMock: s.runtimeMock},
			expect: func(outcome agent.Outcome, err error) {
				s.Error(err)
				s.Equal(agent.Outcome{}, outcome)
			},
		},
		{
			name: "deve retornar erro quando message ausente",
			args: args{
				in: input.InboundInput{
					ResourceID: "user-123",
					ThreadID:   "thread-abc",
					AgentID:    "mecontrola-agent",
				},
			},
			dependencies: dependencies{runtimeMock: s.runtimeMock},
			expect: func(outcome agent.Outcome, err error) {
				s.Error(err)
				s.Equal(agent.Outcome{}, outcome)
			},
		},
		{
			name: "deve retornar erro quando runtime falha",
			args: args{
				in: input.InboundInput{
					ResourceID: "user-123",
					ThreadID:   "thread-abc",
					AgentID:    "mecontrola-agent",
					Message:    "Qual o clima?",
				},
			},
			dependencies: dependencies{
				runtimeMock: func() *agentmocks.AgentRuntime {
					s.runtimeMock.EXPECT().
						Execute(mock.Anything, agent.InboundRequest{
							ResourceID: "user-123",
							ThreadID:   "thread-abc",
							AgentID:    "mecontrola-agent",
							Message:    "Qual o clima?",
						}).
						Return(agent.Outcome{}, errors.New("runtime failure")).
						Once()
					return s.runtimeMock
				}(),
			},
			expect: func(outcome agent.Outcome, err error) {
				s.Error(err)
				s.Equal(agent.Outcome{}, outcome)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewHandleInbound(scenario.dependencies.runtimeMock, s.obs)
			outcome, err := uc.Execute(s.ctx, scenario.args.in)
			scenario.expect(outcome, err)
		})
	}
}

package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type mockPendingEntryEngine struct {
	mock.Mock
}

func (m *mockPendingEntryEngine) Start(ctx context.Context, def workflow.Definition[workflows.PendingEntryState], key string, initial workflows.PendingEntryState) (workflow.RunResult[workflows.PendingEntryState], error) {
	args := m.Called(ctx, def, key, initial)
	return args.Get(0).(workflow.RunResult[workflows.PendingEntryState]), args.Error(1)
}

func (m *mockPendingEntryEngine) Resume(ctx context.Context, def workflow.Definition[workflows.PendingEntryState], key string, resume []byte) (workflow.RunResult[workflows.PendingEntryState], error) {
	args := m.Called(ctx, def, key, resume)
	return args.Get(0).(workflow.RunResult[workflows.PendingEntryState]), args.Error(1)
}

type PendingEntryContinuerSuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	emptyDef   workflow.Definition[workflows.PendingEntryState]
	engineMock *mockPendingEntryEngine
}

func TestPendingEntryContinuerSuite(t *testing.T) {
	suite.Run(t, new(PendingEntryContinuerSuite))
}

func (s *PendingEntryContinuerSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.emptyDef = workflow.Definition[workflows.PendingEntryState]{}
	s.engineMock = &mockPendingEntryEngine{}
	s.engineMock.Test(s.T())
	s.T().Cleanup(func() { s.engineMock.AssertExpectations(s.T()) })
}

func (s *PendingEntryContinuerSuite) TestContinue() {
	type args struct {
		userID    string
		peer      string
		message   string
		messageID string
	}
	type dependencies struct {
		engine *mockPendingEntryEngine
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result workflows.PendingEntryResult, err error)
	}{
		{
			name: "deve retornar Handled=false quando nao ha run suspenso",
			args: args{userID: "user-1", peer: "+5511999999999", message: "sim", messageID: "wamid-001"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-1:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.False(result.Handled)
			},
		},
		{
			name: "deve retornar erro quando engine falha",
			args: args{userID: "user-2", peer: "+5511999999999", message: "sim", messageID: "wamid-002"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-2:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{}, errors.New("engine error")).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.Error(err)
				s.Contains(err.Error(), "pending_entry_continuer")
			},
		},
		{
			name: "deve retornar Handled=true com modo replied quando run ainda suspenso",
			args: args{userID: "user-3", peer: "+5511999999999", message: "pix", messageID: "wamid-003"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-3:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{
							Status: workflow.RunStatusSuspended,
							State: workflows.PendingEntryState{
								Awaiting: workflows.AwaitingSlotConfirmation,
							},
							Suspend: &workflow.Suspension{
								Reason: workflow.SuspendAwaitingInput,
								Prompt: "Confirma o lançamento?",
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.Equal(workflows.PendingEntryModeReplied, result.Mode)
				s.Equal("Confirma o lançamento?", result.Message)
			},
		},
		{
			name: "deve retornar Handled=true com modo completed quando escrita bem sucedida",
			args: args{userID: "user-4", peer: "+5511999999999", message: "sim", messageID: "wamid-004"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-4:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.PendingEntryState{
								Status:       workflows.PendingStatusCompleted,
								ResponseText: "Despesa de R$ 50,00 registrada ✅",
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.Equal(workflows.PendingEntryModeCompleted, result.Mode)
				s.Equal("Despesa de R$ 50,00 registrada ✅", result.Message)
			},
		},
		{
			name: "deve retornar Handled=false com modo replaced quando mensagem e nova operacao",
			args: args{userID: "user-5", peer: "+5511999999999", message: "gastei 100 no mercado", messageID: "wamid-005"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-5:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.PendingEntryState{
								Status: workflows.PendingStatusReplaced,
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.False(result.Handled)
				s.Equal(workflows.PendingEntryModeReplaced, result.Mode)
			},
		},
		{
			name: "deve retornar Handled=true com modo cancelled quando run cancelado",
			args: args{userID: "user-6", peer: "+5511999999999", message: "não", messageID: "wamid-006"},
			dependencies: dependencies{
				engine: func() *mockPendingEntryEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-6:+5511999999999:pending-entry", mock.Anything).
						Return(workflow.RunResult[workflows.PendingEntryState]{
							Status: workflow.RunStatusSucceeded,
							State: workflows.PendingEntryState{
								Status:       workflows.PendingStatusCancelled,
								ResponseText: "Tudo certo, o registro foi cancelado.",
							},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(result workflows.PendingEntryResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.Equal(workflows.PendingEntryModeCancelled, result.Mode)
				s.Equal("Tudo certo, o registro foi cancelado.", result.Message)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewPendingEntryContinuer(scenario.dependencies.engine, s.emptyDef, s.obs)
			result, err := uc.Continue(s.ctx, scenario.args.userID, scenario.args.peer, scenario.args.message, scenario.args.messageID)
			scenario.expect(result, err)
		})
	}
}

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

type mockConfirmEngine struct {
	mock.Mock
}

func (m *mockConfirmEngine) Start(ctx context.Context, def workflow.Definition[workflows.ConfirmState], key string, initial workflows.ConfirmState) (workflow.RunResult[workflows.ConfirmState], error) {
	args := m.Called(ctx, def, key, initial)
	return args.Get(0).(workflow.RunResult[workflows.ConfirmState]), args.Error(1)
}

func (m *mockConfirmEngine) Resume(ctx context.Context, def workflow.Definition[workflows.ConfirmState], key string, resume []byte) (workflow.RunResult[workflows.ConfirmState], error) {
	args := m.Called(ctx, def, key, resume)
	return args.Get(0).(workflow.RunResult[workflows.ConfirmState]), args.Error(1)
}

type DestructiveConfirmContinuerSuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	emptyDef   workflow.Definition[workflows.ConfirmState]
	engineMock *mockConfirmEngine
}

func TestDestructiveConfirmContinuerSuite(t *testing.T) {
	suite.Run(t, new(DestructiveConfirmContinuerSuite))
}

func (s *DestructiveConfirmContinuerSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.emptyDef = workflow.Definition[workflows.ConfirmState]{}
	s.engineMock = &mockConfirmEngine{}
	s.engineMock.Test(s.T())
	s.T().Cleanup(func() { s.engineMock.AssertExpectations(s.T()) })
}

func (s *DestructiveConfirmContinuerSuite) TestContinue() {
	type args struct {
		userID  string
		message string
	}
	type dependencies struct {
		engine *mockConfirmEngine
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(handled bool, reply string, err error)
	}{
		{
			name: "deve retornar nao handled quando nao ha run suspenso",
			args: args{userID: "user-1", message: "sim"},
			dependencies: dependencies{
				engine: func() *mockConfirmEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-1:confirm", mock.Anything).
						Return(workflow.RunResult[workflows.ConfirmState]{}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.False(handled)
				s.Empty(reply)
			},
		},
		{
			name: "deve retornar erro quando engine falha",
			args: args{userID: "user-2", message: "sim"},
			dependencies: dependencies{
				engine: func() *mockConfirmEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-2:confirm", mock.Anything).
						Return(workflow.RunResult[workflows.ConfirmState]{}, errors.New("engine error")).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.Error(err)
				s.Contains(err.Error(), "destructive_confirm_continuer")
			},
		},
		{
			name: "deve retornar handled e reply quando run existe",
			args: args{userID: "user-3", message: "sim"},
			dependencies: dependencies{
				engine: func() *mockConfirmEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-3:confirm", mock.Anything).
						Return(workflow.RunResult[workflows.ConfirmState]{
							Status: workflow.RunStatusSucceeded,
							State:  workflows.ConfirmState{ResponseText: "Feito!"},
						}, nil).Once()
					return s.engineMock
				}(),
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.True(handled)
				s.Equal("Feito!", reply)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewDestructiveConfirmContinuer(scenario.dependencies.engine, s.emptyDef, s.obs)
			handled, reply, err := uc.Continue(s.ctx, scenario.args.userID, scenario.args.message)
			scenario.expect(handled, reply, err)
		})
	}
}

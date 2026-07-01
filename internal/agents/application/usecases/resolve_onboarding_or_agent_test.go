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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	memorymocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type mockOnboardingEngine struct {
	mock.Mock
}

func (m *mockOnboardingEngine) Start(ctx context.Context, def workflow.Definition[workflows.OnboardingState], key string, initial workflows.OnboardingState) (workflow.RunResult[workflows.OnboardingState], error) {
	args := m.Called(ctx, def, key, initial)
	return args.Get(0).(workflow.RunResult[workflows.OnboardingState]), args.Error(1)
}

func (m *mockOnboardingEngine) Resume(ctx context.Context, def workflow.Definition[workflows.OnboardingState], key string, resume []byte) (workflow.RunResult[workflows.OnboardingState], error) {
	args := m.Called(ctx, def, key, resume)
	return args.Get(0).(workflow.RunResult[workflows.OnboardingState]), args.Error(1)
}

type mockOnboardingStore struct {
	mock.Mock
}

func (m *mockOnboardingStore) Load(ctx context.Context, workflowID, key string) (workflow.Snapshot, bool, error) {
	args := m.Called(ctx, workflowID, key)
	return args.Get(0).(workflow.Snapshot), args.Bool(1), args.Error(2)
}

type ResolveOnboardingOrAgentSuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	emptyDef   workflow.Definition[workflows.OnboardingState]
	engineMock *mockOnboardingEngine
	storeMock  *mockOnboardingStore
	wmMock     *memorymocks.WorkingMemory
}

func TestResolveOnboardingOrAgentSuite(t *testing.T) {
	suite.Run(t, new(ResolveOnboardingOrAgentSuite))
}

func (s *ResolveOnboardingOrAgentSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.emptyDef = workflow.Definition[workflows.OnboardingState]{}
	s.engineMock = &mockOnboardingEngine{}
	s.engineMock.Test(s.T())
	s.T().Cleanup(func() { s.engineMock.AssertExpectations(s.T()) })
	s.storeMock = &mockOnboardingStore{}
	s.storeMock.Test(s.T())
	s.T().Cleanup(func() { s.storeMock.AssertExpectations(s.T()) })
	s.wmMock = memorymocks.NewWorkingMemory(s.T())
}

func (s *ResolveOnboardingOrAgentSuite) TestExecute() {
	type args struct {
		userID  string
		message string
	}
	type dependencies struct {
		engine *mockOnboardingEngine
		store  *mockOnboardingStore
		wm     *memorymocks.WorkingMemory
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result OnboardingResult, err error)
	}{
		{
			name: "run suspenso existente deve resumir e retornar handled com prompt",
			args: args{userID: "user-1", message: "continuar"},
			dependencies: dependencies{
				engine: func() *mockOnboardingEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-1", mock.Anything).
						Return(workflow.RunResult[workflows.OnboardingState]{
							Status:  workflow.RunStatusSuspended,
							Suspend: &workflow.Suspension{Prompt: "proximo passo"},
						}, nil).Once()
					return s.engineMock
				}(),
				store: func() *mockOnboardingStore {
					s.storeMock.On("Load", mock.Anything, mock.Anything, "user-1").
						Return(workflow.Snapshot{Status: workflow.RunStatusSuspended}, true, nil).Once()
					return s.storeMock
				}(),
				wm: s.wmMock,
			},
			expect: func(result OnboardingResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.False(result.Done)
				s.Equal("proximo passo", result.Message)
			},
		},
		{
			name: "resume que conclui workflow deve retornar handled done com finalMessage",
			args: args{userID: "user-2", message: "sim"},
			dependencies: dependencies{
				engine: func() *mockOnboardingEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-2", mock.Anything).
						Return(workflow.RunResult[workflows.OnboardingState]{
							Status: workflow.RunStatusSucceeded,
							State:  workflows.OnboardingState{FinalMessage: "parabens!"},
						}, nil).Once()
					return s.engineMock
				}(),
				store: func() *mockOnboardingStore {
					s.storeMock.On("Load", mock.Anything, mock.Anything, "user-2").
						Return(workflow.Snapshot{Status: workflow.RunStatusSuspended}, true, nil).Once()
					return s.storeMock
				}(),
				wm: s.wmMock,
			},
			expect: func(result OnboardingResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.True(result.Done)
				s.Equal("parabens!", result.Message)
			},
		},
		{
			name: "sem run ativo e WM tem objetivo deve retornar nao handled",
			args: args{userID: "user-3", message: "oi"},
			dependencies: dependencies{
				engine: s.engineMock,
				store: func() *mockOnboardingStore {
					s.storeMock.On("Load", mock.Anything, mock.Anything, "user-3").
						Return(workflow.Snapshot{}, false, nil).Once()
					return s.storeMock
				}(),
				wm: func() *memorymocks.WorkingMemory {
					s.wmMock.EXPECT().Get(mock.Anything, "user-3").
						Return("## Objetivo Financeiro\n\neconomizar", nil).Once()
					return s.wmMock
				}(),
			},
			expect: func(result OnboardingResult, err error) {
				s.NoError(err)
				s.False(result.Handled)
			},
		},
		{
			name: "sem run ativo e sem objetivo deve iniciar workflow",
			args: args{userID: "user-4", message: "oi"},
			dependencies: dependencies{
				engine: func() *mockOnboardingEngine {
					s.engineMock.On("Start", mock.Anything, mock.Anything, "user-4", mock.Anything).
						Return(workflow.RunResult[workflows.OnboardingState]{
							Status:  workflow.RunStatusSuspended,
							Suspend: &workflow.Suspension{Prompt: "bem-vindo!"},
						}, nil).Once()
					return s.engineMock
				}(),
				store: func() *mockOnboardingStore {
					s.storeMock.On("Load", mock.Anything, mock.Anything, "user-4").
						Return(workflow.Snapshot{}, false, nil).Once()
					return s.storeMock
				}(),
				wm: func() *memorymocks.WorkingMemory {
					s.wmMock.EXPECT().Get(mock.Anything, "user-4").Return("", nil).Once()
					return s.wmMock
				}(),
			},
			expect: func(result OnboardingResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.False(result.Done)
				s.Equal("bem-vindo!", result.Message)
			},
		},
		{
			name: "sem run ativo e WM inexistente deve iniciar workflow",
			args: args{userID: "user-9", message: "quero comecar"},
			dependencies: dependencies{
				engine: func() *mockOnboardingEngine {
					s.engineMock.On("Start", mock.Anything, mock.Anything, "user-9", mock.Anything).
						Return(workflow.RunResult[workflows.OnboardingState]{
							Status:  workflow.RunStatusSuspended,
							Suspend: &workflow.Suspension{Prompt: "bem-vindo!"},
						}, nil).Once()
					return s.engineMock
				}(),
				store: func() *mockOnboardingStore {
					s.storeMock.On("Load", mock.Anything, mock.Anything, "user-9").
						Return(workflow.Snapshot{}, false, nil).Once()
					return s.storeMock
				}(),
				wm: func() *memorymocks.WorkingMemory {
					s.wmMock.EXPECT().Get(mock.Anything, "user-9").
						Return("", memory.ErrWorkingMemoryNotFound).Once()
					return s.wmMock
				}(),
			},
			expect: func(result OnboardingResult, err error) {
				s.NoError(err)
				s.True(result.Handled)
				s.False(result.Done)
				s.Equal("bem-vindo!", result.Message)
			},
		},
		{
			name: "erro no load deve retornar erro",
			args: args{userID: "user-5", message: "oi"},
			dependencies: dependencies{
				engine: s.engineMock,
				store: func() *mockOnboardingStore {
					s.storeMock.On("Load", mock.Anything, mock.Anything, "user-5").
						Return(workflow.Snapshot{}, false, errors.New("db error")).Once()
					return s.storeMock
				}(),
				wm: s.wmMock,
			},
			expect: func(result OnboardingResult, err error) {
				s.Error(err)
				s.False(result.Handled)
			},
		},
		{
			name: "erro no resume deve retornar erro",
			args: args{userID: "user-6", message: "sim"},
			dependencies: dependencies{
				engine: func() *mockOnboardingEngine {
					s.engineMock.On("Resume", mock.Anything, mock.Anything, "user-6", mock.Anything).
						Return(workflow.RunResult[workflows.OnboardingState]{}, errors.New("resume error")).Once()
					return s.engineMock
				}(),
				store: func() *mockOnboardingStore {
					s.storeMock.On("Load", mock.Anything, mock.Anything, "user-6").
						Return(workflow.Snapshot{Status: workflow.RunStatusSuspended}, true, nil).Once()
					return s.storeMock
				}(),
				wm: s.wmMock,
			},
			expect: func(result OnboardingResult, err error) {
				s.Error(err)
				s.False(result.Handled)
			},
		},
		{
			name: "erro no start deve retornar erro",
			args: args{userID: "user-7", message: "oi"},
			dependencies: dependencies{
				engine: func() *mockOnboardingEngine {
					s.engineMock.On("Start", mock.Anything, mock.Anything, "user-7", mock.Anything).
						Return(workflow.RunResult[workflows.OnboardingState]{}, errors.New("start error")).Once()
					return s.engineMock
				}(),
				store: func() *mockOnboardingStore {
					s.storeMock.On("Load", mock.Anything, mock.Anything, "user-7").
						Return(workflow.Snapshot{}, false, nil).Once()
					return s.storeMock
				}(),
				wm: func() *memorymocks.WorkingMemory {
					s.wmMock.EXPECT().Get(mock.Anything, "user-7").Return("", nil).Once()
					return s.wmMock
				}(),
			},
			expect: func(result OnboardingResult, err error) {
				s.Error(err)
				s.False(result.Handled)
			},
		},
		{
			name: "erro no get WM deve retornar erro",
			args: args{userID: "user-8", message: "oi"},
			dependencies: dependencies{
				engine: s.engineMock,
				store: func() *mockOnboardingStore {
					s.storeMock.On("Load", mock.Anything, mock.Anything, "user-8").
						Return(workflow.Snapshot{}, false, nil).Once()
					return s.storeMock
				}(),
				wm: func() *memorymocks.WorkingMemory {
					s.wmMock.EXPECT().Get(mock.Anything, "user-8").
						Return("", errors.New("wm error")).Once()
					return s.wmMock
				}(),
			},
			expect: func(result OnboardingResult, err error) {
				s.Error(err)
				s.False(result.Handled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewResolveOnboardingOrAgent(scenario.dependencies.engine, scenario.dependencies.store, scenario.dependencies.wm, s.emptyDef, s.obs)
			result, err := uc.Execute(s.ctx, scenario.args.userID, scenario.args.message)
			scenario.expect(result, err)
		})
	}
}

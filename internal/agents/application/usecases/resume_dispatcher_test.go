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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeDispatcherThreadGateway struct {
	thread memory.Thread
	err    error
}

func (f *fakeDispatcherThreadGateway) GetOrCreate(_ context.Context, _, _ string) (memory.Thread, error) {
	return f.thread, f.err
}

type fakeDispatcherRunStore struct {
	insertErr error
	updateErr error
	inserted  []agent.Run
	updated   []agent.Run
}

func (f *fakeDispatcherRunStore) Insert(_ context.Context, run agent.Run) error {
	f.inserted = append(f.inserted, run)
	return f.insertErr
}

func (f *fakeDispatcherRunStore) Update(_ context.Context, run agent.Run) error {
	f.updated = append(f.updated, run)
	return f.updateErr
}

func (f *fakeDispatcherRunStore) Load(_ context.Context, _ uuid.UUID) (agent.Run, error) {
	return agent.Run{}, nil
}

type mockWorkflowResumer struct {
	mock.Mock
	workflowID string
}

func (m *mockWorkflowResumer) WorkflowID() string { return m.workflowID }

func (m *mockWorkflowResumer) Resume(ctx context.Context, resourceID, threadID, message, messageID string) (bool, string, error) {
	args := m.Called(ctx, resourceID, threadID, message, messageID)
	return args.Bool(0), args.String(1), args.Error(2)
}

type ResumeDispatcherSuite struct {
	suite.Suite
	ctx     context.Context
	obs     observability.Observability
	threads *fakeDispatcherThreadGateway
	runs    *fakeDispatcherRunStore
}

func TestResumeDispatcherSuite(t *testing.T) {
	suite.Run(t, new(ResumeDispatcherSuite))
}

func (s *ResumeDispatcherSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.threads = &fakeDispatcherThreadGateway{thread: memory.Thread{ID: uuid.New()}}
	s.runs = &fakeDispatcherRunStore{}
}

func (s *ResumeDispatcherSuite) TestNewResumeDispatcher_ErroQuandoWorkflowDuplicado() {
	r1 := &mockWorkflowResumer{workflowID: "transaction-write"}
	r2 := &mockWorkflowResumer{workflowID: "transaction-write"}

	store := newFakeSuspendedRunStore()
	index := NewSuspendedRunIndex(store, "transaction-write")

	dispatcher, err := NewResumeDispatcher(index, s.threads, s.runs, s.obs, r1, r2)
	s.Error(err)
	s.Nil(dispatcher)
}

func (s *ResumeDispatcherSuite) TestContinue() {
	type args struct {
		resourceID string
		threadID   string
	}
	type dependencies struct {
		store            *fakeSuspendedRunStore
		indexWorkflowIDs []string
		resumers         []WorkflowResumer
		threads          *fakeDispatcherThreadGateway
		runs             *fakeDispatcherRunStore
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(handled bool, reply string, err error)
	}{
		{
			name: "deve retornar handled=false quando nao ha run suspenso",
			args: args{resourceID: "user-0", threadID: "+5511"},
			dependencies: dependencies{
				store:            newFakeSuspendedRunStore(),
				indexWorkflowIDs: []string{"transaction-write"},
				resumers:         []WorkflowResumer{&mockWorkflowResumer{workflowID: "transaction-write"}},
				threads:          &fakeDispatcherThreadGateway{thread: memory.Thread{ID: uuid.New()}},
				runs:             &fakeDispatcherRunStore{},
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.False(handled)
				s.Empty(reply)
			},
		},
		{
			name: "deve despachar para o resumer correto quando ha run suspenso",
			args: args{resourceID: "user-1", threadID: "+5511"},
			dependencies: dependencies{
				store: func() *fakeSuspendedRunStore {
					st := newFakeSuspendedRunStore()
					st.put("card-manage", "user-1:+5511:card-manage", workflow.RunStatusSuspended)
					return st
				}(),
				indexWorkflowIDs: []string{"card-manage"},
				resumers: []WorkflowResumer{
					func() *mockWorkflowResumer {
						m := &mockWorkflowResumer{workflowID: "card-manage"}
						m.On("Resume", mock.Anything, "user-1", "+5511", "sim", mock.Anything).
							Return(true, "✅ Cartão atualizado.", nil).Once()
						return m
					}(),
				},
				threads: &fakeDispatcherThreadGateway{thread: memory.Thread{ID: uuid.New()}},
				runs:    &fakeDispatcherRunStore{},
			},
			expect: func(handled bool, reply string, err error) {
				s.NoError(err)
				s.True(handled)
				s.Equal("✅ Cartão atualizado.", reply)
			},
		},
		{
			name: "deve retornar erro quando o indice falha",
			args: args{resourceID: "user-x", threadID: "+5511"},
			dependencies: dependencies{
				store: func() *fakeSuspendedRunStore {
					st := newFakeSuspendedRunStore()
					st.loadErr = errors.New("db indisponivel")
					return st
				}(),
				indexWorkflowIDs: []string{"card-manage"},
				resumers:         []WorkflowResumer{&mockWorkflowResumer{workflowID: "card-manage"}},
				threads:          &fakeDispatcherThreadGateway{thread: memory.Thread{ID: uuid.New()}},
				runs:             &fakeDispatcherRunStore{},
			},
			expect: func(handled bool, reply string, err error) {
				s.Error(err)
				s.False(handled)
				s.Empty(reply)
			},
		},
		{
			name: "deve retornar erro quando workflow suspenso nao tem resumer registrado",
			args: args{resourceID: "user-2", threadID: "+5511"},
			dependencies: dependencies{
				store: func() *fakeSuspendedRunStore {
					st := newFakeSuspendedRunStore()
					st.put("goal-edit", "user-2:+5511:goal-edit", workflow.RunStatusSuspended)
					return st
				}(),
				indexWorkflowIDs: []string{"goal-edit", "card-manage"},
				resumers:         []WorkflowResumer{&mockWorkflowResumer{workflowID: "card-manage"}},
				threads:          &fakeDispatcherThreadGateway{thread: memory.Thread{ID: uuid.New()}},
				runs:             &fakeDispatcherRunStore{},
			},
			expect: func(handled bool, reply string, err error) {
				s.Error(err)
				s.True(errors.Is(err, ErrUnknownSuspendedWorkflow))
				s.False(handled)
			},
		},
		{
			name: "deve retornar erro quando o resumer falha",
			args: args{resourceID: "user-3", threadID: "+5511"},
			dependencies: dependencies{
				store: func() *fakeSuspendedRunStore {
					st := newFakeSuspendedRunStore()
					st.put("budget-manage", "user-3:+5511:budget-manage", workflow.RunStatusSuspended)
					return st
				}(),
				indexWorkflowIDs: []string{"budget-manage"},
				resumers: []WorkflowResumer{
					func() *mockWorkflowResumer {
						m := &mockWorkflowResumer{workflowID: "budget-manage"}
						m.On("Resume", mock.Anything, "user-3", "+5511", "sim", mock.Anything).
							Return(false, "", errors.New("engine falhou")).Once()
						return m
					}(),
				},
				threads: &fakeDispatcherThreadGateway{thread: memory.Thread{ID: uuid.New()}},
				runs:    &fakeDispatcherRunStore{},
			},
			expect: func(handled bool, reply string, err error) {
				s.Error(err)
				s.False(handled)
				s.Empty(reply)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			index := NewSuspendedRunIndex(scenario.dependencies.store, scenario.dependencies.indexWorkflowIDs...)
			dispatcher, err := NewResumeDispatcher(index, scenario.dependencies.threads, scenario.dependencies.runs, s.obs, scenario.dependencies.resumers...)
			s.Require().NoError(err)

			handled, reply, dispatchErr := dispatcher.Continue(s.ctx, scenario.args.resourceID, scenario.args.threadID, "sim", "wamid-001")
			scenario.expect(handled, reply, dispatchErr)
		})
	}
}

func (s *ResumeDispatcherSuite) TestContinue_InjetaIdentidadeInboundNoContexto() {
	store := newFakeSuspendedRunStore()
	store.put("transaction-write", "user-6:+5511:transaction-write", workflow.RunStatusSuspended)
	index := NewSuspendedRunIndex(store, "transaction-write")

	var (
		gotResourceID string
		gotMessageID  string
		gotOK         bool
	)
	resumer := &mockWorkflowResumer{workflowID: "transaction-write"}
	resumer.On("Resume", mock.Anything, "user-6", "+5511", "sim", mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			gotResourceID, gotMessageID, _, gotOK = agent.InboundIdentityFromContext(ctx)
		}).
		Return(true, "✅ Prontinho.", nil).Once()

	dispatcher, err := NewResumeDispatcher(index, s.threads, s.runs, s.obs, resumer)
	s.Require().NoError(err)

	handled, reply, dispatchErr := dispatcher.Continue(s.ctx, "user-6", "+5511", "sim", "wamid-006")
	s.NoError(dispatchErr)
	s.True(handled)
	s.Equal("✅ Prontinho.", reply)

	s.True(gotOK)
	s.Equal("user-6", gotResourceID)
	s.Equal("wamid-006", gotMessageID)
}

func (s *ResumeDispatcherSuite) TestContinue_AbreEFechaRunAuditavel() {
	store := newFakeSuspendedRunStore()
	store.put("card-manage", "user-4:+5511:card-manage", workflow.RunStatusSuspended)
	index := NewSuspendedRunIndex(store, "card-manage")

	resumer := &mockWorkflowResumer{workflowID: "card-manage"}
	resumer.On("Resume", mock.Anything, "user-4", "+5511", "sim", mock.Anything).
		Return(true, "✅ Cartão atualizado.", nil).Once()

	dispatcher, err := NewResumeDispatcher(index, s.threads, s.runs, s.obs, resumer)
	s.Require().NoError(err)

	_, _, dispatchErr := dispatcher.Continue(s.ctx, "user-4", "+5511", "sim", "wamid-002")
	s.NoError(dispatchErr)

	s.Require().Len(s.runs.inserted, 1)
	s.Equal(agent.RunStatusRunning, s.runs.inserted[0].Status)
	s.Equal("card-manage", s.runs.inserted[0].Workflow)
	s.Require().Len(s.runs.updated, 1)
	s.Equal(agent.RunStatusSucceeded, s.runs.updated[0].Status)
}

func (s *ResumeDispatcherSuite) TestContinue_FalhaAoAbrirThreadNaoAbreRun() {
	store := newFakeSuspendedRunStore()
	store.put("card-manage", "user-5:+5511:card-manage", workflow.RunStatusSuspended)
	index := NewSuspendedRunIndex(store, "card-manage")

	resumer := &mockWorkflowResumer{workflowID: "card-manage"}

	s.threads.err = errors.New("thread store down")

	dispatcher, err := NewResumeDispatcher(index, s.threads, s.runs, s.obs, resumer)
	s.Require().NoError(err)

	handled, reply, dispatchErr := dispatcher.Continue(s.ctx, "user-5", "+5511", "sim", "wamid-003")
	s.Error(dispatchErr)
	s.False(handled)
	s.Empty(reply)
	s.Empty(s.runs.inserted)
	resumer.AssertNotCalled(s.T(), "Resume", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

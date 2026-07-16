package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeBudgetManageEngine struct {
	startResult wf.RunResult[workflows.BudgetManageState]
	startErr    error
	startCalled bool
	lastState   workflows.BudgetManageState
}

func (f *fakeBudgetManageEngine) Start(_ context.Context, _ wf.Definition[workflows.BudgetManageState], _ string, initial workflows.BudgetManageState) (wf.RunResult[workflows.BudgetManageState], error) {
	f.startCalled = true
	f.lastState = initial
	return f.startResult, f.startErr
}

func (f *fakeBudgetManageEngine) Resume(_ context.Context, _ wf.Definition[workflows.BudgetManageState], _ string, _ []byte) (wf.RunResult[workflows.BudgetManageState], error) {
	return wf.RunResult[workflows.BudgetManageState]{}, nil
}

func (f *fakeBudgetManageEngine) LoadLatestState(_ context.Context, _ wf.Definition[workflows.BudgetManageState], _ string) (workflows.BudgetManageState, wf.Snapshot, bool, error) {
	return workflows.BudgetManageState{}, wf.Snapshot{}, false, nil
}

func newFakeBudgetManageEngine() *fakeBudgetManageEngine {
	return &fakeBudgetManageEngine{
		startResult: wf.RunResult[workflows.BudgetManageState]{
			Status: wf.RunStatusSuspended,
			State: workflows.BudgetManageState{
				ResponseText: "Vamos criar seu orçamento. Qual é o valor total (em R$)?",
			},
		},
	}
}

func fakeBudgetManageDef() wf.Definition[workflows.BudgetManageState] {
	return wf.Definition[workflows.BudgetManageState]{
		ID:      workflows.BudgetManageWorkflowID,
		Durable: true,
	}
}

var testBudgetCreationUserID = uuid.MustParse("00000000-0000-0000-0000-000000000031")

func budgetCreationInboundCtx(messageID string) context.Context {
	req := agent.InboundRequest{
		ResourceID: testBudgetCreationUserID.String(),
		ThreadID:   "thread-1",
		AgentID:    "mecontrola-agent",
		Message:    "quero criar um orçamento",
		MessageID:  messageID,
	}
	return wf.WithRuntime(context.Background(), req)
}

type CreateBudgetToolSuite struct {
	suite.Suite
	engine *fakeBudgetManageEngine
}

func TestCreateBudgetToolSuite(t *testing.T) {
	suite.Run(t, new(CreateBudgetToolSuite))
}

func (s *CreateBudgetToolSuite) SetupTest() {
	s.engine = newFakeBudgetManageEngine()
}

func (s *CreateBudgetToolSuite) TestExecute() {
	type args struct {
		ctx   context.Context
		input CreateBudgetToolInput
	}
	type dependencies struct {
		engine *fakeBudgetManageEngine
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(engine *fakeBudgetManageEngine, output CreateBudgetToolOutput, err error)
	}{
		{
			name: "deve iniciar o workflow para mês corrente",
			args: args{
				ctx:   budgetCreationInboundCtx("wamid-1"),
				input: CreateBudgetToolInput{MonthRefKind: "current"},
			},
			dependencies: dependencies{
				engine: func() *fakeBudgetManageEngine {
					return newFakeBudgetManageEngine()
				}(),
			},
			expect: func(engine *fakeBudgetManageEngine, output CreateBudgetToolOutput, err error) {
				s.NoError(err)
				s.Equal(createBudgetOutcomeStarted, output.Outcome)
				s.NotEmpty(output.Competence)
				s.NotEmpty(output.ConfirmationPrompt)
				s.True(engine.startCalled)
				s.Equal(testBudgetCreationUserID, engine.lastState.UserID)
				s.Equal(workflows.BudgetManageOpCreateRetroactive, engine.lastState.Operation)
				s.Equal(workflows.BudgetManageActive, engine.lastState.Status)
			},
		},
		{
			name: "deve iniciar o workflow para mês explícito retroativo",
			args: args{
				ctx:   budgetCreationInboundCtx("wamid-2"),
				input: CreateBudgetToolInput{MonthRefKind: "explicit", Year: 2026, Month: 6},
			},
			dependencies: dependencies{
				engine: func() *fakeBudgetManageEngine {
					return newFakeBudgetManageEngine()
				}(),
			},
			expect: func(engine *fakeBudgetManageEngine, output CreateBudgetToolOutput, err error) {
				s.NoError(err)
				s.Equal(createBudgetOutcomeStarted, output.Outcome)
				s.Equal("2026-06", output.Competence)
				s.True(engine.startCalled)
				s.Equal("2026-06", engine.lastState.Competence)
			},
		},
		{
			name: "mês nomeado sem ano retorna clarify sem iniciar workflow",
			args: args{
				ctx:   budgetCreationInboundCtx("wamid-3"),
				input: CreateBudgetToolInput{MonthRefKind: "named_without_year"},
			},
			dependencies: dependencies{
				engine: func() *fakeBudgetManageEngine {
					return newFakeBudgetManageEngine()
				}(),
			},
			expect: func(engine *fakeBudgetManageEngine, output CreateBudgetToolOutput, err error) {
				s.NoError(err)
				s.Equal(createBudgetOutcomeClarify, output.Outcome)
				s.NotEmpty(output.ClarifyPrompt)
				s.False(engine.startCalled)
			},
		},
		{
			name: "referência desconhecida retorna clarify sem iniciar workflow",
			args: args{
				ctx:   budgetCreationInboundCtx("wamid-4"),
				input: CreateBudgetToolInput{MonthRefKind: "unknown"},
			},
			dependencies: dependencies{
				engine: func() *fakeBudgetManageEngine {
					return newFakeBudgetManageEngine()
				}(),
			},
			expect: func(engine *fakeBudgetManageEngine, output CreateBudgetToolOutput, err error) {
				s.NoError(err)
				s.Equal(createBudgetOutcomeClarify, output.Outcome)
				s.NotEmpty(output.ClarifyPrompt)
				s.False(engine.startCalled)
			},
		},
		{
			name: "ErrRunAlreadyExists retorna pending_creation_exists",
			args: args{
				ctx:   budgetCreationInboundCtx("wamid-5"),
				input: CreateBudgetToolInput{MonthRefKind: "current"},
			},
			dependencies: dependencies{
				engine: func() *fakeBudgetManageEngine {
					return &fakeBudgetManageEngine{startErr: wf.ErrRunAlreadyExists}
				}(),
			},
			expect: func(engine *fakeBudgetManageEngine, output CreateBudgetToolOutput, err error) {
				s.NoError(err)
				s.Equal(createBudgetOutcomePendingCreationExists, output.Outcome)
				s.NotEmpty(output.ClarifyPrompt)
				s.True(engine.startCalled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			handle := BuildCreateBudgetTool(scenario.dependencies.engine, fakeBudgetManageDef())
			argsJSON, marshalErr := json.Marshal(scenario.args.input)
			s.Require().NoError(marshalErr)

			resultJSON, _, err := handle.Invoke(scenario.args.ctx, argsJSON)

			var output CreateBudgetToolOutput
			if err == nil {
				s.Require().NoError(json.Unmarshal(resultJSON, &output))
			}
			scenario.expect(scenario.dependencies.engine, output, err)
		})
	}
}

func (s *CreateBudgetToolSuite) TestExecute_InputInvalido() {
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetManageDef())
	argsJSON := []byte(`{"monthRefKind":""}`)

	_, _, invokeErr := handle.Invoke(budgetCreationInboundCtx("wamid-6"), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_MonthRefKindInvalido() {
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetManageDef())
	argsJSON := []byte(`{"monthRefKind":"nao-existe"}`)

	_, _, invokeErr := handle.Invoke(budgetCreationInboundCtx("wamid-7"), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_TotalCentsNegativo() {
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetManageDef())
	argsJSON := []byte(`{"monthRefKind":"current","totalCents":-100}`)

	_, _, invokeErr := handle.Invoke(budgetCreationInboundCtx("wamid-8"), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_IdentidadeSempreDeRuntimeFrom() {
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetManageDef())
	argsJSON, err := json.Marshal(CreateBudgetToolInput{MonthRefKind: "current"})
	s.Require().NoError(err)

	_, _, invokeErr := handle.Invoke(context.Background(), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_ResourceIDInvalido() {
	req := agent.InboundRequest{ResourceID: "not-a-uuid", MessageID: "wamid-x"}
	ctx := wf.WithRuntime(context.Background(), req)

	handle := BuildCreateBudgetTool(s.engine, fakeBudgetManageDef())
	argsJSON, err := json.Marshal(CreateBudgetToolInput{MonthRefKind: "current"})
	s.Require().NoError(err)

	_, _, invokeErr := handle.Invoke(ctx, argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_ExplicitCompetenciaInvalida() {
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetManageDef())
	argsJSON, err := json.Marshal(CreateBudgetToolInput{MonthRefKind: "explicit", Year: 2026, Month: 13})
	s.Require().NoError(err)

	_, _, invokeErr := handle.Invoke(budgetCreationInboundCtx("wamid-9"), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

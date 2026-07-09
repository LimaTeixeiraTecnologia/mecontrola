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

type fakeBudgetCreationEngine struct {
	startResult wf.RunResult[workflows.BudgetCreationState]
	startErr    error
	startCalled bool
	lastState   workflows.BudgetCreationState
}

func (f *fakeBudgetCreationEngine) Start(_ context.Context, _ wf.Definition[workflows.BudgetCreationState], _ string, initial workflows.BudgetCreationState) (wf.RunResult[workflows.BudgetCreationState], error) {
	f.startCalled = true
	f.lastState = initial
	return f.startResult, f.startErr
}

func (f *fakeBudgetCreationEngine) Resume(_ context.Context, _ wf.Definition[workflows.BudgetCreationState], _ string, _ []byte) (wf.RunResult[workflows.BudgetCreationState], error) {
	return wf.RunResult[workflows.BudgetCreationState]{}, nil
}

func (f *fakeBudgetCreationEngine) LoadLatestState(_ context.Context, _ wf.Definition[workflows.BudgetCreationState], _ string) (workflows.BudgetCreationState, wf.Snapshot, bool, error) {
	return workflows.BudgetCreationState{}, wf.Snapshot{}, false, nil
}

func newFakeBudgetCreationEngine() *fakeBudgetCreationEngine {
	return &fakeBudgetCreationEngine{
		startResult: wf.RunResult[workflows.BudgetCreationState]{
			Status: wf.RunStatusSuspended,
			State: workflows.BudgetCreationState{
				ResponseText: "Vamos criar seu orçamento. Qual é o valor total (em R$)?",
			},
		},
	}
}

func fakeBudgetCreationDef() wf.Definition[workflows.BudgetCreationState] {
	return wf.Definition[workflows.BudgetCreationState]{
		ID:      workflows.BudgetCreationWorkflowID,
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
	engine *fakeBudgetCreationEngine
}

func TestCreateBudgetToolSuite(t *testing.T) {
	suite.Run(t, new(CreateBudgetToolSuite))
}

func (s *CreateBudgetToolSuite) SetupTest() {
	s.engine = newFakeBudgetCreationEngine()
}

func (s *CreateBudgetToolSuite) TestExecute() {
	type args struct {
		ctx   context.Context
		input CreateBudgetToolInput
	}
	type dependencies struct {
		engine *fakeBudgetCreationEngine
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(engine *fakeBudgetCreationEngine, output CreateBudgetToolOutput, err error)
	}{
		{
			name: "deve iniciar o workflow para mês corrente",
			args: args{
				ctx:   budgetCreationInboundCtx("wamid-1"),
				input: CreateBudgetToolInput{MonthRefKind: "current"},
			},
			dependencies: dependencies{
				engine: func() *fakeBudgetCreationEngine {
					return newFakeBudgetCreationEngine()
				}(),
			},
			expect: func(engine *fakeBudgetCreationEngine, output CreateBudgetToolOutput, err error) {
				s.NoError(err)
				s.Equal(createBudgetOutcomeStarted, output.Outcome)
				s.NotEmpty(output.Competence)
				s.NotEmpty(output.ConfirmationPrompt)
				s.True(engine.startCalled)
				s.Equal(testBudgetCreationUserID, engine.lastState.UserID)
				s.Equal(workflows.AwaitingBudgetTotal, engine.lastState.Awaiting)
				s.Equal(workflows.BudgetCreationActive, engine.lastState.Status)
			},
		},
		{
			name: "deve iniciar o workflow para mês explícito retroativo",
			args: args{
				ctx:   budgetCreationInboundCtx("wamid-2"),
				input: CreateBudgetToolInput{MonthRefKind: "explicit", Year: 2026, Month: 6},
			},
			dependencies: dependencies{
				engine: func() *fakeBudgetCreationEngine {
					return newFakeBudgetCreationEngine()
				}(),
			},
			expect: func(engine *fakeBudgetCreationEngine, output CreateBudgetToolOutput, err error) {
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
				engine: func() *fakeBudgetCreationEngine {
					return newFakeBudgetCreationEngine()
				}(),
			},
			expect: func(engine *fakeBudgetCreationEngine, output CreateBudgetToolOutput, err error) {
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
				engine: func() *fakeBudgetCreationEngine {
					return newFakeBudgetCreationEngine()
				}(),
			},
			expect: func(engine *fakeBudgetCreationEngine, output CreateBudgetToolOutput, err error) {
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
				engine: func() *fakeBudgetCreationEngine {
					return &fakeBudgetCreationEngine{startErr: wf.ErrRunAlreadyExists}
				}(),
			},
			expect: func(engine *fakeBudgetCreationEngine, output CreateBudgetToolOutput, err error) {
				s.NoError(err)
				s.Equal(createBudgetOutcomePendingCreationExists, output.Outcome)
				s.NotEmpty(output.ClarifyPrompt)
				s.True(engine.startCalled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			handle := BuildCreateBudgetTool(scenario.dependencies.engine, fakeBudgetCreationDef())
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
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetCreationDef())
	argsJSON := []byte(`{"monthRefKind":""}`)

	_, _, invokeErr := handle.Invoke(budgetCreationInboundCtx("wamid-6"), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_MonthRefKindInvalido() {
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetCreationDef())
	argsJSON := []byte(`{"monthRefKind":"nao-existe"}`)

	_, _, invokeErr := handle.Invoke(budgetCreationInboundCtx("wamid-7"), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_TotalCentsNegativo() {
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetCreationDef())
	argsJSON := []byte(`{"monthRefKind":"current","totalCents":-100}`)

	_, _, invokeErr := handle.Invoke(budgetCreationInboundCtx("wamid-8"), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_IdentidadeSempreDeRuntimeFrom() {
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetCreationDef())
	argsJSON, err := json.Marshal(CreateBudgetToolInput{MonthRefKind: "current"})
	s.Require().NoError(err)

	_, _, invokeErr := handle.Invoke(context.Background(), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_ResourceIDInvalido() {
	req := agent.InboundRequest{ResourceID: "not-a-uuid", MessageID: "wamid-x"}
	ctx := wf.WithRuntime(context.Background(), req)

	handle := BuildCreateBudgetTool(s.engine, fakeBudgetCreationDef())
	argsJSON, err := json.Marshal(CreateBudgetToolInput{MonthRefKind: "current"})
	s.Require().NoError(err)

	_, _, invokeErr := handle.Invoke(ctx, argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateBudgetToolSuite) TestExecute_ExplicitCompetenciaInvalida() {
	handle := BuildCreateBudgetTool(s.engine, fakeBudgetCreationDef())
	argsJSON, err := json.Marshal(CreateBudgetToolInput{MonthRefKind: "explicit", Year: 2026, Month: 13})
	s.Require().NoError(err)

	_, _, invokeErr := handle.Invoke(budgetCreationInboundCtx("wamid-9"), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

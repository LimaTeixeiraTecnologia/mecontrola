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

type fakeTreatmentNameEditEngine struct {
	startResult wf.RunResult[workflows.TreatmentNameEditState]
	startErr    error
	startCalled bool
	lastState   workflows.TreatmentNameEditState
}

func (f *fakeTreatmentNameEditEngine) Start(_ context.Context, _ wf.Definition[workflows.TreatmentNameEditState], _ string, initial workflows.TreatmentNameEditState) (wf.RunResult[workflows.TreatmentNameEditState], error) {
	f.startCalled = true
	f.lastState = initial
	return f.startResult, f.startErr
}

func (f *fakeTreatmentNameEditEngine) Resume(_ context.Context, _ wf.Definition[workflows.TreatmentNameEditState], _ string, _ []byte) (wf.RunResult[workflows.TreatmentNameEditState], error) {
	return wf.RunResult[workflows.TreatmentNameEditState]{}, nil
}

func (f *fakeTreatmentNameEditEngine) LoadLatestState(_ context.Context, _ wf.Definition[workflows.TreatmentNameEditState], _ string) (workflows.TreatmentNameEditState, wf.Snapshot, bool, error) {
	return workflows.TreatmentNameEditState{}, wf.Snapshot{}, false, nil
}

func newFakeTreatmentNameEditEngine(responseText string) *fakeTreatmentNameEditEngine {
	return &fakeTreatmentNameEditEngine{
		startResult: wf.RunResult[workflows.TreatmentNameEditState]{
			Status: wf.RunStatusSuspended,
			State: workflows.TreatmentNameEditState{
				ResponseText: responseText,
			},
		},
	}
}

func fakeTreatmentNameEditDef() wf.Definition[workflows.TreatmentNameEditState] {
	return wf.Definition[workflows.TreatmentNameEditState]{
		ID:      "treatment-name-edit",
		Durable: true,
	}
}

var testTreatmentNameEditUserID = uuid.MustParse("00000000-0000-0000-0000-000000000041")

func treatmentNameEditInboundCtx(messageID, message string) context.Context {
	req := agent.InboundRequest{
		ResourceID: testTreatmentNameEditUserID.String(),
		ThreadID:   "thread-1",
		AgentID:    "mecontrola-agent",
		Message:    message,
		MessageID:  messageID,
	}
	return wf.WithRuntime(context.Background(), req)
}

type EditTreatmentNameToolSuite struct {
	suite.Suite
	engine *fakeTreatmentNameEditEngine
}

func TestEditTreatmentNameToolSuite(t *testing.T) {
	suite.Run(t, new(EditTreatmentNameToolSuite))
}

func (s *EditTreatmentNameToolSuite) SetupTest() {
	s.engine = newFakeTreatmentNameEditEngine("")
}

func (s *EditTreatmentNameToolSuite) TestExecute() {
	type args struct {
		ctx   context.Context
		input EditTreatmentNameInput
	}
	type dependencies struct {
		engine *fakeTreatmentNameEditEngine
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(engine *fakeTreatmentNameEditEngine, output EditTreatmentNameOutput, err error)
	}{
		{
			name: "deve iniciar o workflow e aplicar em turno único quando o nome vem na mensagem",
			args: args{
				ctx:   treatmentNameEditInboundCtx("wamid-1", "quero que me chame de Stef"),
				input: EditTreatmentNameInput{Name: "Stef"},
			},
			dependencies: dependencies{
				engine: func() *fakeTreatmentNameEditEngine {
					return newFakeTreatmentNameEditEngine("Combinado! Vou te chamar de Stef a partir de agora 💚")
				}(),
			},
			expect: func(engine *fakeTreatmentNameEditEngine, output EditTreatmentNameOutput, err error) {
				s.NoError(err)
				s.Equal(editTreatmentNameStatusStarted, output.Status)
				s.Equal("Combinado! Vou te chamar de Stef a partir de agora 💚", output.Message)
				s.True(engine.startCalled)
				s.Equal(testTreatmentNameEditUserID.String(), engine.lastState.ResourceID)
				s.Equal("Stef", engine.lastState.ProvidedName)
				s.Equal(workflows.TreatmentNameEditActive, engine.lastState.Status)
				s.Equal("wamid-1", engine.lastState.MessageID)
			},
		},
		{
			name: "deve iniciar o workflow e suspender perguntando o nome quando ele não vem na mensagem",
			args: args{
				ctx:   treatmentNameEditInboundCtx("wamid-2", "quero trocar meu nome de tratamento"),
				input: EditTreatmentNameInput{},
			},
			dependencies: dependencies{
				engine: func() *fakeTreatmentNameEditEngine {
					return newFakeTreatmentNameEditEngine("Claro! Como você gostaria que eu te chamasse a partir de agora? 💚")
				}(),
			},
			expect: func(engine *fakeTreatmentNameEditEngine, output EditTreatmentNameOutput, err error) {
				s.NoError(err)
				s.Equal(editTreatmentNameStatusStarted, output.Status)
				s.Equal("Claro! Como você gostaria que eu te chamasse a partir de agora? 💚", output.Message)
				s.True(engine.startCalled)
				s.Empty(engine.lastState.ProvidedName)
			},
		},
		{
			name: "ErrRunAlreadyExists retorna pending_exists",
			args: args{
				ctx:   treatmentNameEditInboundCtx("wamid-3", "quero trocar meu nome de tratamento"),
				input: EditTreatmentNameInput{},
			},
			dependencies: dependencies{
				engine: func() *fakeTreatmentNameEditEngine {
					return &fakeTreatmentNameEditEngine{startErr: wf.ErrRunAlreadyExists}
				}(),
			},
			expect: func(engine *fakeTreatmentNameEditEngine, output EditTreatmentNameOutput, err error) {
				s.NoError(err)
				s.Equal(editTreatmentNameStatusPendingExists, output.Status)
				s.NotEmpty(output.Message)
				s.True(engine.startCalled)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			handle := BuildEditTreatmentNameTool(scenario.dependencies.engine, fakeTreatmentNameEditDef())
			argsJSON, marshalErr := json.Marshal(scenario.args.input)
			s.Require().NoError(marshalErr)

			resultJSON, _, err := handle.Invoke(scenario.args.ctx, argsJSON)

			var output EditTreatmentNameOutput
			if err == nil {
				s.Require().NoError(json.Unmarshal(resultJSON, &output))
			}
			scenario.expect(scenario.dependencies.engine, output, err)
		})
	}
}

func (s *EditTreatmentNameToolSuite) TestExecute_IdentidadeSempreDeRuntimeFrom() {
	handle := BuildEditTreatmentNameTool(s.engine, fakeTreatmentNameEditDef())
	argsJSON, err := json.Marshal(EditTreatmentNameInput{})
	s.Require().NoError(err)

	_, _, invokeErr := handle.Invoke(context.Background(), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *EditTreatmentNameToolSuite) TestExecute_TipoDeRuntimeInvalido() {
	handle := BuildEditTreatmentNameTool(s.engine, fakeTreatmentNameEditDef())
	argsJSON, err := json.Marshal(EditTreatmentNameInput{})
	s.Require().NoError(err)

	ctx := wf.WithRuntime(context.Background(), "not-an-inbound-request")
	_, _, invokeErr := handle.Invoke(ctx, argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

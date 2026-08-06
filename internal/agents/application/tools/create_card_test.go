package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeCardManageEngine struct {
	startResult wf.RunResult[workflows.CardManageState]
	startErr    error
	startCalled bool
	lastState   workflows.CardManageState
}

func (f *fakeCardManageEngine) Start(_ context.Context, _ wf.Definition[workflows.CardManageState], _ string, initial workflows.CardManageState) (wf.RunResult[workflows.CardManageState], error) {
	f.startCalled = true
	f.lastState = initial
	return f.startResult, f.startErr
}

func (f *fakeCardManageEngine) Resume(_ context.Context, _ wf.Definition[workflows.CardManageState], _ string, _ []byte) (wf.RunResult[workflows.CardManageState], error) {
	return wf.RunResult[workflows.CardManageState]{}, nil
}

func (f *fakeCardManageEngine) LoadLatestState(_ context.Context, _ wf.Definition[workflows.CardManageState], _ string) (workflows.CardManageState, wf.Snapshot, bool, error) {
	return workflows.CardManageState{}, wf.Snapshot{}, false, nil
}

func fakeCardManageDef() wf.Definition[workflows.CardManageState] {
	return wf.Definition[workflows.CardManageState]{
		ID:      workflows.CardManageWorkflowID,
		Durable: true,
	}
}

func cardManageEngineAwaiting(awaiting workflows.CardManageAwaiting, response string) *fakeCardManageEngine {
	return &fakeCardManageEngine{
		startResult: wf.RunResult[workflows.CardManageState]{
			State: workflows.CardManageState{Awaiting: awaiting, ResponseText: response},
		},
	}
}

var testCardCreateUserID = uuid.MustParse("00000000-0000-0000-0000-000000000021")

func cardCreateInboundCtx(messageID string) context.Context {
	req := agent.InboundRequest{
		ResourceID: testCardCreateUserID.String(),
		ThreadID:   "thread-1",
		AgentID:    "mecontrola-agent",
		Message:    "cadastrar cartao",
		MessageID:  messageID,
	}
	return wf.WithRuntime(context.Background(), req)
}

func cardCreateInput(closingDay *int) CreateCardInput {
	return CreateCardInput{
		Nickname:   "Nu",
		Bank:       "Nubank",
		DueDay:     10,
		ClosingDay: closingDay,
	}
}

type CreateCardToolSuite struct {
	suite.Suite
	cardsMock *imocks.CardManager
	engine    *fakeCardManageEngine
}

func TestCreateCardToolSuite(t *testing.T) {
	suite.Run(t, new(CreateCardToolSuite))
}

func (s *CreateCardToolSuite) SetupTest() {
	s.cardsMock = imocks.NewCardManager(s.T())
	s.engine = cardManageEngineAwaiting(workflows.CardManageAwaitingConfirm, "⚠️ Confirma o cadastro do 💳?")
}

func (s *CreateCardToolSuite) TestExecute() {
	type args struct {
		ctx   context.Context
		input CreateCardInput
	}
	type dependencies struct {
		cardsMock *imocks.CardManager
		engine    *fakeCardManageEngine
	}

	closing := 5

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(engine *fakeCardManageEngine, output CreateCardOutput, err error)
	}{
		{
			name: "apelido ausente inicia workflow duravel e devolve needs_slot",
			args: args{
				ctx:   cardCreateInboundCtx("wamid-1"),
				input: CreateCardInput{Bank: "Nubank", DueDay: 10},
			},
			dependencies: dependencies{
				cardsMock: s.cardsMock,
				engine:    cardManageEngineAwaiting(workflows.CardManageAwaitingNickname, "Qual apelido você quer dar para esse 💳?"),
			},
			expect: func(engine *fakeCardManageEngine, output CreateCardOutput, err error) {
				s.NoError(err)
				s.Equal(createCardOutcomeNeedsSlot, output.Outcome)
				s.NotEmpty(output.ClarifyPrompt)
				s.True(engine.startCalled)
				s.False(engine.lastState.NicknameProvided)
				s.True(engine.lastState.BankProvided)
			},
		},
		{
			name: "banco ausente inicia workflow duravel e devolve needs_slot",
			args: args{
				ctx:   cardCreateInboundCtx("wamid-2"),
				input: CreateCardInput{Nickname: "Roxinho", DueDay: 10},
			},
			dependencies: dependencies{
				cardsMock: s.cardsMock,
				engine:    cardManageEngineAwaiting(workflows.CardManageAwaitingBank, "Qual é o banco desse 💳?"),
			},
			expect: func(engine *fakeCardManageEngine, output CreateCardOutput, err error) {
				s.NoError(err)
				s.Equal(createCardOutcomeNeedsSlot, output.Outcome)
				s.True(engine.startCalled)
				s.True(engine.lastState.NicknameProvided)
				s.False(engine.lastState.BankProvided)
			},
		},
		{
			name: "workflow aguardando fechamento devolve needs_closing",
			args: args{
				ctx:   cardCreateInboundCtx("wamid-3"),
				input: cardCreateInput(nil),
			},
			dependencies: dependencies{
				cardsMock: s.cardsMock,
				engine:    cardManageEngineAwaiting(workflows.CardManageAwaitingClosingDay, "Qual é o dia de fechamento?"),
			},
			expect: func(engine *fakeCardManageEngine, output CreateCardOutput, err error) {
				s.NoError(err)
				s.Equal(createCardOutcomeNeedsClosing, output.Outcome)
				s.NotEmpty(output.ClarifyPrompt)
				s.True(engine.startCalled)
			},
		},
		{
			name: "closingDay informado pelo LLM e repassado ao workflow",
			args: args{
				ctx:   cardCreateInboundCtx("wamid-4"),
				input: cardCreateInput(&closing),
			},
			dependencies: dependencies{cardsMock: s.cardsMock, engine: s.engine},
			expect: func(engine *fakeCardManageEngine, output CreateCardOutput, err error) {
				s.NoError(err)
				s.Equal(createCardOutcomeNeedsConfirmation, output.Outcome)
				s.NotEmpty(output.ConfirmationPrompt)
				s.True(engine.startCalled)
				s.True(engine.lastState.ClosingDayProvided)
				s.Equal(closing, engine.lastState.ClosingDay)
			},
		},
		{
			name: "dados completos chama engine.Start com identidade do runtime",
			args: args{
				ctx:   cardCreateInboundCtx("wamid-6"),
				input: cardCreateInput(nil),
			},
			dependencies: dependencies{cardsMock: s.cardsMock, engine: s.engine},
			expect: func(engine *fakeCardManageEngine, output CreateCardOutput, err error) {
				s.NoError(err)
				s.Equal(createCardOutcomeNeedsConfirmation, output.Outcome)
				s.True(engine.startCalled)
				s.Equal(testCardCreateUserID, engine.lastState.UserID)
				s.Equal("Nu", engine.lastState.Nickname)
				s.Equal("wamid-6", engine.lastState.MessageID)
				s.Equal(workflows.CardManageOpCreate, engine.lastState.Operation)
			},
		},
		{
			name: "ErrRunAlreadyExists retorna pending_confirmation_exists",
			args: args{
				ctx:   cardCreateInboundCtx("wamid-7"),
				input: cardCreateInput(nil),
			},
			dependencies: dependencies{
				cardsMock: s.cardsMock,
				engine:    &fakeCardManageEngine{startErr: wf.ErrRunAlreadyExists},
			},
			expect: func(engine *fakeCardManageEngine, output CreateCardOutput, err error) {
				s.NoError(err)
				s.Equal(createCardOutcomePendingConfirmationExists, output.Outcome)
				s.NotEmpty(output.ClarifyPrompt)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			handle := BuildCreateCardTool(scenario.dependencies.engine, fakeCardManageDef())
			argsJSON, marshalErr := json.Marshal(scenario.args.input)
			s.Require().NoError(marshalErr)

			resultJSON, _, err := handle.Invoke(scenario.args.ctx, argsJSON)

			var output CreateCardOutput
			if err == nil {
				s.Require().NoError(json.Unmarshal(resultJSON, &output))
			}
			scenario.expect(scenario.dependencies.engine, output, err)
		})
	}
}

func (s *CreateCardToolSuite) TestExecute_IdentidadeSempreDeRuntimeFrom() {
	handle := BuildCreateCardTool(s.engine, fakeCardManageDef())
	argsJSON, err := json.Marshal(cardCreateInput(nil))
	s.Require().NoError(err)

	_, _, invokeErr := handle.Invoke(context.Background(), argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateCardToolSuite) TestExecute_ResourceIDInvalido() {
	req := agent.InboundRequest{ResourceID: "not-a-uuid", MessageID: "wamid-x"}
	ctx := wf.WithRuntime(context.Background(), req)

	handle := BuildCreateCardTool(s.engine, fakeCardManageDef())
	argsJSON, err := json.Marshal(cardCreateInput(nil))
	s.Require().NoError(err)

	_, _, invokeErr := handle.Invoke(ctx, argsJSON)
	s.Error(invokeErr)
	s.False(s.engine.startCalled)
}

func (s *CreateCardToolSuite) TestExecute_SchemaPermiteOmissaoDeSlotParaClarify() {
	engine := cardManageEngineAwaiting(workflows.CardManageAwaitingNickname, "Qual apelido você quer dar para esse 💳?")
	handle := BuildCreateCardTool(engine, fakeCardManageDef())
	argsJSON := []byte(`{"bank":"Nubank","dueDay":10}`)

	resultJSON, _, invokeErr := handle.Invoke(cardCreateInboundCtx("wamid-8"), argsJSON)
	s.Require().NoError(invokeErr)

	var output CreateCardOutput
	s.Require().NoError(json.Unmarshal(resultJSON, &output))
	s.Equal(createCardOutcomeNeedsSlot, output.Outcome)
	s.NotEmpty(output.ClarifyPrompt)
	s.True(engine.startCalled)
}

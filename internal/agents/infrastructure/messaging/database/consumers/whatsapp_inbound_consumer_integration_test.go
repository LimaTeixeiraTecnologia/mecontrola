//go:build integration

package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	mempostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type consumerIntegrationLedger struct{}

func (consumerIntegrationLedger) CreateTransaction(_ context.Context, _ agentsifaces.RawTransaction) (agentsifaces.EntryRef, error) {
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindTransaction}, nil
}

func (consumerIntegrationLedger) UpdateTransaction(_ context.Context, _ agentsifaces.RawUpdateTransaction) (agentsifaces.EntryRef, error) {
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindTransaction}, nil
}

func (consumerIntegrationLedger) CreateRecurringTemplate(_ context.Context, _ agentsifaces.RawRecurringTemplate) (agentsifaces.EntryRef, error) {
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindRecurringTemplate}, nil
}

func (consumerIntegrationLedger) DeleteTransaction(_ context.Context, _ agentsifaces.EntryRef, _ int64) error {
	return nil
}

func (consumerIntegrationLedger) ListMonthlyEntries(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]agentsifaces.MonthlyEntry, error) {
	return nil, nil
}

func (consumerIntegrationLedger) GetMonthlySummary(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.MonthlySummary, error) {
	return agentsifaces.MonthlySummary{}, nil
}

func (consumerIntegrationLedger) GetCardInvoice(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.CardInvoice, error) {
	return agentsifaces.CardInvoice{}, nil
}

func (consumerIntegrationLedger) SearchTransactions(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]agentsifaces.Entry, error) {
	return nil, nil
}

func (consumerIntegrationLedger) GetTransaction(_ context.Context, _ string) (agentsifaces.Entry, error) {
	return agentsifaces.Entry{}, nil
}

func (consumerIntegrationLedger) SearchEditCandidates(_ context.Context, _ uuid.UUID, _ agentsifaces.EditCandidateQuery) ([]agentsifaces.Entry, error) {
	return nil, nil
}

type consumerIntegrationBudgetPlanner struct{}

func (consumerIntegrationBudgetPlanner) CreateBudget(_ context.Context, _ agentsifaces.DraftBudget) (agentsifaces.BudgetRef, error) {
	return agentsifaces.BudgetRef{ID: uuid.NewString(), Competence: "2026-07", State: "ACTIVE"}, nil
}

func (consumerIntegrationBudgetPlanner) DeleteDraftBudget(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (consumerIntegrationBudgetPlanner) ActivateBudget(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (consumerIntegrationBudgetPlanner) CreateRecurrence(_ context.Context, _ uuid.UUID, _ string, _ int) error {
	return nil
}

func (consumerIntegrationBudgetPlanner) EditCategoryPercentage(_ context.Context, _ uuid.UUID, _, _ string, _ int) error {
	return nil
}

func (consumerIntegrationBudgetPlanner) EditBudgetTotal(_ context.Context, _ uuid.UUID, _ string, _ int64) error {
	return nil
}

func (consumerIntegrationBudgetPlanner) GetMonthlySummary(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.BudgetSummary, error) {
	return agentsifaces.BudgetSummary{}, nil
}

func (consumerIntegrationBudgetPlanner) ListAlerts(_ context.Context, _ uuid.UUID) ([]agentsifaces.Alert, error) {
	return nil, nil
}

func (consumerIntegrationBudgetPlanner) SuggestAllocation(_ context.Context, totalCents int64, allocations []agentsifaces.AllocationBP) ([]agentsifaces.AllocationCents, error) {
	out := make([]agentsifaces.AllocationCents, 0, len(allocations))
	for _, a := range allocations {
		out = append(out, agentsifaces.AllocationCents{
			RootSlug:     a.RootSlug,
			BasisPoints:  a.BasisPoints,
			PlannedCents: totalCents * int64(a.BasisPoints) / 10000,
		})
	}
	return out, nil
}

type WhatsAppInboundConsumerIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestWhatsAppInboundConsumerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(WhatsAppInboundConsumerIntegrationSuite))
}

func (s *WhatsAppInboundConsumerIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
}

func (s *WhatsAppInboundConsumerIntegrationSuite) newUser() uuid.UUID {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)
	return userID
}

func (s *WhatsAppInboundConsumerIntegrationSuite) buildOnboardingResolver() *usecases.ResolveOnboardingOrAgent {
	o11y := fake.NewProvider()
	workflowStore := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.OnboardingState](workflowStore, o11y)

	threads := mempostgres.NewThreadRepository(s.db, o11y)
	messages := mempostgres.NewMessageRepository(s.db, o11y)
	workingMem := mempostgres.NewWorkingMemoryRepository(s.db, o11y)

	a := agentmocks.NewAgent(s.T())
	def := workflows.BuildOnboardingWorkflow(a, nil, consumerIntegrationBudgetPlanner{}, workingMem, threads, messages, nil)
	return usecases.NewResolveOnboardingOrAgent(engine, workflowStore, workingMem, def, o11y)
}

func (s *WhatsAppInboundConsumerIntegrationSuite) buildTransactionWriteResumeDispatcher() (
	*usecases.ResumeDispatcher,
	workflow.Engine[workflows.TransactionWriteState],
	workflow.Definition[workflows.TransactionWriteState],
) {
	o11y := fake.NewProvider()
	workflowStore := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.TransactionWriteState](workflowStore, o11y)

	threads := mempostgres.NewThreadRepository(s.db, o11y)
	runs := agentpostgres.NewRunStore(s.db)

	ledger := consumerIntegrationLedger{}
	def := workflows.BuildTransactionWriteWorkflowWithObservability(ledger, nil, nil, nil, nil)

	registry := agent.NewWorkflowRegistry[workflows.TransactionWriteState]()
	registry.Register(def)

	resumer, err := usecases.NewWorkflowResumer(
		workflows.TransactionWriteWorkflowID,
		registry,
		engine,
		workflows.TransactionWriteKey,
		workflows.ContinueTransactionWrite,
	)
	s.Require().NoError(err)

	index := usecases.NewSuspendedRunIndex(workflowStore, workflows.TransactionWriteWorkflowID)
	dispatcher, err := usecases.NewResumeDispatcher(index, threads, runs, o11y, resumer)
	s.Require().NoError(err)

	return dispatcher, engine, def
}

func (s *WhatsAppInboundConsumerIntegrationSuite) TestInteg_ConsumerIniciaOnboarding_EnviaPrimeiraMensagemCombinadaComoUnicaResposta() {
	userID := s.newUser()
	peer := "+55119" + uuid.NewString()[:8]
	wamid := "wamid-onboarding-start-" + uuid.NewString()

	onboardingResolver := s.buildOnboardingResolver()

	gatewayMock := &mockWhatsAppSender{}
	var sentText string
	gatewayMock.On("SendTextMessage", mock.Anything, peer, mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			sentText = args.Get(2).(string)
		}).
		Return(nil).Once()

	inboundMock := &mockHandleInbound{}

	consumer := NewWhatsAppInboundConsumer(
		inboundMock,
		gatewayMock,
		fake.NewProvider(),
		WithOnboardingResolver(onboardingResolver),
	)

	err := consumer.Handle(s.ctx, &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload:   buildEnvelope(whatsAppInboundPayload{UserID: userID.String(), Peer: peer, Text: "Ativar o meu plano", MessageID: wamid}),
	})
	s.Require().NoError(err)

	gatewayMock.AssertExpectations(s.T())
	s.Contains(sentText, "🎉 Bem-vindo ao MeControla! 🎉")
	s.Contains(sentText, "como você gostaria que eu te chamasse")
	inboundMock.AssertNotCalled(s.T(), "Execute")

	var count int
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.platform_messages WHERE resource_id = $1 AND role = 'assistant'`,
		userID.String(),
	).Scan(&count)
	s.Require().NoError(scanErr)
	s.Equal(1, count, "RF-01/RF-28: primeira resposta do onboarding deve ser uma única mensagem assistente")
}

func (s *WhatsAppInboundConsumerIntegrationSuite) TestInteg_TransactionWriteAtivo_RetomadoPeloDispatcherAntesDoAgenteGeralEDoOnboarding() {
	userID := s.newUser()
	peer := "+55119" + uuid.NewString()[:8]
	wamid := "wamid-transaction-write-resume-" + uuid.NewString()

	onboardingResolver := s.buildOnboardingResolver()
	dispatcher, txEngine, txDef := s.buildTransactionWriteResumeDispatcher()

	_, startErr := onboardingResolver.StartOnboarding(s.ctx, userID.String(), peer)
	s.Require().NoError(startErr)

	key := workflows.TransactionWriteKey(userID.String(), peer)
	state := workflows.TransactionWriteState{
		Status:        workflows.TransactionWriteStatusActive,
		Awaiting:      workflows.TransactionAwaitingConfirmation,
		OperationKind: workflows.TransactionOpRegisterExpense,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      peer,
		MessageID:     "wamid-transaction-write-original-" + uuid.NewString(),
		AmountCents:   5000,
		Description:   "supermercado",
		PaymentMethod: "pix",
		Kind:          agentsifaces.CategoryKindExpense,
		Candidates: []workflows.PendingCategoryCandidate{{
			RootCategoryID:  uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62"),
			RootSlug:        "custo-fixo",
			SubcategoryID:   uuid.MustParse("97fa4b86-d43c-5ad5-a99b-c88c8427fb30"),
			SubcategorySlug: "supermercado",
			Path:            "Custo Fixo > Supermercado",
		}},
		OccurredAt: time.Now().UTC().Format("2006-01-02"),
	}

	startResult, err := txEngine.Start(s.ctx, txDef, key, state)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, startResult.Status)

	gatewayMock := &mockWhatsAppSender{}
	var sentText string
	gatewayMock.On("SendTextMessage", mock.Anything, peer, mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			sentText = args.Get(2).(string)
		}).
		Return(nil).Once()

	inboundMock := &mockHandleInbound{}

	consumer := NewWhatsAppInboundConsumer(
		inboundMock,
		gatewayMock,
		fake.NewProvider(),
		WithResumeDispatcher(dispatcher),
		WithOnboardingResolver(onboardingResolver),
	)

	err = consumer.Handle(s.ctx, &mockEvent{
		eventType: "agents.whatsapp.inbound.v1",
		payload:   buildEnvelope(whatsAppInboundPayload{UserID: userID.String(), Peer: peer, Text: "sim", MessageID: wamid}),
	})
	s.Require().NoError(err)

	gatewayMock.AssertExpectations(s.T())
	s.NotEmpty(sentText)
	inboundMock.AssertNotCalled(s.T(), "Execute")

	var onboardingStatus string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.OnboardingWorkflowID, userID.String(),
	).Scan(&onboardingStatus)
	s.Require().NoError(scanErr)
	s.Equal("suspended", onboardingStatus, "onboarding suspenso não deve ter sido retomado enquanto transaction-write estava ativo")

	var transactionWriteStatus string
	scanErr = s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.TransactionWriteWorkflowID, key,
	).Scan(&transactionWriteStatus)
	s.Require().NoError(scanErr)
	s.Equal("succeeded", transactionWriteStatus, "transaction-write deve ter sido concluído")
}

func agentResultRawJSON(rawJSON []byte) agent.Result {
	return agent.Result{RawJSON: rawJSON}
}

func (s *WhatsAppInboundConsumerIntegrationSuite) TestInteg_OnboardingFluxoDeCartao_CriaUmUnicoCartaoSemLoop() {
	userID := s.newUser()
	peer := "+55119" + uuid.NewString()[:8]

	treatmentNameExtract, _ := json.Marshal(map[string]any{"hasName": true, "name": "Stef"})
	goalExtract, _ := json.Marshal(map[string]any{"goal": "comprar uma casa", "hasAmount": false, "amountBRL": 0})
	goalValueExtract, _ := json.Marshal(map[string]any{"hasAmount": false, "amountBRL": 0})
	budgetExtract, _ := json.Marshal(map[string]any{"amountBRL": 1000})
	distributionIntentAcceptExtract, _ := json.Marshal(map[string]any{"action": "accept", "mixed_unit": false})
	summaryConfirmExtract, _ := json.Marshal(map[string]any{"confirmed": true})
	recurrenceExtract, _ := json.Marshal(map[string]any{"intent": "negative", "hasMonths": false, "months": 0})
	cardCreateExtract, _ := json.Marshal(map[string]any{"wantsCard": true, "nickname": "Nubank", "bank": "Nubank", "dueDay": 10})
	cardRefuseExtract, _ := json.Marshal(map[string]any{"wantsCard": false, "nickname": "", "bank": "", "dueDay": 0})

	a := agentmocks.NewAgent(s.T())
	a.On("Execute", mock.Anything, mock.Anything).Return(agentResultRawJSON(treatmentNameExtract), nil).Once()
	a.On("Execute", mock.Anything, mock.Anything).Return(agentResultRawJSON(goalExtract), nil).Once()
	a.On("Execute", mock.Anything, mock.Anything).Return(agentResultRawJSON(goalValueExtract), nil).Once()
	a.On("Execute", mock.Anything, mock.Anything).Return(agentResultRawJSON(budgetExtract), nil).Once()
	a.On("Execute", mock.Anything, mock.Anything).Return(agentResultRawJSON(distributionIntentAcceptExtract), nil).Once()
	a.On("Execute", mock.Anything, mock.Anything).Return(agentResultRawJSON(summaryConfirmExtract), nil).Once()
	a.On("Execute", mock.Anything, mock.Anything).Return(agentResultRawJSON(recurrenceExtract), nil).Once()
	a.On("Execute", mock.Anything, mock.Anything).Return(agentResultRawJSON(cardCreateExtract), nil).Once()
	a.On("Execute", mock.Anything, mock.Anything).Return(agentResultRawJSON(cardRefuseExtract), nil).Once()

	o11y := fake.NewProvider()
	workflowStore := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.OnboardingState](workflowStore, o11y)

	threads := mempostgres.NewThreadRepository(s.db, o11y)
	messages := mempostgres.NewMessageRepository(s.db, o11y)
	workingMem := mempostgres.NewWorkingMemoryRepository(s.db, o11y)

	cardsMock := interfacemocks.NewCardManager(s.T())
	cardsMock.On("ListCards", mock.Anything, userID).Return([]agentsifaces.Card{}, nil).Once()

	var createdCard agentsifaces.NewCard
	cardsMock.On("CreateCard", mock.Anything, mock.AnythingOfType("interfaces.NewCard")).
		Run(func(args mock.Arguments) {
			createdCard = args.Get(1).(agentsifaces.NewCard)
		}).
		Return(agentsifaces.CardRef{ID: uuid.NewString()}, nil).Once()

	cardsMock.On("ListCards", mock.Anything, userID).Return([]agentsifaces.Card{{ID: uuid.NewString(), Nickname: "Nubank"}}, nil).Twice()

	def := workflows.BuildOnboardingWorkflow(a, cardsMock, consumerIntegrationBudgetPlanner{}, workingMem, threads, messages, nil)
	onboardingResolver := usecases.NewResolveOnboardingOrAgent(engine, workflowStore, workingMem, def, o11y)

	gatewayMock := &mockWhatsAppSender{}
	var replies []string
	gatewayMock.On("SendTextMessage", mock.Anything, peer, mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			replies = append(replies, args.Get(2).(string))
		}).
		Return(nil).Times(10)

	inboundMock := &mockHandleInbound{}

	consumer := NewWhatsAppInboundConsumer(
		inboundMock,
		gatewayMock,
		fake.NewProvider(),
		WithOnboardingResolver(onboardingResolver),
	)

	turns := []string{
		"Ativar o meu plano",
		"pode me chamar de Stef",
		"comprar uma casa",
		"não sei",
		"R$ 1.000,00",
		"sim",
		"sim",
		"não",
		"Nubank, vencimento dia 10",
		"não",
	}

	for i, text := range turns {
		err := consumer.Handle(s.ctx, &mockEvent{
			eventType: "agents.whatsapp.inbound.v1",
			payload:   buildEnvelope(whatsAppInboundPayload{UserID: userID.String(), Peer: peer, Text: text, MessageID: fmt.Sprintf("wamid-card-turn-%d", i)}),
		})
		s.Require().NoError(err)
	}

	gatewayMock.AssertExpectations(s.T())
	s.Require().GreaterOrEqual(len(replies), 10)
	s.Contains(replies[7], "💳")
	s.Contains(replies[8], "outro")
	s.Contains(replies[8], "💳")
	s.Equal("Nubank", createdCard.Nickname)
	s.Equal("Nubank", createdCard.Bank)
	s.Equal(10, createdCard.DueDay)
	s.NotEmpty(replies[9])
	s.Contains(replies[9], "Resumo de Onboarding")
	inboundMock.AssertNotCalled(s.T(), "Execute")

	var onboardingStatus string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.OnboardingWorkflowID, userID.String(),
	).Scan(&onboardingStatus)
	s.Require().NoError(scanErr)
	s.Equal("succeeded", onboardingStatus, "onboarding deve concluir após recusa de segundo cartão")

	cardsMock.AssertNumberOfCalls(s.T(), "CreateCard", 1)
}

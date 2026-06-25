package services_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeParser struct {
	intent       intent.Intent
	directReply  string
	confidence   float64
	llmModel     string
	promptSHA256 string
	rawResponse  []byte
	err          error
}

func (f *fakeParser) Parse(_ context.Context, _ uuid.UUID, _ string) (services.ParsedIntent, error) {
	raw := f.confidence
	if raw <= 0 {
		raw = 1
	}
	confidence, confErr := valueobjects.NewConfidence(raw)
	if confErr != nil {
		return services.ParsedIntent{}, confErr
	}
	return services.ParsedIntent{
		Intent:       f.intent,
		Confidence:   confidence,
		DirectReply:  f.directReply,
		LLMModel:     f.llmModel,
		PromptSHA256: f.promptSHA256,
		Raw:          f.rawResponse,
	}, f.err
}

type fakeMonthlySummary struct {
	out budgetsoutput.MonthlySummaryOutput
	err error
}

func (f *fakeMonthlySummary) Execute(_ context.Context, _ string, _ string) (budgetsoutput.MonthlySummaryOutput, error) {
	return f.out, f.err
}

type fakeCardLister struct {
	out cardoutput.CardList
	err error
}

func (f *fakeCardLister) Execute(_ context.Context, _ cardinput.ListCards) (cardoutput.CardList, error) {
	return f.out, f.err
}

type fakeCardInvoice struct {
	out cardoutput.Invoice
	err error
}

func (f *fakeCardInvoice) Execute(_ context.Context, _ cardinput.InvoiceFor) (cardoutput.Invoice, error) {
	return f.out, f.err
}

type fakeFallback struct {
	reply string
	err   error
	calls int
}

func (f *fakeFallback) Reply(_ context.Context, _ uuid.UUID, _, _ string) (string, error) {
	f.calls++
	return f.reply, f.err
}

type fakeExpenseLogger struct {
	result tools.ExpenseRecorderResult
	err    error
	calls  int
}

func (f *fakeExpenseLogger) Execute(_ context.Context, _ tools.ExpenseRecorderInput) (tools.ExpenseRecorderResult, error) {
	f.calls++
	return f.result, f.err
}

type fakeWhatsAppGateway struct {
	sent  []sentMessage
	err   error
	calls int
}

type sentMessage struct {
	To   string
	Text string
}

func (f *fakeWhatsAppGateway) SendTextMessage(_ context.Context, to, text string) error {
	f.calls++
	f.sent = append(f.sent, sentMessage{To: to, Text: text})
	return f.err
}

type routerTestStore struct {
	mu   sync.Mutex
	runs map[string]platform.Snapshot
}

func newRouterTestStore() *routerTestStore {
	return &routerTestStore{runs: make(map[string]platform.Snapshot)}
}

func (s *routerTestStore) key(wf, k string) string { return wf + ":" + k }

func (s *routerTestStore) Insert(_ context.Context, snap platform.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *routerTestStore) Load(_ context.Context, wf, k string) (platform.Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.runs[s.key(wf, k)]
	return snap, ok, nil
}

func (s *routerTestStore) Save(_ context.Context, snap platform.Snapshot, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *routerTestStore) AppendStep(_ context.Context, _ platform.StepRecord) error { return nil }
func (s *routerTestStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

func newMinimalKernel() *services.KernelDeps {
	obs := fake.NewProvider()
	store := newRouterTestStore()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)
	return &services.KernelDeps{
		Engine:    engine,
		SettleReg: services.NewSettleRegistry(),
		CategoryResolver: func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
			st.CategoryID = "test-category"
			st.CategoryPath = "Test"
			return st, nil
		},
		PersistFn: func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
			return steps.PersistResult{AmountCents: 5800, CategoryPath: "Test"}, nil
		},
	}
}

func newKernelWithExpenseRecorder(recorder tools.ExpenseRecorder, categoryErr error) *services.KernelDeps {
	obs := fake.NewProvider()
	store := newRouterTestStore()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)
	return &services.KernelDeps{
		Engine:    engine,
		SettleReg: services.NewSettleRegistry(),
		CategoryResolver: func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
			if recorder == nil {
				return st, tools.ErrCategoryHintMissing
			}
			if categoryErr != nil {
				return st, categoryErr
			}
			st.CategoryID = "test-category"
			st.CategoryPath = "Test"
			return st, nil
		},
		PersistFn: func(ctx context.Context, st steps.ExpenseState) (steps.PersistResult, error) {
			if recorder == nil {
				return steps.PersistResult{}, errors.New("recorder not configured")
			}
			result, err := recorder.Execute(ctx, tools.ExpenseRecorderInput{
				UserID:        st.UserID.String(),
				AmountCents:   st.AmountCents,
				Merchant:      st.Merchant,
				PaymentMethod: st.PaymentMethod,
				Direction:     st.Direction,
			})
			if err != nil {
				return steps.PersistResult{}, err
			}
			return steps.PersistResult{AmountCents: result.AmountCents, CategoryPath: result.CategoryPath}, nil
		},
	}
}

type IntentRouterSuite struct {
	suite.Suite
	wa       *fakeWhatsAppGateway
	fallback *fakeFallback
	parser   *fakeParser
	summary  *fakeMonthlySummary
	cards    *fakeCardLister
	invoice  *fakeCardInvoice
	expenses *fakeExpenseLogger
}

func (s *IntentRouterSuite) SetupTest() {
	s.wa = &fakeWhatsAppGateway{}
	s.fallback = &fakeFallback{reply: "fallback livre"}
	s.parser = &fakeParser{}
	s.summary = &fakeMonthlySummary{}
	s.cards = &fakeCardLister{}
	s.invoice = &fakeCardInvoice{}
	s.expenses = nil
}

func (s *IntentRouterSuite) categoryErrFromExpenses() error {
	if s.expenses == nil {
		return nil
	}
	if isCategoryError(s.expenses.err) {
		return s.expenses.err
	}
	return nil
}

func (s *IntentRouterSuite) newRouter() *services.IntentRouter {
	var expenseRec tools.ExpenseRecorder
	if s.expenses != nil {
		expenseRec = s.expenses
	}
	deps := services.IntentRouterDeps{
		Parser:          s.parser,
		MonthlySummary:  s.summary,
		CardLister:      s.cards,
		CardInvoice:     s.invoice,
		Fallback:        s.fallback,
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		Kernel:          newKernelWithExpenseRecorder(expenseRec, s.categoryErrFromExpenses()),
		ExpenseRecorder: expenseRec,
	}
	router, err := services.NewIntentRouter(noop.NewProvider(), deps)
	require.NoError(s.T(), err)
	return router
}

func isCategoryError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, tools.ErrCategoryNotFound) || errors.Is(err, tools.ErrCategoryHintMissing) {
		return true
	}
	var ambiguous *tools.CategoryAmbiguousError
	if errors.As(err, &ambiguous) {
		return true
	}
	var needsConfirmation *tools.CategoryNeedsConfirmationError
	return errors.As(err, &needsConfirmation)
}

func (s *IntentRouterSuite) TestNew_NilDeps() {
	_, err := services.NewIntentRouter(nil, services.IntentRouterDeps{})
	require.ErrorIs(s.T(), err, services.ErrObservabilityNil)

	_, err = services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{})
	require.ErrorIs(s.T(), err, services.ErrIntentParserNil)

	_, err = services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{Parser: s.parser})
	require.ErrorIs(s.T(), err, services.ErrFallbackNil)

	_, err = services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{Parser: s.parser, Fallback: s.fallback})
	require.ErrorIs(s.T(), err, services.ErrWhatsAppGatewayNil)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_EmptyText() {
	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "   ", WhatsAppTo: "+5511999"})
	s.Equal(tools.OutcomeEmptyText, result.Outcome)
	s.Equal(intent.KindUnknown, result.Kind)
	s.Len(s.wa.sent, 1)
}

func (s *IntentRouterSuite) buildLogExpense() intent.Intent {
	expense, err := intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents:  5800,
		Merchant:     "iFood",
		CategoryHint: "Prazeres",
	})
	require.NoError(s.T(), err)
	return expense
}

func (s *IntentRouterSuite) TestRouteWhatsApp_LogExpense_MissingResolverIsHonest() {
	s.parser.intent = s.buildLogExpense()

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "gastei 58 no iFood", WhatsAppTo: "+5511999"})

	s.Equal(intent.KindRecordExpense, result.Kind)
	s.Equal(tools.OutcomeClarify, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "categoria")
	s.NotContains(s.wa.sent[0].Text, "Transação realizada")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_LogExpense_PersistedConfirms() {
	s.expenses = &fakeExpenseLogger{result: tools.ExpenseRecorderResult{
		Persisted:    true,
		AmountCents:  5800,
		CategoryPath: "Prazeres > Delivery",
	}}
	s.parser.intent = s.buildLogExpense()

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "gastei 58 no iFood", WhatsAppTo: "+5511999"})

	s.Equal(intent.KindRecordExpense, result.Kind)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, s.expenses.calls)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "💸 *Transação realizada!*")
	s.Contains(s.wa.sent[0].Text, "R$ 58,00")
	s.Contains(s.wa.sent[0].Text, "iFood")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_LogExpense_FailureIsHonest() {
	s.expenses = &fakeExpenseLogger{err: errors.New("boom")}
	s.parser.intent = s.buildLogExpense()

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "gastei 58 no iFood", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "Transação realizada")
	s.Contains(s.wa.sent[0].Text, "Pode tentar de novo")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_LogExpense_AmbiguousAsksToChoose() {
	s.expenses = &fakeExpenseLogger{err: &tools.CategoryAmbiguousError{
		Hint:       "academia",
		Candidates: []string{"Prazeres > Academia", "Custo Fixo > Academia"},
	}}
	s.parser.intent = s.buildLogExpense()

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "gastei 58 na academia", WhatsAppTo: "+5511999"})

	s.Equal(intent.KindRecordExpense, result.Kind)
	s.Equal(tools.OutcomeClarify, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "mais de uma categoria")
	s.Contains(s.wa.sent[0].Text, "Prazeres > Academia")
	s.Contains(s.wa.sent[0].Text, "Custo Fixo > Academia")
	s.NotContains(s.wa.sent[0].Text, "Não consegui registrar")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_LogExpense_NotFoundAsksToRephrase() {
	s.expenses = &fakeExpenseLogger{err: tools.ErrCategoryNotFound}
	s.parser.intent = s.buildLogExpense()

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "gastei 58 no iFood", WhatsAppTo: "+5511999"})

	s.Equal(intent.KindRecordExpense, result.Kind)
	s.Equal(tools.OutcomeClarify, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Não encontrei a categoria")
	s.Contains(s.wa.sent[0].Text, "Prazeres")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_LogExpense_NoHintAsksWhichCategory() {
	s.expenses = &fakeExpenseLogger{err: tools.ErrCategoryHintMissing}
	s.parser.intent = s.buildLogExpense()

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "gastei 58", WhatsAppTo: "+5511999"})

	s.Equal(intent.KindRecordExpense, result.Kind)
	s.Equal(tools.OutcomeClarify, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "categoria")
	s.NotContains(s.wa.sent[0].Text, "Não consegui registrar")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_LogIncome_PersistedConfirms() {
	s.expenses = &fakeExpenseLogger{result: tools.ExpenseRecorderResult{
		Persisted:    true,
		AmountCents:  1640000,
		CategoryPath: "Salário",
	}}
	income, err := intent.NewRecordIncome(intent.RecordIncomeFields{AmountCents: 1640000, Source: "salário"})
	require.NoError(s.T(), err)
	s.parser.intent = income

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "recebi salario 16400", WhatsAppTo: "+5511999"})

	s.Equal(intent.KindRecordIncome, result.Kind)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, s.expenses.calls)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "💰 *Recebimento registrado!*")
	s.Contains(s.wa.sent[0].Text, "R$ 16.400,00")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_LogIncome_MissingResolverIsHonest() {
	income, err := intent.NewRecordIncome(intent.RecordIncomeFields{AmountCents: 1640000, Source: "salário"})
	require.NoError(s.T(), err)
	s.parser.intent = income

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "recebi salario 16400", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeClarify, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "categoria")
	s.NotContains(s.wa.sent[0].Text, "registrado")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_MonthlySummary() {
	parsed, err := intent.NewMonthlySummary("2026-06")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	planned := int64(120000)
	s.summary.out = budgetsoutput.MonthlySummaryOutput{
		Competence:        "2026-06",
		TotalSpentCents:   45000,
		TotalPlannedCents: &planned,
		Allocations: []budgetsoutput.AllocationSummary{
			{RootSlug: "essentials", SpentCents: 30000, PlannedCents: &planned},
		},
	}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "resumo", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "Resumo")
	s.Contains(s.wa.sent[0].Text, "2026-06")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_MonthlySummary_UsecaseError() {
	parsed, _ := intent.NewMonthlySummary("")
	s.parser.intent = parsed
	s.summary.err = errors.New("boom")

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "resumo", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Equal(intent.KindMonthlySummary, result.Kind)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryCategory() {
	parsed, err := intent.NewQueryCategory("essentials")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	planned := int64(80000)
	s.summary.out = budgetsoutput.MonthlySummaryOutput{
		Competence: "2026-06",
		Allocations: []budgetsoutput.AllocationSummary{
			{RootSlug: "essentials", SpentCents: 30000, PlannedCents: &planned},
		},
	}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "quanto gastei em essentials", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "essentials")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryGoal_MissingResolver_NoSummary() {
	parsed, err := intent.NewQueryGoal("Viagem")
	require.NoError(s.T(), err)
	s.parser.intent = parsed

	router, err := services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{
		Parser:          s.parser,
		Fallback:        s.fallback,
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
	})
	require.NoError(s.T(), err)

	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "como está minha meta de viagem", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeMissingResolver, result.Outcome)
	s.Equal(intent.KindQueryGoal, result.Kind)
	s.Contains(s.wa.sent[0].Text, "Viagem")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryGoal_Routed() {
	parsed, err := intent.NewQueryGoal("Viagem")
	require.NoError(s.T(), err)
	s.parser.intent = parsed

	planned := int64(500000)
	pct := 60.0
	s.summary.out = budgetsoutput.MonthlySummaryOutput{
		Competence:      "2026-06",
		TotalSpentCents: 300000,
		Allocations: []budgetsoutput.AllocationSummary{
			{RootSlug: "expense.metas", SpentCents: 300000, PlannedCents: &planned, PercentageSpent: &pct},
		},
	}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "como está minha meta de viagem", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindQueryGoal, result.Kind)
	s.Contains(s.wa.sent[0].Text, "guardou")
	s.Contains(s.wa.sent[0].Text, "R$ 3.000,00")
	s.Contains(s.wa.sent[0].Text, "R$ 5.000,00")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryGoal_UsecaseError() {
	parsed, err := intent.NewQueryGoal("Poupança")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	s.summary.err = errors.New("db error")

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "meta poupança", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Equal(intent.KindQueryGoal, result.Kind)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryCard_Found() {
	parsed, err := intent.NewQueryCard("nubank")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	cardID := uuid.New().String()
	s.cards.out = cardoutput.CardList{
		Items: []cardoutput.Card{
			{ID: cardID, Name: "Nubank Roxinho", Nickname: "Roxinho"},
		},
	}
	s.invoice.out = cardoutput.Invoice{ClosingDate: "2026-06-25", DueDate: "2026-07-05"}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "qual a fatura do nubank", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "Nubank")
	s.Contains(s.wa.sent[0].Text, "2026-06-25")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryCard_WithLimit_IncludesLimite() {
	parsed, err := intent.NewQueryCard("nubank")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	cardID := uuid.New().String()
	s.cards.out = cardoutput.CardList{
		Items: []cardoutput.Card{
			{ID: cardID, Name: "Nubank Roxinho", Nickname: "Roxinho", LimitCents: 500000},
		},
	}
	s.invoice.out = cardoutput.Invoice{ClosingDate: "2026-06-25", DueDate: "2026-07-05"}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "qual a fatura do nubank", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "Limite: R$ 5.000,00")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryCard_WithoutLimit_OmitsLimite() {
	parsed, err := intent.NewQueryCard("nubank")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	cardID := uuid.New().String()
	s.cards.out = cardoutput.CardList{
		Items: []cardoutput.Card{
			{ID: cardID, Name: "Nubank Roxinho", Nickname: "Roxinho", LimitCents: 0},
		},
	}
	s.invoice.out = cardoutput.Invoice{ClosingDate: "2026-06-25", DueDate: "2026-07-05"}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "qual a fatura do nubank", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.NotContains(s.wa.sent[0].Text, "Limite")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryCard_NotFound() {
	parsed, err := intent.NewQueryCard("itau")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	s.cards.out = cardoutput.CardList{
		Items: []cardoutput.Card{{ID: uuid.New().String(), Name: "Nubank"}},
	}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "fatura do itau", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeMissingResolver, result.Outcome)
	s.Contains(strings.ToLower(s.wa.sent[0].Text), "itau")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryCard_ListerError() {
	parsed, _ := intent.NewQueryCard("nubank")
	s.parser.intent = parsed
	s.cards.err = errors.New("db down")

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "fatura", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_HowAmIDoing_Above90() {
	parsed := intent.NewHowAmIDoing()
	s.parser.intent = parsed
	planned := int64(100000)
	pct := 95.0
	s.summary.out = budgetsoutput.MonthlySummaryOutput{
		Competence:        "2026-06",
		TotalSpentCents:   95000,
		TotalPlannedCents: &planned,
		PercentageTotal:   &pct,
	}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "como estou", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "Atenção")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_HowAmIDoing_AlertToneAt80() {
	parsed := intent.NewHowAmIDoing()
	s.parser.intent = parsed
	planned := int64(100000)
	pct := 82.0
	s.summary.out = budgetsoutput.MonthlySummaryOutput{
		Competence:        "2026-06",
		TotalSpentCents:   82000,
		TotalPlannedCents: &planned,
		PercentageTotal:   &pct,
	}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "como estou", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "⚠️ *Atenção Proativa*")
	s.Contains(s.wa.sent[0].Text, "82%")
	s.Contains(s.wa.sent[0].Text, "🎯")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_HowAmIDoing_NormalToneBelow80() {
	parsed := intent.NewHowAmIDoing()
	s.parser.intent = parsed
	planned := int64(100000)
	pct := 40.0
	s.summary.out = budgetsoutput.MonthlySummaryOutput{
		Competence:        "2026-06",
		TotalSpentCents:   40000,
		TotalPlannedCents: &planned,
		PercentageTotal:   &pct,
	}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "como estou", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "📊 *Como você está*")
	s.NotContains(s.wa.sent[0].Text, "Atenção")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_MonthlySummary_RootSlugLabelHumanized() {
	parsed, err := intent.NewMonthlySummary("2026-06")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	planned := int64(120000)
	s.summary.out = budgetsoutput.MonthlySummaryOutput{
		Competence:        "2026-06",
		TotalSpentCents:   45000,
		TotalPlannedCents: &planned,
		Allocations: []budgetsoutput.AllocationSummary{
			{RootSlug: "expense.custo_fixo", SpentCents: 30000, PlannedCents: &planned},
			{RootSlug: "expense.liberdade_financeira", SpentCents: 15000, PlannedCents: &planned},
		},
	}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "resumo", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "📊 *Resumo de 2026-06*")
	s.Contains(s.wa.sent[0].Text, "Custo Fixo")
	s.Contains(s.wa.sent[0].Text, "Liberdade Financeira")
	s.NotContains(s.wa.sent[0].Text, "expense.custo_fixo")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryGoal_GoalEmojiAndNoFabricatedPercent() {
	parsed, err := intent.NewQueryGoal("Viagem")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	s.summary.out = budgetsoutput.MonthlySummaryOutput{
		Competence: "2026-06",
		Allocations: []budgetsoutput.AllocationSummary{
			{RootSlug: "expense.metas", SpentCents: 50000},
		},
	}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "como está minha meta de viagem", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "🎯")
	s.Contains(s.wa.sent[0].Text, "R$ 500,00")
	s.NotContains(s.wa.sent[0].Text, "%")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_Unknown_DelegatesFallback() {
	parsed, err := intent.NewUnknown("conversa qualquer")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	s.fallback.reply = "te entendi parcialmente"

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "conversa qualquer", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeFallback, result.Outcome)
	s.Equal(1, s.fallback.calls)
	s.Equal("te entendi parcialmente", s.wa.sent[0].Text)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_Unknown_WithDirectReply_SkipsFallback() {
	parsed, err := intent.NewUnknown("oi tudo bem?")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	s.parser.directReply = "Oi! Tô por aqui. Como posso ajudar nas suas finanças? 😊"

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "oi tudo bem?", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindUnknown, result.Kind)
	s.Equal("Oi! Tô por aqui. Como posso ajudar nas suas finanças? 😊", result.Reply)
	s.Equal(0, s.fallback.calls)
	s.Require().Len(s.wa.sent, 1)
	s.Equal("Oi! Tô por aqui. Como posso ajudar nas suas finanças? 😊", s.wa.sent[0].Text)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_ParserError_DelegatesFallback() {
	s.parser.err = errors.New("provider down")
	s.fallback.reply = "estou aqui"

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "oi", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeParseError, result.Outcome)
	s.Equal(1, s.fallback.calls)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_FallbackFailure_UsesDefaultReply() {
	parsed, _ := intent.NewUnknown("conversa")
	s.parser.intent = parsed
	s.fallback.err = errors.New("nope")

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "conversa", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeFallback, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "Não entendi direito")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_ListCards_Empty() {
	s.parser.intent = intent.NewListCards()
	s.cards.out = cardoutput.CardList{}
	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "quais meus cartões?", WhatsAppTo: "+5511999"})
	s.Equal(intent.KindListCards, result.Kind)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "ainda não tem cartões")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_ListCards_WithCards() {
	s.parser.intent = intent.NewListCards()
	s.cards.out = cardoutput.CardList{Items: []cardoutput.Card{
		{Name: "Nubank", Nickname: "Nu", ClosingDay: 3, DueDay: 10, LimitCents: 500000},
	}}
	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "lista meus cartões", WhatsAppTo: "+5511999"})
	s.Equal(intent.KindListCards, result.Kind)
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Nu")
	s.Contains(s.wa.sent[0].Text, "R$ 5.000,00")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_ListCards_ListerError() {
	s.parser.intent = intent.NewListCards()
	s.cards.err = errors.New("db error")
	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "meus cartões", WhatsAppTo: "+5511999"})
	s.Equal(intent.KindListCards, result.Kind)
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_ListCards_MissingResolver() {
	s.parser.intent = intent.NewListCards()
	router, err := services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{
		Parser: s.parser, Fallback: s.fallback, WhatsAppGateway: s.wa,
	})
	require.NoError(s.T(), err)
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "meus cartões", WhatsAppTo: "+5511999"})
	s.Equal(tools.OutcomeMissingResolver, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "entendi direito", "MissingResolver não deve sugerir reformular")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_GatewayFailure_OutcomePreserved() {
	s.expenses = &fakeExpenseLogger{result: tools.ExpenseRecorderResult{
		Persisted:    true,
		AmountCents:  5800,
		CategoryPath: "Prazeres > Delivery",
	}}
	s.parser.intent = s.buildLogExpense()
	s.wa.err = errors.New("connection timeout")

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "gastei 58 no iFood", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindRecordExpense, result.Kind)
	s.Equal(1, s.expenses.calls)
	s.Equal(1, s.wa.calls)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_PolicyBlocked_WriteWithLowConfidence() {
	s.parser.intent = s.buildLogExpense()
	s.parser.confidence = 0.5

	router, err := services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{
		Parser:              s.parser,
		Fallback:            s.fallback,
		WhatsAppGateway:     s.wa,
		Location:            time.UTC,
		PolicyMinConfidence: 0.8,
		Kernel:              newMinimalKernel(),
	})
	s.Require().NoError(err)

	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "gastei 58 no iFood", WhatsAppTo: "+5511999"})

	s.Equal(tools.OutcomePolicyBlocked, result.Outcome)
	s.Equal(intent.KindRecordExpense, result.Kind)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Pode reescrever")
}

func TestIntentRouterSuite(t *testing.T) {
	suite.Run(t, new(IntentRouterSuite))
}

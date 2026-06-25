package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeCardPurchaseLogger struct {
	result tools.CardPurchaseLoggerResult
	err    error
	calls  int
}

func (f *fakeCardPurchaseLogger) Execute(_ context.Context, _ tools.CardPurchaseLoggerInput) (tools.CardPurchaseLoggerResult, error) {
	f.calls++
	return f.result, f.err
}

type fakeTransactionLister struct {
	result tools.TransactionListResult
	err    error
	calls  int
}

func (f *fakeTransactionLister) Execute(_ context.Context, in tools.TransactionListInput) (tools.TransactionListResult, error) {
	f.calls++
	out := f.result
	if out.RefMonth == "" {
		out.RefMonth = in.RefMonth
	}
	return out, f.err
}

type fakeLastDeleter struct {
	err     error
	calls   int
	gotID   string
	gotVer  int64
	gotUser string
}

func (f *fakeLastDeleter) Execute(_ context.Context, userID, txID string, version int64) error {
	f.calls++
	f.gotUser = userID
	f.gotID = txID
	f.gotVer = version
	return f.err
}

type fakeLastEditor struct {
	result tools.EditTransactionResult
	err    error
	calls  int
	gotIn  tools.EditTransactionInput
}

func (f *fakeLastEditor) Execute(_ context.Context, in tools.EditTransactionInput) (tools.EditTransactionResult, error) {
	f.calls++
	f.gotIn = in
	return f.result, f.err
}

type fakeRecurringCreator struct {
	result tools.RecurringCreatorResult
	err    error
	calls  int
}

func (f *fakeRecurringCreator) Execute(_ context.Context, _ tools.RecurringCreatorInput) (tools.RecurringCreatorResult, error) {
	f.calls++
	return f.result, f.err
}

type fakeRecurringLister struct {
	views []tools.RecurringView
	err   error
	calls int
}

func (f *fakeRecurringLister) Execute(_ context.Context, _ string) ([]tools.RecurringView, error) {
	f.calls++
	return f.views, f.err
}

type NewKindsRouterSuite struct {
	suite.Suite
	wa       *fakeWhatsAppGateway
	parser   *fakeParser
	fallback *fakeFallback
	cardPur  *fakeCardPurchaseLogger
	lister   *fakeTransactionLister
	deleter  *fakeLastDeleter
	editor   *fakeLastEditor
	recCreat *fakeRecurringCreator
	recList  *fakeRecurringLister
}

func (s *NewKindsRouterSuite) SetupTest() {
	s.wa = &fakeWhatsAppGateway{}
	s.parser = &fakeParser{}
	s.fallback = &fakeFallback{reply: "fallback livre"}
	s.cardPur = nil
	s.lister = nil
	s.deleter = nil
	s.editor = nil
	s.recCreat = nil
	s.recList = nil
}

func newKernelWithCardPurchaseLogger(logger tools.CardPurchaseLogger) *services.KernelDeps {
	obs := fake.NewProvider()
	store := newRouterTestStore()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)
	return &services.KernelDeps{
		Engine:    engine,
		SettleReg: services.NewSettleRegistry(),
		CategoryResolver: func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
			if logger == nil {
				return st, tools.ErrCategoryHintMissing
			}
			st.CategoryID = "test-category"
			st.CategoryPath = "Test"
			return st, nil
		},
		PersistFn: func(ctx context.Context, st steps.ExpenseState) (steps.PersistResult, error) {
			if logger == nil {
				return steps.PersistResult{}, errors.New("card purchase logger not configured")
			}
			result, err := logger.Execute(ctx, tools.CardPurchaseLoggerInput{
				UserID:        st.UserID.String(),
				AmountCents:   st.AmountCents,
				Merchant:      st.Merchant,
				PaymentMethod: st.PaymentMethod,
				CardHint:      st.CardHint,
				Installments:  st.Installments,
			})
			if err != nil {
				return steps.PersistResult{}, err
			}
			if !result.CardFound {
				return steps.PersistResult{
					ShortCircuit:    true,
					ShortCircuitOut: tools.OutcomeMissingResolver,
					ShortCircuitMsg: tools.FormatCardPurchaseCardMissing(st.CardHint),
				}, nil
			}
			return steps.PersistResult{
				AmountCents:  result.AmountCents,
				CategoryPath: result.CategoryPath,
				CardFound:    result.CardFound,
				CardName:     result.CardName,
			}, nil
		},
	}
}

func (s *NewKindsRouterSuite) newRouter() *services.IntentRouter {
	var cardPurLogger tools.CardPurchaseLogger
	if s.cardPur != nil {
		cardPurLogger = s.cardPur
	}
	deps := services.IntentRouterDeps{
		Parser:          s.parser,
		Fallback:        s.fallback,
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
		Kernel:          newKernelWithCardPurchaseLogger(cardPurLogger),
		CardPurchaseLog: cardPurLogger,
	}
	if s.lister != nil {
		deps.TransactionLister = s.lister
	}
	if s.deleter != nil {
		deps.LastDeleter = s.deleter
	}
	if s.editor != nil {
		deps.LastEditor = s.editor
	}
	if s.recCreat != nil {
		deps.RecurringCreator = s.recCreat
	}
	if s.recList != nil {
		deps.RecurringLister = s.recList
	}
	router, err := services.NewIntentRouter(noop.NewProvider(), deps)
	require.NoError(s.T(), err)
	return router
}

func (s *NewKindsRouterSuite) route(in intent.Intent, text string) services.RouteResult {
	s.parser.intent = in
	return s.newRouter().RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: text, WhatsAppTo: "+5511999"})
}

func (s *NewKindsRouterSuite) buildCardPurchase() intent.Intent {
	in, err := intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{AmountCents: 120000, Merchant: "Magalu", CardHint: "nubank", Installments: 6})
	require.NoError(s.T(), err)
	return in
}

func (s *NewKindsRouterSuite) TestCardPurchase_MissingResolverIsHonest() {
	result := s.route(s.buildCardPurchase(), "parcelei 1200 em 6x no nubank")
	s.Equal(intent.KindRecordCardPurchase, result.Kind)
	s.Equal(tools.OutcomeClarify, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "registrada")
}

func (s *NewKindsRouterSuite) TestCardPurchase_PersistedConfirms() {
	s.cardPur = &fakeCardPurchaseLogger{result: tools.CardPurchaseLoggerResult{
		Persisted: true, CardFound: true, CardName: "Nubank", AmountCents: 120000, Installments: 6, CategoryPath: "Casa > Eletro",
	}}
	result := s.route(s.buildCardPurchase(), "parcelei 1200 em 6x no nubank")
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, s.cardPur.calls)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Compra parcelada registrada")
	s.Contains(s.wa.sent[0].Text, "R$ 1.200,00")
	s.Contains(s.wa.sent[0].Text, "6x")
	s.Contains(s.wa.sent[0].Text, "Nubank")
}

func (s *NewKindsRouterSuite) TestCardPurchase_CardNotFoundIsHonest() {
	s.cardPur = &fakeCardPurchaseLogger{result: tools.CardPurchaseLoggerResult{Persisted: false, CardFound: false}}
	result := s.route(s.buildCardPurchase(), "parcelei 1200 em 6x no nubank")
	s.Equal(tools.OutcomeMissingResolver, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "registrada")
	s.Contains(s.wa.sent[0].Text, "nubank")
}

func (s *NewKindsRouterSuite) TestCardPurchase_UsecaseErrorIsHonest() {
	s.cardPur = &fakeCardPurchaseLogger{err: errors.New("boom")}
	result := s.route(s.buildCardPurchase(), "parcelei 1200 em 6x no nubank")
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "registrada")
}

func (s *NewKindsRouterSuite) TestListTransactions_Routed() {
	s.lister = &fakeTransactionLister{result: tools.TransactionListResult{
		RefMonth: "2026-06",
		Transactions: []tools.TransactionView{
			{ID: uuid.NewString(), Direction: "income", AmountCents: 500000, OccurredAt: time.Now()},
			{ID: uuid.NewString(), Direction: "outcome", AmountCents: 5800, OccurredAt: time.Now()},
		},
	}}
	in, err := intent.NewListTransactions("2026-06")
	require.NoError(s.T(), err)
	result := s.route(in, "meus lançamentos de junho")
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Lançamentos de 2026-06")
	s.Contains(s.wa.sent[0].Text, "R$ 5.000,00")
	s.Contains(s.wa.sent[0].Text, "R$ 58,00")
}

func (s *NewKindsRouterSuite) TestListTransactions_MissingResolverIsHonest() {
	in, err := intent.NewListTransactions("")
	require.NoError(s.T(), err)
	result := s.route(in, "meus lançamentos")
	s.Equal(tools.OutcomeMissingResolver, result.Outcome)
}

func (s *NewKindsRouterSuite) TestDeleteLast_WithoutConfirmEngine_ReturnsUsecaseError() {
	result := s.route(intent.NewDeleteLastTransaction(), "apaga o último")
	s.Equal(intent.KindDeleteLastTransaction, result.Kind)
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
}

func (s *NewKindsRouterSuite) TestEditLast_WithoutConfirmEngine_ReturnsUsecaseError() {
	in, err := intent.NewEditLastTransaction(8000)
	require.NoError(s.T(), err)
	result := s.route(in, "corrige para 80")
	s.Equal(intent.KindEditLastTransaction, result.Kind)
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
}

func (s *NewKindsRouterSuite) buildRecurring() intent.Intent {
	in, err := intent.NewCreateRecurring(intent.CreateRecurringFields{AmountCents: 500000, Merchant: "salário", Direction: "income", DayOfMonth: 5})
	require.NoError(s.T(), err)
	return in
}

func (s *NewKindsRouterSuite) TestCreateRecurring_PersistedConfirms() {
	s.recCreat = &fakeRecurringCreator{result: tools.RecurringCreatorResult{
		Persisted: true, Direction: "income", AmountCents: 500000, Frequency: "monthly", DayOfMonth: 5, Description: "salário",
	}}
	result := s.route(s.buildRecurring(), "todo mês recebo 5000 de salário")
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, s.recCreat.calls)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Recorrência criada")
	s.Contains(s.wa.sent[0].Text, "R$ 5.000,00")
	s.Contains(s.wa.sent[0].Text, "mensal")
}

func (s *NewKindsRouterSuite) TestCreateRecurring_MissingResolverIsHonest() {
	result := s.route(s.buildRecurring(), "todo mês recebo 5000 de salário")
	s.Equal(tools.OutcomeMissingResolver, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "criada")
}

func (s *NewKindsRouterSuite) TestCreateRecurring_UsecaseErrorIsHonest() {
	s.recCreat = &fakeRecurringCreator{err: errors.New("boom")}
	result := s.route(s.buildRecurring(), "todo mês recebo 5000 de salário")
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "criada")
}

func (s *NewKindsRouterSuite) TestListRecurring_Routed() {
	s.recList = &fakeRecurringLister{views: []tools.RecurringView{
		{Direction: "income", AmountCents: 500000, Description: "salário", Frequency: "monthly", DayOfMonth: 5},
	}}
	result := s.route(intent.NewListRecurring(), "minhas recorrências")
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Recorrências")
	s.Contains(s.wa.sent[0].Text, "R$ 5.000,00")
	s.Contains(s.wa.sent[0].Text, "salário")
}

func (s *NewKindsRouterSuite) TestListRecurring_Empty() {
	s.recList = &fakeRecurringLister{views: nil}
	result := s.route(intent.NewListRecurring(), "minhas recorrências")
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "ainda não tem")
}

func TestNewKindsRouterSuite(t *testing.T) {
	suite.Run(t, new(NewKindsRouterSuite))
}

package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type fakeParser struct {
	intent intent.Intent
	err    error
}

func (f *fakeParser) Parse(_ context.Context, _ uuid.UUID, _ string) (services.ParsedIntent, error) {
	return services.ParsedIntent{Intent: f.intent}, f.err
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

type fakeTelegramGateway struct {
	sent []sentTelegram
	err  error
}

type sentTelegram struct {
	ChatID int64
	Text   string
}

func (f *fakeTelegramGateway) SendTextMessage(_ context.Context, chatID int64, text string) error {
	f.sent = append(f.sent, sentTelegram{ChatID: chatID, Text: text})
	return f.err
}

type IntentRouterSuite struct {
	suite.Suite
	wa       *fakeWhatsAppGateway
	tg       *fakeTelegramGateway
	fallback *fakeFallback
	parser   *fakeParser
	summary  *fakeMonthlySummary
	cards    *fakeCardLister
	invoice  *fakeCardInvoice
}

func (s *IntentRouterSuite) SetupTest() {
	s.wa = &fakeWhatsAppGateway{}
	s.tg = &fakeTelegramGateway{}
	s.fallback = &fakeFallback{reply: "fallback livre"}
	s.parser = &fakeParser{}
	s.summary = &fakeMonthlySummary{}
	s.cards = &fakeCardLister{}
	s.invoice = &fakeCardInvoice{}
}

func (s *IntentRouterSuite) newRouter() *services.IntentRouter {
	router, err := services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{
		Parser:          s.parser,
		MonthlySummary:  s.summary,
		CardLister:      s.cards,
		CardInvoice:     s.invoice,
		Fallback:        s.fallback,
		WhatsAppGateway: s.wa,
		TelegramGateway: s.tg,
		Location:        time.UTC,
	})
	require.NoError(s.T(), err)
	return router
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
	s.Equal(services.OutcomeEmptyText, result.Outcome)
	s.Equal(intent.KindUnknown, result.Kind)
	s.Len(s.wa.sent, 1)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_LogExpense() {
	expense, err := intent.NewLogExpense(intent.LogExpenseFields{
		AmountCents:  5800,
		Merchant:     "iFood",
		CategoryHint: "Prazeres",
	})
	require.NoError(s.T(), err)
	s.parser.intent = expense

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "gastei 58 no iFood", WhatsAppTo: "+5511999"})

	s.Equal(intent.KindLogExpense, result.Kind)
	s.Equal(services.OutcomeMissingResolver, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Anotei")
	s.Contains(s.wa.sent[0].Text, "iFood")
	s.Contains(s.wa.sent[0].Text, "Prazeres")
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

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "Resumo")
	s.Contains(s.wa.sent[0].Text, "2026-06")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_MonthlySummary_UsecaseError() {
	parsed, _ := intent.NewMonthlySummary("")
	s.parser.intent = parsed
	s.summary.err = errors.New("boom")

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "resumo", WhatsAppTo: "+5511999"})

	s.Equal(services.OutcomeUsecaseError, result.Outcome)
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

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "essentials")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryGoal_MissingResolver() {
	parsed, err := intent.NewQueryGoal("Viagem")
	require.NoError(s.T(), err)
	s.parser.intent = parsed

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "como está minha meta de viagem", WhatsAppTo: "+5511999"})

	s.Equal(services.OutcomeMissingResolver, result.Outcome)
	s.Equal(intent.KindQueryGoal, result.Kind)
	s.Contains(s.wa.sent[0].Text, "Viagem")
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

	s.Equal(services.OutcomeRouted, result.Outcome)
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

	s.Equal(services.OutcomeRouted, result.Outcome)
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

	s.Equal(services.OutcomeRouted, result.Outcome)
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

	s.Equal(services.OutcomeMissingResolver, result.Outcome)
	s.Contains(strings.ToLower(s.wa.sent[0].Text), "itau")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_QueryCard_ListerError() {
	parsed, _ := intent.NewQueryCard("nubank")
	s.parser.intent = parsed
	s.cards.err = errors.New("db down")

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "fatura", WhatsAppTo: "+5511999"})

	s.Equal(services.OutcomeUsecaseError, result.Outcome)
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

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "Atenção")
}

func (s *IntentRouterSuite) TestRouteWhatsApp_Unknown_DelegatesFallback() {
	parsed, err := intent.NewUnknown("conversa qualquer")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	s.fallback.reply = "te entendi parcialmente"

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "conversa qualquer", WhatsAppTo: "+5511999"})

	s.Equal(services.OutcomeFallback, result.Outcome)
	s.Equal(1, s.fallback.calls)
	s.Equal("te entendi parcialmente", s.wa.sent[0].Text)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_ParserError_DelegatesFallback() {
	s.parser.err = errors.New("provider down")
	s.fallback.reply = "estou aqui"

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "oi", WhatsAppTo: "+5511999"})

	s.Equal(services.OutcomeParseError, result.Outcome)
	s.Equal(1, s.fallback.calls)
}

func (s *IntentRouterSuite) TestRouteWhatsApp_FallbackFailure_UsesDefaultReply() {
	parsed, _ := intent.NewUnknown("conversa")
	s.parser.intent = parsed
	s.fallback.err = errors.New("nope")

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "conversa", WhatsAppTo: "+5511999"})

	s.Equal(services.OutcomeFallback, result.Outcome)
	s.Contains(s.wa.sent[0].Text, "Não entendi direito")
}

func (s *IntentRouterSuite) TestRouteTelegram_Success() {
	parsed, err := intent.NewMonthlySummary("2026-06")
	require.NoError(s.T(), err)
	s.parser.intent = parsed
	s.summary.out = budgetsoutput.MonthlySummaryOutput{Competence: "2026-06", TotalSpentCents: 10000}

	router := s.newRouter()
	result := router.RouteTelegram(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "resumo", TelegramTo: 42})

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Require().Len(s.tg.sent, 1)
	s.Equal(int64(42), s.tg.sent[0].ChatID)
}

func (s *IntentRouterSuite) TestRouteTelegram_NoGateway() {
	router, err := services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{
		Parser:          s.parser,
		Fallback:        s.fallback,
		WhatsAppGateway: s.wa,
		MonthlySummary:  s.summary,
		Location:        time.UTC,
	})
	require.NoError(s.T(), err)

	parsed, _ := intent.NewMonthlySummary("")
	s.parser.intent = parsed

	result := router.RouteTelegram(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "resumo", TelegramTo: 1})
	s.Equal(services.OutcomeRouted, result.Outcome)
	assert.Empty(s.T(), s.tg.sent)
}

func TestIntentRouterSuite(t *testing.T) {
	suite.Run(t, new(IntentRouterSuite))
}

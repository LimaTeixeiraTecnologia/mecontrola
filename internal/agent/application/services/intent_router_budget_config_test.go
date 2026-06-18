package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type fakeBudgetConvo struct {
	result   services.BudgetConversationResult
	err      error
	calls    int
	lastText string
}

func (f *fakeBudgetConvo) Configure(_ context.Context, text string, draft budgetdraft.Draft) (services.BudgetConversationResult, error) {
	f.calls++
	f.lastText = text
	if f.err != nil {
		return services.BudgetConversationResult{}, f.err
	}
	result := f.result
	if result.Draft.Competence() == "" {
		result.Draft = draft
	}
	return result, nil
}

type fakeBudgetCommitter struct {
	reply string
	err   error
	calls int
}

func (f *fakeBudgetCommitter) Commit(_ context.Context, _ uuid.UUID, _ budgetdraft.Draft) (string, error) {
	f.calls++
	return f.reply, f.err
}

type fakeBudgetSession struct {
	draft      budgetdraft.Draft
	found      bool
	loadErr    error
	saved      int
	savedDraft budgetdraft.Draft
	cleared    int
}

func (f *fakeBudgetSession) Load(_ context.Context, _ uuid.UUID, _ string) (budgetdraft.Draft, bool, error) {
	return f.draft, f.found, f.loadErr
}

func (f *fakeBudgetSession) Save(_ context.Context, _ uuid.UUID, _ string, draft budgetdraft.Draft) error {
	f.saved++
	f.savedDraft = draft
	return nil
}

func (f *fakeBudgetSession) Clear(_ context.Context, _ uuid.UUID, _ string) error {
	f.cleared++
	return nil
}

type BudgetConfigRouterSuite struct {
	suite.Suite
	wa        *fakeWhatsAppGateway
	fallback  *fakeFallback
	parser    *fakeParser
	convo     *fakeBudgetConvo
	committer *fakeBudgetCommitter
	session   *fakeBudgetSession
}

func TestBudgetConfigRouterSuite(t *testing.T) {
	suite.Run(t, new(BudgetConfigRouterSuite))
}

func (s *BudgetConfigRouterSuite) SetupTest() {
	s.wa = &fakeWhatsAppGateway{}
	s.fallback = &fakeFallback{reply: "fallback livre"}
	s.parser = &fakeParser{}
	s.convo = &fakeBudgetConvo{}
	s.committer = &fakeBudgetCommitter{reply: "✅ orçamento ativado"}
	s.session = &fakeBudgetSession{}
}

func (s *BudgetConfigRouterSuite) newRouter() *services.IntentRouter {
	router, err := services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{
		Parser:          s.parser,
		Fallback:        s.fallback,
		WhatsAppGateway: s.wa,
		BudgetConvo:     s.convo,
		BudgetCommitter: s.committer,
		BudgetSession:   s.session,
		Location:        time.UTC,
	})
	require.NoError(s.T(), err)
	return router
}

func (s *BudgetConfigRouterSuite) configureIntent() intent.Intent {
	return intent.NewConfigureBudget()
}

func (s *BudgetConfigRouterSuite) TestStartIncompleteAsksAndSaves() {
	partial, err := budgetdraft.New("2026-06").Merge(budgetdraft.Change{TotalCents: 500000})
	require.NoError(s.T(), err)
	s.convo.result = services.BudgetConversationResult{Draft: partial, Complete: false, Reply: "Quais categorias?"}
	s.parser.intent = s.configureIntent()
	s.session.found = false

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{
		Text: "quero configurar meu orçamento, ganho 5 mil", WhatsAppTo: "+5511999",
	})

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindConfigureBudget, result.Kind)
	s.Equal("Quais categorias?", result.Reply)
	s.Equal(1, s.convo.calls)
	s.Equal(1, s.session.saved)
	s.Equal(0, s.committer.calls)
}

func (s *BudgetConfigRouterSuite) TestPendingSessionContinuesWithoutParsing() {
	s.session.found = true
	s.session.draft, _ = budgetdraft.New("2026-06").Merge(budgetdraft.Change{TotalCents: 500000})
	partial, _ := budgetdraft.New("2026-06").Merge(budgetdraft.Change{
		TotalCents:  500000,
		Allocations: map[string]int{budgetdraft.SlugCustoFixo: 3500},
	})
	s.convo.result = services.BudgetConversationResult{Draft: partial, Complete: false, Reply: "Faltam categorias"}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{
		Text: "custos fixos 35%", WhatsAppTo: "+5511999",
	})

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal("Faltam categorias", result.Reply)
	s.Equal("custos fixos 35%", s.convo.lastText)
	s.Equal(1, s.convo.calls)
	s.Equal(1, s.session.saved)
}

func (s *BudgetConfigRouterSuite) TestFinalTurnCompletesCommitsAndClears() {
	s.session.found = true
	s.session.draft, _ = budgetdraft.New("2026-06").Merge(budgetdraft.Change{TotalCents: 800000})
	complete, _ := budgetdraft.New("2026-06").Merge(budgetdraft.Change{
		TotalCents: 800000,
		Allocations: map[string]int{
			budgetdraft.SlugCustoFixo:           3500,
			budgetdraft.SlugConhecimento:        1000,
			budgetdraft.SlugPrazeres:            2000,
			budgetdraft.SlugMetas:               2000,
			budgetdraft.SlugLiberdadeFinanceira: 1500,
		},
	})
	require.True(s.T(), complete.IsComplete())
	s.convo.result = services.BudgetConversationResult{Draft: complete, Complete: true}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{
		Text: "resto liberdade financeira 15%", WhatsAppTo: "+5511999",
	})

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal("✅ orçamento ativado", result.Reply)
	s.Equal(1, s.committer.calls)
	s.Equal(1, s.session.cleared)
	s.Equal(0, s.session.saved)
}

func (s *BudgetConfigRouterSuite) TestCommitErrorReturnsMessageAndKeepsSession() {
	s.session.found = true
	complete, _ := budgetdraft.New("2026-06").Merge(budgetdraft.Change{
		TotalCents: 800000,
		Allocations: map[string]int{
			budgetdraft.SlugCustoFixo:           3500,
			budgetdraft.SlugConhecimento:        1000,
			budgetdraft.SlugPrazeres:            2000,
			budgetdraft.SlugMetas:               2000,
			budgetdraft.SlugLiberdadeFinanceira: 1500,
		},
	})
	s.convo.result = services.BudgetConversationResult{Draft: complete, Complete: true}
	s.committer.err = errors.New("conflict")
	s.committer.reply = "Já existe um orçamento neste mês. Quer substituir?"

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{
		Text: "fecha", WhatsAppTo: "+5511999",
	})

	s.Equal(services.OutcomeUsecaseError, result.Outcome)
	s.Equal("Já existe um orçamento neste mês. Quer substituir?", result.Reply)
	s.Equal(1, s.committer.calls)
	s.Equal(0, s.session.cleared)
	s.Equal(1, s.session.saved)
}

func (s *BudgetConfigRouterSuite) TestPendingSessionCancelClearsAndConfirms() {
	s.session.found = true
	s.session.draft, _ = budgetdraft.New("2026-06").Merge(budgetdraft.Change{TotalCents: 500000})

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{
		Text: "cancelar", WhatsAppTo: "+5511999",
	})

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindConfigureBudget, result.Kind)
	s.Contains(result.Reply, "cancelei a configuração do orçamento")
	s.Equal(1, s.session.cleared)
	s.Equal(0, s.convo.calls)
	s.Equal(0, s.session.saved)
	s.Equal(0, s.committer.calls)
}

func (s *BudgetConfigRouterSuite) TestPendingSessionNonCancelTextStillProcesses() {
	s.session.found = true
	s.session.draft, _ = budgetdraft.New("2026-06").Merge(budgetdraft.Change{TotalCents: 500000})
	partial, _ := budgetdraft.New("2026-06").Merge(budgetdraft.Change{
		TotalCents:  500000,
		Allocations: map[string]int{budgetdraft.SlugCustoFixo: 3500},
	})
	s.convo.result = services.BudgetConversationResult{Draft: partial, Complete: false, Reply: "Faltam categorias"}

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{
		Text: "custos fixos 35%", WhatsAppTo: "+5511999",
	})

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal("Faltam categorias", result.Reply)
	s.Equal(1, s.convo.calls)
	s.Equal(0, s.session.cleared)
}

func (s *BudgetConfigRouterSuite) TestNoPendingSessionFallsThroughToParser() {
	s.session.found = false
	s.parser.intent = intent.NewHowAmIDoing()

	router := s.newRouter()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{
		Text: "como estou indo?", WhatsAppTo: "+5511999",
	})

	s.Equal(intent.KindHowAmIDoing, result.Kind)
	s.Equal(0, s.convo.calls)
}

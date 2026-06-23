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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type fakeCardUpdater struct {
	result services.CardUpdaterResult
	err    error
	calls  int
	gotIn  intent.Intent
}

func (f *fakeCardUpdater) Execute(_ context.Context, _ uuid.UUID, in intent.Intent) (services.CardUpdaterResult, error) {
	f.calls++
	f.gotIn = in
	return f.result, f.err
}

type fakeCardDeleter struct {
	result  services.CardDeleterResult
	err     error
	calls   int
	gotName string
}

func (f *fakeCardDeleter) Execute(_ context.Context, _ uuid.UUID, cardName string) (services.CardDeleterResult, error) {
	f.calls++
	f.gotName = cardName
	return f.result, f.err
}

type fakeCategoryPercentageEditor struct {
	result services.CategoryPercentageEditorResult
	err    error
	calls  int
	gotIn  services.CategoryPercentageEditorInput
}

func (f *fakeCategoryPercentageEditor) Execute(_ context.Context, in services.CategoryPercentageEditorInput) (services.CategoryPercentageEditorResult, error) {
	f.calls++
	f.gotIn = in
	return f.result, f.err
}

type NewWritesRouterSuite struct {
	suite.Suite
	wa        *fakeWhatsAppGateway
	parser    *fakeParser
	fallback  *fakeFallback
	updater   *fakeCardUpdater
	deleter   *fakeCardDeleter
	pctEditor *fakeCategoryPercentageEditor
}

func TestNewWritesRouterSuite(t *testing.T) {
	suite.Run(t, new(NewWritesRouterSuite))
}

func (s *NewWritesRouterSuite) SetupTest() {
	s.wa = &fakeWhatsAppGateway{}
	s.parser = &fakeParser{}
	s.fallback = &fakeFallback{reply: "fallback livre"}
	s.updater = nil
	s.deleter = nil
	s.pctEditor = nil
}

func (s *NewWritesRouterSuite) newRouter() *services.IntentRouter {
	deps := services.IntentRouterDeps{
		Parser:          s.parser,
		Fallback:        s.fallback,
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
	}
	if s.updater != nil {
		deps.CardUpdater = s.updater
	}
	if s.deleter != nil {
		deps.CardDeleter = s.deleter
	}
	if s.pctEditor != nil {
		deps.CategoryPercentageEditor = s.pctEditor
	}
	router, err := services.NewIntentRouter(noop.NewProvider(), deps)
	require.NoError(s.T(), err)
	return router
}

func (s *NewWritesRouterSuite) route(in intent.Intent, text string) services.RouteResult {
	s.parser.intent = in
	return s.newRouter().RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: text, WhatsAppTo: "+5511999"})
}

func (s *NewWritesRouterSuite) buildUpdateCard() intent.Intent {
	day := 5
	in, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank", ClosingDay: &day})
	require.NoError(s.T(), err)
	return in
}

func (s *NewWritesRouterSuite) buildDeleteCard() intent.Intent {
	in, err := intent.NewDeleteCard("nubank")
	require.NoError(s.T(), err)
	return in
}

func (s *NewWritesRouterSuite) buildEditPercentage() intent.Intent {
	in, err := intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{CategoryName: "Prazeres", Percentage: 30})
	require.NoError(s.T(), err)
	return in
}

func (s *NewWritesRouterSuite) TestUpdateCard_MissingResolverIsHonest() {
	result := s.route(s.buildUpdateCard(), "muda o fechamento do nubank pra dia 5")
	s.Equal(intent.KindUpdateCard, result.Kind)
	s.Equal(services.OutcomeMissingResolver, result.Outcome)
}

func (s *NewWritesRouterSuite) TestUpdateCard_Routed() {
	s.updater = &fakeCardUpdater{result: services.CardUpdaterResult{Nickname: "nubank", ClosingDay: 5, DueDay: 17}}
	result := s.route(s.buildUpdateCard(), "muda o fechamento do nubank pra dia 5")
	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal(1, s.updater.calls)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Cartão atualizado")
	s.Contains(s.wa.sent[0].Text, "Fecha dia 5")
}

func (s *NewWritesRouterSuite) TestUpdateCard_NotFoundClarify() {
	s.updater = &fakeCardUpdater{err: services.ErrAgentCardNotFound}
	result := s.route(s.buildUpdateCard(), "muda o fechamento do premium")
	s.Equal(services.OutcomeClarify, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "atualizado")
}

func (s *NewWritesRouterSuite) TestUpdateCard_AmbiguousClarify() {
	s.updater = &fakeCardUpdater{err: services.ErrAgentCardAmbiguous}
	result := s.route(s.buildUpdateCard(), "muda o fechamento do cartao")
	s.Equal(services.OutcomeClarify, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "mais de um cartão")
}

func (s *NewWritesRouterSuite) TestUpdateCard_UsecaseError() {
	s.updater = &fakeCardUpdater{err: errors.New("boom")}
	result := s.route(s.buildUpdateCard(), "muda o fechamento do nubank")
	s.Equal(services.OutcomeUsecaseError, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "atualizado")
}

func (s *NewWritesRouterSuite) TestDeleteCard_MissingResolverIsHonest() {
	result := s.route(s.buildDeleteCard(), "apaga o nubank")
	s.Equal(intent.KindDeleteCard, result.Kind)
	s.Equal(services.OutcomeMissingResolver, result.Outcome)
}

func (s *NewWritesRouterSuite) TestDeleteCard_Routed() {
	s.deleter = &fakeCardDeleter{result: services.CardDeleterResult{Name: "nubank"}}
	result := s.route(s.buildDeleteCard(), "apaga o nubank")
	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal(1, s.deleter.calls)
	s.Equal("nubank", s.deleter.gotName)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Cartão apagado")
	s.Contains(s.wa.sent[0].Text, "nubank")
}

func (s *NewWritesRouterSuite) TestDeleteCard_NotFoundClarify() {
	s.deleter = &fakeCardDeleter{err: services.ErrAgentCardNotFound}
	result := s.route(s.buildDeleteCard(), "apaga o premium")
	s.Equal(services.OutcomeClarify, result.Outcome)
}

func (s *NewWritesRouterSuite) TestDeleteCard_UsecaseError() {
	s.deleter = &fakeCardDeleter{err: errors.New("boom")}
	result := s.route(s.buildDeleteCard(), "apaga o nubank")
	s.Equal(services.OutcomeUsecaseError, result.Outcome)
}

func (s *NewWritesRouterSuite) TestEditCategoryPercentage_MissingResolverIsHonest() {
	result := s.route(s.buildEditPercentage(), "coloca 30% em prazeres")
	s.Equal(intent.KindEditCategoryPercentage, result.Kind)
	s.Equal(services.OutcomeMissingResolver, result.Outcome)
}

func (s *NewWritesRouterSuite) TestEditCategoryPercentage_Routed() {
	s.pctEditor = &fakeCategoryPercentageEditor{result: services.CategoryPercentageEditorResult{Competence: "2026-06", RootSlug: "expense.prazeres", Percentage: 30}}
	result := s.route(s.buildEditPercentage(), "coloca 30% em prazeres")
	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal(1, s.pctEditor.calls)
	s.Equal("Prazeres", s.pctEditor.gotIn.CategoryName)
	s.Equal(30, s.pctEditor.gotIn.Percentage)
	s.NotEmpty(s.pctEditor.gotIn.Competence)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Orçamento ajustado")
	s.Contains(s.wa.sent[0].Text, "30%")
}

func (s *NewWritesRouterSuite) TestEditCategoryPercentage_UnknownCategoryClarify() {
	s.pctEditor = &fakeCategoryPercentageEditor{err: services.ErrCategoryPercentageUnknownCategory}
	result := s.route(s.buildEditPercentage(), "coloca 30% em viagens")
	s.Equal(services.OutcomeClarify, result.Outcome)
}

func (s *NewWritesRouterSuite) TestEditCategoryPercentage_NoBudgetClarify() {
	s.pctEditor = &fakeCategoryPercentageEditor{err: services.ErrCategoryPercentageNoBudget}
	result := s.route(s.buildEditPercentage(), "coloca 30% em prazeres")
	s.Equal(services.OutcomeClarify, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "orçamento ativo")
}

func (s *NewWritesRouterSuite) TestEditCategoryPercentage_UsecaseError() {
	s.pctEditor = &fakeCategoryPercentageEditor{err: errors.New("boom")}
	result := s.route(s.buildEditPercentage(), "coloca 30% em prazeres")
	s.Equal(services.OutcomeUsecaseError, result.Outcome)
}

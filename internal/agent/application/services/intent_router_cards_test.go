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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type fakeCardCreator struct {
	result  tools.CardCreatorResult
	err     error
	calls   int
	gotUser uuid.UUID
	gotIn   intent.Intent
}

func (f *fakeCardCreator) Execute(_ context.Context, userID uuid.UUID, in intent.Intent) (tools.CardCreatorResult, error) {
	f.calls++
	f.gotUser = userID
	f.gotIn = in
	return f.result, f.err
}

type fakeCardCounter struct {
	total int64
	err   error
	calls int
}

func (f *fakeCardCounter) Execute(_ context.Context, _ uuid.UUID) (int64, error) {
	f.calls++
	return f.total, f.err
}

type CardsRouterSuite struct {
	suite.Suite
	wa       *fakeWhatsAppGateway
	parser   *fakeParser
	fallback *fakeFallback
	creator  *fakeCardCreator
	counter  *fakeCardCounter
}

func (s *CardsRouterSuite) SetupTest() {
	s.wa = &fakeWhatsAppGateway{}
	s.parser = &fakeParser{}
	s.fallback = &fakeFallback{reply: "fallback livre"}
	s.creator = nil
	s.counter = nil
}

func (s *CardsRouterSuite) newRouter() *services.IntentRouter {
	deps := services.IntentRouterDeps{
		Parser:          s.parser,
		Fallback:        s.fallback,
		WhatsAppGateway: s.wa,
		Location:        time.UTC,
	}
	if s.creator != nil {
		deps.CardCreator = s.creator
	}
	if s.counter != nil {
		deps.CardCounter = s.counter
	}
	router, err := services.NewIntentRouter(noop.NewProvider(), deps)
	require.NoError(s.T(), err)
	return router
}

func (s *CardsRouterSuite) route(in intent.Intent, text string) services.RouteResult {
	s.parser.intent = in
	return s.newRouter().RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: text, WhatsAppTo: "+5511999"})
}

func (s *CardsRouterSuite) buildCreateCard() intent.Intent {
	in, err := intent.NewCreateCard(intent.CreateCardFields{Nickname: "nubank", Name: "Nubank Roxinho", ClosingDay: 10, DueDay: 17, LimitCents: 500000})
	require.NoError(s.T(), err)
	return in
}

func (s *CardsRouterSuite) TestCreateCard_MissingResolverIsHonest() {
	result := s.route(s.buildCreateCard(), "cadastra meu nubank")
	s.Equal(intent.KindCreateCard, result.Kind)
	s.Equal(tools.OutcomeMissingResolver, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.NotContains(s.wa.sent[0].Text, "cadastrado")
}

func (s *CardsRouterSuite) TestCreateCard_PersistedConfirms() {
	s.creator = &fakeCardCreator{result: tools.CardCreatorResult{
		Nickname: "nubank", Name: "Nubank Roxinho", ClosingDay: 10, DueDay: 17, LimitCents: 500000,
	}}
	result := s.route(s.buildCreateCard(), "cadastra meu nubank")
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, s.creator.calls)
	s.Equal("nubank", s.creator.gotIn.CardNickname())
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Cartão cadastrado")
	s.Contains(s.wa.sent[0].Text, "nubank")
	s.Contains(s.wa.sent[0].Text, "R$ 5.000,00")
	s.Contains(s.wa.sent[0].Text, "Fecha dia 10")
}

func (s *CardsRouterSuite) TestCreateCard_NicknameConflictFriendlyMessage() {
	s.creator = &fakeCardCreator{err: carddomain.ErrNicknameConflict}
	result := s.route(s.buildCreateCard(), "cadastra meu nubank")
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "já tem um cartão com esse apelido")
	s.NotContains(s.wa.sent[0].Text, "cadastrado!")
}

func (s *CardsRouterSuite) TestCreateCard_InvalidClosingDayFriendlyMessage() {
	s.creator = &fakeCardCreator{err: carddomain.ErrInvalidClosingDay}
	result := s.route(s.buildCreateCard(), "cadastra meu nubank")
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "fechamento precisa estar entre 1 e 31")
}

func (s *CardsRouterSuite) TestCreateCard_GenericErrorFallback() {
	s.creator = &fakeCardCreator{err: errors.New("boom")}
	result := s.route(s.buildCreateCard(), "cadastra meu nubank")
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Não consegui cadastrar o cartão")
}

func (s *CardsRouterSuite) TestCountCards_MissingResolverIsHonest() {
	result := s.route(intent.NewCountCards(), "quantos cartoes eu tenho")
	s.Equal(intent.KindCountCards, result.Kind)
	s.Equal(tools.OutcomeMissingResolver, result.Outcome)
}

func (s *CardsRouterSuite) TestCountCards_Zero() {
	s.counter = &fakeCardCounter{total: 0}
	result := s.route(intent.NewCountCards(), "quantos cartoes eu tenho")
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(1, s.counter.calls)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "ainda não tem cartões")
}

func (s *CardsRouterSuite) TestCountCards_Singular() {
	s.counter = &fakeCardCounter{total: 1}
	result := s.route(intent.NewCountCards(), "quantos cartoes eu tenho")
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "1 cartão")
}

func (s *CardsRouterSuite) TestCountCards_Plural() {
	s.counter = &fakeCardCounter{total: 3}
	result := s.route(intent.NewCountCards(), "quantos cartoes eu tenho")
	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "3 cartões")
}

func (s *CardsRouterSuite) TestCountCards_UsecaseErrorIsHonest() {
	s.counter = &fakeCardCounter{err: errors.New("boom")}
	result := s.route(intent.NewCountCards(), "quantos cartoes eu tenho")
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Require().Len(s.wa.sent, 1)
}

func TestCardsRouterSuite(t *testing.T) {
	suite.Run(t, new(CardsRouterSuite))
}

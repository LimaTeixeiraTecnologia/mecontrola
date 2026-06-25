package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type recordingIncomeReader struct {
	result   tools.IncomeSummaryResult
	err      error
	lastIn   tools.IncomeSummaryInput
	executed bool
}

func (r *recordingIncomeReader) Execute(_ context.Context, in tools.IncomeSummaryInput) (tools.IncomeSummaryResult, error) {
	r.executed = true
	r.lastIn = in
	return r.result, r.err
}

type IncomeRoutingSuite struct {
	suite.Suite
	ctx context.Context
	wa  *fakeWhatsAppGateway
}

func TestIncomeRoutingSuite(t *testing.T) {
	suite.Run(t, new(IncomeRoutingSuite))
}

func (s *IncomeRoutingSuite) SetupTest() {
	s.ctx = context.Background()
	s.wa = &fakeWhatsAppGateway{}
}

func (s *IncomeRoutingSuite) buildRouter(reader tools.IncomeSummaryReader) *services.IntentRouter {
	obs := fake.NewProvider()
	queryIntent, err := intent.NewQueryIncomeSummary("2026-06")
	s.Require().NoError(err)
	router, err := services.NewIntentRouter(obs, services.IntentRouterDeps{
		Parser:              &fakeParser{intent: queryIntent},
		IncomeSummaryReader: reader,
		Fallback:            &fakeFallback{reply: "fallback"},
		WhatsAppGateway:     s.wa,
		Location:            time.UTC,
	})
	s.Require().NoError(err)
	return router
}

func (s *IncomeRoutingSuite) TestRoutesQueryIncomeSummaryEndToEnd() {
	reader := &recordingIncomeReader{
		result: tools.IncomeSummaryResult{
			RefMonth:   "2026-06",
			TotalCents: 500000,
			Sources: []tools.IncomeSourceView{
				{Description: "Salário", AmountCents: 500000},
			},
		},
	}
	router := s.buildRouter(reader)

	result := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "quanto recebi esse mês?", WhatsAppTo: "+5511999"},
	)

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindQueryIncomeSummary, result.Kind)
	s.True(reader.executed, "o reader de income deve ser chamado via registry → tool → binding")
	s.Equal("2026-06", reader.lastIn.RefMonth)
	s.Contains(result.Reply, "Entradas de 2026-06")
	s.Contains(result.Reply, "Salário")
	s.Len(s.wa.sent, 1)
}

func (s *IncomeRoutingSuite) TestQueryIncomeSummaryEmptyMonth() {
	reader := &recordingIncomeReader{
		result: tools.IncomeSummaryResult{RefMonth: "2026-06", TotalCents: 0},
	}
	router := s.buildRouter(reader)

	result := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "quanto recebi esse mês?", WhatsAppTo: "+5511999"},
	)

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal(intent.KindQueryIncomeSummary, result.Kind)
	s.Contains(result.Reply, "Nenhuma entrada")
}

func (s *IncomeRoutingSuite) TestQueryIncomeSummaryReaderError() {
	reader := &recordingIncomeReader{err: errors.New("falha no banco")}
	router := s.buildRouter(reader)

	result := router.RouteWhatsApp(
		s.ctx,
		services.Principal{UserID: uuid.New()},
		services.InboundMessage{Text: "quanto recebi esse mês?", WhatsAppTo: "+5511999"},
	)

	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
	s.Equal(intent.KindQueryIncomeSummary, result.Kind)
}

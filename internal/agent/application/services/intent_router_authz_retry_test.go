package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

type capturingExpenseLogger struct {
	seenUserID string
	calls      int
	err        error
	result     services.ExpenseRecorderResult
}

func (c *capturingExpenseLogger) Execute(_ context.Context, in services.ExpenseRecorderInput) (services.ExpenseRecorderResult, error) {
	c.calls++
	c.seenUserID = in.UserID
	return c.result, c.err
}

type flakyMonthlySummary struct {
	calls     int
	failUntil int
	transient error
	out       budgetsoutput.MonthlySummaryOutput
}

func (f *flakyMonthlySummary) Execute(_ context.Context, _ string, _ string) (budgetsoutput.MonthlySummaryOutput, error) {
	f.calls++
	if f.calls < f.failUntil {
		return budgetsoutput.MonthlySummaryOutput{}, f.transient
	}
	return f.out, nil
}

type AuthzRetrySuite struct {
	suite.Suite
}

func TestAuthzRetrySuite(t *testing.T) {
	suite.Run(t, new(AuthzRetrySuite))
}

func (s *AuthzRetrySuite) buildLogExpense() intent.Intent {
	expense, err := intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents:  5800,
		Merchant:     "iFood",
		CategoryHint: "Prazeres",
	})
	require.NoError(s.T(), err)
	return expense
}

func (s *AuthzRetrySuite) TestWrite_UsesPrincipalUserID() {
	logger := &capturingExpenseLogger{result: services.ExpenseRecorderResult{Persisted: true, AmountCents: 5800}}
	parser := &fakeParser{intent: s.buildLogExpense()}
	router, err := services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{
		Parser:          parser,
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: &fakeWhatsAppGateway{},
		ExpenseRecorder: logger,
		Location:        time.UTC,
	})
	require.NoError(s.T(), err)

	owner := uuid.New()
	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: owner}, services.InboundMessage{Text: "gastei 58 no iFood", WhatsAppTo: "+5511999"})

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal(owner.String(), logger.seenUserID)
	s.Equal(1, logger.calls)
}

func (s *AuthzRetrySuite) TestWrite_NotRetriedOnTransientError() {
	logger := &capturingExpenseLogger{err: context.DeadlineExceeded}
	parser := &fakeParser{intent: s.buildLogExpense()}
	router, err := services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{
		Parser:          parser,
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: &fakeWhatsAppGateway{},
		ExpenseRecorder: logger,
		Location:        time.UTC,
	})
	require.NoError(s.T(), err)

	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "gastei 58 no iFood", WhatsAppTo: "+5511999"})

	s.Equal(services.OutcomeUsecaseError, result.Outcome)
	s.Equal(1, logger.calls)
}

func (s *AuthzRetrySuite) TestRead_RetriedOnTransientThenSucceeds() {
	planned := int64(120000)
	summary := &flakyMonthlySummary{
		failUntil: 2,
		transient: context.DeadlineExceeded,
		out: budgetsoutput.MonthlySummaryOutput{
			Competence:        "2026-06",
			TotalSpentCents:   45000,
			TotalPlannedCents: &planned,
		},
	}
	parsed, err := intent.NewMonthlySummary("2026-06")
	require.NoError(s.T(), err)
	parser := &fakeParser{intent: parsed}
	router, err := services.NewIntentRouter(noop.NewProvider(), services.IntentRouterDeps{
		Parser:          parser,
		MonthlySummary:  summary,
		Fallback:        &fakeFallback{reply: "fallback"},
		WhatsAppGateway: &fakeWhatsAppGateway{},
		Location:        time.UTC,
	})
	require.NoError(s.T(), err)

	result := router.RouteWhatsApp(context.Background(), services.Principal{UserID: uuid.New()}, services.InboundMessage{Text: "resumo do mês", WhatsAppTo: "+5511999"})

	s.Equal(services.OutcomeRouted, result.Outcome)
	s.Equal(2, summary.calls)
}

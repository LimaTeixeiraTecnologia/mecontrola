package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AlertWorkflowSuite struct {
	suite.Suite
}

func TestAlertWorkflowSuite(t *testing.T) {
	suite.Run(t, new(AlertWorkflowSuite))
}

func (s *AlertWorkflowSuite) comp(raw string) valueobjects.Competence {
	c, err := valueobjects.NewCompetence(raw)
	s.Require().NoError(err)
	return c
}

func (s *AlertWorkflowSuite) TestIsRetroactiveAlert() {
	type tc struct {
		name    string
		expense string
		cutoff  string
		want    bool
	}

	cases := []tc{
		{name: "competência atual não é retroativa", expense: "2026-06", cutoff: "2026-06", want: false},
		{name: "competência futura não é retroativa", expense: "2026-07", cutoff: "2026-06", want: false},
		{name: "competência anterior é retroativa", expense: "2026-05", cutoff: "2026-06", want: true},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			s.Equal(c.want, services.IsRetroactiveAlert(s.comp(c.expense), s.comp(c.cutoff)))
		})
	}
}

func (s *AlertWorkflowSuite) TestDecideAlertForInsert() {
	type tc struct {
		name           string
		isRetroactive  bool
		deliveredCount int
		wantState      entities.AlertState
		wantLogKey     string
		wantErrCtx     string
	}

	cases := []tc{
		{
			name:           "retroativo prevalece sobre rate-limit",
			isRetroactive:  true,
			deliveredCount: services.MaxDeliveredAlerts + 99,
			wantState:      entities.AlertStateSuppressedRetroactive,
			wantLogKey:     "budgets.usecase.evaluate_alert.suppressed_retroactive",
			wantErrCtx:     "inserir alerta retroativo",
		},
		{
			name:           "rate-limited quando count == max",
			isRetroactive:  false,
			deliveredCount: services.MaxDeliveredAlerts,
			wantState:      entities.AlertStateRateLimited,
			wantLogKey:     "budgets.usecase.evaluate_alert.rate_limited",
			wantErrCtx:     "inserir alerta rate_limited",
		},
		{
			name:           "rate-limited quando count > max",
			isRetroactive:  false,
			deliveredCount: services.MaxDeliveredAlerts + 1,
			wantState:      entities.AlertStateRateLimited,
			wantLogKey:     "budgets.usecase.evaluate_alert.rate_limited",
			wantErrCtx:     "inserir alerta rate_limited",
		},
		{
			name:           "delivered quando dentro do limite",
			isRetroactive:  false,
			deliveredCount: services.MaxDeliveredAlerts - 1,
			wantState:      entities.AlertStateDelivered,
			wantLogKey:     "",
			wantErrCtx:     "inserir alerta",
		},
		{
			name:           "delivered quando count zero",
			isRetroactive:  false,
			deliveredCount: 0,
			wantState:      entities.AlertStateDelivered,
			wantLogKey:     "",
			wantErrCtx:     "inserir alerta",
		},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			got := services.DecideAlertForInsert(c.isRetroactive, c.deliveredCount)
			s.Equal(c.wantState, got.State)
			s.Equal(c.wantLogKey, got.LogKey)
			s.Equal(c.wantErrCtx, got.ErrorContext)
		})
	}
}

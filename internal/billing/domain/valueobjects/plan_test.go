package valueobjects_test

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type PlanSuite struct {
	suite.Suite
}

func TestPlanSuite(t *testing.T) {
	suite.Run(t, new(PlanSuite))
}

func (s *PlanSuite) SetupTest() {}

func (s *PlanSuite) TestNewPlan() {
	type args struct {
		code         string
		durationDays int
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(valueobjects.Plan, error)
	}{
		{
			name: "deve criar plano mensal",
			args: args{code: "MONTHLY", durationDays: 30},
			expect: func(plan valueobjects.Plan, err error) {
				require.NoError(s.T(), err)
				assert.Equal(s.T(), valueobjects.PlanCodeMonthly, plan.Code())
				assert.Equal(s.T(), 30, plan.DurationDays())
				assert.Equal(s.T(), 30*24*time.Hour, plan.Duration())
			},
		},
		{
			name: "deve criar plano trimestral",
			args: args{code: "QUARTERLY", durationDays: 90},
			expect: func(plan valueobjects.Plan, err error) {
				require.NoError(s.T(), err)
				assert.Equal(s.T(), valueobjects.PlanCodeQuarterly, plan.Code())
				assert.Equal(s.T(), 90, plan.DurationDays())
				assert.Equal(s.T(), 90*24*time.Hour, plan.Duration())
			},
		},
		{
			name: "deve criar plano anual",
			args: args{code: "ANNUAL", durationDays: 365},
			expect: func(plan valueobjects.Plan, err error) {
				require.NoError(s.T(), err)
				assert.Equal(s.T(), valueobjects.PlanCodeAnnual, plan.Code())
				assert.Equal(s.T(), 365, plan.DurationDays())
				assert.Equal(s.T(), 365*24*time.Hour, plan.Duration())
			},
		},
		{
			name: "deve rejeitar codigo invalido",
			args: args{code: "WEEKLY", durationDays: 7},
			expect: func(plan valueobjects.Plan, err error) {
				require.ErrorIs(s.T(), err, valueobjects.ErrPlanCodeInvalid)
				assert.Equal(s.T(), valueobjects.Plan{}, plan)
			},
		},
		{
			name: "deve rejeitar duracao invalida",
			args: args{code: "MONTHLY", durationDays: 0},
			expect: func(plan valueobjects.Plan, err error) {
				require.ErrorIs(s.T(), err, valueobjects.ErrPlanDurationInvalid)
				assert.Equal(s.T(), valueobjects.Plan{}, plan)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			plan, err := valueobjects.NewPlan(scenario.args.code, scenario.args.durationDays)
			scenario.expect(plan, err)
		})
	}
}

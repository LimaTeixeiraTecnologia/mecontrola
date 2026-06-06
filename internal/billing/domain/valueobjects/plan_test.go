package valueobjects_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

func TestNewPlan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		code         string
		durationDays int
		wantCode     valueobjects.PlanCode
		wantErr      error
	}{
		{
			name:         "monthly plan",
			code:         "MONTHLY",
			durationDays: 30,
			wantCode:     valueobjects.PlanCodeMonthly,
		},
		{
			name:         "quarterly plan",
			code:         "QUARTERLY",
			durationDays: 90,
			wantCode:     valueobjects.PlanCodeQuarterly,
		},
		{
			name:         "annual plan",
			code:         "ANNUAL",
			durationDays: 365,
			wantCode:     valueobjects.PlanCodeAnnual,
		},
		{
			name:         "invalid code",
			code:         "WEEKLY",
			durationDays: 7,
			wantErr:      valueobjects.ErrPlanCodeInvalid,
		},
		{
			name:         "invalid duration",
			code:         "MONTHLY",
			durationDays: 0,
			wantErr:      valueobjects.ErrPlanDurationInvalid,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan, err := valueobjects.NewPlan(tt.code, tt.durationDays)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantCode, plan.Code())
			assert.Equal(t, tt.durationDays, plan.DurationDays())
			assert.Equal(t, time.Duration(tt.durationDays)*24*time.Hour, plan.Duration())
		})
	}
}

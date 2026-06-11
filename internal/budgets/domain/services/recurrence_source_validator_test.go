package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type RecurrenceSourceValidatorSuite struct {
	suite.Suite
}

func TestRecurrenceSourceValidatorSuite(t *testing.T) {
	suite.Run(t, new(RecurrenceSourceValidatorSuite))
}

func compRSV(s *RecurrenceSourceValidatorSuite, raw string) valueobjects.Competence {
	c, err := valueobjects.NewCompetence(raw)
	s.Require().NoError(err)
	return c
}

func activeBudgetWithAllocs(s *RecurrenceSourceValidatorSuite, total int64, bps ...int) entities.Budget {
	id := uuid.New()
	now := time.Now().UTC()
	allocs := make([]entities.Allocation, 0, len(bps))
	slugs := valueobjects.CanonicalOrder()
	for i, bp := range bps {
		allocs = append(allocs, entities.NewAllocation(id, slugs[i], bp, 0))
	}
	return entities.HydrateBudget(id, uuid.New(), compRSV(s, "2026-06"), total, entities.BudgetStateActive, &now, false, allocs, now, now)
}

func draftBudgetWithAllocs(s *RecurrenceSourceValidatorSuite, autoDraft bool, total int64, bps ...int) entities.Budget {
	id := uuid.New()
	now := time.Now().UTC()
	allocs := make([]entities.Allocation, 0, len(bps))
	slugs := valueobjects.CanonicalOrder()
	for i, bp := range bps {
		allocs = append(allocs, entities.NewAllocation(id, slugs[i], bp, 0))
	}
	return entities.HydrateBudget(id, uuid.New(), compRSV(s, "2026-06"), total, entities.BudgetStateDraft, nil, autoDraft, allocs, now, now)
}

func (s *RecurrenceSourceValidatorSuite) TestValidate() {
	type tc struct {
		name    string
		source  func() entities.Budget
		wantErr error
	}

	cases := []tc{
		{
			name:    "ativo com alocações completas — ok",
			source:  func() entities.Budget { return activeBudgetWithAllocs(s, 100000, 5000, 2000, 1500, 1000, 500) },
			wantErr: nil,
		},
		{
			name:    "total zero — NegativeTotal",
			source:  func() entities.Budget { return activeBudgetWithAllocs(s, 0, 5000, 2000, 1500, 1000, 500) },
			wantErr: services.ErrRecurrenceSourceNegativeTotal,
		},
		{
			name:    "auto-draft sem alocações — AutoDraftWithoutAllocs",
			source:  func() entities.Budget { return draftBudgetWithAllocs(s, true, 100000) },
			wantErr: services.ErrRecurrenceSourceAutoDraftWithoutAllocs,
		},
		{
			name:    "draft manual com soma errada — DraftWithoutFullAllocs",
			source:  func() entities.Budget { return draftBudgetWithAllocs(s, false, 100000, 5000, 2000) },
			wantErr: services.ErrRecurrenceSourceDraftWithoutFullAllocs,
		},
		{
			name:    "draft manual com soma 10000 — ok",
			source:  func() entities.Budget { return draftBudgetWithAllocs(s, false, 100000, 5000, 2000, 1500, 1000, 500) },
			wantErr: nil,
		},
	}

	v := services.NewRecurrenceSourceValidator()
	for _, c := range cases {
		s.Run(c.name, func() {
			err := v.Validate(c.source())
			if c.wantErr == nil {
				s.NoError(err)
				return
			}
			s.ErrorIs(err, c.wantErr)
		})
	}
}

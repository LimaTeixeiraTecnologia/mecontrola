package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type BudgetSuite struct {
	suite.Suite
	now        time.Time
	userID     uuid.UUID
	competence valueobjects.Competence
}

func TestBudgetSuite(t *testing.T) {
	suite.Run(t, new(BudgetSuite))
}

func (s *BudgetSuite) SetupTest() {
	s.now = time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.userID = uuid.New()
	c, _ := valueobjects.NewCompetence("2025-06")
	s.competence = c
}

func (s *BudgetSuite) TestNewBudget() {
	b := entities.NewBudget(s.userID, s.competence, 10000, s.now)
	s.True(b.IsDraft())
	s.False(b.IsActive())
	s.Equal(int64(10000), b.TotalCents())
	s.Nil(b.ActivatedAt())
}

func (s *BudgetSuite) TestActivate() {
	type testCase struct {
		name        string
		totalCents  int64
		allocations []entities.Allocation
		wantErr     bool
		errTarget   error
	}

	cases := []testCase{
		{
			name:       "ativa com soma 10000 e total > 0",
			totalCents: 10000,
			allocations: []entities.Allocation{
				entities.NewAllocation(uuid.New(), valueobjects.RootSlugCustoFixo, 5000, 5000),
				entities.NewAllocation(uuid.New(), valueobjects.RootSlugConhecimento, 5000, 5000),
			},
			wantErr: false,
		},
		{
			name:       "falha se total zero",
			totalCents: 0,
			allocations: []entities.Allocation{
				entities.NewAllocation(uuid.New(), valueobjects.RootSlugCustoFixo, 10000, 0),
			},
			wantErr:   true,
			errTarget: entities.ErrBudgetTotalMustBePositive,
		},
		{
			name:       "falha se soma != 10000",
			totalCents: 10000,
			allocations: []entities.Allocation{
				entities.NewAllocation(uuid.New(), valueobjects.RootSlugCustoFixo, 5000, 5000),
			},
			wantErr:   true,
			errTarget: entities.ErrBudgetAllocationSumMustBe10000,
		},
		{
			name:       "falha se já ativo",
			totalCents: 10000,
			allocations: []entities.Allocation{
				entities.NewAllocation(uuid.New(), valueobjects.RootSlugCustoFixo, 10000, 10000),
			},
			wantErr:   true,
			errTarget: entities.ErrBudgetAlreadyActive,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			b := entities.NewBudget(s.userID, s.competence, tc.totalCents, s.now)
			b.SetAllocations(tc.allocations)

			if tc.errTarget == entities.ErrBudgetAlreadyActive {
				_ = b.Activate(s.now)
			}

			err := b.Activate(s.now)
			if tc.wantErr {
				s.Error(err)
				if tc.errTarget != nil {
					s.ErrorIs(err, tc.errTarget)
				}
				return
			}
			s.NoError(err)
			s.True(b.IsActive())
			s.NotNil(b.ActivatedAt())
		})
	}
}

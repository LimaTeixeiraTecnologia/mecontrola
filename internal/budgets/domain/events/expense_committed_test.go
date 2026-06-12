package events_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExpenseCommittedSuite struct {
	suite.Suite
	expenseID     uuid.UUID
	userID        uuid.UUID
	subcategoryID uuid.UUID
	rootSlug      valueobjects.RootSlug
	competence    valueobjects.Competence
	cutoff        valueobjects.Competence
	now           time.Time
}

func TestExpenseCommittedSuite(t *testing.T) {
	suite.Run(t, new(ExpenseCommittedSuite))
}

func (s *ExpenseCommittedSuite) SetupTest() {
	s.expenseID = uuid.New()
	s.userID = uuid.New()
	s.subcategoryID = uuid.New()
	s.rootSlug = valueobjects.RootSlugCustoFixo
	c, _ := valueobjects.NewCompetence("2025-06")
	s.competence = c
	cu, _ := valueobjects.NewCompetence("2025-05")
	s.cutoff = cu
	s.now = time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
}

func (s *ExpenseCommittedSuite) TestNewExpenseCommittedSuccess() {
	evt, err := events.NewExpenseCommitted(
		s.expenseID, s.userID, s.subcategoryID, s.rootSlug, s.competence,
		valueobjects.MutationKindCreate, s.now, s.cutoff,
	)
	s.Require().NoError(err)
	s.Equal(s.expenseID, evt.ExpenseID)
	s.Equal(s.userID, evt.UserID)
	s.Equal(s.subcategoryID, evt.SubcategoryID)
	s.Equal(s.rootSlug, evt.RootSlug)
	s.Equal(s.competence, evt.Competence)
	s.Equal(valueobjects.MutationKindCreate, evt.MutationKind)
	s.Equal(s.now, evt.CommittedAt)
	s.Equal(s.cutoff, evt.CutoffCompetenceBR)
}

func (s *ExpenseCommittedSuite) TestNewExpenseCommittedRejectsInvalid() {
	type testCase struct {
		name          string
		expenseID     uuid.UUID
		userID        uuid.UUID
		subcategoryID uuid.UUID
		mutationKind  valueobjects.MutationKind
		committedAt   time.Time
	}

	cases := []testCase{
		{name: "expense_id nil", expenseID: uuid.Nil, userID: s.userID, subcategoryID: s.subcategoryID, mutationKind: valueobjects.MutationKindCreate, committedAt: s.now},
		{name: "user_id nil", expenseID: s.expenseID, userID: uuid.Nil, subcategoryID: s.subcategoryID, mutationKind: valueobjects.MutationKindCreate, committedAt: s.now},
		{name: "subcategory_id nil", expenseID: s.expenseID, userID: s.userID, subcategoryID: uuid.Nil, mutationKind: valueobjects.MutationKindCreate, committedAt: s.now},
		{name: "mutation_kind zero", expenseID: s.expenseID, userID: s.userID, subcategoryID: s.subcategoryID, mutationKind: valueobjects.MutationKind(0), committedAt: s.now},
		{name: "committed_at zero", expenseID: s.expenseID, userID: s.userID, subcategoryID: s.subcategoryID, mutationKind: valueobjects.MutationKindCreate, committedAt: time.Time{}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := events.NewExpenseCommitted(
				tc.expenseID, tc.userID, tc.subcategoryID, s.rootSlug, s.competence,
				tc.mutationKind, tc.committedAt, s.cutoff,
			)
			s.Require().Error(err)
			s.ErrorIs(err, events.ErrExpenseCommittedInvalid)
		})
	}
}

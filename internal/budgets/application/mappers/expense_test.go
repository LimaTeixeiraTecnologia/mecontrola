package mappers_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExpenseMapperSuite struct {
	suite.Suite
}

func TestExpenseMapperSuite(t *testing.T) {
	suite.Run(t, new(ExpenseMapperSuite))
}

func (s *ExpenseMapperSuite) TestExpense() {
	id := uuid.New()
	userID := uuid.New()
	subID := uuid.New()
	source, err := valueobjects.NewProducerSource("user")
	s.Require().NoError(err)
	extIDRaw := uuid.New().String()
	extID, err := valueobjects.NewExternalTransactionID(extIDRaw)
	s.Require().NoError(err)
	rootSlug, err := valueobjects.ParseRootSlug("expense.custo_fixo")
	s.Require().NoError(err)
	competence, err := valueobjects.NewCompetence("2026-06")
	s.Require().NoError(err)

	occurredAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	tombstone := int64(7)
	deletedAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		expense  entities.Expense
		wantTomb *int64
		wantDel  *time.Time
	}{
		{
			name: "expense ativo sem tombstone",
			expense: entities.HydrateExpense(
				id, userID, source, extID, subID, rootSlug, competence,
				1500, occurredAt, 2, nil, nil, createdAt, updatedAt,
			),
		},
		{
			name: "expense excluido com tombstone",
			expense: entities.HydrateExpense(
				id, userID, source, extID, subID, rootSlug, competence,
				1500, occurredAt, 7, &tombstone, &deletedAt, createdAt, updatedAt,
			),
			wantTomb: &tombstone,
			wantDel:  &deletedAt,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			out := mappers.M.Expense(tt.expense)
			s.Equal(id.String(), out.ID)
			s.Equal(userID.String(), out.UserID)
			s.Equal("user", out.Source)
			s.Equal(extIDRaw, out.ExternalTransactionID)
			s.Equal(subID.String(), out.SubcategoryID)
			s.Equal("expense.custo_fixo", out.RootSlug)
			s.Equal("2026-06", out.Competence)
			s.Equal(int64(1500), out.AmountCents)
			s.Equal(occurredAt, out.OccurredAt)
			s.Equal(tt.expense.Version(), out.Version)
			s.Equal(tt.wantTomb, out.TombstoneVersion)
			s.Equal(tt.wantDel, out.DeletedAt)
			s.Equal(createdAt, out.CreatedAt)
			s.Equal(updatedAt, out.UpdatedAt)
		})
	}
}

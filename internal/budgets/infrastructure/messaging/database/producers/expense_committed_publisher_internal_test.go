package producers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExpenseCommittedPayloadGoldenSuite struct {
	suite.Suite
}

func TestExpenseCommittedPayloadGoldenSuite(t *testing.T) {
	suite.Run(t, new(ExpenseCommittedPayloadGoldenSuite))
}

func (s *ExpenseCommittedPayloadGoldenSuite) TestJSONShapeFrozen() {
	expenseID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	subcategoryID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	competence, err := valueobjects.NewCompetence("2025-06")
	s.Require().NoError(err)
	cutoff, err := valueobjects.NewCompetence("2025-05")
	s.Require().NoError(err)
	committedAt := time.Date(2025, 6, 15, 12, 30, 45, 0, time.UTC)

	evt, err := events.NewExpenseCommitted(
		expenseID, userID, subcategoryID,
		valueobjects.RootSlugCustoFixo, competence,
		valueobjects.MutationKindCreate, committedAt, cutoff,
	)
	s.Require().NoError(err)

	payload := expenseCommittedPayload{
		UserID:             evt.UserID.String(),
		Competence:         evt.Competence.String(),
		SubcategoryID:      evt.SubcategoryID.String(),
		RootSlug:           evt.RootSlug.String(),
		MutationKind:       evt.MutationKind.String(),
		CommittedAt:        evt.CommittedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		CutoffCompetenceBR: evt.CutoffCompetenceBR.String(),
	}

	raw, err := json.Marshal(payload)
	s.Require().NoError(err)

	expected := `{"user_id":"22222222-2222-2222-2222-222222222222","competence":"2025-06","subcategory_id":"33333333-3333-3333-3333-333333333333","root_slug":"expense.custo_fixo","mutation_kind":"create","committed_at":"2025-06-15T12:30:45Z","cutoff_competence_br":"2025-05"}`
	s.JSONEq(expected, string(raw))
	s.Equal(expected, string(raw))
}

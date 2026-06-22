package usecases

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
)

type BudgetDraftEnvelopeSuite struct {
	suite.Suite
}

func TestBudgetDraftEnvelopeSuite(t *testing.T) {
	suite.Run(t, new(BudgetDraftEnvelopeSuite))
}

func (s *BudgetDraftEnvelopeSuite) TestEncodeDecodeRoundTrip() {
	draft, err := budgetdraft.New("2026-06").Merge(budgetdraft.Change{
		TotalCents:  500000,
		Allocations: map[string]int{budgetdraft.SlugCustoFixo: 3500},
	})
	s.Require().NoError(err)

	raw, err := EncodeBudgetDraft(draft)
	s.Require().NoError(err)
	s.True(IsBudgetConfigPending(raw))

	decoded, err := DecodeBudgetDraft(raw)
	s.Require().NoError(err)
	s.Equal(int64(500000), decoded.TotalCents())
	s.Equal("2026-06", decoded.Competence())
	s.Equal(3500, decoded.SumBasisPoints())
}

func (s *BudgetDraftEnvelopeSuite) TestIsBudgetConfigPendingFalseForEmptyOrOther() {
	s.False(IsBudgetConfigPending(nil))
	s.False(IsBudgetConfigPending([]byte("{}")))
	s.False(IsBudgetConfigPending([]byte(`{"kind":"awaiting_amount"}`)))
	s.False(IsBudgetConfigPending([]byte("garbage")))
}

func (s *BudgetDraftEnvelopeSuite) TestDecodeRejectsWrongKind() {
	_, err := DecodeBudgetDraft([]byte(`{"kind":"other","total_cents":1}`))
	s.Require().Error(err)
}

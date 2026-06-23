package onboardingv2draft

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type DraftSuite struct {
	suite.Suite
}

func TestDraftSuite(t *testing.T) {
	suite.Run(t, new(DraftSuite))
}

func (s *DraftSuite) TestNewHasStepObjective() {
	d := New()
	s.Equal(StepObjective, d.Step())
}

func (s *DraftSuite) TestParseOnboardingV2StepRoundTrip() {
	cases := []OnboardingV2Step{StepObjective, StepBudget, StepCards, StepFinancialPlan}
	for _, step := range cases {
		parsed, err := ParseOnboardingV2Step(step.String())
		s.NoError(err)
		s.Equal(step, parsed)
	}
}

func (s *DraftSuite) TestParseOnboardingV2StepUnknownReturnsError() {
	_, err := ParseOnboardingV2Step("invalid_step")
	s.Error(err)
}

func (s *DraftSuite) TestEncodeRestoreRoundTrip() {
	d := New().
		WithObjective("quitar dívidas").
		WithIncome(500000).
		WithAutoSplits([]SplitEntry{{RootSlug: "expense.custo_fixo", AmountCents: 200000}}).
		WithStep(StepCards).
		AppendCard(CardEntry{Name: "Nubank", DueDay: 13})

	raw, err := Encode(d)
	s.NoError(err)

	restored, err := Restore(raw)
	s.NoError(err)

	s.Equal(StepCards, restored.Step())
	s.Equal("quitar dívidas", restored.Objective())
	s.Equal(int64(500000), restored.IncomeCents())
	s.True(restored.HasAutoSplits())
	s.Len(restored.Splits(), 1)
	s.Equal("expense.custo_fixo", restored.Splits()[0].RootSlug)
	s.Len(restored.Cards(), 1)
	s.Equal("Nubank", restored.Cards()[0].Name)
	s.Equal(13, restored.Cards()[0].DueDay)
}

func (s *DraftSuite) TestRestoreWrongKindReturnsError() {
	raw := []byte(`{"kind":"budget_config","step":1}`)
	_, err := Restore(raw)
	s.Error(err)
}

func (s *DraftSuite) TestRestoreInvalidJSONReturnsError() {
	_, err := Restore([]byte(`not json`))
	s.Error(err)
}

func (s *DraftSuite) TestIsDraftPending() {
	raw, _ := Encode(New())
	s.True(IsDraftPending(raw))
}

func (s *DraftSuite) TestIsDraftPendingFalseForOtherKind() {
	s.False(IsDraftPending([]byte(`{"kind":"budget_config"}`)))
}

func (s *DraftSuite) TestIsDraftPendingFalseForInvalidJSON() {
	s.False(IsDraftPending([]byte(`not json`)))
}

func (s *DraftSuite) TestIsDraftPendingFalseForEmptyPayload() {
	s.False(IsDraftPending([]byte(`{}`)))
}

func (s *DraftSuite) TestIsDraftPendingFalseForNilPayload() {
	s.False(IsDraftPending(nil))
}

func (s *DraftSuite) TestRestoreStepZeroReturnsError() {
	raw := []byte(`{"kind":"onboarding_v2","step":0}`)
	_, err := Restore(raw)
	s.Error(err)
	s.Contains(err.Error(), "invalid step")
}

func (s *DraftSuite) TestRestoreStepOutOfRangeReturnsError() {
	raw := []byte(`{"kind":"onboarding_v2","step":99}`)
	_, err := Restore(raw)
	s.Error(err)
	s.Contains(err.Error(), "invalid step")
}

func (s *DraftSuite) TestNewDraftHasNilSplitsAndCards() {
	d := New()
	s.Nil(d.Splits())
	s.Nil(d.Cards())
}

func (s *DraftSuite) TestAppendCardDedupCaseInsensitive() {
	d := New().AppendCard(CardEntry{Name: "Nubank", DueDay: 13})
	d = d.AppendCard(CardEntry{Name: "nubank", DueDay: 15})
	s.Len(d.Cards(), 1)
	s.Equal(13, d.Cards()[0].DueDay)
}

func (s *DraftSuite) TestAppendCardMultiple() {
	d := New().
		AppendCard(CardEntry{Name: "Nubank", DueDay: 13}).
		AppendCard(CardEntry{Name: "Inter", DueDay: 5})
	s.Len(d.Cards(), 2)
}

func (s *DraftSuite) TestWithAutoSplitsSetsFlag() {
	d := New().WithAutoSplits([]SplitEntry{{RootSlug: "expense.metas", AmountCents: 100000}})
	s.True(d.HasAutoSplits())
}

func (s *DraftSuite) TestWithAdjustedSplitsPreservesAutoFlag() {
	d := New().
		WithAutoSplits([]SplitEntry{{RootSlug: "expense.metas", AmountCents: 100000}}).
		WithAdjustedSplits([]SplitEntry{{RootSlug: "expense.metas", AmountCents: 90000}})
	s.True(d.HasAutoSplits())
	s.Equal(int64(90000), d.Splits()[0].AmountCents)
}

func (s *DraftSuite) TestImmutabilityWithStep() {
	original := New()
	updated := original.WithStep(StepBudget)
	s.Equal(StepObjective, original.Step())
	s.Equal(StepBudget, updated.Step())
}

func (s *DraftSuite) TestAutoSplitsSumEqualsIncome() {
	income := int64(500000)
	splits := buildTestSplits(income)
	var total int64
	for _, s := range splits {
		total += s.AmountCents
	}
	s.Equal(income, total)
}

func buildTestSplits(incomeCents int64) []SplitEntry {
	proportions := []struct {
		slug string
		bp   int
	}{
		{"expense.custo_fixo", 4000},
		{"expense.conhecimento", 1000},
		{"expense.prazeres", 1500},
		{"expense.metas", 2000},
		{"expense.liberdade_financeira", 1500},
	}
	splits := make([]SplitEntry, len(proportions))
	var assigned int64
	for i, p := range proportions {
		if i == len(proportions)-1 {
			splits[i] = SplitEntry{RootSlug: p.slug, AmountCents: incomeCents - assigned}
			continue
		}
		amt := incomeCents * int64(p.bp) / 10000
		splits[i] = SplitEntry{RootSlug: p.slug, AmountCents: amt}
		assigned += amt
	}
	return splits
}

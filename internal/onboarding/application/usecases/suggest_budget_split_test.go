package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	budgetusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	onbvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type mockBudgetAllocator struct {
	fn func(ctx context.Context, totalCents int64, bp []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error)
}

func (m *mockBudgetAllocator) Suggest(ctx context.Context, totalCents int64, bp []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error) {
	return m.fn(ctx, totalCents, bp)
}

type SuggestBudgetSplitSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
	uid uuid.UUID
}

func TestSuggestBudgetSplitSuite(t *testing.T) {
	suite.Run(t, new(SuggestBudgetSplitSuite))
}

func (s *SuggestBudgetSplitSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.uid = uuid.MustParse("11111111-1111-1111-1111-111111111111")
}

func (s *SuggestBudgetSplitSuite) TestResolvesByHint() {
	var capturedBP []budgetusecases.AllocationBP
	alloc := &mockBudgetAllocator{fn: func(_ context.Context, _ int64, bp []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error) {
		capturedBP = bp
		out := make([]budgetusecases.AllocationCents, len(bp))
		for i, b := range bp {
			out[i] = budgetusecases.AllocationCents{RootSlug: b.RootSlug, BasisPoints: b.BasisPoints, PlannedCents: int64(b.BasisPoints)}
		}
		return out, nil
	}}

	uc := NewSuggestBudgetSplit(alloc, s.obs)
	result, err := uc.Execute(s.ctx, SuggestBudgetSplitInput{
		UserID:           s.uid,
		ObjectiveProfile: "payoff_debt",
		Objective:        "",
		IncomeCents:      100000,
	})

	s.NoError(err)
	s.Len(result.Splits, 5)
	expected := onbvo.SplitTemplate(onbvo.ProfilePayoffDebt)
	s.Len(capturedBP, len(expected))
	s.Equal(expected[0].BasisPoints, capturedBP[0].BasisPoints)
	s.Equal(expected[0].RootSlug, capturedBP[0].RootSlug)
}

func (s *SuggestBudgetSplitSuite) TestFallbackToKeyword() {
	var capturedBP []budgetusecases.AllocationBP
	alloc := &mockBudgetAllocator{fn: func(_ context.Context, _ int64, bp []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error) {
		capturedBP = bp
		out := make([]budgetusecases.AllocationCents, len(bp))
		for i, b := range bp {
			out[i] = budgetusecases.AllocationCents{RootSlug: b.RootSlug, BasisPoints: b.BasisPoints, PlannedCents: int64(b.BasisPoints)}
		}
		return out, nil
	}}

	uc := NewSuggestBudgetSplit(alloc, s.obs)
	result, err := uc.Execute(s.ctx, SuggestBudgetSplitInput{
		UserID:           s.uid,
		ObjectiveProfile: "",
		Objective:        "quero quitar minha dívida",
		IncomeCents:      100000,
	})

	s.NoError(err)
	s.Len(result.Splits, 5)
	expected := onbvo.SplitTemplate(onbvo.ProfilePayoffDebt)
	s.Equal(expected[0].BasisPoints, capturedBP[0].BasisPoints)
}

func (s *SuggestBudgetSplitSuite) TestDefaultWhenAmbiguous() {
	var capturedBP []budgetusecases.AllocationBP
	alloc := &mockBudgetAllocator{fn: func(_ context.Context, _ int64, bp []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error) {
		capturedBP = bp
		out := make([]budgetusecases.AllocationCents, len(bp))
		for i, b := range bp {
			out[i] = budgetusecases.AllocationCents{RootSlug: b.RootSlug, BasisPoints: b.BasisPoints, PlannedCents: int64(b.BasisPoints)}
		}
		return out, nil
	}}

	uc := NewSuggestBudgetSplit(alloc, s.obs)
	result, err := uc.Execute(s.ctx, SuggestBudgetSplitInput{
		UserID:           s.uid,
		ObjectiveProfile: "",
		Objective:        "quero organizar minha vida financeira",
		IncomeCents:      100000,
	})

	s.NoError(err)
	s.Len(result.Splits, 5)
	expected := onbvo.SplitTemplate(onbvo.ProfileOrganizeSpending)
	s.Equal(expected[0].BasisPoints, capturedBP[0].BasisPoints)
}

func (s *SuggestBudgetSplitSuite) TestInvalidHintFallsBackToKeyword() {
	var capturedBP []budgetusecases.AllocationBP
	alloc := &mockBudgetAllocator{fn: func(_ context.Context, _ int64, bp []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error) {
		capturedBP = bp
		out := make([]budgetusecases.AllocationCents, len(bp))
		for i, b := range bp {
			out[i] = budgetusecases.AllocationCents{RootSlug: b.RootSlug, BasisPoints: b.BasisPoints, PlannedCents: int64(b.BasisPoints)}
		}
		return out, nil
	}}

	uc := NewSuggestBudgetSplit(alloc, s.obs)
	result, err := uc.Execute(s.ctx, SuggestBudgetSplitInput{
		UserID:           s.uid,
		ObjectiveProfile: "invalid_hint",
		Objective:        "quero montar reserva de emergência",
		IncomeCents:      100000,
	})

	s.NoError(err)
	s.Len(result.Splits, 5)
	expected := onbvo.SplitTemplate(onbvo.ProfileEmergencyFund)
	s.Equal(expected[0].BasisPoints, capturedBP[0].BasisPoints)
}

func (s *SuggestBudgetSplitSuite) TestNilUserIDRejected() {
	alloc := &mockBudgetAllocator{}
	uc := NewSuggestBudgetSplit(alloc, s.obs)
	_, err := uc.Execute(s.ctx, SuggestBudgetSplitInput{
		UserID:      uuid.Nil,
		IncomeCents: 100000,
	})
	s.Error(err)
}

func (s *SuggestBudgetSplitSuite) TestZeroIncomeCentsRejected() {
	alloc := &mockBudgetAllocator{}
	uc := NewSuggestBudgetSplit(alloc, s.obs)
	_, err := uc.Execute(s.ctx, SuggestBudgetSplitInput{
		UserID:      s.uid,
		IncomeCents: 0,
	})
	s.Error(err)
}

func (s *SuggestBudgetSplitSuite) TestAllocatorErrorPropagates() {
	alloc := &mockBudgetAllocator{fn: func(_ context.Context, _ int64, _ []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error) {
		return nil, errors.New("allocator failure")
	}}
	uc := NewSuggestBudgetSplit(alloc, s.obs)
	_, err := uc.Execute(s.ctx, SuggestBudgetSplitInput{
		UserID:      s.uid,
		IncomeCents: 100000,
	})
	s.Error(err)
	s.ErrorContains(err, "allocator failure")
}

func (s *SuggestBudgetSplitSuite) TestDelegatesNoCentsMathInUsecase() {
	alloc := &mockBudgetAllocator{fn: func(_ context.Context, totalCents int64, bp []budgetusecases.AllocationBP) ([]budgetusecases.AllocationCents, error) {
		out := make([]budgetusecases.AllocationCents, len(bp))
		for i, b := range bp {
			out[i] = budgetusecases.AllocationCents{RootSlug: b.RootSlug, BasisPoints: b.BasisPoints, PlannedCents: totalCents * int64(b.BasisPoints) / 10000}
		}
		return out, nil
	}}

	uc := NewSuggestBudgetSplit(alloc, s.obs)
	income := int64(500000)
	result, err := uc.Execute(s.ctx, SuggestBudgetSplitInput{
		UserID:      s.uid,
		IncomeCents: income,
	})

	s.NoError(err)
	var total int64
	for _, sp := range result.Splits {
		total += sp.PlannedCents
	}
	s.True(total <= income, "sum of planned cents must not exceed income")
}

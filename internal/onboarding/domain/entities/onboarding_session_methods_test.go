package entities_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func newSession(t *testing.T) entities.OnboardingSession {
	t.Helper()
	session, err := entities.NewOnboardingSession(
		uuid.New(),
		entities.OnboardingChannelWhatsApp,
		valueobjects.OnboardingStateAwaitingIncome,
		time.Now().UTC(),
	)
	require.NoError(t, err)
	return session
}

func fullAllocation(t *testing.T) valueobjects.BudgetAllocation {
	t.Helper()
	alloc, err := valueobjects.NewBudgetAllocationFromAmounts([]valueobjects.CategoryAmount{
		{Kind: valueobjects.CategoryKindFixedCost, AmountCents: 200000},
		{Kind: valueobjects.CategoryKindKnowledge, AmountCents: 50000},
		{Kind: valueobjects.CategoryKindPleasures, AmountCents: 75000},
		{Kind: valueobjects.CategoryKindGoals, AmountCents: 100000},
		{Kind: valueobjects.CategoryKindFinancialFreedom, AmountCents: 75000},
	}, 500000)
	require.NoError(t, err)
	return alloc
}

func TestNewOnboardingCardDraft_HappyPath(t *testing.T) {
	t.Parallel()
	got, err := entities.NewOnboardingCardDraft("  nubank ", 17)
	require.NoError(t, err)
	require.Equal(t, "nubank", got.Name)
	require.Equal(t, 17, got.DueDay)
	require.Equal(t, 0, got.ClosingDay)
	require.Equal(t, int64(0), got.LimitCents)
}

func TestNewOnboardingCardDraft_EmptyNickname(t *testing.T) {
	t.Parallel()
	_, err := entities.NewOnboardingCardDraft("  ", 17)
	require.Error(t, err)
	require.True(t, errors.Is(err, entities.ErrOnboardingCardNicknameRequired))
}

func TestNewOnboardingCardDraft_InvalidDueDay(t *testing.T) {
	t.Parallel()
	_, err := entities.NewOnboardingCardDraft("nubank", 40)
	require.Error(t, err)
}

func TestWithObjective(t *testing.T) {
	t.Parallel()
	objective, err := valueobjects.NewFinancialObjective("fazer uma viagem")
	require.NoError(t, err)
	got := newSession(t).WithObjective(objective, time.Now().UTC())
	require.Equal(t, "fazer uma viagem", got.Payload().Objective)
}

func TestWithIncome(t *testing.T) {
	t.Parallel()
	income, err := valueobjects.NewMonthlyIncome(500000)
	require.NoError(t, err)
	got := newSession(t).WithIncome(income, time.Now().UTC())
	require.Equal(t, int64(500000), got.Payload().IncomeCents)
}

func TestWithAppendedCard_Dedupes(t *testing.T) {
	t.Parallel()
	card, err := entities.NewOnboardingCardDraft("nubank", 17)
	require.NoError(t, err)
	session := newSession(t).WithAppendedCard(card, time.Now().UTC())
	again, err := entities.NewOnboardingCardDraft("NuBank", 20)
	require.NoError(t, err)
	session = session.WithAppendedCard(again, time.Now().UTC())
	require.Len(t, session.Payload().Cards, 1)
	require.Equal(t, 20, session.Payload().Cards[0].DueDay)
}

func TestWithCustomSplit(t *testing.T) {
	t.Parallel()
	got := newSession(t).WithCustomSplit(fullAllocation(t), time.Now().UTC())
	require.Len(t, got.Payload().CustomSplit, 5)
}

func TestIsReadyToComplete(t *testing.T) {
	t.Parallel()
	objective, err := valueobjects.NewFinancialObjective("viagem")
	require.NoError(t, err)
	income, err := valueobjects.NewMonthlyIncome(500000)
	require.NoError(t, err)

	session := newSession(t).
		WithObjective(objective, time.Now().UTC()).
		WithIncome(income, time.Now().UTC()).
		WithCustomSplit(fullAllocation(t), time.Now().UTC())
	require.False(t, session.IsReadyToComplete())
	require.False(t, session.HasFirstTransaction())

	session = session.WithFirstTransactionRecorded(time.Now().UTC())
	require.True(t, session.HasFirstTransaction())
	require.True(t, session.IsReadyToComplete())
}

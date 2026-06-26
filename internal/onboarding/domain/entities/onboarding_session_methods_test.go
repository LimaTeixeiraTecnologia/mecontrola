package entities_test

import (
	"errors"
	"fmt"
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
	got, err := entities.NewOnboardingCardDraft("  nubank ", 20, 10)
	require.NoError(t, err)
	require.Equal(t, "nubank", got.Name)
	require.Equal(t, 10, got.ClosingDay)
	require.Equal(t, 20, got.DueDay)
	require.Equal(t, int64(0), got.LimitCents)
}

func TestNewOnboardingCardDraft_EmptyNickname(t *testing.T) {
	t.Parallel()
	_, err := entities.NewOnboardingCardDraft("  ", 20, 10)
	require.Error(t, err)
	require.True(t, errors.Is(err, entities.ErrOnboardingCardNicknameRequired))
}

func TestNewOnboardingCardDraft_InvalidClosingDay(t *testing.T) {
	t.Parallel()
	_, err := entities.NewOnboardingCardDraft("nubank", 20, 40)
	require.Error(t, err)
}

func TestNewOnboardingCardDraft_InvalidDueDay(t *testing.T) {
	t.Parallel()
	_, err := entities.NewOnboardingCardDraft("nubank", 40, 10)
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
	card, err := entities.NewOnboardingCardDraft("nubank", 20, 10)
	require.NoError(t, err)
	session := newSession(t).WithAppendedCard(card, time.Now().UTC())
	again, err := entities.NewOnboardingCardDraft("NuBank", 25, 15)
	require.NoError(t, err)
	session = session.WithAppendedCard(again, time.Now().UTC())
	require.Len(t, session.Payload().Cards, 1)
	require.Equal(t, 15, session.Payload().Cards[0].ClosingDay)
	require.Equal(t, 25, session.Payload().Cards[0].DueDay)
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
	require.True(t, session.IsReadyToComplete())
	require.False(t, session.HasFirstTransaction())

	session = session.WithFirstTransactionRecorded(time.Now().UTC())
	require.True(t, session.HasFirstTransaction())
	require.True(t, session.IsReadyToComplete())
}

func TestIsReadyToComplete_MissingObjective(t *testing.T) {
	t.Parallel()
	income, err := valueobjects.NewMonthlyIncome(500000)
	require.NoError(t, err)

	session := newSession(t).
		WithIncome(income, time.Now().UTC()).
		WithCustomSplit(fullAllocation(t), time.Now().UTC())
	require.False(t, session.IsReadyToComplete())
}

func TestIsReadyToComplete_MissingIncome(t *testing.T) {
	t.Parallel()
	objective, err := valueobjects.NewFinancialObjective("viagem")
	require.NoError(t, err)

	session := newSession(t).
		WithObjective(objective, time.Now().UTC()).
		WithCustomSplit(fullAllocation(t), time.Now().UTC())
	require.False(t, session.IsReadyToComplete())
}

func TestIsReadyToComplete_IncompleteSplit(t *testing.T) {
	t.Parallel()
	objective, err := valueobjects.NewFinancialObjective("viagem")
	require.NoError(t, err)
	income, err := valueobjects.NewMonthlyIncome(500000)
	require.NoError(t, err)

	session := newSession(t).
		WithObjective(objective, time.Now().UTC()).
		WithIncome(income, time.Now().UTC())
	require.False(t, session.IsReadyToComplete())
}

func TestWithAppendedTurn_BoundThreePairs(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	roles := []string{"user", "assistant", "user", "assistant", "user", "assistant", "user"}
	session := newSession(t)
	for i, role := range roles {
		session = session.WithAppendedTurn(role, fmt.Sprintf("msg%d", i), now)
	}
	require.Len(t, session.Payload().RecentTurns, 6)
	require.Equal(t, "assistant", session.Payload().RecentTurns[0].Role)
	require.Equal(t, "msg1", session.Payload().RecentTurns[0].Text)
	require.Equal(t, "user", session.Payload().RecentTurns[5].Role)
	require.Equal(t, "msg6", session.Payload().RecentTurns[5].Text)
}

func TestWithWelcomeSent_Idempotent(t *testing.T) {
	t.Parallel()
	now1 := time.Now().UTC()
	now2 := now1.Add(time.Hour)
	session := newSession(t).WithWelcomeSent(now1)
	require.NotNil(t, session.Payload().WelcomeSentAt)
	require.Equal(t, now1, *session.Payload().WelcomeSentAt)
	session = session.WithWelcomeSent(now2)
	require.Equal(t, now1, *session.Payload().WelcomeSentAt)
}

func TestWithCompletion_ClearsRecentTurns(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	session := newSession(t).
		WithAppendedTurn("user", "oi", now).
		WithAppendedTurn("assistant", "olá", now)
	require.Len(t, session.Payload().RecentTurns, 2)
	session = session.WithCompletion(now)
	require.Empty(t, session.Payload().RecentTurns)
	require.NotNil(t, session.Payload().CompletedAt)
}

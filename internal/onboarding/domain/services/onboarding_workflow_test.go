package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func newSession(t *testing.T, state valueobjects.OnboardingState, payload entities.OnboardingSessionPayload) entities.OnboardingSession {
	t.Helper()
	uid := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	return entities.HydrateOnboardingSession(uid, entities.OnboardingChannelWhatsApp, state, payload, time.Unix(0, 0).UTC())
}

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.New()
	}
	return out
}

func TestDecideNext_AwaitingToken_ReplyOnly(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	s := newSession(t, valueobjects.OnboardingStateAwaitingToken, entities.OnboardingSessionPayload{})
	got, err := w.DecideNext(s, services.InboundMessage{Text: "oi"}, nil, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, services.DecisionKindReplyOnly, got.Kind)
	require.NotEmpty(t, got.OutboundText)
}

func TestDecideNext_AwaitingIncome_HappyPath(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	s := newSession(t, valueobjects.OnboardingStateAwaitingIncome, entities.OnboardingSessionPayload{})
	now := time.Now().UTC()
	got, err := w.DecideNext(s, services.InboundMessage{Text: "R$ 3500"}, ids(1), now)
	require.NoError(t, err)
	require.Equal(t, services.DecisionKindAdvanceState, got.Kind)
	require.Equal(t, valueobjects.OnboardingStateAwaitingCardDecision, got.NewState)
	require.Equal(t, int64(350000), got.NewPayload.IncomeCents)
	require.Len(t, got.DomainEvents, 1)
	_, ok := got.DomainEvents[0].(entities.IncomeRegistered)
	require.True(t, ok)
}

func TestDecideNext_AwaitingIncome_InvalidNumber(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	s := newSession(t, valueobjects.OnboardingStateAwaitingIncome, entities.OnboardingSessionPayload{})
	got, err := w.DecideNext(s, services.InboundMessage{Text: "abc"}, ids(1), time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, services.DecisionKindReplyOnly, got.Kind)
}

func TestDecideNext_AwaitingIncome_BelowMinimum(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	s := newSession(t, valueobjects.OnboardingStateAwaitingIncome, entities.OnboardingSessionPayload{})
	got, err := w.DecideNext(s, services.InboundMessage{Text: "10"}, ids(1), time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, services.DecisionKindReplyOnly, got.Kind)
}

func TestDecideNext_CardDecision_No_GoesToSplit(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	s := newSession(t, valueobjects.OnboardingStateAwaitingCardDecision, entities.OnboardingSessionPayload{IncomeCents: 350000})
	got, err := w.DecideNext(s, services.InboundMessage{Text: "nao"}, nil, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, services.DecisionKindAdvanceState, got.Kind)
	require.Equal(t, valueobjects.OnboardingStateAwaitingSplitConfirm, got.NewState)
	require.Len(t, got.NewPayload.Split, 5)
}

func TestDecideNext_CardDecision_Yes_GoesToCardName(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	s := newSession(t, valueobjects.OnboardingStateAwaitingCardDecision, entities.OnboardingSessionPayload{IncomeCents: 350000})
	got, err := w.DecideNext(s, services.InboundMessage{Text: "sim"}, nil, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, valueobjects.OnboardingStateAwaitingCardName, got.NewState)
	require.True(t, got.NewPayload.HasPending)
}

func TestDecideNext_FullCardFlow(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	now := time.Now().UTC()

	s := newSession(t, valueobjects.OnboardingStateAwaitingCardName, entities.OnboardingSessionPayload{IncomeCents: 350000, HasPending: true})
	r, err := w.DecideNext(s, services.InboundMessage{Text: "Nubank"}, nil, now)
	require.NoError(t, err)
	require.Equal(t, valueobjects.OnboardingStateAwaitingCardLimit, r.NewState)

	s = newSession(t, valueobjects.OnboardingStateAwaitingCardLimit, r.NewPayload)
	r, err = w.DecideNext(s, services.InboundMessage{Text: "R$ 5000"}, nil, now)
	require.NoError(t, err)
	require.Equal(t, valueobjects.OnboardingStateAwaitingCardClosingDay, r.NewState)
	require.Equal(t, int64(500000), r.NewPayload.PendingCard.LimitCents)

	s = newSession(t, valueobjects.OnboardingStateAwaitingCardClosingDay, r.NewPayload)
	r, err = w.DecideNext(s, services.InboundMessage{Text: "dia 27"}, nil, now)
	require.NoError(t, err)
	require.Equal(t, valueobjects.OnboardingStateAwaitingCardDueDay, r.NewState)
	require.Equal(t, 27, r.NewPayload.PendingCard.ClosingDay)

	s = newSession(t, valueobjects.OnboardingStateAwaitingCardDueDay, r.NewPayload)
	r, err = w.DecideNext(s, services.InboundMessage{Text: "5"}, ids(1), now)
	require.NoError(t, err)
	require.Equal(t, valueobjects.OnboardingStateAwaitingMoreCards, r.NewState)
	require.Len(t, r.NewPayload.Cards, 1)
	require.Equal(t, "Nubank", r.NewPayload.Cards[0].Name)
	require.Len(t, r.DomainEvents, 1)
	_, ok := r.DomainEvents[0].(entities.CardRegistered)
	require.True(t, ok)
}

func TestDecideNext_MoreCards_No_GoesToSplit(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	s := newSession(t, valueobjects.OnboardingStateAwaitingMoreCards, entities.OnboardingSessionPayload{
		Cards: []entities.OnboardingCardDraft{{Name: "Nubank", LimitCents: 500000, ClosingDay: 27, DueDay: 5}},
	})
	got, err := w.DecideNext(s, services.InboundMessage{Text: "nao"}, nil, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, valueobjects.OnboardingStateAwaitingSplitConfirm, got.NewState)
}

func TestDecideNext_SplitConfirm_Yes_CompletesOnboarding(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	split := []entities.OnboardingCardSplitEntry{
		{Kind: "fixed_cost", Percent: 40},
		{Kind: "knowledge", Percent: 10},
		{Kind: "pleasures", Percent: 15},
		{Kind: "goals", Percent: 20},
		{Kind: "financial_freedom", Percent: 15},
	}
	s := newSession(t, valueobjects.OnboardingStateAwaitingSplitConfirm, entities.OnboardingSessionPayload{Split: split})
	got, err := w.DecideNext(s, services.InboundMessage{Text: "sim"}, ids(2), time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, services.DecisionKindComplete, got.Kind)
	require.Equal(t, valueobjects.OnboardingStateActive, got.NewState)
	require.Len(t, got.DomainEvents, 2)
}

func TestDecideNext_SplitConfirm_AdjustIntent_RepliesOnly(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	s := newSession(t, valueobjects.OnboardingStateAwaitingSplitConfirm, entities.OnboardingSessionPayload{})
	got, err := w.DecideNext(s, services.InboundMessage{Text: "quero ajustar"}, nil, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, services.DecisionKindReplyOnly, got.Kind)
}

func TestDecideNext_Active_NoOp(t *testing.T) {
	t.Parallel()
	w := services.NewOnboardingWorkflow()
	s := newSession(t, valueobjects.OnboardingStateActive, entities.OnboardingSessionPayload{})
	got, err := w.DecideNext(s, services.InboundMessage{Text: "ola"}, nil, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, services.DecisionKindNoOp, got.Kind)
}

package entities

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

var (
	ErrOnboardingSessionUserIDRequired  = errors.New("onboarding: session user id required")
	ErrOnboardingSessionChannelRequired = errors.New("onboarding: session channel required")
	ErrOnboardingCardNicknameRequired   = errors.New("onboarding: card nickname required")
)

type OnboardingChannel uint8

const OnboardingChannelWhatsApp OnboardingChannel = iota + 1

func (c OnboardingChannel) String() string {
	switch c {
	case OnboardingChannelWhatsApp:
		return "whatsapp"
	default:
		return "unknown"
	}
}

func ParseOnboardingChannel(raw string) (OnboardingChannel, error) {
	switch raw {
	case "whatsapp":
		return OnboardingChannelWhatsApp, nil
	default:
		return 0, fmt.Errorf("onboarding: %q: invalid channel", raw)
	}
}

type OnboardingCardDraft struct {
	Name       string
	LimitCents int64
	ClosingDay int
	DueDay     int
}

func NewOnboardingCardDraft(nickname string, dueDay, closingDay int) (OnboardingCardDraft, error) {
	name := strings.TrimSpace(nickname)
	if name == "" {
		return OnboardingCardDraft{}, ErrOnboardingCardNicknameRequired
	}
	due, err := valueobjects.NewCardDueDay(dueDay)
	if err != nil {
		return OnboardingCardDraft{}, err
	}
	closing, err := valueobjects.NewCardClosingDay(closingDay)
	if err != nil {
		return OnboardingCardDraft{}, err
	}
	return OnboardingCardDraft{Name: name, ClosingDay: closing.Value(), DueDay: due.Value()}, nil
}

type OnboardingTurn struct {
	Role       string
	Text       string
	OccurredAt time.Time
}

type OnboardingSessionPayload struct {
	IncomeCents      int64
	Cards            []OnboardingCardDraft
	PendingCard      OnboardingCardDraft
	HasPending       bool
	Split            []OnboardingCardSplitEntry
	Objective        string
	CustomSplit      []OnboardingBudgetAllocationEntry
	FirstTxRecorded  bool
	Phase            valueobjects.OnboardingPhase
	RecentTurns      []OnboardingTurn
	WelcomeSentAt    *time.Time
	CompletedAt      *time.Time
	ObjectiveProfile string
}

type OnboardingCardSplitEntry struct {
	Kind    string
	Percent int
}

type OnboardingBudgetAllocationEntry struct {
	Kind        string
	BasisPoints int
}

type OnboardingSession struct {
	userID    uuid.UUID
	channel   OnboardingChannel
	payload   OnboardingSessionPayload
	updatedAt time.Time
}

func NewOnboardingSession(
	userID uuid.UUID,
	channel OnboardingChannel,
	updatedAt time.Time,
) (OnboardingSession, error) {
	if userID == uuid.Nil {
		return OnboardingSession{}, ErrOnboardingSessionUserIDRequired
	}
	if channel != OnboardingChannelWhatsApp {
		return OnboardingSession{}, ErrOnboardingSessionChannelRequired
	}
	return OnboardingSession{
		userID:    userID,
		channel:   channel,
		updatedAt: updatedAt,
		payload:   OnboardingSessionPayload{Phase: valueobjects.PhaseWelcome},
	}, nil
}

func HydrateOnboardingSession(
	userID uuid.UUID,
	channel OnboardingChannel,
	payload OnboardingSessionPayload,
	updatedAt time.Time,
) OnboardingSession {
	return OnboardingSession{
		userID:    userID,
		channel:   channel,
		payload:   payload,
		updatedAt: updatedAt,
	}
}

func (s OnboardingSession) UserID() uuid.UUID                 { return s.userID }
func (s OnboardingSession) Channel() OnboardingChannel        { return s.channel }
func (s OnboardingSession) Payload() OnboardingSessionPayload { return s.payload }
func (s OnboardingSession) UpdatedAt() time.Time              { return s.updatedAt }
func (s OnboardingSession) IsActive() bool                    { return s.payload.CompletedAt != nil }

func (s OnboardingSession) WithObjective(objective valueobjects.FinancialObjective, updatedAt time.Time) OnboardingSession {
	s.payload.Objective = objective.String()
	s.updatedAt = updatedAt
	return s
}

func (s OnboardingSession) WithIncome(income valueobjects.MonthlyIncome, updatedAt time.Time) OnboardingSession {
	s.payload.IncomeCents = income.Cents()
	s.updatedAt = updatedAt
	return s
}

func (s OnboardingSession) WithAppendedCard(card OnboardingCardDraft, updatedAt time.Time) OnboardingSession {
	deduped := make([]OnboardingCardDraft, 0, len(s.payload.Cards)+1)
	for _, existing := range s.payload.Cards {
		if !strings.EqualFold(strings.TrimSpace(existing.Name), strings.TrimSpace(card.Name)) {
			deduped = append(deduped, existing)
		}
	}
	deduped = append(deduped, card)
	s.payload.Cards = deduped
	s.updatedAt = updatedAt
	return s
}

func (s OnboardingSession) WithCustomSplit(allocation valueobjects.BudgetAllocation, updatedAt time.Time) OnboardingSession {
	entries := make([]OnboardingBudgetAllocationEntry, 0, 5)
	for _, a := range allocation.Allocations() {
		entries = append(entries, OnboardingBudgetAllocationEntry{Kind: a.Kind.String(), BasisPoints: a.BasisPoints})
	}
	s.payload.CustomSplit = entries
	s.updatedAt = updatedAt
	return s
}

func (s OnboardingSession) WithPhase(phase valueobjects.OnboardingPhase, updatedAt time.Time) OnboardingSession {
	s.payload.Phase = phase
	s.updatedAt = updatedAt
	return s
}

func (s OnboardingSession) WithFirstTransactionRecorded(updatedAt time.Time) OnboardingSession {
	s.payload.FirstTxRecorded = true
	s.updatedAt = updatedAt
	return s
}

func (s OnboardingSession) HasFirstTransaction() bool {
	return s.payload.FirstTxRecorded
}

func (s OnboardingSession) IsReadyToComplete() bool {
	return strings.TrimSpace(s.payload.Objective) != "" &&
		s.payload.IncomeCents > 0 &&
		len(s.payload.CustomSplit) == 5
}

const maxRecentTurnPairs = 3

func (s OnboardingSession) WithAppendedTurn(role, text string, now time.Time) OnboardingSession {
	turn := OnboardingTurn{Role: role, Text: text, OccurredAt: now}
	turns := append(append([]OnboardingTurn(nil), s.payload.RecentTurns...), turn)
	maxEntries := maxRecentTurnPairs * 2
	if len(turns) > maxEntries {
		turns = turns[len(turns)-maxEntries:]
	}
	s.payload.RecentTurns = turns
	s.updatedAt = now
	return s
}

func (s OnboardingSession) WithWelcomeSent(now time.Time) OnboardingSession {
	if s.payload.WelcomeSentAt != nil {
		return s
	}
	t := now
	s.payload.WelcomeSentAt = &t
	s.updatedAt = now
	return s
}

func (s OnboardingSession) WithCompletion(now time.Time) OnboardingSession {
	t := now
	s.payload.CompletedAt = &t
	s.payload.RecentTurns = nil
	s.updatedAt = now
	return s
}

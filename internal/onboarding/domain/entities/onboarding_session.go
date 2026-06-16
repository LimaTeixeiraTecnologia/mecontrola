package entities

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

var (
	ErrOnboardingSessionUserIDRequired  = errors.New("onboarding: session user id required")
	ErrOnboardingSessionChannelRequired = errors.New("onboarding: session channel required")
)

type OnboardingChannel uint8

const (
	OnboardingChannelWhatsApp OnboardingChannel = iota + 1
	OnboardingChannelTelegram
)

func (c OnboardingChannel) String() string {
	switch c {
	case OnboardingChannelWhatsApp:
		return "whatsapp"
	case OnboardingChannelTelegram:
		return "telegram"
	default:
		return "unknown"
	}
}

func ParseOnboardingChannel(raw string) (OnboardingChannel, error) {
	switch raw {
	case "whatsapp":
		return OnboardingChannelWhatsApp, nil
	case "telegram":
		return OnboardingChannelTelegram, nil
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

type OnboardingSessionPayload struct {
	IncomeCents int64
	Cards       []OnboardingCardDraft
	PendingCard OnboardingCardDraft
	HasPending  bool
	Split       []OnboardingCardSplitEntry
}

type OnboardingCardSplitEntry struct {
	Kind    string
	Percent int
}

type OnboardingSession struct {
	userID    uuid.UUID
	channel   OnboardingChannel
	state     valueobjects.OnboardingState
	payload   OnboardingSessionPayload
	updatedAt time.Time
}

func NewOnboardingSession(
	userID uuid.UUID,
	channel OnboardingChannel,
	state valueobjects.OnboardingState,
	updatedAt time.Time,
) (OnboardingSession, error) {
	if userID == uuid.Nil {
		return OnboardingSession{}, ErrOnboardingSessionUserIDRequired
	}
	if channel != OnboardingChannelWhatsApp && channel != OnboardingChannelTelegram {
		return OnboardingSession{}, ErrOnboardingSessionChannelRequired
	}
	return OnboardingSession{
		userID:    userID,
		channel:   channel,
		state:     state,
		updatedAt: updatedAt,
	}, nil
}

func HydrateOnboardingSession(
	userID uuid.UUID,
	channel OnboardingChannel,
	state valueobjects.OnboardingState,
	payload OnboardingSessionPayload,
	updatedAt time.Time,
) OnboardingSession {
	return OnboardingSession{
		userID:    userID,
		channel:   channel,
		state:     state,
		payload:   payload,
		updatedAt: updatedAt,
	}
}

func (s OnboardingSession) UserID() uuid.UUID                   { return s.userID }
func (s OnboardingSession) Channel() OnboardingChannel          { return s.channel }
func (s OnboardingSession) State() valueobjects.OnboardingState { return s.state }
func (s OnboardingSession) Payload() OnboardingSessionPayload   { return s.payload }
func (s OnboardingSession) UpdatedAt() time.Time                { return s.updatedAt }
func (s OnboardingSession) IsActive() bool                      { return s.state.IsTerminal() }

func (s OnboardingSession) With(state valueobjects.OnboardingState, payload OnboardingSessionPayload, updatedAt time.Time) OnboardingSession {
	s.state = state
	s.payload = payload
	s.updatedAt = updatedAt
	return s
}

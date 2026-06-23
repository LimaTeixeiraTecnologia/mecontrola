package binding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/onboardingv2draft"
)

type onboardingSessionRepo interface {
	GetByUserAndChannel(ctx context.Context, userID uuid.UUID, channel string) (appinterfaces.AgentSessionRecord, error)
	Upsert(ctx context.Context, record appinterfaces.AgentSessionRecord) error
}

type OnboardingSessionGateway struct {
	repo onboardingSessionRepo
}

func NewOnboardingSessionGateway(repo onboardingSessionRepo) *OnboardingSessionGateway {
	return &OnboardingSessionGateway{repo: repo}
}

func (g *OnboardingSessionGateway) Load(ctx context.Context, userID uuid.UUID, channel string) (onboardingv2draft.Draft, bool, error) {
	rec, err := g.repo.GetByUserAndChannel(ctx, userID, channel)
	if err != nil {
		return onboardingv2draft.Draft{}, false, nil
	}
	if !onboardingv2draft.IsDraftPending(rec.PendingAction) {
		return onboardingv2draft.Draft{}, false, nil
	}
	draft, err := onboardingv2draft.Restore(rec.PendingAction)
	if err != nil {
		return onboardingv2draft.Draft{}, false, fmt.Errorf("binding.onboarding_session_gateway: restore: %w", err)
	}
	return draft, true, nil
}

func (g *OnboardingSessionGateway) Save(ctx context.Context, userID uuid.UUID, channel string, draft onboardingv2draft.Draft) error {
	encoded, err := onboardingv2draft.Encode(draft)
	if err != nil {
		return fmt.Errorf("binding.onboarding_session_gateway: encode: %w", err)
	}
	rec, err := g.repo.GetByUserAndChannel(ctx, userID, channel)
	if err != nil && !errors.Is(err, appinterfaces.ErrAgentSessionNotFound) {
		return fmt.Errorf("binding.onboarding_session_gateway: load for save: %w", err)
	}
	if errors.Is(err, appinterfaces.ErrAgentSessionNotFound) {
		rec = appinterfaces.AgentSessionRecord{
			ID:          uuid.New(),
			UserID:      userID,
			Channel:     channel,
			RecentTurns: []byte("[]"),
		}
	}
	rec.PendingAction = encoded
	rec.UpdatedAt = time.Now().UTC()
	rec.ExpiresAt = time.Now().UTC().Add(24 * time.Hour)
	if err := g.repo.Upsert(ctx, rec); err != nil {
		return fmt.Errorf("binding.onboarding_session_gateway: upsert: %w", err)
	}
	return nil
}

func (g *OnboardingSessionGateway) Clear(ctx context.Context, userID uuid.UUID, channel string) error {
	rec, err := g.repo.GetByUserAndChannel(ctx, userID, channel)
	if err != nil {
		return nil
	}
	rec.PendingAction = []byte("{}")
	rec.UpdatedAt = time.Now().UTC()
	rec.ExpiresAt = time.Now().UTC().Add(24 * time.Hour)
	if err := g.repo.Upsert(ctx, rec); err != nil {
		return fmt.Errorf("binding.onboarding_session_gateway: clear: %w", err)
	}
	return nil
}

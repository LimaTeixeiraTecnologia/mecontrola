package consumers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	agentinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	agententities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

const (
	greetingIntentKind = "onboarding.greeting"
	greetingLLMModel   = "system"
)

type greetingUoW interface {
	Do(ctx context.Context, fn func(ctx context.Context, db database.DBTX) error) error
}

type GreetingDecisionStore struct {
	factory agentinterfaces.AgentDecisionRepositoryFactory
	uow     greetingUoW
}

func NewGreetingDecisionStore(factory agentinterfaces.AgentDecisionRepositoryFactory, u uow.UnitOfWork) *GreetingDecisionStore {
	return &GreetingDecisionStore{factory: factory, uow: u}
}

func (s *GreetingDecisionStore) FindByMessageID(ctx context.Context, userID uuid.UUID, channel, messageID string) (bool, error) {
	var found bool
	err := s.uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		_, ok, findErr := s.factory.AgentDecisionRepository(db).FindByMessage(ctx, userID, channel, messageID, 0)
		if findErr != nil {
			return findErr
		}
		found = ok
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("greeting_decision_store: find by message: %w", err)
	}
	return found, nil
}

func (s *GreetingDecisionStore) RegisterGreeting(ctx context.Context, userID uuid.UUID, channel, messageID string, now time.Time) error {
	decision, err := agententities.NewPendingDecision(agententities.AgentDecisionParams{
		ID:            uuid.New(),
		UserID:        userID,
		Channel:       channel,
		MessageID:     messageID,
		IntentKind:    greetingIntentKind,
		PromptSHA256:  greetingPromptDigest(messageID),
		LLMModel:      greetingLLMModel,
		DecidedAction: greetingIntentKind,
		CreatedAt:     now,
	})
	if err != nil {
		return fmt.Errorf("greeting_decision_store: build decision: %w", err)
	}
	insertErr := s.uow.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		return s.factory.AgentDecisionRepository(db).Insert(ctx, decision)
	})
	if insertErr != nil {
		if errors.Is(insertErr, agentinterfaces.ErrAgentDecisionConflict) {
			return nil
		}
		return fmt.Errorf("greeting_decision_store: insert: %w", insertErr)
	}
	return nil
}

func greetingPromptDigest(messageID string) string {
	sum := sha256.Sum256([]byte("onboarding.greeting:" + messageID))
	return hex.EncodeToString(sum[:])
}

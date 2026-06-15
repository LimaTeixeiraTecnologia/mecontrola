package interfaces

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
)

type DispatchResult struct {
	ReplyText  string
	WasApplied bool
}

type IntentDispatcher interface {
	Dispatch(ctx context.Context, userID uuid.UUID, outcome services.IntentOutcome) (DispatchResult, error)
}

type PromptContextLoader interface {
	Load(ctx context.Context, userID uuid.UUID, channel string) (PromptSeed, error)
}

type PromptSeed struct {
	Permissions []string
	Categories  []CategorySeed
	Cards       []CardSeed
}

type CategorySeed struct {
	ID   string
	Name string
}

type CardSeed struct {
	ID       string
	Nickname string
	Brand    string
	LastFour string
}

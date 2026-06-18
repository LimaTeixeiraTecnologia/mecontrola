package interfaces

import (
	"context"

	"github.com/google/uuid"
)

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
	ID         string
	Name       string
	Nickname   string
	ClosingDay int
	DueDay     int
	LimitCents int64
}

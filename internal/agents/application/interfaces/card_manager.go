package interfaces

import (
	"context"

	"github.com/google/uuid"
)

type CardManager interface {
	CreateCard(ctx context.Context, in NewCard) (CardRef, error)
	ListCards(ctx context.Context, userID uuid.UUID) ([]Card, error)
	SoftDeleteCard(ctx context.Context, cardID, userID uuid.UUID) error
	HasOpenInstallments(ctx context.Context, cardID, userID uuid.UUID) (bool, error)
}

package interfaces

import (
	"context"

	"github.com/google/uuid"
)

type CardManager interface {
	CreateCard(ctx context.Context, in NewCard) (CardRef, error)
	ListCards(ctx context.Context, userID uuid.UUID) ([]Card, error)
	GetCard(ctx context.Context, cardID, userID uuid.UUID) (Card, error)
	ResolveCardByNickname(ctx context.Context, userID uuid.UUID, nickname string) (Card, error)
	CountCards(ctx context.Context, userID uuid.UUID) (int64, error)
	BestPurchaseDay(ctx context.Context, bank string, dueDay int) (BestPurchaseDay, error)
	BankRecognized(ctx context.Context, bank string) (bool, error)
	UpdateCard(ctx context.Context, in CardUpdate) (Card, error)
	SoftDeleteCard(ctx context.Context, cardID, userID uuid.UUID) error
	HasOpenInstallments(ctx context.Context, cardID, userID uuid.UUID) (bool, error)
}

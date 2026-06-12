package interfaces

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrCardNotFound = errors.New("transactions: cartão não encontrado")

type CardLookup interface {
	GetForUser(ctx context.Context, cardID, userID uuid.UUID) (valueobjects.CardBillingSnapshot, error)
}

package input

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

const purchaseDateLayout = "2006-01-02"

type InvoiceFor struct {
	CardID   uuid.UUID
	UserID   uuid.UUID
	Purchase time.Time
}

func NewInvoiceFor(cardID, userID uuid.UUID, forRaw string) (InvoiceFor, error) {
	purchase, err := time.Parse(purchaseDateLayout, forRaw)
	if err != nil {
		return InvoiceFor{}, fmt.Errorf("card/input.NewInvoiceFor: %w", domain.ErrInvalidPurchaseDate)
	}
	return InvoiceFor{CardID: cardID, UserID: userID, Purchase: purchase}, nil
}

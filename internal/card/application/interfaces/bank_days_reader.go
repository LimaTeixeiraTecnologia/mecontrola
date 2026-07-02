package interfaces

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type BankDaysReader interface {
	DaysBeforeDue(ctx context.Context, bank valueobjects.BankCode) (int, error)
}

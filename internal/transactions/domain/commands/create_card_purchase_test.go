package commands_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func validCreateCardPurchaseRaw() commands.RawCreateCardPurchase {
	return commands.RawCreateCardPurchase{
		CardID:            uuid.New().String(),
		TotalAmountCents:  5000,
		InstallmentsTotal: 3,
		Description:       "Notebook",
		CategoryID:        uuid.New().String(),
		PurchasedAt:       time.Now(),
	}
}

func TestNewCreateCardPurchase_Valid(t *testing.T) {
	raw := validCreateCardPurchaseRaw()
	userID := uuid.New()

	cmd, err := commands.NewCreateCardPurchase(raw, userID)
	require.NoError(t, err)
	assert.Equal(t, 3, cmd.Installments.Value())
	assert.Equal(t, int64(5000), cmd.TotalAmount.Cents())
}

func TestNewCreateCardPurchase_AccumulatesMultipleErrors(t *testing.T) {
	raw := commands.RawCreateCardPurchase{
		CardID:            "bad-uuid",
		TotalAmountCents:  0,
		InstallmentsTotal: 25,
		Description:       "",
		CategoryID:        "bad-uuid",
		PurchasedAt:       time.Time{},
	}
	userID := uuid.New()

	_, err := commands.NewCreateCardPurchase(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrInvalidCardID))
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
	assert.True(t, errors.Is(err, valueobjects.ErrInstallmentCountOutOfRange))
	assert.True(t, errors.Is(err, valueobjects.ErrDescriptionEmpty))
	assert.True(t, errors.Is(err, valueobjects.ErrInvalidCategoryID))
}

func TestNewCreateCardPurchase_SingleInstallment(t *testing.T) {
	raw := validCreateCardPurchaseRaw()
	raw.InstallmentsTotal = 1
	userID := uuid.New()

	cmd, err := commands.NewCreateCardPurchase(raw, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, cmd.Installments.Value())
}

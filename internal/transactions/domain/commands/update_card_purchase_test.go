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

func validUpdateCardPurchaseRaw() commands.RawUpdateCardPurchase {
	return commands.RawUpdateCardPurchase{
		PurchaseID:        uuid.New().String(),
		TotalAmountCents:  8000,
		InstallmentsTotal: 4,
		Description:       "TV",
		CategoryID:        uuid.New().String(),
		PurchasedAt:       time.Now(),
		Version:           1,
	}
}

func TestNewUpdateCardPurchase_Valid(t *testing.T) {
	raw := validUpdateCardPurchaseRaw()
	userID := uuid.New()

	cmd, err := commands.NewUpdateCardPurchase(raw, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, cmd.UserID.UUID())
	assert.Equal(t, int64(8000), cmd.TotalAmount.Cents())
	assert.Equal(t, 4, cmd.Installments.Value())
}

func TestNewUpdateCardPurchase_InvalidAmountCents(t *testing.T) {
	raw := validUpdateCardPurchaseRaw()
	raw.TotalAmountCents = 0
	userID := uuid.New()

	_, err := commands.NewUpdateCardPurchase(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
}

func TestNewUpdateCardPurchase_NegativeAmountCents(t *testing.T) {
	raw := validUpdateCardPurchaseRaw()
	raw.TotalAmountCents = -500
	userID := uuid.New()

	_, err := commands.NewUpdateCardPurchase(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
}

func TestNewUpdateCardPurchase_InvalidPurchaseID(t *testing.T) {
	raw := validUpdateCardPurchaseRaw()
	raw.PurchaseID = "not-a-uuid"
	userID := uuid.New()

	_, err := commands.NewUpdateCardPurchase(raw, userID)
	require.Error(t, err)
}

func TestNewUpdateCardPurchase_MissingPurchasedAt(t *testing.T) {
	raw := validUpdateCardPurchaseRaw()
	raw.PurchasedAt = time.Time{}
	userID := uuid.New()

	_, err := commands.NewUpdateCardPurchase(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, commands.ErrCommandMissingOccurredAt))
}

func TestNewUpdateCardPurchase_AccumulatesMultipleErrors(t *testing.T) {
	raw := commands.RawUpdateCardPurchase{
		PurchaseID:        "bad-uuid",
		TotalAmountCents:  0,
		InstallmentsTotal: 25,
		Description:       "",
		CategoryID:        "bad-uuid",
		PurchasedAt:       time.Time{},
		Version:           0,
	}
	userID := uuid.New()

	_, err := commands.NewUpdateCardPurchase(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
	assert.True(t, errors.Is(err, valueobjects.ErrInstallmentCountOutOfRange))
	assert.True(t, errors.Is(err, valueobjects.ErrDescriptionEmpty))
	assert.True(t, errors.Is(err, commands.ErrCommandMissingOccurredAt))
}

func TestNewUpdateCardPurchase_WithSubcategory(t *testing.T) {
	raw := validUpdateCardPurchaseRaw()
	raw.SubcategoryID = uuid.New().String()
	userID := uuid.New()

	cmd, err := commands.NewUpdateCardPurchase(raw, userID)
	require.NoError(t, err)
	assert.True(t, cmd.SubcategoryID.IsPresent())
}

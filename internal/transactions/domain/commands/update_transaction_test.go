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

func validUpdateTransactionRaw() commands.RawUpdateTransaction {
	return commands.RawUpdateTransaction{
		TransactionID: uuid.New().String(),
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   3000,
		Description:   "Mercado",
		CategoryID:    uuid.New().String(),
		OccurredAt:    time.Now(),
		Version:       1,
	}
}

func TestNewUpdateTransaction_Valid(t *testing.T) {
	raw := validUpdateTransactionRaw()
	userID := uuid.New()

	cmd, err := commands.NewUpdateTransaction(raw, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, cmd.UserID.UUID())
	assert.Equal(t, valueobjects.DirectionOutcome, cmd.Direction)
	assert.Equal(t, int64(3000), cmd.Amount.Cents())
}

func TestNewUpdateTransaction_InvalidAmountCents(t *testing.T) {
	raw := validUpdateTransactionRaw()
	raw.AmountCents = 0
	userID := uuid.New()

	_, err := commands.NewUpdateTransaction(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
}

func TestNewUpdateTransaction_NegativeAmountCents(t *testing.T) {
	raw := validUpdateTransactionRaw()
	raw.AmountCents = -100
	userID := uuid.New()

	_, err := commands.NewUpdateTransaction(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
}

func TestNewUpdateTransaction_InvalidTransactionID(t *testing.T) {
	raw := validUpdateTransactionRaw()
	raw.TransactionID = "not-a-uuid"
	userID := uuid.New()

	_, err := commands.NewUpdateTransaction(raw, userID)
	require.Error(t, err)
}

func TestNewUpdateTransaction_MissingOccurredAt(t *testing.T) {
	raw := validUpdateTransactionRaw()
	raw.OccurredAt = time.Time{}
	userID := uuid.New()

	_, err := commands.NewUpdateTransaction(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, commands.ErrCommandMissingOccurredAt))
}

func TestNewUpdateTransaction_AccumulatesMultipleErrors(t *testing.T) {
	raw := commands.RawUpdateTransaction{
		TransactionID: "bad-uuid",
		Direction:     "invalid",
		PaymentMethod: "doc",
		AmountCents:   -1,
		Description:   "",
		CategoryID:    "bad-uuid",
		OccurredAt:    time.Time{},
		Version:       0,
	}
	userID := uuid.New()

	_, err := commands.NewUpdateTransaction(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrDirectionUnknown))
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
	assert.True(t, errors.Is(err, valueobjects.ErrDescriptionEmpty))
	assert.True(t, errors.Is(err, commands.ErrCommandMissingOccurredAt))
}

func TestNewUpdateTransaction_WithSubcategory(t *testing.T) {
	raw := validUpdateTransactionRaw()
	raw.SubcategoryID = uuid.New().String()
	userID := uuid.New()

	cmd, err := commands.NewUpdateTransaction(raw, userID)
	require.NoError(t, err)
	assert.True(t, cmd.SubcategoryID.IsPresent())
}

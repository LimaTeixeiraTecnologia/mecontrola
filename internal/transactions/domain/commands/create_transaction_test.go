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

func validCreateTransactionRaw() commands.RawCreateTransaction {
	return commands.RawCreateTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Salário",
		CategoryID:    uuid.New().String(),
		OccurredAt:    time.Now(),
	}
}

func TestNewCreateTransaction_Valid(t *testing.T) {
	raw := validCreateTransactionRaw()
	userID := uuid.New()

	cmd, err := commands.NewCreateTransaction(raw, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, cmd.UserID.UUID())
	assert.Equal(t, valueobjects.DirectionIncome, cmd.Direction)
}

func TestNewCreateTransaction_MultipleErrors(t *testing.T) {
	raw := commands.RawCreateTransaction{
		Direction:     "invalid",
		PaymentMethod: "doc",
		AmountCents:   -1,
		Description:   "",
		CategoryID:    "not-a-uuid",
		OccurredAt:    time.Time{},
	}
	userID := uuid.New()

	_, err := commands.NewCreateTransaction(raw, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrDirectionUnknown))
	assert.True(t, errors.Is(err, valueobjects.ErrMoneyMustBePositive))
	assert.True(t, errors.Is(err, valueobjects.ErrDescriptionEmpty))
}

func TestNewCreateTransaction_DocAccepted(t *testing.T) {
	raw := validCreateTransactionRaw()
	raw.PaymentMethod = "doc"
	userID := uuid.New()

	cmd, err := commands.NewCreateTransaction(raw, userID)
	require.NoError(t, err)
	assert.Equal(t, valueobjects.PaymentMethodDoc, cmd.PaymentMethod)
}

func TestNewCreateTransaction_WithSubcategory(t *testing.T) {
	raw := validCreateTransactionRaw()
	raw.SubcategoryID = uuid.New().String()
	userID := uuid.New()

	cmd, err := commands.NewCreateTransaction(raw, userID)
	require.NoError(t, err)
	assert.True(t, cmd.SubcategoryID.IsPresent())
}

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

func validCreditCardRaw() commands.RawCreateTransaction {
	return commands.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "credit_card",
		AmountCents:   12000,
		Description:   "Notebook",
		CategoryID:    uuid.New().String(),
		SubcategoryID: uuid.New().String(),
		CardID:        uuid.New().String(),
		Installments:  12,
		OccurredAt:    time.Now(),
	}
}

func TestNewCreateTransaction_CreditCardValid(t *testing.T) {
	raw := validCreditCardRaw()
	cmd, err := commands.NewCreateTransaction(raw, uuid.New())
	require.NoError(t, err)
	assert.True(t, cmd.CardID.IsPresent())
	assert.True(t, cmd.Installments.IsPresent())
	ic, ok := cmd.Installments.Get()
	require.True(t, ok)
	assert.Equal(t, 12, ic.Value())
	assert.Equal(t, valueobjects.PaymentMethodCreditCard, cmd.PaymentMethod)
}

func TestNewCreateTransaction_CreditCardMissingCardID(t *testing.T) {
	raw := validCreditCardRaw()
	raw.CardID = ""
	_, err := commands.NewCreateTransaction(raw, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, commands.ErrCommandCreditCardRequiresCardID))
}

func TestNewCreateTransaction_CreditCardRequiresOutcome(t *testing.T) {
	raw := validCreditCardRaw()
	raw.Direction = "income"
	_, err := commands.NewCreateTransaction(raw, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, commands.ErrCommandCreditCardRequiresOutcome))
}

func TestNewCreateTransaction_InstallmentsOutOfRange(t *testing.T) {
	raw := validCreditCardRaw()
	raw.Installments = 25
	_, err := commands.NewCreateTransaction(raw, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrInstallmentCountOutOfRange))
}

func TestNewCreateTransaction_InstallmentsAbsentIsNone(t *testing.T) {
	raw := validCreditCardRaw()
	raw.Installments = 0
	cmd, err := commands.NewCreateTransaction(raw, uuid.New())
	require.NoError(t, err)
	assert.False(t, cmd.Installments.IsPresent())
	assert.True(t, cmd.CardID.IsPresent())
}

func TestNewCreateTransaction_VoucherSimple(t *testing.T) {
	for _, pm := range []string{"vale_refeicao", "vale_alimentacao"} {
		raw := validCreateTransactionRaw()
		raw.Direction = "outcome"
		raw.PaymentMethod = pm
		raw.SubcategoryID = uuid.New().String()
		cmd, err := commands.NewCreateTransaction(raw, uuid.New())
		require.NoError(t, err, pm)
		assert.False(t, cmd.CardID.IsPresent())
		assert.False(t, cmd.Installments.IsPresent())
		assert.Equal(t, pm, cmd.PaymentMethod.String())
	}
}

func TestNewCreateTransaction_InvalidCardID(t *testing.T) {
	raw := validCreditCardRaw()
	raw.CardID = "not-a-uuid"
	_, err := commands.NewCreateTransaction(raw, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrInvalidCardID))
}

package commands_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func validUpdateCreditCardRaw() commands.RawUpdateTransaction {
	raw := validUpdateTransactionRaw()
	raw.PaymentMethod = "credit_card"
	raw.SubcategoryID = uuid.New().String()
	raw.CardID = uuid.New().String()
	raw.Installments = 6
	return raw
}

func TestNewUpdateTransaction_CreditCardValid(t *testing.T) {
	cmd, err := commands.NewUpdateTransaction(validUpdateCreditCardRaw(), uuid.New())
	require.NoError(t, err)
	assert.True(t, cmd.CardID.IsPresent())
	ic, ok := cmd.Installments.Get()
	require.True(t, ok)
	assert.Equal(t, 6, ic.Value())
}

func TestNewUpdateTransaction_CreditCardMissingCardID(t *testing.T) {
	raw := validUpdateCreditCardRaw()
	raw.CardID = ""
	_, err := commands.NewUpdateTransaction(raw, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, commands.ErrCommandCreditCardRequiresCardID))
}

func TestNewUpdateTransaction_CreditCardRequiresOutcome(t *testing.T) {
	raw := validUpdateCreditCardRaw()
	raw.Direction = "income"
	_, err := commands.NewUpdateTransaction(raw, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, commands.ErrCommandCreditCardRequiresOutcome))
}

func TestNewUpdateTransaction_InstallmentsOutOfRange(t *testing.T) {
	raw := validUpdateCreditCardRaw()
	raw.Installments = 25
	_, err := commands.NewUpdateTransaction(raw, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrInstallmentCountOutOfRange))
}

func TestNewUpdateTransaction_VoucherSimple(t *testing.T) {
	raw := validUpdateTransactionRaw()
	raw.PaymentMethod = "vale_refeicao"
	raw.SubcategoryID = uuid.New().String()
	cmd, err := commands.NewUpdateTransaction(raw, uuid.New())
	require.NoError(t, err)
	assert.False(t, cmd.CardID.IsPresent())
	assert.Equal(t, "vale_refeicao", cmd.PaymentMethod.String())
}

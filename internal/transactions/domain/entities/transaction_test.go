package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func newTestTransaction(t *testing.T) *entities.Transaction {
	t.Helper()
	now := time.Now()
	amount, _ := valueobjects.NewMoney(1000)
	desc, _ := valueobjects.NewDescription("Salário")
	catID, _ := valueobjects.ParseCategoryID(uuid.New().String())
	refMonth, _ := valueobjects.NewRefMonth("2026-06")
	tx := entities.NewTransaction(
		uuid.New(),
		valueobjects.UserIDFromUUID(uuid.New()),
		valueobjects.DirectionIncome,
		valueobjects.PaymentMethodPix,
		amount,
		desc,
		catID,
		option.None[valueobjects.SubcategoryID](),
		"Alimentação",
		"",
		refMonth,
		now,
		now,
	)
	return &tx
}

func TestNewTransaction_InitialVersion(t *testing.T) {
	tx := newTestTransaction(t)
	assert.Equal(t, int64(1), tx.Version())
	assert.Nil(t, tx.DeletedAt())
}

func TestTransaction_Update_IncrementsVersion(t *testing.T) {
	tx := newTestTransaction(t)
	now := time.Now()
	amount, _ := valueobjects.NewMoney(2000)
	desc, _ := valueobjects.NewDescription("Bônus")
	catID, _ := valueobjects.ParseCategoryID(uuid.New().String())
	refMonth, _ := valueobjects.NewRefMonth("2026-06")

	tx.Update(
		valueobjects.DirectionIncome,
		valueobjects.PaymentMethodPix,
		amount,
		desc,
		catID,
		option.None[valueobjects.SubcategoryID](),
		"Alimentação",
		"",
		refMonth,
		now,
		now,
	)

	assert.Equal(t, int64(2), tx.Version())
}

func TestTransaction_SoftDelete(t *testing.T) {
	tx := newTestTransaction(t)
	now := time.Now()

	err := tx.SoftDelete(now)
	require.NoError(t, err)
	assert.NotNil(t, tx.DeletedAt())
	assert.Equal(t, int64(2), tx.Version())
}

func TestTransaction_SoftDelete_AlreadyDeleted(t *testing.T) {
	tx := newTestTransaction(t)
	now := time.Now()

	err := tx.SoftDelete(now)
	require.NoError(t, err)

	err = tx.SoftDelete(now)
	require.Error(t, err)
	assert.ErrorIs(t, err, entities.ErrTransactionAlreadyDeleted)
}

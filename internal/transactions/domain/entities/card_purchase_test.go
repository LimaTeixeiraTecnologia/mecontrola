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

func newTestCardPurchase(t *testing.T) *entities.CardPurchase {
	t.Helper()
	now := time.Now()
	amount, _ := valueobjects.NewMoney(9000)
	inst, _ := valueobjects.NewInstallmentCount(3)
	desc, _ := valueobjects.NewDescription("Notebook")
	catID, _ := valueobjects.ParseCategoryID(uuid.New().String())
	snap, _ := valueobjects.NewCardBillingSnapshot(10, 17)

	p := entities.NewCardPurchase(
		uuid.New(),
		valueobjects.UserIDFromUUID(uuid.New()),
		valueobjects.CardIDFromUUID(uuid.New()),
		amount,
		inst,
		desc,
		catID,
		option.None[valueobjects.SubcategoryID](),
		"Eletrônicos",
		"",
		now,
		snap,
		now,
	)
	return &p
}

func TestNewCardPurchase_InitialVersion(t *testing.T) {
	p := newTestCardPurchase(t)
	assert.Equal(t, int64(1), p.Version())
	assert.Nil(t, p.DeletedAt())
}

func TestCardPurchase_SoftDelete(t *testing.T) {
	p := newTestCardPurchase(t)
	now := time.Now()
	err := p.SoftDelete(now)
	require.NoError(t, err)
	assert.NotNil(t, p.DeletedAt())
}

func TestCardPurchase_SoftDelete_AlreadyDeleted(t *testing.T) {
	p := newTestCardPurchase(t)
	now := time.Now()
	require.NoError(t, p.SoftDelete(now))
	err := p.SoftDelete(now)
	require.Error(t, err)
	assert.ErrorIs(t, err, entities.ErrCardPurchaseAlreadyDeleted)
}

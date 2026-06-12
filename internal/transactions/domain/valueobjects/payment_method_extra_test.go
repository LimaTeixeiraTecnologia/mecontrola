package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestPaymentMethodString_Zero(t *testing.T) {
	var pm valueobjects.PaymentMethod
	assert.Equal(t, "", pm.String())
}

func TestParsePaymentMethodForCreate_Valid(t *testing.T) {
	pm, err := valueobjects.ParsePaymentMethodForCreate("pix")
	require.NoError(t, err)
	assert.Equal(t, valueobjects.PaymentMethodPix, pm)
}

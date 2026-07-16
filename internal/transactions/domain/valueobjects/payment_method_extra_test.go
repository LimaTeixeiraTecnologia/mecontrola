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

func TestParsePaymentMethod_NewWallets(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  valueobjects.PaymentMethod
	}{
		{name: "transferencia", input: "transferencia", want: valueobjects.PaymentMethodTransferencia},
		{name: "apple_pay", input: "apple_pay", want: valueobjects.PaymentMethodApplePay},
		{name: "google_pay", input: "google_pay", want: valueobjects.PaymentMethodGooglePay},
		{name: "picpay", input: "picpay", want: valueobjects.PaymentMethodPicPay},
		{name: "mercado_pago", input: "mercado_pago", want: valueobjects.PaymentMethodMercadoPago},
		{name: "cheque", input: "cheque", want: valueobjects.PaymentMethodCheque},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pm, err := valueobjects.ParsePaymentMethod(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, pm)
			assert.Equal(t, tc.input, pm.String())
			assert.False(t, pm.IsCreditCard())

			created, createErr := valueobjects.ParsePaymentMethodForCreate(tc.input)
			require.NoError(t, createErr)
			assert.Equal(t, tc.want, created)
		})
	}
}

func TestPaymentMethodNewWalletsIota(t *testing.T) {
	assert.Equal(t, valueobjects.PaymentMethod(11), valueobjects.PaymentMethodTransferencia)
	assert.Equal(t, valueobjects.PaymentMethod(12), valueobjects.PaymentMethodApplePay)
	assert.Equal(t, valueobjects.PaymentMethod(13), valueobjects.PaymentMethodGooglePay)
	assert.Equal(t, valueobjects.PaymentMethod(14), valueobjects.PaymentMethodPicPay)
	assert.Equal(t, valueobjects.PaymentMethod(15), valueobjects.PaymentMethodMercadoPago)
	assert.Equal(t, valueobjects.PaymentMethod(16), valueobjects.PaymentMethodCheque)
}

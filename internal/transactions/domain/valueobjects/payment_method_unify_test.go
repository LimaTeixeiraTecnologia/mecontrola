package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestParsePaymentMethod_Vouchers(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  valueobjects.PaymentMethod
	}{
		{name: "vale_refeicao", input: "vale_refeicao", want: valueobjects.PaymentMethodMealVoucher},
		{name: "vale_alimentacao", input: "vale_alimentacao", want: valueobjects.PaymentMethodFoodVoucher},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pm, err := valueobjects.ParsePaymentMethod(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, pm)
			assert.Equal(t, tc.input, pm.String())
		})
	}
}

func TestParsePaymentMethodForCreate_Vouchers(t *testing.T) {
	for _, s := range []string{"vale_refeicao", "vale_alimentacao"} {
		pm, err := valueobjects.ParsePaymentMethodForCreate(s)
		require.NoError(t, err)
		assert.Equal(t, s, pm.String())
	}
}

func TestPaymentMethodVoucherIota(t *testing.T) {
	assert.Equal(t, valueobjects.PaymentMethod(9), valueobjects.PaymentMethodMealVoucher)
	assert.Equal(t, valueobjects.PaymentMethod(10), valueobjects.PaymentMethodFoodVoucher)
}

func TestPaymentMethodFromInt_Bounds(t *testing.T) {
	cases := []struct {
		name    string
		v       int
		wantErr bool
		want    valueobjects.PaymentMethod
	}{
		{name: "min", v: 1, want: valueobjects.PaymentMethodPix},
		{name: "doc readable", v: 8, want: valueobjects.PaymentMethodDoc},
		{name: "meal voucher", v: 9, want: valueobjects.PaymentMethodMealVoucher},
		{name: "food voucher", v: 10, want: valueobjects.PaymentMethodFoodVoucher},
		{name: "transferencia", v: 11, want: valueobjects.PaymentMethodTransferencia},
		{name: "apple_pay", v: 12, want: valueobjects.PaymentMethodApplePay},
		{name: "google_pay", v: 13, want: valueobjects.PaymentMethodGooglePay},
		{name: "picpay", v: 14, want: valueobjects.PaymentMethodPicPay},
		{name: "mercado_pago", v: 15, want: valueobjects.PaymentMethodMercadoPago},
		{name: "cheque max", v: 16, want: valueobjects.PaymentMethodCheque},
		{name: "zero", v: 0, wantErr: true},
		{name: "seventeen", v: 17, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pm, err := valueobjects.PaymentMethodFromInt(tc.v)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, valueobjects.ErrPaymentMethodUnknown))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, pm)
		})
	}
}

func TestPaymentMethodRoundtrip_AllSixteen(t *testing.T) {
	for v := 1; v <= 16; v++ {
		pm, err := valueobjects.PaymentMethodFromInt(v)
		require.NoError(t, err)
		s := pm.String()
		require.NotEmpty(t, s)
		parsed, perr := valueobjects.ParsePaymentMethod(s)
		require.NoError(t, perr)
		assert.Equal(t, pm, parsed)
	}
}

func TestParsePaymentMethodForCreate_DocAcceptedEverywhere(t *testing.T) {
	pm, err := valueobjects.ParsePaymentMethod("doc")
	require.NoError(t, err)
	assert.Equal(t, valueobjects.PaymentMethodDoc, pm)

	fromInt, ferr := valueobjects.PaymentMethodFromInt(8)
	require.NoError(t, ferr)
	assert.Equal(t, valueobjects.PaymentMethodDoc, fromInt)

	created, cerr := valueobjects.ParsePaymentMethodForCreate("doc")
	require.NoError(t, cerr)
	assert.Equal(t, valueobjects.PaymentMethodDoc, created)
}

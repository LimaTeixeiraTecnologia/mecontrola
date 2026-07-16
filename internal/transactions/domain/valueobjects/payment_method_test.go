package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestParsePaymentMethod(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    valueobjects.PaymentMethod
		wantErr error
	}{
		{name: "pix", input: "pix", want: valueobjects.PaymentMethodPix},
		{name: "ted", input: "ted", want: valueobjects.PaymentMethodTED},
		{name: "debit_in_account", input: "debit_in_account", want: valueobjects.PaymentMethodDebitInAccount},
		{name: "debit_card", input: "debit_card", want: valueobjects.PaymentMethodDebitCard},
		{name: "cash", input: "cash", want: valueobjects.PaymentMethodCash},
		{name: "boleto", input: "boleto", want: valueobjects.PaymentMethodBoleto},
		{name: "credit_card", input: "credit_card", want: valueobjects.PaymentMethodCreditCard},
		{name: "doc", input: "doc", want: valueobjects.PaymentMethodDoc},
		{name: "unknown", input: "wire", wantErr: valueobjects.ErrPaymentMethodUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pm, err := valueobjects.ParsePaymentMethod(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, pm)
			assert.Equal(t, tc.input, pm.String())
		})
	}
}

func TestParsePaymentMethodForCreate_AcceptsDoc(t *testing.T) {
	pm, err := valueobjects.ParsePaymentMethodForCreate("doc")
	require.NoError(t, err)
	assert.Equal(t, valueobjects.PaymentMethodDoc, pm)
}

func TestPaymentMethodIota(t *testing.T) {
	assert.Equal(t, valueobjects.PaymentMethod(1), valueobjects.PaymentMethodPix)
	assert.Equal(t, valueobjects.PaymentMethod(8), valueobjects.PaymentMethodDoc)
}

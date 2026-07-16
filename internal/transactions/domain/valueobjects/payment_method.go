package valueobjects

import (
	"errors"
	"fmt"
)

var ErrPaymentMethodUnknown = errors.New("transactions: payment method desconhecido")

type PaymentMethod uint8

const (
	PaymentMethodPix PaymentMethod = iota + 1
	PaymentMethodTED
	PaymentMethodDebitInAccount
	PaymentMethodDebitCard
	PaymentMethodCash
	PaymentMethodBoleto
	PaymentMethodCreditCard
	PaymentMethodDoc
	PaymentMethodMealVoucher
	PaymentMethodFoodVoucher
	PaymentMethodTransferencia
	PaymentMethodApplePay
	PaymentMethodGooglePay
	PaymentMethodPicPay
	PaymentMethodMercadoPago
	PaymentMethodCheque
)

var paymentMethodByString = map[string]PaymentMethod{
	"pix":              PaymentMethodPix,
	"ted":              PaymentMethodTED,
	"debit_in_account": PaymentMethodDebitInAccount,
	"debit_card":       PaymentMethodDebitCard,
	"cash":             PaymentMethodCash,
	"boleto":           PaymentMethodBoleto,
	"credit_card":      PaymentMethodCreditCard,
	"doc":              PaymentMethodDoc,
	"vale_refeicao":    PaymentMethodMealVoucher,
	"vale_alimentacao": PaymentMethodFoodVoucher,
	"transferencia":    PaymentMethodTransferencia,
	"apple_pay":        PaymentMethodApplePay,
	"google_pay":       PaymentMethodGooglePay,
	"picpay":           PaymentMethodPicPay,
	"mercado_pago":     PaymentMethodMercadoPago,
	"cheque":           PaymentMethodCheque,
}

var paymentMethodToString = map[PaymentMethod]string{
	PaymentMethodPix:            "pix",
	PaymentMethodTED:            "ted",
	PaymentMethodDebitInAccount: "debit_in_account",
	PaymentMethodDebitCard:      "debit_card",
	PaymentMethodCash:           "cash",
	PaymentMethodBoleto:         "boleto",
	PaymentMethodCreditCard:     "credit_card",
	PaymentMethodDoc:            "doc",
	PaymentMethodMealVoucher:    "vale_refeicao",
	PaymentMethodFoodVoucher:    "vale_alimentacao",
	PaymentMethodTransferencia:  "transferencia",
	PaymentMethodApplePay:       "apple_pay",
	PaymentMethodGooglePay:      "google_pay",
	PaymentMethodPicPay:         "picpay",
	PaymentMethodMercadoPago:    "mercado_pago",
	PaymentMethodCheque:         "cheque",
}

func ParsePaymentMethod(s string) (PaymentMethod, error) {
	if p, ok := paymentMethodByString[s]; ok {
		return p, nil
	}
	return 0, fmt.Errorf("transactions: %q: %w", s, ErrPaymentMethodUnknown)
}

func ParsePaymentMethodForCreate(s string) (PaymentMethod, error) {
	return ParsePaymentMethod(s)
}

func PaymentMethodFromInt(v int) (PaymentMethod, error) {
	if v < 1 || v > 16 {
		return 0, fmt.Errorf("transactions: payment method int %d: %w", v, ErrPaymentMethodUnknown)
	}
	return PaymentMethod(v), nil
}

func (p PaymentMethod) IsCreditCard() bool {
	return p == PaymentMethodCreditCard
}

func (p PaymentMethod) String() string {
	return paymentMethodToString[p]
}

package valueobjects

import (
	"errors"
	"fmt"
)

var ErrPaymentMethodUnknown = errors.New("transactions: payment method desconhecido")
var ErrPaymentMethodDocReadOnly = errors.New("transactions: payment method doc é somente leitura (registros legados)")

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
)

func ParsePaymentMethod(s string) (PaymentMethod, error) {
	switch s {
	case "pix":
		return PaymentMethodPix, nil
	case "ted":
		return PaymentMethodTED, nil
	case "debit_in_account":
		return PaymentMethodDebitInAccount, nil
	case "debit_card":
		return PaymentMethodDebitCard, nil
	case "cash":
		return PaymentMethodCash, nil
	case "boleto":
		return PaymentMethodBoleto, nil
	case "credit_card":
		return PaymentMethodCreditCard, nil
	case "doc":
		return PaymentMethodDoc, nil
	default:
		return 0, fmt.Errorf("transactions: %q: %w", s, ErrPaymentMethodUnknown)
	}
}

func ParsePaymentMethodForCreate(s string) (PaymentMethod, error) {
	if s == "doc" {
		return 0, fmt.Errorf("transactions: %q: %w", s, ErrPaymentMethodDocReadOnly)
	}
	return ParsePaymentMethod(s)
}

func PaymentMethodFromInt(v int) (PaymentMethod, error) {
	if v < 1 || v > 8 {
		return 0, fmt.Errorf("transactions: payment method int %d: %w", v, ErrPaymentMethodUnknown)
	}
	return PaymentMethod(v), nil
}

func (p PaymentMethod) String() string {
	switch p {
	case PaymentMethodPix:
		return "pix"
	case PaymentMethodTED:
		return "ted"
	case PaymentMethodDebitInAccount:
		return "debit_in_account"
	case PaymentMethodDebitCard:
		return "debit_card"
	case PaymentMethodCash:
		return "cash"
	case PaymentMethodBoleto:
		return "boleto"
	case PaymentMethodCreditCard:
		return "credit_card"
	case PaymentMethodDoc:
		return "doc"
	default:
		return ""
	}
}

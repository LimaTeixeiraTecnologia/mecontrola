package usecases

import (
	"errors"
	"fmt"
	"time"
)

var ErrUsecaseUnauthorized = errors.New("transactions: não autorizado")
var ErrTransactionNotFound = errors.New("transactions: lançamento não encontrado")
var ErrTransactionVersionConflict = errors.New("transactions: conflito de versão")
var ErrCardPurchaseNotFound = errors.New("transactions: compra de cartão não encontrada")
var ErrCardInvoiceNotFound = errors.New("transactions: fatura de cartão não encontrada")
var ErrCardPurchaseConflict = errors.New("transactions: conflito de versão na compra de cartão")

func parseISO8601(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("data obrigatória")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("formato de data inválido: %s", s)
}

package usecases

import (
	"errors"
	"fmt"
	"time"
)

var ErrUsecaseUnauthorized = errors.New("transactions: não autorizado")
var ErrTransactionNotFound = errors.New("transactions: lançamento não encontrado")
var ErrTransactionVersionConflict = errors.New("transactions: conflito de versão")
var ErrCardInvoiceNotFound = errors.New("transactions: fatura de cartão não encontrada")
var ErrPaymentMethodMigrationNotAllowed = errors.New("transactions: forma de pagamento não pode migrar de/para cartão de crédito")
var ErrOutcomeTransactionRequiresSubcategory = errors.New("transactions: outcome exige subcategory_id")
var ErrCategoryKindDirectionMismatch = errors.New("transactions: kind da categoria diverge da direction")

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

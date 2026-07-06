package handlers

import (
	"errors"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/client"
	repopkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

func mapError(w http.ResponseWriter, span observability.Span, err error, _ ...error) {
	switch {
	case errors.Is(err, usecases.ErrUsecaseUnauthorized):
		span.SetAttributes(observability.String("outcome", "unauthorized"))
		responses.ErrorWithDetails(w, http.StatusUnauthorized, "não autenticado", map[string]string{"code": "unauthorized"})
	case errors.Is(err, interfaces.ErrTransactionNotFound),
		errors.Is(err, usecases.ErrTransactionNotFound),
		errors.Is(err, usecases.ErrCardInvoiceNotFound),
		errors.Is(err, repopkg.ErrRecurringTemplateNotFound),
		errors.Is(err, interfaces.ErrCardNotFound):
		span.SetAttributes(observability.String("outcome", "not_found"))
		responses.ErrorWithDetails(w, http.StatusNotFound, "recurso não encontrado", map[string]string{"code": "not_found"})
	case errors.Is(err, interfaces.ErrCategoryNotFound):
		span.SetAttributes(observability.String("outcome", "category_not_found"))
		responses.ErrorWithDetails(w, http.StatusNotFound, "categoria não encontrada", map[string]string{"code": "category_not_found"})
	case errors.Is(err, usecases.ErrTransactionRequiresSubcategory):
		span.SetAttributes(observability.String("outcome", "validation_error"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, err.Error(), map[string]string{"code": "validation_error"})
	case errors.Is(err, usecases.ErrPaymentMethodMigrationNotAllowed):
		span.SetAttributes(observability.String("outcome", "unprocessable"))
		responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, err.Error(), map[string]string{"code": "payment_migration_forbidden"})
	case errors.Is(err, interfaces.ErrTransactionVersionConflict),
		errors.Is(err, usecases.ErrTransactionVersionConflict),
		errors.Is(err, repopkg.ErrRecurringTemplateVersionConflict):
		span.SetAttributes(observability.String("outcome", "conflict"))
		responses.ErrorWithDetails(w, http.StatusConflict, "conflito de versão", map[string]string{"code": "version_conflict"})
	case errors.Is(err, client.ErrCardLookupFailed):
		span.SetAttributes(observability.String("outcome", "card_lookup_failed"))
		responses.ErrorWithDetails(w, http.StatusBadGateway, "falha ao consultar cartão", map[string]string{"code": "card_lookup_failed"})
	default:
		if isValidationErr(err) {
			span.SetAttributes(observability.String("outcome", "validation_error"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, err.Error(), map[string]string{"code": "validation_error"})
			return
		}
		span.SetAttributes(observability.String("outcome", "internal_error"))
		responses.Error(w, http.StatusInternalServerError, "erro interno")
	}
}

func isValidationErr(err error) bool {
	msg := err.Error()
	for _, prefix := range []string{"commands/", "validation", "inválido", "obrigatório", "formato"} {
		if len(msg) >= len(prefix) {
			for i := 0; i <= len(msg)-len(prefix); i++ {
				if msg[i:i+len(prefix)] == prefix {
					return true
				}
			}
		}
	}
	return false
}

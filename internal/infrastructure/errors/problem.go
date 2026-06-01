package errors

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
)

// ProblemDetails representa uma resposta de erro no formato RFC 7807 (application/problem+json).
type ProblemDetails struct {
	Type       string            `json:"type"`
	Title      string            `json:"title"`
	Status     int               `json:"status"`
	Detail     string            `json:"detail,omitempty"`
	Instance   string            `json:"instance,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	Extensions map[string]string `json:"extensions,omitempty"`
}

// problemFactory constrói ProblemDetails a partir de status HTTP ou erros de validação.
// Separado de ProblemDetails para que ToProblemDetails não precise de funções standalone.
type problemFactory struct{}

func (f problemFactory) create(status int, detail string) ProblemDetails {
	return ProblemDetails{
		Type:      "https://httpstatuses.com/" + strconv.Itoa(status),
		Title:     http.StatusText(status),
		Status:    status,
		Detail:    detail,
		Timestamp: time.Now().UTC(),
	}
}

func (f problemFactory) fromValidation(ve validator.ValidationErrors) ProblemDetails {
	extensions := make(map[string]string, len(ve))
	for _, fe := range ve {
		extensions[fe.Field()] = fe.Tag()
	}

	pd := f.create(http.StatusBadRequest, "a requisição contém campos inválidos")
	pd.Extensions = extensions
	return pd
}

// ToProblemDetails maps known sentinel errors to ProblemDetails — never leaks stack, SQL, or paths (R-SEC-001).
func ToProblemDetails(err error) (ProblemDetails, int) {
	f := problemFactory{}

	switch {
	case errors.Is(err, database.ErrConnection):
		return f.create(http.StatusServiceUnavailable, "serviço de banco de dados indisponível"), http.StatusServiceUnavailable

	case errors.Is(err, context.DeadlineExceeded):
		return f.create(http.StatusGatewayTimeout, "a operação excedeu o tempo limite"), http.StatusGatewayTimeout

	default:
		var ve validator.ValidationErrors
		if errors.As(err, &ve) { //nolint:errorsastype
			return f.fromValidation(ve), http.StatusBadRequest
		}

		return f.create(http.StatusInternalServerError, "erro interno do servidor"), http.StatusInternalServerError
	}
}

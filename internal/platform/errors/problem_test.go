package errors_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	infraerrors "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type ProblemSuite struct {
	suite.Suite
	ctx context.Context
}

func TestProblem(t *testing.T) {
	suite.Run(t, new(ProblemSuite))
}

func (s *ProblemSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ProblemSuite) TestToProblemDetails() {
	cenarios := []struct {
		nome           string
		err            error
		statusEsperado int
		tipoEsperado   string
	}{
		{
			nome:           "erro de conexão com banco retorna 503",
			err:            database.ErrConnection,
			statusEsperado: http.StatusServiceUnavailable,
			tipoEsperado:   "database-unavailable",
		},
		{
			nome:           "erro de conexão wrappado retorna 503",
			err:            fmt.Errorf("camada de adapter: %w", database.ErrConnection),
			statusEsperado: http.StatusServiceUnavailable,
			tipoEsperado:   "database-unavailable",
		},
		{
			nome:           "deadline excedido retorna 504",
			err:            context.DeadlineExceeded,
			statusEsperado: http.StatusGatewayTimeout,
			tipoEsperado:   "timeout",
		},
		{
			nome:           "deadline wrappado retorna 504",
			err:            fmt.Errorf("operação lenta: %w", context.DeadlineExceeded),
			statusEsperado: http.StatusGatewayTimeout,
			tipoEsperado:   "timeout",
		},
		{
			nome:           "erro desconhecido retorna 500",
			err:            fmt.Errorf("erro inesperado"),
			statusEsperado: http.StatusInternalServerError,
			tipoEsperado:   "internal-error",
		},
	}

	for _, c := range cenarios {
		s.Run(c.nome, func() {
			pd, status := infraerrors.ToProblemDetails(c.err)

			s.Equal(c.statusEsperado, status)
			s.Equal(c.statusEsperado, pd.Status)
			s.Contains(pd.Type, fmt.Sprintf("%d", c.statusEsperado))
			s.NotEmpty(pd.Title)
			s.NotEmpty(pd.Detail)
			s.False(pd.Timestamp.IsZero())
		})
	}
}

func (s *ProblemSuite) TestToProblemDetailsNaoExpoeStackTrace() {
	err := fmt.Errorf("SQL: SELECT * FROM users WHERE id=1; driver error: %w", fmt.Errorf("internal detail"))

	pd, status := infraerrors.ToProblemDetails(err)

	s.Equal(http.StatusInternalServerError, status)
	s.NotContains(pd.Detail, "SQL")
	s.NotContains(pd.Detail, "SELECT")
	s.NotContains(pd.Detail, "internal detail")
}

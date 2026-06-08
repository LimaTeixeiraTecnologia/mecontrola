package server_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
	handlermocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers/mocks"
)

type RouterSuite struct {
	suite.Suite
	ctx context.Context
}

func TestRouterSuite(t *testing.T) {
	suite.Run(t, new(RouterSuite))
}

func (s *RouterSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *RouterSuite) TestRegister() {
	type dependencies struct {
		useCase *handlermocks.MockUpsertUseCase
	}

	scenarios := []struct {
		name   string
		setup  func(dependencies) http.Handler
		expect func(*httptest.ResponseRecorder)
	}{
		{
			name: "deve manter 404 sem handler",
			setup: func(deps dependencies) http.Handler {
				_ = deps
				router := chi.NewRouter()
				server.NewUserRouter(nil).Register(router)
				return router
			},
			expect: func(response *httptest.ResponseRecorder) {
				s.Equal(http.StatusNotFound, response.Code)
			},
		},
		{
			name: "deve registrar endpoint com handler configurado",
			setup: func(deps dependencies) http.Handler {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.UpsertUserByWhatsApp{},
				).Return(output.UpsertUserByWhatsApp{ID: "stub-id"}, nil).Once()

				router := chi.NewRouter()
				handler := handlers.NewUpsertUserByWhatsAppHandler(deps.useCase, noop.NewProvider())
				server.NewUserRouter(handler).Register(router)
				return router
			},
			expect: func(response *httptest.ResponseRecorder) {
				s.NotEqual(http.StatusNotFound, response.Code)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				useCase: handlermocks.NewMockUpsertUseCase(s.T()),
			}
			router := scenario.setup(deps)

			request := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users/", bytes.NewBufferString(`{}`)).WithContext(s.ctx)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			scenario.expect(response)
		})
	}
}

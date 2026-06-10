package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type mockListAlertsUseCase struct {
	mock.Mock
}

func (m *mockListAlertsUseCase) Execute(ctx context.Context, in input.ListAlertsInput) (output.ListAlertsOutput, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.ListAlertsOutput), args.Error(1)
}

type ListAlertsHandlerSuite struct {
	suite.Suite
	handler *handlers.ListAlertsHandler
	mockUC  *mockListAlertsUseCase
	userID  uuid.UUID
}

func (s *ListAlertsHandlerSuite) SetupTest() {
	s.mockUC = new(mockListAlertsUseCase)
	s.handler = handlers.NewListAlertsHandler(s.mockUC, noop.NewProvider())
	s.userID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
}

func TestListAlertsHandlerSuite(t *testing.T) {
	suite.Run(t, new(ListAlertsHandlerSuite))
}

func (s *ListAlertsHandlerSuite) withPrincipal(r *http.Request) *http.Request {
	ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: s.userID})
	return r.WithContext(ctx)
}

func (s *ListAlertsHandlerSuite) TestHandle_Success() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/budgets/alerts", nil))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.ListAlertsInput) bool {
		return in.UserID == s.userID.String()
	})).Return(output.ListAlertsOutput{Alerts: []output.AlertOutput{}}, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListAlertsHandlerSuite) TestHandle_Unauthenticated() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/budgets/alerts", nil)
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *ListAlertsHandlerSuite) TestHandle_InvalidLimit() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/budgets/alerts?limit=abc", nil))
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ListAlertsHandlerSuite) TestHandle_InvalidCompetence() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/budgets/alerts?competence=invalid", nil))
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ListAlertsHandlerSuite) TestHandle_UserIsolation() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/budgets/alerts", nil))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.ListAlertsInput) bool {
		return in.UserID == s.userID.String()
	})).Return(output.ListAlertsOutput{}, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.mockUC.AssertExpectations(s.T())
}

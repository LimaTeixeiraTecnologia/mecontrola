package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type mockGetMonthlySummaryUseCase struct {
	mock.Mock
}

func (m *mockGetMonthlySummaryUseCase) Execute(ctx context.Context, userID string, competence string) (output.MonthlySummaryOutput, error) {
	args := m.Called(ctx, userID, competence)
	return args.Get(0).(output.MonthlySummaryOutput), args.Error(1)
}

type GetMonthlySummaryHandlerSuite struct {
	suite.Suite
	handler *handlers.GetMonthlySummaryHandler
	mockUC  *mockGetMonthlySummaryUseCase
	userID  uuid.UUID
}

func (s *GetMonthlySummaryHandlerSuite) SetupTest() {
	s.mockUC = new(mockGetMonthlySummaryUseCase)
	s.handler = handlers.NewGetMonthlySummaryHandler(s.mockUC, noop.NewProvider())
	s.userID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
}

func TestGetMonthlySummaryHandlerSuite(t *testing.T) {
	suite.Run(t, new(GetMonthlySummaryHandlerSuite))
}

func (s *GetMonthlySummaryHandlerSuite) withPrincipalAndCompetence(competence string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/budgets/"+competence+"/summary", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("competence", competence)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: s.userID})
	return req.WithContext(ctx)
}

func (s *GetMonthlySummaryHandlerSuite) TestHandle_Success() {
	req := s.withPrincipalAndCompetence("2025-01")
	rec := httptest.NewRecorder()

	tc := int64(100000)
	s.mockUC.On("Execute", mock.Anything, s.userID.String(), "2025-01").
		Return(output.MonthlySummaryOutput{
			UserID:     s.userID.String(),
			Competence: "2025-01",
			TotalCents: &tc,
		}, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetMonthlySummaryHandlerSuite) TestHandle_Unauthenticated() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/budgets/2025-01/summary", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("competence", "2025-01")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *GetMonthlySummaryHandlerSuite) TestHandle_NotFound() {
	req := s.withPrincipalAndCompetence("2025-01")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, s.userID.String(), "2025-01").
		Return(output.MonthlySummaryOutput{}, interfaces.ErrBudgetNotFound)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
}

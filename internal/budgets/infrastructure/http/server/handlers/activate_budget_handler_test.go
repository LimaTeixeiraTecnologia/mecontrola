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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type mockActivateBudgetUseCase struct {
	mock.Mock
}

func (m *mockActivateBudgetUseCase) Execute(ctx context.Context, in input.ActivateBudgetInput) (output.BudgetOutput, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.BudgetOutput), args.Error(1)
}

type ActivateBudgetHandlerSuite struct {
	suite.Suite
	handler *handlers.ActivateBudgetHandler
	mockUC  *mockActivateBudgetUseCase
	userID  uuid.UUID
}

func (s *ActivateBudgetHandlerSuite) SetupTest() {
	s.mockUC = new(mockActivateBudgetUseCase)
	s.handler = handlers.NewActivateBudgetHandler(s.mockUC, noop.NewProvider())
	s.userID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
}

func TestActivateBudgetHandlerSuite(t *testing.T) {
	suite.Run(t, new(ActivateBudgetHandlerSuite))
}

func (s *ActivateBudgetHandlerSuite) withPrincipal(r *http.Request) *http.Request {
	ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: s.userID})
	return r.WithContext(ctx)
}

func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func (s *ActivateBudgetHandlerSuite) TestHandle_Success() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/2025-01/activate", nil))
	req = withChiParam(req, "competence", "2025-01")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.ActivateBudgetInput) bool {
		return in.UserID == s.userID.String() && in.Competence == "2025-01"
	})).Return(output.BudgetOutput{ID: uuid.New().String(), Competence: "2025-01"}, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ActivateBudgetHandlerSuite) TestHandle_Unauthenticated() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/budgets/2025-01/activate", nil)
	req = withChiParam(req, "competence", "2025-01")
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *ActivateBudgetHandlerSuite) TestHandle_MissingCompetence() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets//activate", nil))
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ActivateBudgetHandlerSuite) TestHandle_NotFound() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/2025-01/activate", nil))
	req = withChiParam(req, "competence", "2025-01")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(output.BudgetOutput{}, interfaces.ErrBudgetNotFound)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *ActivateBudgetHandlerSuite) TestHandle_AlreadyActive() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/2025-01/activate", nil))
	req = withChiParam(req, "competence", "2025-01")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(output.BudgetOutput{}, entities.ErrBudgetAlreadyActive)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

func (s *ActivateBudgetHandlerSuite) TestHandle_AllocationSumInvalid() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/2025-01/activate", nil))
	req = withChiParam(req, "competence", "2025-01")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(output.BudgetOutput{}, entities.ErrBudgetAllocationSumMustBe10000)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
}

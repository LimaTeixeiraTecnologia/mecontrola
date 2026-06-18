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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type mockDeleteBudgetUseCase struct {
	mock.Mock
}

func (m *mockDeleteBudgetUseCase) Execute(ctx context.Context, in input.DeleteDraftInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type DeleteBudgetHandlerSuite struct {
	suite.Suite
	handler *handlers.DeleteBudgetHandler
	mockUC  *mockDeleteBudgetUseCase
	userID  uuid.UUID
}

func (s *DeleteBudgetHandlerSuite) SetupTest() {
	s.mockUC = new(mockDeleteBudgetUseCase)
	s.handler = handlers.NewDeleteBudgetHandler(s.mockUC, noop.NewProvider())
	s.userID = uuid.MustParse("44444444-4444-4444-4444-444444444444")
}

func TestDeleteBudgetHandlerSuite(t *testing.T) {
	suite.Run(t, new(DeleteBudgetHandlerSuite))
}

func (s *DeleteBudgetHandlerSuite) withPrincipal(r *http.Request) *http.Request {
	ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: s.userID})
	return r.WithContext(ctx)
}

func (s *DeleteBudgetHandlerSuite) TestHandle_Success() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/2025-01", nil))
	req = withChiParam(req, "competence", "2025-01")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.DeleteDraftInput) bool {
		return in.UserID == s.userID.String() && in.Competence == "2025-01"
	})).Return(nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteBudgetHandlerSuite) TestHandle_Unauthenticated() {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/2025-01", nil)
	req = withChiParam(req, "competence", "2025-01")
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *DeleteBudgetHandlerSuite) TestHandle_NotFound() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/2025-01", nil))
	req = withChiParam(req, "competence", "2025-01")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(interfaces.ErrBudgetNotFound)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *DeleteBudgetHandlerSuite) TestHandle_AlreadyActive() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/2025-01", nil))
	req = withChiParam(req, "competence", "2025-01")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(entities.ErrBudgetAlreadyActive)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

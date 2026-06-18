package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type mockDeleteExpenseUseCase struct {
	mock.Mock
}

func (m *mockDeleteExpenseUseCase) Execute(ctx context.Context, in input.DeleteExpenseInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type DeleteExpenseHandlerSuite struct {
	suite.Suite
	handler *handlers.DeleteExpenseHandler
	mockUC  *mockDeleteExpenseUseCase
	userID  uuid.UUID
}

func (s *DeleteExpenseHandlerSuite) SetupTest() {
	s.mockUC = new(mockDeleteExpenseUseCase)
	s.handler = handlers.NewDeleteExpenseHandler(s.mockUC, noop.NewProvider())
	s.userID = uuid.MustParse("77777777-7777-7777-7777-777777777777")
}

func TestDeleteExpenseHandlerSuite(t *testing.T) {
	suite.Run(t, new(DeleteExpenseHandlerSuite))
}

func (s *DeleteExpenseHandlerSuite) withPrincipal(r *http.Request) *http.Request {
	ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: s.userID})
	return r.WithContext(ctx)
}

func (s *DeleteExpenseHandlerSuite) validBody() []byte {
	b, _ := json.Marshal(map[string]any{"expected_version": int64(1)})
	return b
}

func (s *DeleteExpenseHandlerSuite) TestHandle_Success() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/expenses/ext-001", bytes.NewReader(s.validBody())))
	req = withChiParam(req, "id", "ext-001")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.DeleteExpenseInput) bool {
		return in.UserID == s.userID.String() && in.ExternalTransactionID == "ext-001"
	})).Return(nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteExpenseHandlerSuite) TestHandle_Unauthenticated() {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/expenses/ext-001", bytes.NewReader(s.validBody()))
	req = withChiParam(req, "id", "ext-001")
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *DeleteExpenseHandlerSuite) TestHandle_InvalidBody() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/expenses/ext-001", bytes.NewReader([]byte("not-json"))))
	req = withChiParam(req, "id", "ext-001")
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *DeleteExpenseHandlerSuite) TestHandle_NotFound() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/expenses/ext-001", bytes.NewReader(s.validBody())))
	req = withChiParam(req, "id", "ext-001")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(interfaces.ErrExpenseNotFound)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *DeleteExpenseHandlerSuite) TestHandle_Conflict() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/budgets/expenses/ext-001", bytes.NewReader(s.validBody())))
	req = withChiParam(req, "id", "ext-001")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(interfaces.ErrExpenseConflict)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

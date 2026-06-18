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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type mockUpsertExpenseUseCase struct {
	mock.Mock
}

func (m *mockUpsertExpenseUseCase) Execute(ctx context.Context, in input.UpsertExpenseInput) (output.ExpenseOutput, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.ExpenseOutput), args.Error(1)
}

type UpsertExpenseHandlerSuite struct {
	suite.Suite
	handler *handlers.UpsertExpenseHandler
	mockUC  *mockUpsertExpenseUseCase
	userID  uuid.UUID
}

func (s *UpsertExpenseHandlerSuite) SetupTest() {
	s.mockUC = new(mockUpsertExpenseUseCase)
	s.handler = handlers.NewUpsertExpenseHandler(s.mockUC, noop.NewProvider())
	s.userID = uuid.MustParse("66666666-6666-6666-6666-666666666666")
}

func TestUpsertExpenseHandlerSuite(t *testing.T) {
	suite.Run(t, new(UpsertExpenseHandlerSuite))
}

func (s *UpsertExpenseHandlerSuite) withPrincipal(r *http.Request) *http.Request {
	ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: s.userID})
	return r.WithContext(ctx)
}

func (s *UpsertExpenseHandlerSuite) validCreateBody() []byte {
	b, _ := json.Marshal(map[string]any{
		"external_transaction_id": "ext-001",
		"subcategory_id":          uuid.New().String(),
		"competence":              "2025-01",
		"amount_cents":            5000,
	})
	return b
}

func (s *UpsertExpenseHandlerSuite) validUpdateBody() []byte {
	version := int64(1)
	b, _ := json.Marshal(map[string]any{
		"external_transaction_id": "ext-001",
		"subcategory_id":          uuid.New().String(),
		"competence":              "2025-01",
		"amount_cents":            5000,
		"expected_version":        version,
	})
	return b
}

func (s *UpsertExpenseHandlerSuite) TestHandleCreate_Success() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/expenses", bytes.NewReader(s.validCreateBody())))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.UpsertExpenseInput) bool {
		return in.UserID == s.userID.String() && in.ExpectedVersion == nil
	})).Return(output.ExpenseOutput{}, nil)

	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *UpsertExpenseHandlerSuite) TestHandleCreate_Unauthenticated() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/budgets/expenses", bytes.NewReader(s.validCreateBody()))
	rec := httptest.NewRecorder()

	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *UpsertExpenseHandlerSuite) TestHandleCreate_InvalidBody() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/expenses", bytes.NewReader([]byte("not-json"))))
	rec := httptest.NewRecorder()

	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *UpsertExpenseHandlerSuite) TestHandleUpdate_Success() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPut, "/api/v1/budgets/expenses/ext-001", bytes.NewReader(s.validUpdateBody())))
	req = withChiParam(req, "id", "ext-001")
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.UpsertExpenseInput) bool {
		return in.UserID == s.userID.String() && in.ExpectedVersion != nil
	})).Return(output.ExpenseOutput{}, nil)

	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpsertExpenseHandlerSuite) TestHandleUpdate_Unauthenticated() {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/budgets/expenses/ext-001", bytes.NewReader(s.validUpdateBody()))
	req = withChiParam(req, "id", "ext-001")
	rec := httptest.NewRecorder()

	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *UpsertExpenseHandlerSuite) TestHandleUpdate_InvalidBody() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPut, "/api/v1/budgets/expenses/ext-001", bytes.NewReader([]byte("not-json"))))
	req = withChiParam(req, "id", "ext-001")
	rec := httptest.NewRecorder()

	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *UpsertExpenseHandlerSuite) TestHandleUpdate_MissingExpectedVersion() {
	b, _ := json.Marshal(map[string]any{
		"external_transaction_id": "ext-001",
		"competence":              "2025-01",
		"amount_cents":            5000,
	})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPut, "/api/v1/budgets/expenses/ext-001", bytes.NewReader(b)))
	req = withChiParam(req, "id", "ext-001")
	rec := httptest.NewRecorder()

	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *UpsertExpenseHandlerSuite) TestHandleCreate_NotFound() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/expenses", bytes.NewReader(s.validCreateBody())))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(output.ExpenseOutput{}, interfaces.ErrExpenseNotFound)

	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *UpsertExpenseHandlerSuite) TestHandleCreate_Conflict() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/expenses", bytes.NewReader(s.validCreateBody())))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(output.ExpenseOutput{}, interfaces.ErrExpenseConflict)

	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

func (s *UpsertExpenseHandlerSuite) TestHandleCreate_TombstoneConflict() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/expenses", bytes.NewReader(s.validCreateBody())))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(output.ExpenseOutput{}, interfaces.ErrExpenseTombstoneConflict)

	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

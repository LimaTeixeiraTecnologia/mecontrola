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

type mockCreateBudgetUseCase struct {
	mock.Mock
}

func (m *mockCreateBudgetUseCase) Execute(ctx context.Context, in input.CreateBudgetInput) (output.BudgetOutput, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.BudgetOutput), args.Error(1)
}

type CreateBudgetHandlerSuite struct {
	suite.Suite
	handler *handlers.CreateBudgetHandler
	mockUC  *mockCreateBudgetUseCase
	userID  uuid.UUID
}

func (s *CreateBudgetHandlerSuite) SetupTest() {
	s.mockUC = new(mockCreateBudgetUseCase)
	s.handler = handlers.NewCreateBudgetHandler(s.mockUC, noop.NewProvider())
	s.userID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
}

func TestCreateBudgetHandlerSuite(t *testing.T) {
	suite.Run(t, new(CreateBudgetHandlerSuite))
}

func (s *CreateBudgetHandlerSuite) withPrincipal(r *http.Request) *http.Request {
	ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: s.userID})
	return r.WithContext(ctx)
}

func (s *CreateBudgetHandlerSuite) TestHandle_Success() {
	body, _ := json.Marshal(map[string]any{
		"competence":  "2025-01",
		"total_cents": 100000,
	})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets", bytes.NewReader(body)))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.CreateBudgetInput) bool {
		return in.UserID == s.userID.String() && in.Competence == "2025-01"
	})).Return(output.BudgetOutput{ID: uuid.New().String(), Competence: "2025-01"}, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateBudgetHandlerSuite) TestHandle_Unauthenticated() {
	body, _ := json.Marshal(map[string]any{"competence": "2025-01"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/budgets", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *CreateBudgetHandlerSuite) TestHandle_InvalidPayload() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets", bytes.NewReader([]byte("not-json"))))
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *CreateBudgetHandlerSuite) TestHandle_Conflict() {
	body, _ := json.Marshal(map[string]any{"competence": "2025-01", "total_cents": 100000})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets", bytes.NewReader(body)))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(output.BudgetOutput{}, interfaces.ErrBudgetConflict)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

func (s *CreateBudgetHandlerSuite) TestHandle_UserIsolation() {
	body, _ := json.Marshal(map[string]any{"competence": "2025-01", "total_cents": 100000})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets", bytes.NewReader(body)))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.CreateBudgetInput) bool {
		return in.UserID == s.userID.String()
	})).Return(output.BudgetOutput{ID: uuid.New().String()}, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.mockUC.AssertExpectations(s.T())
}

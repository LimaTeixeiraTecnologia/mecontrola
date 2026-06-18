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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type mockCreateRecurrenceUseCase struct {
	mock.Mock
}

func (m *mockCreateRecurrenceUseCase) Execute(ctx context.Context, in input.CreateRecurrenceInput) (output.RecurrenceResultOutput, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.RecurrenceResultOutput), args.Error(1)
}

type CreateRecurrenceHandlerSuite struct {
	suite.Suite
	handler *handlers.CreateRecurrenceHandler
	mockUC  *mockCreateRecurrenceUseCase
	userID  uuid.UUID
}

func (s *CreateRecurrenceHandlerSuite) SetupTest() {
	s.mockUC = new(mockCreateRecurrenceUseCase)
	s.handler = handlers.NewCreateRecurrenceHandler(s.mockUC, noop.NewProvider())
	s.userID = uuid.MustParse("55555555-5555-5555-5555-555555555555")
}

func TestCreateRecurrenceHandlerSuite(t *testing.T) {
	suite.Run(t, new(CreateRecurrenceHandlerSuite))
}

func (s *CreateRecurrenceHandlerSuite) withPrincipal(r *http.Request) *http.Request {
	ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: s.userID})
	return r.WithContext(ctx)
}

func (s *CreateRecurrenceHandlerSuite) TestHandle_Success() {
	body, _ := json.Marshal(map[string]any{
		"source_competence": "2025-01",
		"months":            3,
	})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/recurrences", bytes.NewReader(body)))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.CreateRecurrenceInput) bool {
		return in.UserID == s.userID.String() && in.SourceCompetence == "2025-01" && in.Months == 3
	})).Return(output.RecurrenceResultOutput{}, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusMultiStatus, rec.Code)
}

func (s *CreateRecurrenceHandlerSuite) TestHandle_Unauthenticated() {
	body, _ := json.Marshal(map[string]any{"source_competence": "2025-01", "months": 3})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/budgets/recurrences", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *CreateRecurrenceHandlerSuite) TestHandle_InvalidBody() {
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/recurrences", bytes.NewReader([]byte("not-json"))))
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *CreateRecurrenceHandlerSuite) TestHandle_InvalidMonths() {
	body, _ := json.Marshal(map[string]any{"source_competence": "2025-01", "months": 0})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/budgets/recurrences", bytes.NewReader(body)))
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(output.RecurrenceResultOutput{}, usecases.ErrRecurrenceInvalidMonths)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
}

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	dtoinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/server/handlers"
)

type mockCreateTransactionUC struct{ mock.Mock }

func (m *mockCreateTransactionUC) Execute(ctx context.Context, raw dtoinput.RawCreateTransaction) (dtooutput.Transaction, error) {
	args := m.Called(ctx, raw)
	return args.Get(0).(dtooutput.Transaction), args.Error(1)
}

type mockUpdateTransactionUC struct{ mock.Mock }

func (m *mockUpdateTransactionUC) Execute(ctx context.Context, txID string, raw dtoinput.RawUpdateTransaction) (dtooutput.Transaction, error) {
	args := m.Called(ctx, txID, raw)
	return args.Get(0).(dtooutput.Transaction), args.Error(1)
}

type mockDeleteTransactionUC struct{ mock.Mock }

func (m *mockDeleteTransactionUC) Execute(ctx context.Context, txID string, version int64) error {
	args := m.Called(ctx, txID, version)
	return args.Error(0)
}

type mockGetTransactionUC struct{ mock.Mock }

func (m *mockGetTransactionUC) Execute(ctx context.Context, txID string) (dtooutput.Transaction, error) {
	args := m.Called(ctx, txID)
	return args.Get(0).(dtooutput.Transaction), args.Error(1)
}

type mockListTransactionsUC struct{ mock.Mock }

func (m *mockListTransactionsUC) Execute(ctx context.Context, refMonthStr, cursor string, limit int) (usecases.TransactionPage, error) {
	args := m.Called(ctx, refMonthStr, cursor, limit)
	return args.Get(0).(usecases.TransactionPage), args.Error(1)
}

type mockGetMonthlySummaryUC struct{ mock.Mock }

func (m *mockGetMonthlySummaryUC) Execute(ctx context.Context, refMonthStr string) (dtooutput.MonthlySummary, error) {
	args := m.Called(ctx, refMonthStr)
	return args.Get(0).(dtooutput.MonthlySummary), args.Error(1)
}

type HandlersSuite struct {
	suite.Suite
	userID uuid.UUID
	o11y   *noop.Provider
}

func (s *HandlersSuite) SetupTest() {
	s.userID = uuid.New()
	s.o11y = noop.NewProvider()
}

func TestHandlersSuite(t *testing.T) {
	suite.Run(t, new(HandlersSuite))
}

func (s *HandlersSuite) withPrincipal(r *http.Request) *http.Request {
	ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: s.userID})
	return r.WithContext(ctx)
}

func (s *HandlersSuite) TestCreateTransaction_Success() {
	uc := new(mockCreateTransactionUC)
	h := handlers.NewCreateTransactionHandler(uc, s.o11y)

	body, _ := json.Marshal(dtoinput.RawCreateTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   10000,
		Description:   "Salário",
		CategoryID:    uuid.New(),
		OccurredAt:    time.Now().Format(time.RFC3339),
	})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewReader(body)))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, mock.Anything).Return(dtooutput.Transaction{ID: uuid.New()}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestCreateTransaction_InvalidPayload() {
	uc := new(mockCreateTransactionUC)
	h := handlers.NewCreateTransactionHandler(uc, s.o11y)

	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewReader([]byte("invalid"))))
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *HandlersSuite) TestCreateTransaction_Unauthorized() {
	uc := new(mockCreateTransactionUC)
	h := handlers.NewCreateTransactionHandler(uc, s.o11y)

	uc.On("Execute", mock.Anything, mock.Anything).Return(dtooutput.Transaction{}, usecases.ErrUsecaseUnauthorized)
	body, _ := json.Marshal(dtoinput.RawCreateTransaction{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *HandlersSuite) TestUpdateTransaction_Success() {
	uc := new(mockUpdateTransactionUC)
	h := handlers.NewUpdateTransactionHandler(uc, s.o11y)

	txID := uuid.New().String()
	body, _ := json.Marshal(dtoinput.RawUpdateTransaction{Version: 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPatch, "/api/v1/transactions/"+txID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", txID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, txID, mock.Anything).Return(dtooutput.Transaction{ID: uuid.MustParse(txID)}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestDeleteTransaction_Success() {
	uc := new(mockDeleteTransactionUC)
	h := handlers.NewDeleteTransactionHandler(uc, s.o11y)

	txID := uuid.New().String()
	body, _ := json.Marshal(map[string]any{"version": 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/transactions/"+txID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", txID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, txID, int64(1)).Return(nil)
	h.Handle(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestGetTransaction_Success() {
	uc := new(mockGetTransactionUC)
	h := handlers.NewGetTransactionHandler(uc, s.o11y)

	txID := uuid.New().String()
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+txID, nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", txID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, txID).Return(dtooutput.Transaction{ID: uuid.MustParse(txID)}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestGetTransaction_NotFound() {
	uc := new(mockGetTransactionUC)
	h := handlers.NewGetTransactionHandler(uc, s.o11y)

	txID := uuid.New().String()
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+txID, nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", txID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, txID).Return(dtooutput.Transaction{}, usecases.ErrTransactionNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestListTransactions_Success() {
	uc := new(mockListTransactionsUC)
	h := handlers.NewListTransactionsHandler(uc, s.o11y)

	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/transactions?ref_month=2025-01", nil))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, "2025-01", "", 50).Return(usecases.TransactionPage{Transactions: []dtooutput.Transaction{}}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestGetMonthlySummary_Success() {
	uc := new(mockGetMonthlySummaryUC)
	h := handlers.NewGetMonthlySummaryHandler(uc, s.o11y)

	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/months/2025-01", nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ref_month", "2025-01")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, "2025-01").Return(dtooutput.MonthlySummary{RefMonth: "2025-01"}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

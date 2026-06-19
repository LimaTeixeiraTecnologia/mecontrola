package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	dtoinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/server/handlers"
	repopkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type mockCreateCardPurchaseUC struct{ mock.Mock }

func (m *mockCreateCardPurchaseUC) Execute(ctx context.Context, raw dtoinput.RawCreateCardPurchase) (dtooutput.CardPurchase, error) {
	args := m.Called(ctx, raw)
	return args.Get(0).(dtooutput.CardPurchase), args.Error(1)
}

type mockUpdateCardPurchaseUC struct{ mock.Mock }

func (m *mockUpdateCardPurchaseUC) Execute(ctx context.Context, purchaseID uuid.UUID, raw dtoinput.RawUpdateCardPurchase) (dtooutput.CardPurchase, error) {
	args := m.Called(ctx, purchaseID, raw)
	return args.Get(0).(dtooutput.CardPurchase), args.Error(1)
}

type mockDeleteCardPurchaseUC struct{ mock.Mock }

func (m *mockDeleteCardPurchaseUC) Execute(ctx context.Context, purchaseID uuid.UUID, version int64) error {
	args := m.Called(ctx, purchaseID, version)
	return args.Error(0)
}

type mockGetCardPurchaseUC struct{ mock.Mock }

func (m *mockGetCardPurchaseUC) Execute(ctx context.Context, purchaseID uuid.UUID) (dtooutput.CardPurchase, error) {
	args := m.Called(ctx, purchaseID)
	return args.Get(0).(dtooutput.CardPurchase), args.Error(1)
}

type mockListCardPurchasesUC struct{ mock.Mock }

func (m *mockListCardPurchasesUC) Execute(ctx context.Context, in usecases.ListCardPurchasesInput) (usecases.ListCardPurchasesOutput, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(usecases.ListCardPurchasesOutput), args.Error(1)
}

type mockGetCardInvoiceUC struct{ mock.Mock }

func (m *mockGetCardInvoiceUC) Execute(ctx context.Context, cardID uuid.UUID, refMonthStr string) (dtooutput.CardInvoice, error) {
	args := m.Called(ctx, cardID, refMonthStr)
	return args.Get(0).(dtooutput.CardInvoice), args.Error(1)
}

type mockCreateRecurringTemplateUC struct{ mock.Mock }

func (m *mockCreateRecurringTemplateUC) Execute(ctx context.Context, raw dtoinput.RawCreateRecurringTemplate) (dtooutput.RecurringTemplate, error) {
	args := m.Called(ctx, raw)
	return args.Get(0).(dtooutput.RecurringTemplate), args.Error(1)
}

type mockUpdateRecurringTemplateUC struct{ mock.Mock }

func (m *mockUpdateRecurringTemplateUC) Execute(ctx context.Context, templateID string, raw dtoinput.RawUpdateRecurringTemplate) (dtooutput.RecurringTemplate, error) {
	args := m.Called(ctx, templateID, raw)
	return args.Get(0).(dtooutput.RecurringTemplate), args.Error(1)
}

type mockDeleteRecurringTemplateUC struct{ mock.Mock }

func (m *mockDeleteRecurringTemplateUC) Execute(ctx context.Context, templateID string, version int64) error {
	args := m.Called(ctx, templateID, version)
	return args.Error(0)
}

type mockGetRecurringTemplateUC struct{ mock.Mock }

func (m *mockGetRecurringTemplateUC) Execute(ctx context.Context, templateID string) (dtooutput.RecurringTemplate, error) {
	args := m.Called(ctx, templateID)
	return args.Get(0).(dtooutput.RecurringTemplate), args.Error(1)
}

type mockListRecurringTemplatesUC struct{ mock.Mock }

func (m *mockListRecurringTemplatesUC) Execute(ctx context.Context, activeOnly bool, cursor string, limit int) (usecases.RecurringTemplatePage, error) {
	args := m.Called(ctx, activeOnly, cursor, limit)
	return args.Get(0).(usecases.RecurringTemplatePage), args.Error(1)
}

type mockListMonthlyEntriesUC struct{ mock.Mock }

func (m *mockListMonthlyEntriesUC) Execute(ctx context.Context, refMonthStr, cursor string, limit int) (dtooutput.MonthlyEntriesPage, error) {
	args := m.Called(ctx, refMonthStr, cursor, limit)
	return args.Get(0).(dtooutput.MonthlyEntriesPage), args.Error(1)
}

func (s *HandlersSuite) TestCreateCardPurchase_Success() {
	uc := new(mockCreateCardPurchaseUC)
	h := handlers.NewCreateCardPurchaseHandler(uc, s.o11y)

	body, _ := json.Marshal(dtoinput.RawCreateCardPurchase{
		CardID:            uuid.New(),
		TotalAmountCents:  50000,
		InstallmentsTotal: 3,
		Description:       "TV",
		CategoryID:        uuid.New(),
		PurchasedAt:       time.Now().Format(time.RFC3339),
	})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/card-purchases", bytes.NewReader(body)))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, mock.Anything).Return(dtooutput.CardPurchase{ID: uuid.New()}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestCreateCardPurchase_InvalidPayload() {
	uc := new(mockCreateCardPurchaseUC)
	h := handlers.NewCreateCardPurchaseHandler(uc, s.o11y)

	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/card-purchases", bytes.NewReader([]byte("invalid"))))
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *HandlersSuite) TestCreateCardPurchase_Unauthorized() {
	uc := new(mockCreateCardPurchaseUC)
	h := handlers.NewCreateCardPurchaseHandler(uc, s.o11y)

	body, _ := json.Marshal(dtoinput.RawCreateCardPurchase{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/card-purchases", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, mock.Anything).Return(dtooutput.CardPurchase{}, usecases.ErrUsecaseUnauthorized)
	h.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *HandlersSuite) TestUpdateCardPurchase_Success() {
	uc := new(mockUpdateCardPurchaseUC)
	h := handlers.NewUpdateCardPurchaseHandler(uc, s.o11y)

	purchaseID := uuid.New()
	body, _ := json.Marshal(dtoinput.RawUpdateCardPurchase{Version: 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPatch, "/api/v1/card-purchases/"+purchaseID.String(), bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", purchaseID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, purchaseID, mock.Anything).Return(dtooutput.CardPurchase{ID: purchaseID}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestUpdateCardPurchase_NotFound() {
	uc := new(mockUpdateCardPurchaseUC)
	h := handlers.NewUpdateCardPurchaseHandler(uc, s.o11y)

	purchaseID := uuid.New()
	body, _ := json.Marshal(dtoinput.RawUpdateCardPurchase{Version: 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPatch, "/api/v1/card-purchases/"+purchaseID.String(), bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", purchaseID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, purchaseID, mock.Anything).Return(dtooutput.CardPurchase{}, usecases.ErrCardPurchaseNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestUpdateCardPurchase_Unauthorized() {
	uc := new(mockUpdateCardPurchaseUC)
	h := handlers.NewUpdateCardPurchaseHandler(uc, s.o11y)

	purchaseID := uuid.New()
	body, _ := json.Marshal(dtoinput.RawUpdateCardPurchase{Version: 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPatch, "/api/v1/card-purchases/"+purchaseID.String(), bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", purchaseID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, purchaseID, mock.Anything).Return(dtooutput.CardPurchase{}, usecases.ErrUsecaseUnauthorized)
	h.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestDeleteCardPurchase_Success() {
	uc := new(mockDeleteCardPurchaseUC)
	h := handlers.NewDeleteCardPurchaseHandler(uc, s.o11y)

	purchaseID := uuid.New()
	body, _ := json.Marshal(map[string]any{"version": 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/card-purchases/"+purchaseID.String(), bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", purchaseID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, purchaseID, int64(1)).Return(nil)
	h.Handle(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestDeleteCardPurchase_NotFound() {
	uc := new(mockDeleteCardPurchaseUC)
	h := handlers.NewDeleteCardPurchaseHandler(uc, s.o11y)

	purchaseID := uuid.New()
	body, _ := json.Marshal(map[string]any{"version": 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/card-purchases/"+purchaseID.String(), bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", purchaseID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, purchaseID, int64(1)).Return(usecases.ErrCardPurchaseNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestGetCardPurchase_Success() {
	uc := new(mockGetCardPurchaseUC)
	h := handlers.NewGetCardPurchaseHandler(uc, s.o11y)

	purchaseID := uuid.New()
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/card-purchases/"+purchaseID.String(), nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", purchaseID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, purchaseID).Return(dtooutput.CardPurchase{ID: purchaseID}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestGetCardPurchase_NotFound() {
	uc := new(mockGetCardPurchaseUC)
	h := handlers.NewGetCardPurchaseHandler(uc, s.o11y)

	purchaseID := uuid.New()
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/card-purchases/"+purchaseID.String(), nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", purchaseID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, purchaseID).Return(dtooutput.CardPurchase{}, usecases.ErrCardPurchaseNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestListCardPurchases_Success() {
	uc := new(mockListCardPurchasesUC)
	h := handlers.NewListCardPurchasesHandler(uc, s.o11y)

	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/card-purchases", nil))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, mock.Anything).Return(usecases.ListCardPurchasesOutput{Items: []dtooutput.CardPurchase{}}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestGetCardInvoice_Success() {
	uc := new(mockGetCardInvoiceUC)
	h := handlers.NewGetCardInvoiceHandler(uc, s.o11y)

	cardID := uuid.New()
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/cards/"+cardID.String()+"/invoices/2025-01", nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", cardID.String())
	rctx.URLParams.Add("ref_month", "2025-01")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, cardID, "2025-01").Return(dtooutput.CardInvoice{CardID: cardID}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestGetCardInvoice_NotFound() {
	uc := new(mockGetCardInvoiceUC)
	h := handlers.NewGetCardInvoiceHandler(uc, s.o11y)

	cardID := uuid.New()
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/cards/"+cardID.String()+"/invoices/2025-01", nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", cardID.String())
	rctx.URLParams.Add("ref_month", "2025-01")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, cardID, "2025-01").Return(dtooutput.CardInvoice{}, usecases.ErrCardInvoiceNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestCreateRecurringTemplate_Success() {
	uc := new(mockCreateRecurringTemplateUC)
	h := handlers.NewCreateRecurringTemplateHandler(uc, s.o11y)

	body, _ := json.Marshal(dtoinput.RawCreateRecurringTemplate{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   15000,
		Description:   "Aluguel",
		CategoryID:    uuid.New(),
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     time.Now().Format(time.RFC3339),
	})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/recurring-templates", bytes.NewReader(body)))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, mock.Anything).Return(dtooutput.RecurringTemplate{ID: uuid.New()}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestCreateRecurringTemplate_InvalidPayload() {
	uc := new(mockCreateRecurringTemplateUC)
	h := handlers.NewCreateRecurringTemplateHandler(uc, s.o11y)

	req := s.withPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/recurring-templates", bytes.NewReader([]byte("invalid"))))
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *HandlersSuite) TestCreateRecurringTemplate_Unauthorized() {
	uc := new(mockCreateRecurringTemplateUC)
	h := handlers.NewCreateRecurringTemplateHandler(uc, s.o11y)

	body, _ := json.Marshal(dtoinput.RawCreateRecurringTemplate{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recurring-templates", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, mock.Anything).Return(dtooutput.RecurringTemplate{}, usecases.ErrUsecaseUnauthorized)
	h.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *HandlersSuite) TestUpdateRecurringTemplate_Success() {
	uc := new(mockUpdateRecurringTemplateUC)
	h := handlers.NewUpdateRecurringTemplateHandler(uc, s.o11y)

	templateID := uuid.New().String()
	body, _ := json.Marshal(dtoinput.RawUpdateRecurringTemplate{Version: 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPatch, "/api/v1/recurring-templates/"+templateID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, templateID, mock.Anything).Return(dtooutput.RecurringTemplate{ID: uuid.MustParse(templateID)}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestUpdateRecurringTemplate_NotFound() {
	uc := new(mockUpdateRecurringTemplateUC)
	h := handlers.NewUpdateRecurringTemplateHandler(uc, s.o11y)

	templateID := uuid.New().String()
	body, _ := json.Marshal(dtoinput.RawUpdateRecurringTemplate{Version: 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPatch, "/api/v1/recurring-templates/"+templateID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, templateID, mock.Anything).Return(dtooutput.RecurringTemplate{}, repopkg.ErrRecurringTemplateNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestUpdateRecurringTemplate_Unauthorized() {
	uc := new(mockUpdateRecurringTemplateUC)
	h := handlers.NewUpdateRecurringTemplateHandler(uc, s.o11y)

	templateID := uuid.New().String()
	body, _ := json.Marshal(dtoinput.RawUpdateRecurringTemplate{Version: 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPatch, "/api/v1/recurring-templates/"+templateID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, templateID, mock.Anything).Return(dtooutput.RecurringTemplate{}, usecases.ErrUsecaseUnauthorized)
	h.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestDeleteRecurringTemplate_Success() {
	uc := new(mockDeleteRecurringTemplateUC)
	h := handlers.NewDeleteRecurringTemplateHandler(uc, s.o11y)

	templateID := uuid.New().String()
	body, _ := json.Marshal(map[string]any{"version": 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/recurring-templates/"+templateID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, templateID, int64(1)).Return(nil)
	h.Handle(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestDeleteRecurringTemplate_NotFound() {
	uc := new(mockDeleteRecurringTemplateUC)
	h := handlers.NewDeleteRecurringTemplateHandler(uc, s.o11y)

	templateID := uuid.New().String()
	body, _ := json.Marshal(map[string]any{"version": 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/recurring-templates/"+templateID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, templateID, int64(1)).Return(repopkg.ErrRecurringTemplateNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestGetRecurringTemplate_Success() {
	uc := new(mockGetRecurringTemplateUC)
	h := handlers.NewGetRecurringTemplateHandler(uc, s.o11y)

	templateID := uuid.New().String()
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/recurring-templates/"+templateID, nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, templateID).Return(dtooutput.RecurringTemplate{ID: uuid.MustParse(templateID)}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestGetRecurringTemplate_NotFound() {
	uc := new(mockGetRecurringTemplateUC)
	h := handlers.NewGetRecurringTemplateHandler(uc, s.o11y)

	templateID := uuid.New().String()
	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/recurring-templates/"+templateID, nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, templateID).Return(dtooutput.RecurringTemplate{}, repopkg.ErrRecurringTemplateNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestListRecurringTemplates_Success() {
	uc := new(mockListRecurringTemplatesUC)
	h := handlers.NewListRecurringTemplatesHandler(uc, s.o11y)

	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/recurring-templates", nil))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, false, "", 50).Return(usecases.RecurringTemplatePage{Templates: []dtooutput.RecurringTemplate{}}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestListMonthlyEntries_Success() {
	uc := new(mockListMonthlyEntriesUC)
	h := handlers.NewListMonthlyEntriesHandler(uc, s.o11y)

	req := s.withPrincipal(httptest.NewRequest(http.MethodGet, "/api/v1/months/2025-01/entries", nil))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ref_month", "2025-01")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, "2025-01", "", 50).Return(dtooutput.MonthlyEntriesPage{Items: []any{}}, nil)
	h.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestUpdateTransaction_NotFound() {
	uc := new(mockUpdateTransactionUC)
	h := handlers.NewUpdateTransactionHandler(uc, s.o11y)

	txID := uuid.New().String()
	body, _ := json.Marshal(dtoinput.RawUpdateTransaction{Version: 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPatch, "/api/v1/transactions/"+txID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", txID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, txID, mock.Anything).Return(dtooutput.Transaction{}, usecases.ErrTransactionNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestUpdateTransaction_Unauthorized() {
	uc := new(mockUpdateTransactionUC)
	h := handlers.NewUpdateTransactionHandler(uc, s.o11y)

	txID := uuid.New().String()
	body, _ := json.Marshal(dtoinput.RawUpdateTransaction{Version: 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodPatch, "/api/v1/transactions/"+txID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", txID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, txID, mock.Anything).Return(dtooutput.Transaction{}, usecases.ErrUsecaseUnauthorized)
	h.Handle(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
	uc.AssertExpectations(s.T())
}

func (s *HandlersSuite) TestDeleteTransaction_NotFound() {
	uc := new(mockDeleteTransactionUC)
	h := handlers.NewDeleteTransactionHandler(uc, s.o11y)

	txID := uuid.New().String()
	body, _ := json.Marshal(map[string]any{"version": 1})
	req := s.withPrincipal(httptest.NewRequest(http.MethodDelete, "/api/v1/transactions/"+txID, bytes.NewReader(body)))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", txID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	uc.On("Execute", mock.Anything, txID, int64(1)).Return(usecases.ErrTransactionNotFound)
	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	uc.AssertExpectations(s.T())
}

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
	idemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

type mockCreateCard struct{ mock.Mock }

func (m *mockCreateCard) Execute(ctx context.Context, in input.CreateCard) (output.Card, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.Card), args.Error(1)
}

type mockListCards struct{ mock.Mock }

func (m *mockListCards) Execute(ctx context.Context, in input.ListCards) (output.CardList, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.CardList), args.Error(1)
}

type mockGetCard struct{ mock.Mock }

func (m *mockGetCard) Execute(ctx context.Context, in input.GetCard) (output.Card, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.Card), args.Error(1)
}

type mockUpdateCard struct{ mock.Mock }

func (m *mockUpdateCard) Execute(ctx context.Context, in input.UpdateCard) (output.Card, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.Card), args.Error(1)
}

type mockSoftDeleteCard struct{ mock.Mock }

func (m *mockSoftDeleteCard) Execute(ctx context.Context, in input.SoftDeleteCard) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

type mockInvoiceFor struct{ mock.Mock }

func (m *mockInvoiceFor) Execute(ctx context.Context, in input.InvoiceFor) (output.Invoice, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(output.Invoice), args.Error(1)
}

type CardRouterSuite struct {
	suite.Suite
	router      chi.Router
	idemStorage *idemocks.Storage
	createUC    *mockCreateCard
	listUC      *mockListCards
	getUC       *mockGetCard
	updateUC    *mockUpdateCard
	deleteUC    *mockSoftDeleteCard
	invoiceUC   *mockInvoiceFor
}

func TestCardRouter(t *testing.T) {
	suite.Run(t, new(CardRouterSuite))
}

func (s *CardRouterSuite) SetupTest() {
	o11y := noop.NewProvider()
	s.idemStorage = idemocks.NewStorage(s.T())
	s.createUC = &mockCreateCard{}
	s.listUC = &mockListCards{}
	s.getUC = &mockGetCard{}
	s.updateUC = &mockUpdateCard{}
	s.deleteUC = &mockSoftDeleteCard{}
	s.invoiceUC = &mockInvoiceFor{}

	createH := handlers.NewCreateCardHandler(s.createUC, o11y)
	listH := handlers.NewListCardsHandler(s.listUC, o11y)
	getH := handlers.NewGetCardHandler(s.getUC, o11y)
	updateH := handlers.NewUpdateCardHandler(s.updateUC, o11y)
	deleteH := handlers.NewDeleteCardHandler(s.deleteUC, o11y)
	invoiceH := handlers.NewInvoiceForHandler(s.invoiceUC, o11y)

	passthrough := func(next http.Handler) http.Handler { return next }
	cardRouter := server.NewCardRouter(createH, listH, getH, updateH, deleteH, invoiceH, s.idemStorage, o11y, passthrough, passthrough)

	r := chi.NewRouter()
	cardRouter.Register(r)
	s.router = r
}

func (s *CardRouterSuite) TestRoutes_NoXUserID_Returns401() {
	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/cards", ""},
		{http.MethodPost, "/api/v1/cards", `{"name":"X","nickname":"X","closing_day":1,"due_day":2}`},
		{http.MethodGet, "/api/v1/cards/" + uuid.New().String(), ""},
		{http.MethodPut, "/api/v1/cards/" + uuid.New().String(), `{"name":"X","nickname":"X","closing_day":1,"due_day":2}`},
		{http.MethodDelete, "/api/v1/cards/" + uuid.New().String(), ""},
		{http.MethodGet, "/api/v1/cards/" + uuid.New().String() + "/invoices?for=2024-01-01", ""},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, strings.NewReader(ep.body))
		rr := httptest.NewRecorder()
		s.router.ServeHTTP(rr, req)
		s.Equal(http.StatusUnauthorized, rr.Code, "expected 401 for %s %s without X-User-ID", ep.method, ep.path)
	}
}

func (s *CardRouterSuite) TestPost_WithoutIdempotencyKey_Returns400() {
	userID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cards",
		strings.NewReader(`{"name":"Nu","nickname":"Nu","closing_day":1,"due_day":2}`))
	req.Header.Set("X-User-ID", userID)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *CardRouterSuite) TestPut_WithoutIdempotencyKey_Returns400() {
	userID := uuid.New().String()
	cardID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cards/"+cardID,
		strings.NewReader(`{"name":"Nu","nickname":"Nu","closing_day":1,"due_day":2}`))
	req.Header.Set("X-User-ID", userID)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *CardRouterSuite) TestDelete_WithoutIdempotencyKey_Returns400() {
	userID := uuid.New().String()
	cardID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cards/"+cardID, nil)
	req.Header.Set("X-User-ID", userID)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *CardRouterSuite) TestGet_NoIdempotencyRequired() {
	userID := uuid.New().String()
	s.listUC.On("Execute", mock.Anything, mock.MatchedBy(func(any) bool { return true })).
		Return(output.CardList{}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards", nil)
	req.Header.Set("X-User-ID", userID)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	s.Equal(http.StatusOK, rr.Code)
}

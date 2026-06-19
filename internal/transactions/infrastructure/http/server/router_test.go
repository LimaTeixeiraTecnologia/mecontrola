package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	dtoinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	txserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/server/handlers"
)

type mockIdemStorage struct{ mock.Mock }

func (m *mockIdemStorage) Get(ctx context.Context, scope, key, userID string) (idempotency.Record, error) {
	args := m.Called(ctx, scope, key, userID)
	return args.Get(0).(idempotency.Record), args.Error(1)
}
func (m *mockIdemStorage) Put(ctx context.Context, rec idempotency.Record) error {
	args := m.Called(ctx, rec)
	return args.Error(0)
}

type dummyCreateTxUC struct{}

func (d *dummyCreateTxUC) Execute(_ context.Context, _ dtoinput.RawCreateTransaction) (dtooutput.Transaction, error) {
	return dtooutput.Transaction{ID: uuid.New()}, nil
}

type dummyUpdateTxUC struct{}

func (d *dummyUpdateTxUC) Execute(_ context.Context, _ string, _ dtoinput.RawUpdateTransaction) (dtooutput.Transaction, error) {
	return dtooutput.Transaction{}, nil
}

type dummyDeleteTxUC struct{}

func (d *dummyDeleteTxUC) Execute(_ context.Context, _ string, _ int64) error { return nil }

type dummyGetTxUC struct{}

func (d *dummyGetTxUC) Execute(_ context.Context, _ string) (dtooutput.Transaction, error) {
	return dtooutput.Transaction{}, nil
}

type dummyListTxUC struct{}

func (d *dummyListTxUC) Execute(_ context.Context, _, _ string, _ int) (usecases.TransactionPage, error) {
	return usecases.TransactionPage{}, nil
}

type dummyCreateCPUC struct{}

func (d *dummyCreateCPUC) Execute(_ context.Context, _ dtoinput.RawCreateCardPurchase) (dtooutput.CardPurchase, error) {
	return dtooutput.CardPurchase{}, nil
}

type dummyUpdateCPUC struct{}

func (d *dummyUpdateCPUC) Execute(_ context.Context, _ uuid.UUID, _ dtoinput.RawUpdateCardPurchase) (dtooutput.CardPurchase, error) {
	return dtooutput.CardPurchase{}, nil
}

type dummyDeleteCPUC struct{}

func (d *dummyDeleteCPUC) Execute(_ context.Context, _ uuid.UUID, _ int64) error { return nil }

type dummyGetCPUC struct{}

func (d *dummyGetCPUC) Execute(_ context.Context, _ uuid.UUID) (dtooutput.CardPurchase, error) {
	return dtooutput.CardPurchase{}, nil
}

type dummyListCPUC struct{}

func (d *dummyListCPUC) Execute(_ context.Context, _ usecases.ListCardPurchasesInput) (usecases.ListCardPurchasesOutput, error) {
	return usecases.ListCardPurchasesOutput{}, nil
}

type dummyCreateRTUC struct{}

func (d *dummyCreateRTUC) Execute(_ context.Context, _ dtoinput.RawCreateRecurringTemplate) (dtooutput.RecurringTemplate, error) {
	return dtooutput.RecurringTemplate{}, nil
}

type dummyUpdateRTUC struct{}

func (d *dummyUpdateRTUC) Execute(_ context.Context, _ string, _ dtoinput.RawUpdateRecurringTemplate) (dtooutput.RecurringTemplate, error) {
	return dtooutput.RecurringTemplate{}, nil
}

type dummyDeleteRTUC struct{}

func (d *dummyDeleteRTUC) Execute(_ context.Context, _ string, _ int64) error { return nil }

type dummyGetRTUC struct{}

func (d *dummyGetRTUC) Execute(_ context.Context, _ string) (dtooutput.RecurringTemplate, error) {
	return dtooutput.RecurringTemplate{}, nil
}

type dummyListRTUC struct{}

func (d *dummyListRTUC) Execute(_ context.Context, _ bool, _ string, _ int) (usecases.RecurringTemplatePage, error) {
	return usecases.RecurringTemplatePage{}, nil
}

type dummyGetMSUC struct{}

func (d *dummyGetMSUC) Execute(_ context.Context, _ string) (dtooutput.MonthlySummary, error) {
	return dtooutput.MonthlySummary{}, nil
}

type dummyListMEUC struct{}

func (d *dummyListMEUC) Execute(_ context.Context, _, _ string, _ int) (dtooutput.MonthlyEntriesPage, error) {
	return dtooutput.MonthlyEntriesPage{}, nil
}

func buildRouter(t *testing.T) *txserver.TransactionsRouter {
	t.Helper()
	o11y := noop.NewProvider()
	idemStorage := new(mockIdemStorage)
	idemStorage.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(idempotency.Record{}, idempotency.ErrNotFound)
	idemStorage.On("Put", mock.Anything, mock.Anything).Return(nil)

	return txserver.NewTransactionsRouter(
		handlers.NewCreateTransactionHandler(&dummyCreateTxUC{}, o11y),
		handlers.NewUpdateTransactionHandler(&dummyUpdateTxUC{}, o11y),
		handlers.NewDeleteTransactionHandler(&dummyDeleteTxUC{}, o11y),
		handlers.NewGetTransactionHandler(&dummyGetTxUC{}, o11y),
		handlers.NewListTransactionsHandler(&dummyListTxUC{}, o11y),
		handlers.NewCreateCardPurchaseHandler(&dummyCreateCPUC{}, o11y),
		handlers.NewUpdateCardPurchaseHandler(&dummyUpdateCPUC{}, o11y),
		handlers.NewDeleteCardPurchaseHandler(&dummyDeleteCPUC{}, o11y),
		handlers.NewGetCardPurchaseHandler(&dummyGetCPUC{}, o11y),
		handlers.NewListCardPurchasesHandler(&dummyListCPUC{}, o11y),
		handlers.NewCreateRecurringTemplateHandler(&dummyCreateRTUC{}, o11y),
		handlers.NewUpdateRecurringTemplateHandler(&dummyUpdateRTUC{}, o11y),
		handlers.NewDeleteRecurringTemplateHandler(&dummyDeleteRTUC{}, o11y),
		handlers.NewGetRecurringTemplateHandler(&dummyGetRTUC{}, o11y),
		handlers.NewListRecurringTemplatesHandler(&dummyListRTUC{}, o11y),
		handlers.NewGetMonthlySummaryHandler(&dummyGetMSUC{}, o11y),
		handlers.NewListMonthlyEntriesHandler(&dummyListMEUC{}, o11y),
		idemStorage,
		24*time.Hour,
		o11y,
		func(next http.Handler) http.Handler { return next },
	)
}

func withAuthPrincipal(r *http.Request) *http.Request {
	ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: uuid.New()})
	return r.WithContext(ctx)
}

func TestRouterRegistersAllTransactionRoutes(t *testing.T) {
	router := buildRouter(t)
	mux := chi.NewRouter()
	router.Register(mux)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/transactions"},
		{http.MethodGet, "/api/v1/transactions"},
		{http.MethodGet, "/api/v1/transactions/" + uuid.New().String()},
		{http.MethodPatch, "/api/v1/transactions/" + uuid.New().String()},
		{http.MethodDelete, "/api/v1/transactions/" + uuid.New().String()},
		{http.MethodPost, "/api/v1/card-purchases"},
		{http.MethodGet, "/api/v1/card-purchases"},
		{http.MethodGet, "/api/v1/card-purchases/" + uuid.New().String()},
		{http.MethodPatch, "/api/v1/card-purchases/" + uuid.New().String()},
		{http.MethodDelete, "/api/v1/card-purchases/" + uuid.New().String()},
		{http.MethodPost, "/api/v1/recurring-templates"},
		{http.MethodGet, "/api/v1/recurring-templates"},
		{http.MethodGet, "/api/v1/recurring-templates/" + uuid.New().String()},
		{http.MethodPatch, "/api/v1/recurring-templates/" + uuid.New().String()},
		{http.MethodDelete, "/api/v1/recurring-templates/" + uuid.New().String()},
		{http.MethodGet, "/api/v1/months/2025-01"},
		{http.MethodGet, "/api/v1/months/2025-01/entries"},
	} {
		req := withAuthPrincipal(httptest.NewRequest(tc.method, tc.path, nil))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.NotEqual(t, http.StatusNotFound, rec.Code, "route not registered: %s %s", tc.method, tc.path)
		assert.NotEqual(t, http.StatusMethodNotAllowed, rec.Code, "method not allowed: %s %s", tc.method, tc.path)
	}
}

func TestRouterNotRegisteredWhenModuleDisabled(t *testing.T) {
	mux := chi.NewRouter()
	req := withAuthPrincipal(httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

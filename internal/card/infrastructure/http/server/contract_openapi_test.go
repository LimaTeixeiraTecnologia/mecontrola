package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	idemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

type ContractOpenAPISuite struct {
	suite.Suite
	doc            *openapi3.T
	apiRouter      routers.Router
	httpRouter     chi.Router
	idemStorage    *idemocks.Storage
	createUC       *mockCreateCard
	listUC         *mockListCards
	getUC          *mockGetCard
	updateUC       *mockUpdateCard
	deleteUC       *mockSoftDeleteCard
	invoiceUC      *mockInvoiceFor
	bestPurchaseUC *mockBestPurchaseDay
}

func TestContractOpenAPI(t *testing.T) {
	suite.Run(t, new(ContractOpenAPISuite))
}

func (s *ContractOpenAPISuite) SetupTest() {
	_, file, _, _ := runtime.Caller(0)
	yamlPath := filepath.Join(filepath.Dir(file), "..", "..", "..", "openapi.yaml")

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(yamlPath)
	s.Require().NoError(err, "openapi.yaml deve carregar sem erro")
	s.Require().NoError(doc.Validate(loader.Context), "openapi.yaml deve ser válido")
	s.doc = doc

	apiRouter, err := gorillamux.NewRouter(doc)
	s.Require().NoError(err, "gorillamux.NewRouter deve construir sem erro")
	s.apiRouter = apiRouter

	o11y := noop.NewProvider()
	s.idemStorage = idemocks.NewStorage(s.T())
	s.createUC = &mockCreateCard{}
	s.listUC = &mockListCards{}
	s.getUC = &mockGetCard{}
	s.updateUC = &mockUpdateCard{}
	s.deleteUC = &mockSoftDeleteCard{}
	s.invoiceUC = &mockInvoiceFor{}
	s.bestPurchaseUC = &mockBestPurchaseDay{}

	createH := handlers.NewCreateCardHandler(s.createUC, o11y)
	listH := handlers.NewListCardsHandler(s.listUC, o11y)
	getH := handlers.NewGetCardHandler(s.getUC, o11y)
	updateH := handlers.NewUpdateCardHandler(s.updateUC, o11y)
	deleteH := handlers.NewDeleteCardHandler(s.deleteUC, o11y)
	invoiceH := handlers.NewInvoiceForHandler(s.invoiceUC, o11y)
	bestPurchaseH := handlers.NewBestPurchaseDayHandler(s.bestPurchaseUC, o11y)

	passthrough := func(next http.Handler) http.Handler { return next }
	cardRouter := server.NewCardRouter(createH, listH, getH, updateH, deleteH, invoiceH, bestPurchaseH, s.idemStorage, o11y, passthrough, passthrough)
	r := chi.NewRouter()
	cardRouter.Register(r)
	s.httpRouter = r
}

func (s *ContractOpenAPISuite) execAndValidate(t *testing.T, method, path string, body string, headers map[string]string) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()

	fullURL := "http://localhost:8080" + path
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, fullURL, reqBody)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	route, pathParams, err := s.apiRouter.FindRoute(req)
	require.NoError(t, err, "rota %s %s deve estar no openapi.yaml", method, path)

	if body != "" {
		reqForValidation := req.Clone(req.Context())
		reqForValidation.Body = io.NopCloser(strings.NewReader(body))

		reqInput := &openapi3filter.RequestValidationInput{
			Request:    reqForValidation,
			PathParams: pathParams,
			Route:      route,
			Options: &openapi3filter.Options{
				AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			},
		}
		err = openapi3filter.ValidateRequest(context.Background(), reqInput)
		require.NoError(t, err, "request deve ser válido contra openapi.yaml")
	}

	chiReq := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		chiReq.Header.Set(k, v)
	}
	if body != "" {
		chiReq.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	s.httpRouter.ServeHTTP(rr, chiReq)

	respBody := rr.Body.Bytes()
	respInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
			Options: &openapi3filter.Options{
				AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			},
		},
		Status: rr.Code,
		Header: rr.Header(),
	}
	respInput.SetBodyBytes(respBody)

	err = openapi3filter.ValidateResponse(context.Background(), respInput)
	require.NoError(t, err, "response %d para %s %s deve bater com schema do openapi.yaml — body=%s", rr.Code, method, path, truncate(respBody, 512))

	return rr, req
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}

func (s *ContractOpenAPISuite) decodeMap(rr *httptest.ResponseRecorder) map[string]any {
	s.T().Helper()
	var m map[string]any
	require.NoError(s.T(), json.Unmarshal(rr.Body.Bytes(), &m), "response deve ser JSON object")
	return m
}

func (s *ContractOpenAPISuite) mutHeaders(idemKey string) map[string]string {
	return map[string]string{
		"X-User-ID":           contractUserID,
		"X-Gateway-Auth":      "stub-signature",
		"X-Gateway-Timestamp": "2026-06-19T00:00:00Z",
		"Idempotency-Key":     idemKey,
	}
}

func (s *ContractOpenAPISuite) authHeaders() map[string]string {
	return map[string]string{
		"X-User-ID":           contractUserID,
		"X-Gateway-Auth":      "stub-signature",
		"X-Gateway-Timestamp": "2026-06-19T00:00:00Z",
	}
}

func (s *ContractOpenAPISuite) TestContract_PostCards_RealValidation() {
	s.idemStorage.On("Get", mock.Anything, "card", "idem-oa-post-001", contractUserID).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()
	s.createUC.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).
		Return(contractCard(), nil).Once()

	body := `{"nickname":"Nu","bank":"nubank","due_day":22}`
	rr, _ := s.execAndValidate(s.T(), http.MethodPost, "/api/v1/cards", body, s.mutHeaders("idem-oa-post-001"))

	s.Equal(http.StatusCreated, rr.Code)
	m := s.decodeMap(rr)
	for _, k := range []string{"id", "user_id", "nickname", "bank", "closing_day", "due_day", "best_purchase_day", "created_at", "updated_at"} {
		s.Contains(m, k, "campo snake_case %q deve existir no payload Card", k)
	}
}

func (s *ContractOpenAPISuite) TestContract_GetCards_RealValidation() {
	s.listUC.On("Execute", mock.Anything, mock.AnythingOfType("input.ListCards")).
		Return(output.CardList{Items: []output.Card{contractCard()}, NextCursor: nil}, nil).Once()

	rr, _ := s.execAndValidate(s.T(), http.MethodGet, "/api/v1/cards", "", s.authHeaders())

	s.Equal(http.StatusOK, rr.Code)
	m := s.decodeMap(rr)
	s.Contains(m, "items", "CardList deve conter chave snake_case 'items'")
	s.Contains(m, "next_cursor", "CardList deve conter chave snake_case 'next_cursor'")

	items, ok := m["items"].([]any)
	s.Require().True(ok, "items deve ser array")
	s.Require().NotEmpty(items, "items deve ter pelo menos um Card")
	first, ok := items[0].(map[string]any)
	s.Require().True(ok, "primeiro item deve ser object")
	for _, k := range []string{"id", "user_id", "closing_day", "due_day", "created_at", "updated_at"} {
		s.Contains(first, k, "campo snake_case %q deve existir no Card listado", k)
	}
}

func (s *ContractOpenAPISuite) TestContract_GetCard_RealValidation() {
	s.getUC.On("Execute", mock.Anything, mock.AnythingOfType("input.GetCard")).
		Return(contractCard(), nil).Once()

	path := "/api/v1/cards/" + contractCardID
	rr, _ := s.execAndValidate(s.T(), http.MethodGet, path, "", s.authHeaders())

	s.Equal(http.StatusOK, rr.Code)
	m := s.decodeMap(rr)
	for _, k := range []string{"id", "user_id", "closing_day", "due_day", "created_at", "updated_at"} {
		s.Contains(m, k)
	}
}

func (s *ContractOpenAPISuite) TestContract_PutCard_RealValidation() {
	s.idemStorage.On("Get", mock.Anything, "card", "idem-oa-put-001", contractUserID).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()
	s.updateUC.On("Execute", mock.Anything, mock.AnythingOfType("input.UpdateCard")).
		Return(contractUpdatedCard(), nil).Once()

	body := `{"nickname":"Nu Gold","bank":"nubank","due_day":27}`
	path := "/api/v1/cards/" + contractCardID
	rr, _ := s.execAndValidate(s.T(), http.MethodPut, path, body, s.mutHeaders("idem-oa-put-001"))

	s.Equal(http.StatusOK, rr.Code)
	m := s.decodeMap(rr)
	for _, k := range []string{"id", "user_id", "closing_day", "due_day", "created_at", "updated_at"} {
		s.Contains(m, k)
	}
}

func (s *ContractOpenAPISuite) TestContract_DeleteCard_RealValidation() {
	s.idemStorage.On("Get", mock.Anything, "card", "idem-oa-del-001", contractUserID).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()
	s.deleteUC.On("Execute", mock.Anything, mock.AnythingOfType("input.SoftDeleteCard")).
		Return(nil).Once()

	path := "/api/v1/cards/" + contractCardID
	rr, _ := s.execAndValidate(s.T(), http.MethodDelete, path, "", s.mutHeaders("idem-oa-del-001"))

	s.Equal(http.StatusNoContent, rr.Code)
	s.Empty(bytes.TrimSpace(rr.Body.Bytes()), "204 deve ter body vazio")
}

func (s *ContractOpenAPISuite) TestContract_GetBestPurchaseDay_RealValidation() {
	s.bestPurchaseUC.On("Execute", mock.Anything, mock.AnythingOfType("input.BestPurchaseDay")).
		Return(output.BestPurchaseDay{ClosingDay: 13, BestPurchaseDay: 14}, nil).Once()

	path := "/api/v1/cards/best-purchase-day?bank=nubank&due_day=20"
	rr, _ := s.execAndValidate(s.T(), http.MethodGet, path, "", s.authHeaders())

	s.Equal(http.StatusOK, rr.Code)
	m := s.decodeMap(rr)
	s.Contains(m, "closing_day", "BestPurchaseDayResponse deve conter 'closing_day'")
	s.Contains(m, "best_purchase_day", "BestPurchaseDayResponse deve conter 'best_purchase_day'")
	closingDay, _ := m["closing_day"].(float64)
	bestDay, _ := m["best_purchase_day"].(float64)
	s.Equal(float64(13), closingDay, "Nubank/20 deve retornar closing_day=13")
	s.Equal(float64(14), bestDay, "Nubank/20 deve retornar best_purchase_day=14")
}

func (s *ContractOpenAPISuite) TestContract_GetInvoices_RealValidation() {
	s.invoiceUC.On("Execute", mock.Anything, mock.AnythingOfType("input.InvoiceFor")).
		Return(contractInvoice(), nil).Once()

	path := "/api/v1/cards/" + contractCardID + "/invoices?for=2026-01-10"
	rr, _ := s.execAndValidate(s.T(), http.MethodGet, path, "", s.authHeaders())

	s.Equal(http.StatusOK, rr.Code)
	m := s.decodeMap(rr)
	s.Contains(m, "closing_date", "Invoice deve conter 'closing_date'")
	s.Contains(m, "due_date", "Invoice deve conter 'due_date'")

	dateRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	closingDate, _ := m["closing_date"].(string)
	dueDate, _ := m["due_date"].(string)
	s.Regexp(dateRe, closingDate, "closing_date deve estar em YYYY-MM-DD")
	s.Regexp(dateRe, dueDate, "due_date deve estar em YYYY-MM-DD")

	_, err := time.Parse("2006-01-02", closingDate)
	s.NoError(err)
	_, err = time.Parse("2006-01-02", dueDate)
	s.NoError(err)
}

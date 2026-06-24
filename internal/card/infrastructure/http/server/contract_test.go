package server_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	idemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

var (
	contractCardID   = "00000000-0000-4000-8000-000000000001"
	contractUserID   = "00000000-0000-4000-8000-000000000002"
	contractCreated  = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	contractUpdated  = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	contractUpdated2 = time.Date(2026, 1, 16, 10, 30, 0, 0, time.UTC)
)

func contractCard() output.Card {
	return output.Card{
		ID:         contractCardID,
		UserID:     contractUserID,
		Name:       "Nubank",
		Nickname:   "Nu",
		ClosingDay: 15,
		DueDay:     22,
		CreatedAt:  contractCreated,
		UpdatedAt:  contractUpdated,
	}
}

func contractUpdatedCard() output.Card {
	return output.Card{
		ID:         contractCardID,
		UserID:     contractUserID,
		Name:       "Nubank Gold",
		Nickname:   "Nu Gold",
		ClosingDay: 20,
		DueDay:     27,
		CreatedAt:  contractCreated,
		UpdatedAt:  contractUpdated2,
	}
}

func contractInvoice() output.Invoice {
	return output.Invoice{
		ClosingDate: "2026-01-15",
		DueDate:     "2026-01-22",
	}
}

func hashBody(body string) string {
	h := sha256.Sum256([]byte(body))
	return hex.EncodeToString(h[:])
}

type ContractSuite struct {
	suite.Suite
	router      chi.Router
	idemStorage *idemocks.Storage
	createUC    *mockCreateCard
	listUC      *mockListCards
	getUC       *mockGetCard
	updateUC    *mockUpdateCard
	updateLimUC *mockUpdateCardLimit
	deleteUC    *mockSoftDeleteCard
	invoiceUC   *mockInvoiceFor
	goldenDir   string
}

func TestContract(t *testing.T) {
	suite.Run(t, new(ContractSuite))
}

func (s *ContractSuite) SetupTest() {
	_, file, _, _ := runtime.Caller(0)
	s.goldenDir = filepath.Join(filepath.Dir(file), "testdata", "golden")

	o11y := noop.NewProvider()
	s.idemStorage = idemocks.NewStorage(s.T())
	s.createUC = &mockCreateCard{}
	s.listUC = &mockListCards{}
	s.getUC = &mockGetCard{}
	s.updateUC = &mockUpdateCard{}
	s.updateLimUC = &mockUpdateCardLimit{}
	s.deleteUC = &mockSoftDeleteCard{}
	s.invoiceUC = &mockInvoiceFor{}

	createH := handlers.NewCreateCardHandler(s.createUC, o11y)
	listH := handlers.NewListCardsHandler(s.listUC, o11y)
	getH := handlers.NewGetCardHandler(s.getUC, o11y)
	updateH := handlers.NewUpdateCardHandler(s.updateUC, o11y)
	updateLimH := handlers.NewUpdateCardLimitHandler(s.updateLimUC, o11y)
	deleteH := handlers.NewDeleteCardHandler(s.deleteUC, o11y)
	invoiceH := handlers.NewInvoiceForHandler(s.invoiceUC, o11y)

	passthrough := func(next http.Handler) http.Handler { return next }
	cardRouter := server.NewCardRouter(createH, listH, getH, updateH, updateLimH, deleteH, invoiceH, s.idemStorage, o11y, passthrough, passthrough)
	r := chi.NewRouter()
	cardRouter.Register(r)
	s.router = r
}

func (s *ContractSuite) readGolden(name string) []byte {
	data, err := os.ReadFile(filepath.Join(s.goldenDir, name))
	s.Require().NoError(err, "golden %s deve existir", name)
	return data
}

func (s *ContractSuite) normalizeJSON(data []byte) string {
	if len(bytes.TrimSpace(data)) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func (s *ContractSuite) assertJSONEqual(goldenFile string, actual []byte) {
	golden := s.readGolden(goldenFile)
	s.Equal(
		s.normalizeJSON(golden),
		s.normalizeJSON(actual),
		"response deve bater com golden %s", goldenFile,
	)
}

func (s *ContractSuite) doRequest(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	return rr
}

func (s *ContractSuite) authHeaders() map[string]string {
	return map[string]string{"X-User-ID": contractUserID}
}

func (s *ContractSuite) mutHeaders(idemKey string) map[string]string {
	return map[string]string{
		"X-User-ID":       contractUserID,
		"Idempotency-Key": idemKey,
	}
}

func (s *ContractSuite) TestContract_PostCards_201() {
	s.idemStorage.On("Get", mock.Anything, "card", "idem-post-001", contractUserID).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	s.createUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.CreateCard) bool {
		return in.Name == "Nubank" && in.Nickname == "Nu" && in.ClosingDay == 15 && in.DueDay != nil && *in.DueDay == 22
	})).Return(contractCard(), nil).Once()

	body := `{"name":"Nubank","nickname":"Nu","closing_day":15,"due_day":22}`
	rr := s.doRequest(http.MethodPost, "/api/v1/cards", body, s.mutHeaders("idem-post-001"))

	s.Equal(http.StatusCreated, rr.Code, "POST /api/v1/cards deve retornar 201")
	s.Contains(rr.Header().Get("Location"), contractCardID)
	s.assertJSONEqual("post_cards_201.json", rr.Body.Bytes())
}

func (s *ContractSuite) TestContract_GetCards_200() {
	s.listUC.On("Execute", mock.Anything, mock.AnythingOfType("input.ListCards")).
		Return(output.CardList{Items: []output.Card{contractCard()}, NextCursor: nil}, nil).Once()

	rr := s.doRequest(http.MethodGet, "/api/v1/cards", "", s.authHeaders())

	s.Equal(http.StatusOK, rr.Code, "GET /api/v1/cards deve retornar 200")
	s.assertJSONEqual("get_cards_200.json", rr.Body.Bytes())
}

func (s *ContractSuite) TestContract_GetCard_200() {
	s.getUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.GetCard) bool {
		return in.ID.String() == contractCardID
	})).Return(contractCard(), nil).Once()

	path := "/api/v1/cards/" + contractCardID
	rr := s.doRequest(http.MethodGet, path, "", s.authHeaders())

	s.Equal(http.StatusOK, rr.Code, "GET /api/v1/cards/{id} deve retornar 200")
	s.assertJSONEqual("get_card_200.json", rr.Body.Bytes())
}

func (s *ContractSuite) TestContract_GetCard_404() {
	s.getUC.On("Execute", mock.Anything, mock.Anything).
		Return(output.Card{}, domain.ErrCardNotFound).Once()

	path := "/api/v1/cards/" + uuid.New().String()
	rr := s.doRequest(http.MethodGet, path, "", s.authHeaders())

	s.Equal(http.StatusNotFound, rr.Code, "GET /api/v1/cards/{id} deve retornar 404 se não encontrado")
	s.assertJSONEqual("get_card_404.json", rr.Body.Bytes())
}

func (s *ContractSuite) TestContract_PutCard_200() {
	s.idemStorage.On("Get", mock.Anything, "card", "idem-put-001", contractUserID).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	s.updateUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.UpdateCard) bool {
		return in.ID.String() == contractCardID
	})).Return(contractUpdatedCard(), nil).Once()

	body := `{"name":"Nubank Gold","nickname":"Nu Gold","closing_day":20,"due_day":27}`
	path := "/api/v1/cards/" + contractCardID
	rr := s.doRequest(http.MethodPut, path, body, s.mutHeaders("idem-put-001"))

	s.Equal(http.StatusOK, rr.Code, "PUT /api/v1/cards/{id} deve retornar 200")
	s.assertJSONEqual("put_card_200.json", rr.Body.Bytes())
}

func (s *ContractSuite) TestContract_DeleteCard_204() {
	s.idemStorage.On("Get", mock.Anything, "card", "idem-del-001", contractUserID).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	s.deleteUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.SoftDeleteCard) bool {
		return in.ID.String() == contractCardID
	})).Return(nil).Once()

	path := "/api/v1/cards/" + contractCardID
	rr := s.doRequest(http.MethodDelete, path, "", s.mutHeaders("idem-del-001"))

	s.Equal(http.StatusNoContent, rr.Code, "DELETE /api/v1/cards/{id} deve retornar 204")
	s.Empty(bytes.TrimSpace(rr.Body.Bytes()), "DELETE deve retornar body vazio (204)")
}

func (s *ContractSuite) TestContract_GetInvoices_200() {
	s.invoiceUC.On("Execute", mock.Anything, mock.MatchedBy(func(in input.InvoiceFor) bool {
		return in.CardID.String() == contractCardID
	})).Return(contractInvoice(), nil).Once()

	path := "/api/v1/cards/" + contractCardID + "/invoices?for=2026-01-10"
	rr := s.doRequest(http.MethodGet, path, "", s.authHeaders())

	s.Equal(http.StatusOK, rr.Code, "GET /api/v1/cards/{id}/invoices deve retornar 200")
	s.assertJSONEqual("get_invoices_200.json", rr.Body.Bytes())
}

func (s *ContractSuite) TestContract_Replay_PostCards_201_ByteIdentical() {
	cardJSON, err := json.Marshal(contractCard())
	s.Require().NoError(err)

	reqBody := `{"name":"Nubank","nickname":"Nu","closing_day":15,"due_day":22}`

	storedRecord := idempotency.Record{
		Scope:          "card",
		Key:            "idem-replay-001",
		UserID:         contractUserID,
		RequestHash:    hashBody(reqBody),
		ResponseStatus: http.StatusCreated,
		ResponseBody:   cardJSON,
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}

	s.idemStorage.On("Get", mock.Anything, "card", "idem-replay-001", contractUserID).
		Return(storedRecord, nil).Once()

	rr := s.doRequest(http.MethodPost, "/api/v1/cards", reqBody, s.mutHeaders("idem-replay-001"))

	s.Equal(http.StatusCreated, rr.Code, "replay deve retornar 201")
	s.assertJSONEqual("replay_post_201.json", rr.Body.Bytes())
}

func (s *ContractSuite) TestContract_NoXUserID_Returns401() {
	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/cards", ""},
		{http.MethodPost, "/api/v1/cards", `{"name":"X","nickname":"X","closing_day":1,"due_day":2}`},
		{http.MethodGet, "/api/v1/cards/" + uuid.New().String(), ""},
		{http.MethodPut, "/api/v1/cards/" + uuid.New().String(), `{"name":"X"}`},
		{http.MethodDelete, "/api/v1/cards/" + uuid.New().String(), ""},
		{http.MethodGet, "/api/v1/cards/" + uuid.New().String() + "/invoices?for=2026-01-01", ""},
	}

	for _, ep := range endpoints {
		rr := s.doRequest(ep.method, ep.path, ep.body, nil)
		s.Equal(http.StatusUnauthorized, rr.Code, "esperado 401 para %s %s sem X-User-ID", ep.method, ep.path)
	}
}

func (s *ContractSuite) TestContract_MissingIdempotencyKey_Returns400() {
	mutations := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/cards", `{"name":"X","nickname":"X","closing_day":1,"due_day":2}`},
		{http.MethodPut, "/api/v1/cards/" + uuid.New().String(), `{"name":"X"}`},
		{http.MethodDelete, "/api/v1/cards/" + uuid.New().String(), ""},
	}

	for _, ep := range mutations {
		rr := s.doRequest(ep.method, ep.path, ep.body, s.authHeaders())
		s.Equal(http.StatusBadRequest, rr.Code, "esperado 400 para %s %s sem Idempotency-Key", ep.method, ep.path)
	}
}

func (s *ContractSuite) TestContract_PostCards_NicknameConflict_409() {
	s.idemStorage.On("Get", mock.Anything, "card", "idem-conflict-001", contractUserID).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	s.createUC.On("Execute", mock.Anything, mock.AnythingOfType("input.CreateCard")).
		Return(output.Card{}, domain.ErrNicknameConflict).Once()

	s.idemStorage.On("Put", mock.Anything, mock.MatchedBy(func(r idempotency.Record) bool {
		return r.ResponseStatus == 409
	})).Return(nil).Maybe()

	body := `{"name":"Nubank","nickname":"Nu","closing_day":15,"due_day":22}`
	rr := s.doRequest(http.MethodPost, "/api/v1/cards", body, s.mutHeaders("idem-conflict-001"))

	s.Equal(http.StatusConflict, rr.Code, "POST com apelido em uso deve retornar 409")
}

func (s *ContractSuite) TestContract_GetInvoices_BadForParam_400() {
	path := "/api/v1/cards/" + contractCardID + "/invoices?for=notadate"
	rr := s.doRequest(http.MethodGet, path, "", s.authHeaders())
	s.Equal(http.StatusBadRequest, rr.Code, "data inválida deve retornar 400")
}

func (s *ContractSuite) TestContract_GetInvoices_MissingForParam_400() {
	path := "/api/v1/cards/" + contractCardID + "/invoices"
	rr := s.doRequest(http.MethodGet, path, "", s.authHeaders())
	s.Equal(http.StatusBadRequest, rr.Code, "param 'for' ausente deve retornar 400")
}

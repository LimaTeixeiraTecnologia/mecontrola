package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
)

type mockDispatcher struct {
	mock.Mock
}

func (m *mockDispatcher) Route(ctx context.Context, raw json.RawMessage) (dispatcher.RouteOutcome, error) {
	args := m.Called(ctx, raw)
	return args.Get(0).(dispatcher.RouteOutcome), args.Error(1)
}

type InboundHandlerSuite struct {
	suite.Suite
}

func TestInboundHandlerSuite(t *testing.T) {
	suite.Run(t, new(InboundHandlerSuite))
}

func withRawBody(h http.Handler) http.Handler {
	return signature.RawBody(h)
}

func (s *InboundHandlerSuite) TestHandle_ValidPayload_Returns200() {
	d := new(mockDispatcher)
	h := handlers.NewInboundHandler(d, noop.NewProvider())

	body := []byte(`{"object":"whatsapp_business_account"}`)
	d.On("Route", mock.Anything, json.RawMessage(body)).Return(dispatcher.OutcomeAgent, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/whatsapp/inbound", bytes.NewReader(body))
	w := httptest.NewRecorder()

	withRawBody(http.HandlerFunc(h.Handle)).ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)
	d.AssertExpectations(s.T())
}

func (s *InboundHandlerSuite) TestHandle_MissingRawBody_Returns500() {
	d := new(mockDispatcher)
	h := handlers.NewInboundHandler(d, noop.NewProvider())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/whatsapp/inbound", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req)

	s.Equal(http.StatusInternalServerError, w.Code)
	d.AssertNotCalled(s.T(), "Route")
}

func (s *InboundHandlerSuite) TestHandle_DispatcherInvalidOutcome_Returns200() {
	d := new(mockDispatcher)
	h := handlers.NewInboundHandler(d, noop.NewProvider())

	body := []byte(`{"object":"whatsapp_business_account"}`)
	d.On("Route", mock.Anything, json.RawMessage(body)).Return(dispatcher.OutcomeInvalid, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/whatsapp/inbound", bytes.NewReader(body))
	w := httptest.NewRecorder()

	withRawBody(http.HandlerFunc(h.Handle)).ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)
}

func (s *InboundHandlerSuite) TestHandle_DuplicateOutcome_Returns200() {
	d := new(mockDispatcher)
	h := handlers.NewInboundHandler(d, noop.NewProvider())

	body := []byte(`{"object":"whatsapp_business_account"}`)
	d.On("Route", mock.Anything, json.RawMessage(body)).Return(dispatcher.OutcomeDuplicate, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/whatsapp/inbound", bytes.NewReader(body))
	w := httptest.NewRecorder()

	withRawBody(http.HandlerFunc(h.Handle)).ServeHTTP(w, req)

	s.Equal(http.StatusOK, w.Code)
}

func (s *InboundHandlerSuite) TestHandle_DispatcherReturnsError_Returns503() {
	d := new(mockDispatcher)
	h := handlers.NewInboundHandler(d, noop.NewProvider())

	body := []byte(`{"object":"whatsapp_business_account"}`)
	d.On("Route", mock.Anything, json.RawMessage(body)).Return(dispatcher.OutcomeInvalid, errors.New("pg unavailable"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/whatsapp/inbound", bytes.NewReader(body))
	w := httptest.NewRecorder()

	withRawBody(http.HandlerFunc(h.Handle)).ServeHTTP(w, req)

	s.Equal(http.StatusServiceUnavailable, w.Code)
}

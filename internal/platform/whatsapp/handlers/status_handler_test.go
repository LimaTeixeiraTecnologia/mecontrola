package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status"
)

type fakeStatusRecorder struct {
	called bool
	count  int
	err    error
}

func (f *fakeStatusRecorder) Execute(_ context.Context, statuses []status.MessageStatus) error {
	f.called = true
	f.count = len(statuses)
	return f.err
}

type StatusHandlerSuite struct {
	suite.Suite
}

func TestStatusHandlerSuite(t *testing.T) {
	suite.Run(t, new(StatusHandlerSuite))
}

func (s *StatusHandlerSuite) serve(h *handlers.StatusHandler, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/whatsapp/status", bytes.NewReader(body))
	w := httptest.NewRecorder()
	signature.RawBody(http.HandlerFunc(h.Handle)).ServeHTTP(w, req)
	return w
}

func (s *StatusHandlerSuite) TestHandle_ValidStatus_Returns200AndPersists() {
	rec := &fakeStatusRecorder{}
	h := handlers.NewStatusHandler(rec, noop.NewProvider())

	body := []byte(`{"entry":[{"id":"e1","changes":[{"field":"messages","value":{"statuses":[{"id":"w1","status":"delivered","timestamp":"1686000000","recipient_id":"5511"}]}}]}]}`)
	w := s.serve(h, body)

	s.Equal(http.StatusOK, w.Code)
	s.True(rec.called)
	s.Equal(1, rec.count)
}

func (s *StatusHandlerSuite) TestHandle_InvalidPayload_Returns200WithoutPersisting() {
	rec := &fakeStatusRecorder{}
	h := handlers.NewStatusHandler(rec, noop.NewProvider())

	w := s.serve(h, []byte(`not-json`))

	s.Equal(http.StatusOK, w.Code)
	s.False(rec.called)
}

func (s *StatusHandlerSuite) TestHandle_EmptyStatuses_Returns200WithoutPersisting() {
	rec := &fakeStatusRecorder{}
	h := handlers.NewStatusHandler(rec, noop.NewProvider())

	w := s.serve(h, []byte(`{"entry":[]}`))

	s.Equal(http.StatusOK, w.Code)
	s.False(rec.called)
}

func (s *StatusHandlerSuite) TestHandle_RecorderError_Returns503() {
	rec := &fakeStatusRecorder{err: errors.New("pg down")}
	h := handlers.NewStatusHandler(rec, noop.NewProvider())

	body := []byte(`{"entry":[{"id":"e1","changes":[{"field":"messages","value":{"statuses":[{"id":"w1","status":"sent","timestamp":"1"}]}}]}]}`)
	w := s.serve(h, body)

	s.Equal(http.StatusServiceUnavailable, w.Code)
	s.True(rec.called)
}

func (s *StatusHandlerSuite) TestHandle_MissingRawBody_Returns500() {
	rec := &fakeStatusRecorder{}
	h := handlers.NewStatusHandler(rec, noop.NewProvider())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/whatsapp/status", nil)
	w := httptest.NewRecorder()
	h.Handle(w, req)

	s.Equal(http.StatusInternalServerError, w.Code)
	s.False(rec.called)
}

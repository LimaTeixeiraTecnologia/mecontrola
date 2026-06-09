package signature_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
)

type RawBodyBufferSuite struct {
	suite.Suite
}

func TestRawBodyBufferSuite(t *testing.T) {
	suite.Run(t, new(RawBodyBufferSuite))
}

func (s *RawBodyBufferSuite) TestRawBody_StoresBodyInContext() {
	body := `{"object":"whatsapp_business_account"}`
	var capturedRaw []byte
	var capturedOK bool

	handler := signature.RawBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRaw, capturedOK = signature.RawBodyFromContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.True(capturedOK)
	s.Equal([]byte(body), capturedRaw)
}

func (s *RawBodyBufferSuite) TestRawBody_RejectsPayloadTooLarge() {
	// payload slightly over 256 KiB
	large := strings.Repeat("x", 256*1024+1)
	handler := signature.RawBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(large))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusRequestEntityTooLarge, rr.Code)
}

func (s *RawBodyBufferSuite) TestRawBodyFromContext_MissingReturnsNotOK() {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", nil)
	raw, ok := signature.RawBodyFromContext(req)
	s.False(ok)
	s.Nil(raw)
}

package signature_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
)

type HMACSuite struct {
	suite.Suite
}

func TestHMACSuite(t *testing.T) {
	suite.Run(t, new(HMACSuite))
}

func (s *HMACSuite) SetupTest() {}

func (s *HMACSuite) buildSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write(body)
	s.Require().NoError(err)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (s *HMACSuite) buildHandler(secretCurrent, secretNext string, statusCode int) http.Handler {
	return signature.RawBody(
		signature.HMAC(secretCurrent, secretNext)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Meta-Sig-Status", signature.StatusFromContext(r))
				w.WriteHeader(statusCode)
			}),
		),
	)
}

func (s *HMACSuite) TestHMAC() {
	payload := []byte(`{"object":"whatsapp_business_account"}`)

	scenarios := []struct {
		name   string
		header string
		setup  func() http.Handler
		expect func(*httptest.ResponseRecorder)
	}{
		{
			name:   "deve aceitar assinatura valida",
			header: s.buildSignature(payload, "secret-current"),
			setup: func() http.Handler {
				return s.buildHandler("secret-current", "", http.StatusOK)
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, recorder.Code)
				s.Equal(signature.StatusValid, recorder.Header().Get("X-Meta-Sig-Status"))
			},
		},
		{
			name:   "deve rejeitar assinatura invalida",
			header: "sha256=invalidsig",
			setup: func() http.Handler {
				return s.buildHandler("secret-current", "", http.StatusOK)
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:   "deve rejeitar header ausente",
			header: "",
			setup: func() http.Handler {
				return s.buildHandler("secret-current", "", http.StatusOK)
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:   "deve aceitar secret rotacionado",
			header: s.buildSignature(payload, "secret-next"),
			setup: func() http.Handler {
				return s.buildHandler("secret-current", "secret-next", http.StatusOK)
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, recorder.Code)
				s.Equal(signature.StatusRotated, recorder.Header().Get("X-Meta-Sig-Status"))
			},
		},
		{
			name:   "deve rejeitar assinatura adulterada",
			header: s.buildSignature(payload, "secret-current")[:len(s.buildSignature(payload, "secret-current"))-1] + "X",
			setup: func() http.Handler {
				return s.buildHandler("secret-current", "", http.StatusOK)
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:   "deve rejeitar prefixo incorreto",
			header: strings.TrimPrefix(s.buildSignature(payload, "secret-current"), "sha256="),
			setup: func() http.Handler {
				return s.buildHandler("secret-current", "", http.StatusOK)
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			request := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
			if scenario.header != "" {
				request.Header.Set("X-Hub-Signature-256", scenario.header)
			}
			recorder := httptest.NewRecorder()
			scenario.setup().ServeHTTP(recorder, request)
			scenario.expect(recorder)
		})
	}
}

package middleware_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
)

const (
	testSecret     = "secret-current"
	testSecretNext = "secret-next"
)

type HmacSignatureSuite struct {
	suite.Suite
}

func TestHmacSignatureSuite(t *testing.T) {
	suite.Run(t, new(HmacSignatureSuite))
}

func (s *HmacSignatureSuite) SetupTest() {}

func buildSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func newHandler(secretCurrent, secretNext string) http.Handler {
	return middleware.RawBody(
		middleware.HMACSignature(secretCurrent, secretNext)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Sig-Status", middleware.SignatureStatusFromContext(r))
				w.WriteHeader(http.StatusAccepted)
			}),
		),
	)
}

func (s *HmacSignatureSuite) TestHMACSignature() {
	payload := []byte(`{"trigger":"compra_aprovada"}`)
	scenarios := []struct {
		name           string
		path           string
		headerValue    string
		secretCurrent  string
		secretNext     string
		expectedStatus string
	}{
		{
			name:           "deve validar assinatura pelo header",
			path:           "/",
			headerValue:    buildSignature(payload, testSecret),
			secretCurrent:  testSecret,
			expectedStatus: middleware.SignatureStatusValid,
		},
		{
			name:           "deve marcar assinatura invalida",
			path:           "/",
			headerValue:    "invalidsignature",
			secretCurrent:  testSecret,
			expectedStatus: middleware.SignatureStatusInvalid,
		},
		{
			name:           "deve aceitar segredo rotacionado",
			path:           "/",
			headerValue:    buildSignature(payload, testSecretNext),
			secretCurrent:  testSecret,
			secretNext:     testSecretNext,
			expectedStatus: middleware.SignatureStatusRotated,
		},
		{
			name:           "deve usar query string como fallback",
			path:           "/?signature=" + buildSignature(payload, testSecret),
			secretCurrent:  testSecret,
			expectedStatus: middleware.SignatureStatusValid,
		},
		{
			name:           "deve priorizar header sobre query string",
			path:           "/?signature=invalidsignature",
			headerValue:    buildSignature(payload, testSecret),
			secretCurrent:  testSecret,
			expectedStatus: middleware.SignatureStatusValid,
		},
		{
			name: "deve comparar em tempo constante para assinatura alterada",
			path: "/",
			headerValue: func() string {
				realSig := buildSignature(payload, testSecret)
				tampered := realSig[:len(realSig)-1] + "X"
				if tampered == realSig {
					return realSig[:len(realSig)-1] + "Y"
				}
				return tampered
			}(),
			secretCurrent:  testSecret,
			expectedStatus: middleware.SignatureStatusInvalid,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			handler := newHandler(scenario.secretCurrent, scenario.secretNext)
			req := httptest.NewRequest(http.MethodPost, scenario.path, strings.NewReader(string(payload)))
			if scenario.headerValue != "" {
				req.Header.Set("X-Kiwify-Signature", scenario.headerValue)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(s.T(), http.StatusAccepted, rr.Code)
			assert.Equal(s.T(), scenario.expectedStatus, rr.Header().Get("X-Sig-Status"))
		})
	}
}

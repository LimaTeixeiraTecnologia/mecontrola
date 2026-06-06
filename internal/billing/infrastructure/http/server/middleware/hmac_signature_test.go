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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
)

const (
	testSecret     = "secret-current"
	testSecretNext = "secret-next"
)

func buildSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func buildHMACMiddlewareChain(secretCurrent, secretNext string) http.Handler {
	body := []byte(`{"trigger":"compra_aprovada"}`)
	inner := middleware.RawBody(
		middleware.HMACSignature(secretCurrent, secretNext)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = body
				w.Header().Set("X-Sig-Status", middleware.SignatureStatusFromContext(r))
				w.WriteHeader(http.StatusAccepted)
			}),
		),
	)
	return inner
}

func TestHMACSignature_ValidSignature(t *testing.T) {
	payload := []byte(`{"trigger":"compra_aprovada"}`)
	sig := buildSignature(payload, testSecret)

	handler := middleware.RawBody(
		middleware.HMACSignature(testSecret, "")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Sig-Status", middleware.SignatureStatusFromContext(r))
				w.WriteHeader(http.StatusAccepted)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("X-Kiwify-Signature", sig)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.Equal(t, middleware.SignatureStatusValid, rr.Header().Get("X-Sig-Status"))
}

func TestHMACSignature_InvalidSignature(t *testing.T) {
	payload := []byte(`{"trigger":"compra_aprovada"}`)

	handler := middleware.RawBody(
		middleware.HMACSignature(testSecret, "")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("next handler must not be called for invalid signature")
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("X-Kiwify-Signature", "invalidsignature")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHMACSignature_RotatedSecret(t *testing.T) {
	payload := []byte(`{"trigger":"subscription_renewed"}`)
	sig := buildSignature(payload, testSecretNext)

	handler := middleware.RawBody(
		middleware.HMACSignature(testSecret, testSecretNext)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Sig-Status", middleware.SignatureStatusFromContext(r))
				w.WriteHeader(http.StatusAccepted)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("X-Kiwify-Signature", sig)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.Equal(t, middleware.SignatureStatusRotated, rr.Header().Get("X-Sig-Status"))
}

func TestHMACSignature_FallbackQueryParam(t *testing.T) {
	payload := []byte(`{"trigger":"compra_aprovada"}`)
	sig := buildSignature(payload, testSecret)

	handler := middleware.RawBody(
		middleware.HMACSignature(testSecret, "")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Sig-Status", middleware.SignatureStatusFromContext(r))
				w.WriteHeader(http.StatusAccepted)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/?signature="+sig, strings.NewReader(string(payload)))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.Equal(t, middleware.SignatureStatusValid, rr.Header().Get("X-Sig-Status"))
}

func TestHMACSignature_ConstantTimeComparison(t *testing.T) {
	payload := []byte(`{"trigger":"compra_aprovada"}`)
	realSig := buildSignature(payload, testSecret)

	tampered := realSig[:len(realSig)-1] + "X"

	handler := middleware.RawBody(
		middleware.HMACSignature(testSecret, "")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("next handler must not be called for tampered signature")
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("X-Kiwify-Signature", tampered)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

var _ = buildHMACMiddlewareChain

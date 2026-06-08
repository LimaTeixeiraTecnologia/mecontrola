package middleware_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
)

func buildMetaSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func buildMetaChain(secretCurrent, secretNext string, statusCode int) http.Handler {
	return middleware.RawBody(
		middleware.MetaSignature(secretCurrent, secretNext)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Meta-Sig-Status", middleware.MetaSignatureStatusFromContext(r))
				w.WriteHeader(statusCode)
			}),
		),
	)
}

func TestMetaSignature_ValidSignature(t *testing.T) {
	payload := []byte(`{"object":"whatsapp_business_account"}`)
	sig := buildMetaSignature(payload, "secret-current")

	handler := buildMetaChain("secret-current", "", http.StatusOK)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", sig)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, middleware.MetaSignatureStatusValid, rr.Header().Get("X-Meta-Sig-Status"))
}

func TestMetaSignature_InvalidSignature(t *testing.T) {
	payload := []byte(`{"object":"whatsapp_business_account"}`)

	handler := buildMetaChain("secret-current", "", http.StatusOK)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsig")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMetaSignature_MissingHeader(t *testing.T) {
	payload := []byte(`{"object":"whatsapp_business_account"}`)

	handler := buildMetaChain("secret-current", "", http.StatusOK)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMetaSignature_RotatedSecret(t *testing.T) {
	payload := []byte(`{"object":"whatsapp_business_account"}`)
	sig := buildMetaSignature(payload, "secret-next")

	handler := buildMetaChain("secret-current", "secret-next", http.StatusOK)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", sig)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, middleware.MetaSignatureStatusRotated, rr.Header().Get("X-Meta-Sig-Status"))
}

func TestMetaSignature_TamperedSignature(t *testing.T) {
	payload := []byte(`{"object":"whatsapp_business_account"}`)
	realSig := buildMetaSignature(payload, "secret-current")
	tampered := realSig[:len(realSig)-1] + "X"

	handler := buildMetaChain("secret-current", "", http.StatusOK)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", tampered)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMetaSignature_WrongPrefix(t *testing.T) {
	payload := []byte(`{"object":"whatsapp_business_account"}`)
	mac := hmac.New(sha256.New, []byte("secret-current"))
	mac.Write(payload)
	hexOnly := hex.EncodeToString(mac.Sum(nil))

	handler := buildMetaChain("secret-current", "", http.StatusOK)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", hexOnly)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

package signature_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/signature"
)

func newReq(body, token string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/telegram/webhook", strings.NewReader(body))
	if token != "" {
		req.Header.Set(signature.HeaderSecretToken, token)
	}
	return req
}

func TestSecretToken_Valid(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, signature.StatusValid, signature.StatusFromContext(r))
		raw, ok := signature.RawBodyFromContext(r)
		require.True(t, ok)
		assert.Equal(t, "ok", string(raw))
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	signature.SecretToken("super-secret-current", "super-secret-next")(next).ServeHTTP(rec, newReq("ok", "super-secret-current"))

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSecretToken_Rotated(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, signature.StatusRotated, signature.StatusFromContext(r))
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	signature.SecretToken("super-secret-current", "super-secret-next")(next).ServeHTTP(rec, newReq("ok", "super-secret-next"))

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSecretToken_MissingHeader_Rejected(t *testing.T) {
	called := false
	onInvalid := false
	onStatus := ""

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	mw := signature.SecretTokenWithMetrics(
		"super-secret-current", "",
		func() { onInvalid = true },
		func(status string) { onStatus = status },
	)

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, newReq("payload", ""))

	assert.False(t, called)
	assert.True(t, onInvalid)
	assert.Equal(t, signature.StatusInvalid, onStatus)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSecretToken_WrongHeader_Rejected(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	rec := httptest.NewRecorder()
	signature.SecretToken("super-secret-current", "")(next).ServeHTTP(rec, newReq("payload", "wrong-token"))

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSecretToken_PayloadTooLarge(t *testing.T) {
	big := bytes.Repeat([]byte("a"), 2<<20)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/telegram/webhook", io.NopCloser(bytes.NewReader(big)))
	req.Header.Set(signature.HeaderSecretToken, "super-secret-current")

	rec := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	signature.SecretToken("super-secret-current", "")(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestStatusFromContext_DefaultsInvalid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	assert.Equal(t, signature.StatusInvalid, signature.StatusFromContext(req))
}

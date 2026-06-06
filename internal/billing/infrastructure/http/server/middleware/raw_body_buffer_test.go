package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
)

func TestRawBody_StoresBodyInContext(t *testing.T) {
	payload := `{"trigger":"compra_aprovada"}`
	var captured []byte

	handler := middleware.RawBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := middleware.RawBodyFromContext(r)
		require.True(t, ok)
		captured = raw
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []byte(payload), captured)
}

func TestRawBody_RejectsBodyExceedingLimit(t *testing.T) {
	big := bytes.Repeat([]byte("x"), 256*1024+1)

	handler := middleware.RawBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler must not be called for oversized body")
	}))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(big))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}

func TestRawBody_AcceptsBodyAtExactLimit(t *testing.T) {
	exact := bytes.Repeat([]byte("x"), 256*1024)
	var captured []byte

	handler := middleware.RawBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := middleware.RawBodyFromContext(r)
		require.True(t, ok)
		captured = raw
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(exact))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Len(t, captured, 256*1024)
}

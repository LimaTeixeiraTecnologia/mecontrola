package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
)

type RawBodyBufferSuite struct {
	suite.Suite
}

func TestRawBodyBufferSuite(t *testing.T) {
	suite.Run(t, new(RawBodyBufferSuite))
}

func (s *RawBodyBufferSuite) SetupTest() {}

func (s *RawBodyBufferSuite) TestRawBody() {
	scenarios := []struct {
		name         string
		body         []byte
		bodyReader   func([]byte) *bytes.Reader
		expectStatus int
		expect       func(*httptest.ResponseRecorder, []byte)
	}{
		{
			name:         "deve armazenar o corpo no contexto",
			body:         []byte(`{"trigger":"order_approved"}`),
			bodyReader:   bytes.NewReader,
			expectStatus: http.StatusOK,
			expect: func(rr *httptest.ResponseRecorder, captured []byte) {
				assert.Equal(s.T(), []byte(`{"trigger":"order_approved"}`), captured)
			},
		},
		{
			name:         "deve rejeitar corpo acima do limite",
			body:         bytes.Repeat([]byte("x"), 256*1024+1),
			bodyReader:   bytes.NewReader,
			expectStatus: http.StatusRequestEntityTooLarge,
			expect: func(rr *httptest.ResponseRecorder, captured []byte) {
				assert.Nil(s.T(), captured)
			},
		},
		{
			name:         "deve aceitar corpo no limite exato",
			body:         bytes.Repeat([]byte("x"), 256*1024),
			bodyReader:   bytes.NewReader,
			expectStatus: http.StatusOK,
			expect: func(rr *httptest.ResponseRecorder, captured []byte) {
				assert.Len(s.T(), captured, 256*1024)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var captured []byte

			handler := middleware.RawBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, ok := middleware.RawBodyFromContext(r)
				require.True(s.T(), ok)
				captured = raw
				w.WriteHeader(http.StatusOK)
			}))

			if scenario.expectStatus == http.StatusRequestEntityTooLarge {
				handler = middleware.RawBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					s.T().Error("next handler must not be called for oversized body")
				}))
			}

			req := httptest.NewRequest(http.MethodPost, "/", scenario.bodyReader(scenario.body))
			if strings.HasPrefix(string(scenario.body), "{") {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(s.T(), scenario.expectStatus, rr.Code)
			scenario.expect(rr, captured)
		})
	}
}

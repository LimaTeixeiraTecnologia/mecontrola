package middleware_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
	middlewaremocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware/mocks"
)

const (
	testSecret = "00000000000000000000000000000001"
	testUserID = "aaaaaaaa-0000-0000-0000-000000000001"
	testWindow = 60 * time.Second
)

func makeHMAC(secret, userID, ts string) string {
	lower := strings.ToLower(userID)
	msg := []byte(lower + "." + ts)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(msg)
	return hex.EncodeToString(h.Sum(nil))
}

type RequireGatewayAuthSuite struct {
	suite.Suite
	o11y *fake.Provider
}

func TestRequireGatewayAuthSuite(t *testing.T) {
	suite.Run(t, new(RequireGatewayAuthSuite))
}

func (s *RequireGatewayAuthSuite) SetupTest() {
	s.o11y = fake.NewProvider()
}

func (s *RequireGatewayAuthSuite) buildDeps(logger *middlewaremocks.MockGatewayAuthFailureLogger) middleware.RequireGatewayAuthDeps {
	return middleware.RequireGatewayAuthDeps{
		Secrets: services.SecretPair{
			Current: []byte(testSecret),
			Next:    nil,
		},
		Window:        testWindow,
		FailureLogger: logger,
		O11y:          s.o11y,
	}
}

func (s *RequireGatewayAuthSuite) TestValid_CallsNext() {
	now := time.Now().UTC()
	ts := strconv.FormatInt(now.Unix(), 10)
	sig := makeHMAC(testSecret, testUserID, ts)

	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Gateway-Auth", sig)
	req.Header.Set("X-Gateway-Timestamp", ts)

	rr := httptest.NewRecorder()
	handler := middleware.RequireGatewayAuth(s.buildDeps(logger))(next)
	handler.ServeHTTP(rr, req)

	s.True(nextCalled)
	s.Equal(http.StatusOK, rr.Code)
}

func (s *RequireGatewayAuthSuite) TestRotated_CallsNext() {
	nextSecret := "00000000000000000000000000000002"

	now := time.Now().UTC()
	ts := strconv.FormatInt(now.Unix(), 10)
	sig := makeHMAC(nextSecret, testUserID, ts)

	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())

	deps := middleware.RequireGatewayAuthDeps{
		Secrets: services.SecretPair{
			Current: []byte(testSecret),
			Next:    []byte(nextSecret),
		},
		Window:        testWindow,
		FailureLogger: logger,
		O11y:          s.o11y,
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Gateway-Auth", sig)
	req.Header.Set("X-Gateway-Timestamp", ts)

	rr := httptest.NewRecorder()
	handler := middleware.RequireGatewayAuth(deps)(next)
	handler.ServeHTTP(rr, req)

	s.True(nextCalled)
	s.Equal(http.StatusOK, rr.Code)
}

func (s *RequireGatewayAuthSuite) TestMissingHeader_Returns401() {
	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())
	logger.EXPECT().Handle(mock.Anything, mock.MatchedBy(func(in input.RecordGatewayAuthFailureInput) bool {
		return in.Reason == "gateway_missing_header"
	})).Return(nil).Once()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rr := httptest.NewRecorder()

	handler := middleware.RequireGatewayAuth(s.buildDeps(logger))(next)
	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Equal(`{"error":"unauthorized"}`, rr.Body.String())
	s.Equal("application/json", rr.Header().Get("Content-Type"))
	s.Equal("no-store", rr.Header().Get("Cache-Control"))
}

func (s *RequireGatewayAuthSuite) TestStaleTimestamp_Returns401() {
	staleTs := strconv.FormatInt(time.Now().Add(-120*time.Second).Unix(), 10)
	sig := makeHMAC(testSecret, testUserID, staleTs)

	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())
	logger.EXPECT().Handle(mock.Anything, mock.MatchedBy(func(in input.RecordGatewayAuthFailureInput) bool {
		return in.Reason == "gateway_stale_timestamp"
	})).Return(nil).Once()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Gateway-Auth", sig)
	req.Header.Set("X-Gateway-Timestamp", staleTs)

	rr := httptest.NewRecorder()
	handler := middleware.RequireGatewayAuth(s.buildDeps(logger))(next)
	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Equal(`{"error":"unauthorized"}`, rr.Body.String())
	s.Equal("no-store", rr.Header().Get("Cache-Control"))
}

func (s *RequireGatewayAuthSuite) TestInvalidTimestamp_Returns401() {
	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())
	logger.EXPECT().Handle(mock.Anything, mock.MatchedBy(func(in input.RecordGatewayAuthFailureInput) bool {
		return in.Reason == "gateway_invalid_timestamp"
	})).Return(nil).Once()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Gateway-Auth", fmt.Sprintf("%064x", 0))
	req.Header.Set("X-Gateway-Timestamp", "not-a-number")

	rr := httptest.NewRecorder()
	handler := middleware.RequireGatewayAuth(s.buildDeps(logger))(next)
	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Equal(`{"error":"unauthorized"}`, rr.Body.String())
	s.Equal("no-store", rr.Header().Get("Cache-Control"))
}

func (s *RequireGatewayAuthSuite) TestInvalidSignature_Returns401() {
	now := time.Now().UTC()
	ts := strconv.FormatInt(now.Unix(), 10)
	badSig := fmt.Sprintf("%064x", 0)

	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())
	logger.EXPECT().Handle(mock.Anything, mock.MatchedBy(func(in input.RecordGatewayAuthFailureInput) bool {
		return in.Reason == "gateway_invalid_signature"
	})).Return(nil).Once()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Gateway-Auth", badSig)
	req.Header.Set("X-Gateway-Timestamp", ts)

	rr := httptest.NewRecorder()
	handler := middleware.RequireGatewayAuth(s.buildDeps(logger))(next)
	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Equal(`{"error":"unauthorized"}`, rr.Body.String())
	s.Equal("no-store", rr.Header().Get("Cache-Control"))
}

func (s *RequireGatewayAuthSuite) TestFailureLoggerError_StillReturns401() {
	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())
	logger.EXPECT().Handle(mock.Anything, mock.Anything).Return(errors.New("publish failed")).Once()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rr := httptest.NewRecorder()

	handler := middleware.RequireGatewayAuth(s.buildDeps(logger))(next)
	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Equal(`{"error":"unauthorized"}`, rr.Body.String())
}

func (s *RequireGatewayAuthSuite) TestMissingHeader_WithXFF_ExtractsLastIP() {
	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())
	logger.EXPECT().Handle(mock.Anything, mock.MatchedBy(func(in input.RecordGatewayAuthFailureInput) bool {
		return in.Reason == "gateway_missing_header" &&
			in.ClientIPRaw == "10.0.0.1" &&
			in.RequestID == "fake-trace-id"
	})).Return(nil).Once()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
	rr := httptest.NewRecorder()

	handler := middleware.RequireGatewayAuth(s.buildDeps(logger))(next)
	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func (s *RequireGatewayAuthSuite) TestMissingHeader_WithInvalidXFF_DegradesToEmptyClientIPAndTraceIDRequestID() {
	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())
	logger.EXPECT().Handle(mock.Anything, mock.MatchedBy(func(in input.RecordGatewayAuthFailureInput) bool {
		return in.Reason == "gateway_missing_header" &&
			in.ClientIPRaw == "" &&
			in.RequestID == "fake-trace-id"
	})).Return(nil).Once()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Forwarded-For", "not-an-ip")
	rr := httptest.NewRecorder()

	handler := middleware.RequireGatewayAuth(s.buildDeps(logger))(next)
	handler.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func (s *RequireGatewayAuthSuite) TestBodyIsFixed_NoCacheControl() {
	logger := middlewaremocks.NewMockGatewayAuthFailureLogger(s.T())
	logger.EXPECT().Handle(mock.Anything, mock.Anything).Return(nil).Once()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rr := httptest.NewRecorder()

	handler := middleware.RequireGatewayAuth(s.buildDeps(logger))(next)
	handler.ServeHTTP(rr, req)

	s.Equal(`{"error":"unauthorized"}`, rr.Body.String())
	s.Equal("no-store", rr.Header().Get("Cache-Control"))
	s.Equal("application/json", rr.Header().Get("Content-Type"))
	s.Empty(rr.Header().Get("WWW-Authenticate"))
}

type noopFailureLogger struct{}

func (n *noopFailureLogger) Handle(_ context.Context, _ input.RecordGatewayAuthFailureInput) error {
	return nil
}

func BenchmarkRequireGatewayAuth_Valid(b *testing.B) {
	now := time.Now().UTC()
	ts := strconv.FormatInt(now.Unix(), 10)
	sig := makeHMAC(testSecret, testUserID, ts)

	logger := &noopFailureLogger{}
	o11y := fake.NewProvider()

	deps := middleware.RequireGatewayAuthDeps{
		Secrets: services.SecretPair{
			Current: []byte(testSecret),
		},
		Window:        testWindow,
		FailureLogger: logger,
		O11y:          o11y,
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.RequireGatewayAuth(deps)(next)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("X-User-ID", testUserID)
		req.Header.Set("X-Gateway-Auth", sig)
		req.Header.Set("X-Gateway-Timestamp", ts)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

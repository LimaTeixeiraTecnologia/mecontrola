package idempotency_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	idemMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

type MiddlewareSuite struct {
	suite.Suite
	ctx     context.Context
	userID  uuid.UUID
	storage *idemMocks.Storage
	o11y    *noop.Provider
}

func TestMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareSuite))
}

func (s *MiddlewareSuite) SetupTest() {
	s.ctx = context.Background()
	s.userID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	s.storage = idemMocks.NewStorage(s.T())
	s.o11y = noop.NewProvider()
}

func (s *MiddlewareSuite) ctxWithPrincipal() context.Context {
	return auth.WithPrincipal(s.ctx, auth.Principal{UserID: s.userID, Source: auth.SourceWhatsApp})
}

func (s *MiddlewareSuite) newRequest(ctx context.Context, body string) *http.Request {
	var bodyReader *bytes.Reader
	if body != "" {
		bodyReader = bytes.NewReader([]byte(body))
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}
	r := httptest.NewRequest(http.MethodPost, "/test", bodyReader)
	return r.WithContext(ctx)
}

func (s *MiddlewareSuite) nextHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

func (s *MiddlewareSuite) TestMissingIdempotencyKey() {
	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := s.nextHandler(http.StatusOK, `{"ok":true}`)
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), `{}`)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	s.NoError(json.Unmarshal(w.Body.Bytes(), &body))
	s.Equal("missing_idempotency_key", body["message"])
}

func (s *MiddlewareSuite) TestInvalidIdempotencyKeyTooLong() {
	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := s.nextHandler(http.StatusOK, `{"ok":true}`)
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), `{}`)
	req.Header.Set("Idempotency-Key", strings.Repeat("x", 129))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	s.NoError(json.Unmarshal(w.Body.Bytes(), &body))
	s.Equal("invalid_idempotency_key", body["message"])
}

func (s *MiddlewareSuite) TestHitMatchReplays() {
	existingBody := []byte(`{"id":"card-1"}`)
	existing := idempotency.Record{
		Scope:          "card",
		Key:            "key-abc",
		UserID:         s.userID.String(),
		RequestHash:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		ResponseStatus: http.StatusCreated,
		ResponseBody:   existingBody,
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}

	s.storage.EXPECT().
		Get(mock.Anything, "card", "key-abc", s.userID.String()).
		Return(existing, nil).Once()

	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := s.nextHandler(http.StatusOK, `{"ok":true}`)
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), "")
	req.Header.Set("Idempotency-Key", "key-abc")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal(http.StatusCreated, w.Code)
	s.Equal(existingBody, w.Body.Bytes())
}

func (s *MiddlewareSuite) TestHitMismatchReturns409() {
	existing := idempotency.Record{
		Scope:          "card",
		Key:            "key-abc",
		UserID:         s.userID.String(),
		RequestHash:    "differenthash0000000000000000000000000000000000000000000000000000",
		ResponseStatus: http.StatusCreated,
		ResponseBody:   []byte(`{"id":"card-1"}`),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}

	s.storage.EXPECT().
		Get(mock.Anything, "card", "key-abc", s.userID.String()).
		Return(existing, nil).Once()

	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := s.nextHandler(http.StatusOK, `{"ok":true}`)
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), `{"name":"card-X"}`)
	req.Header.Set("Idempotency-Key", "key-abc")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal(http.StatusConflict, w.Code)
	var body map[string]string
	s.NoError(json.Unmarshal(w.Body.Bytes(), &body))
	s.Equal("idempotency_conflict", body["message"])
}

func (s *MiddlewareSuite) TestMiss2xxDoesNotSave() {
	s.storage.EXPECT().
		Get(mock.Anything, "card", "key-new", s.userID.String()).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := s.nextHandler(http.StatusCreated, `{"id":"card-2"}`)
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), `{"name":"Visa"}`)
	req.Header.Set("Idempotency-Key", "key-new")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal(http.StatusCreated, w.Code)
	s.Contains(w.Body.String(), "card-2")
}

func (s *MiddlewareSuite) TestMiss4xxSavesBestEffort() {
	respBody := `{"message":"bad request"}`
	s.storage.EXPECT().
		Get(mock.Anything, "card", "key-4xx", s.userID.String()).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	s.storage.EXPECT().
		Put(mock.Anything, mock.MatchedBy(func(r idempotency.Record) bool {
			return r.ResponseStatus == http.StatusBadRequest &&
				string(r.ResponseBody) == respBody &&
				r.Scope == "card" &&
				r.Key == "key-4xx"
		})).
		Return(nil).Once()

	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := s.nextHandler(http.StatusBadRequest, respBody)
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), `{}`)
	req.Header.Set("Idempotency-Key", "key-4xx")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *MiddlewareSuite) TestMiss5xxDoesNotSave() {
	s.storage.EXPECT().
		Get(mock.Anything, "card", "key-5xx", s.userID.String()).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := s.nextHandler(http.StatusInternalServerError, `{"message":"internal error"}`)
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), `{}`)
	req.Header.Set("Idempotency-Key", "key-5xx")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal(http.StatusInternalServerError, w.Code)
}

func (s *MiddlewareSuite) TestOverflowReturns500() {
	s.storage.EXPECT().
		Get(mock.Anything, "card", "key-overflow", s.userID.String()).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	largeBody := strings.Repeat("x", 65*1024)
	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeBody))
	})
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), `{}`)
	req.Header.Set("Idempotency-Key", "key-overflow")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal(http.StatusInternalServerError, w.Code)
}

func (s *MiddlewareSuite) TestMiss4xxPutFailureStillResponds() {
	respBody := `{"message":"bad req"}`
	s.storage.EXPECT().
		Get(mock.Anything, "card", "key-put-fail", s.userID.String()).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	s.storage.EXPECT().
		Put(mock.Anything, mock.Anything).
		Return(errors.New("db error")).Once()

	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := s.nextHandler(http.StatusUnprocessableEntity, respBody)
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), `{}`)
	req.Header.Set("Idempotency-Key", "key-put-fail")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal(http.StatusUnprocessableEntity, w.Code)
}

func (s *MiddlewareSuite) TestContextPropagated() {
	s.storage.EXPECT().
		Get(mock.Anything, "card", "key-ctx", s.userID.String()).
		Return(idempotency.Record{}, idempotency.ErrNotFound).Once()

	var capturedIC idempotency.IdempotencyContext
	mw := idempotency.Middleware("card", s.storage, 24*time.Hour, s.o11y)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ic, ok := idempotency.FromContext(r.Context())
		s.True(ok)
		capturedIC = ic
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	req := s.newRequest(s.ctxWithPrincipal(), `{"name":"Visa"}`)
	req.Header.Set("Idempotency-Key", "key-ctx")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	s.Equal("card", capturedIC.Scope)
	s.Equal("key-ctx", capturedIC.Key)
	s.Equal(s.userID.String(), capturedIC.UserID)
	s.Len(capturedIC.RequestHash, 64)
	s.False(capturedIC.ExpiresAt.IsZero())
}

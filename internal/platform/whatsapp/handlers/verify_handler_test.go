package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
)

type VerifyHandlerSuite struct {
	suite.Suite
}

func TestVerifyHandlerSuite(t *testing.T) {
	suite.Run(t, new(VerifyHandlerSuite))
}

func (s *VerifyHandlerSuite) TestHandle_ValidSubscribe() {
	h := handlers.NewVerifyHandler("my-token")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/whatsapp/verify?hub.mode=subscribe&hub.verify_token=my-token&hub.challenge=abc123", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req)

	s.Equal(http.StatusOK, w.Code)
	s.Equal("abc123", w.Body.String())
}

func (s *VerifyHandlerSuite) TestHandle_WrongToken() {
	h := handlers.NewVerifyHandler("correct-token")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/whatsapp/verify?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=abc123", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req)

	s.Equal(http.StatusForbidden, w.Code)
}

func (s *VerifyHandlerSuite) TestHandle_WrongMode() {
	h := handlers.NewVerifyHandler("my-token")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/whatsapp/verify?hub.mode=unsubscribe&hub.verify_token=my-token&hub.challenge=abc123", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req)

	s.Equal(http.StatusForbidden, w.Code)
}

func (s *VerifyHandlerSuite) TestHandle_MissingMode() {
	h := handlers.NewVerifyHandler("my-token")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/whatsapp/verify?hub.verify_token=my-token&hub.challenge=abc123", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req)

	s.Equal(http.StatusForbidden, w.Code)
}

func (s *VerifyHandlerSuite) TestHandle_EmptyChallenge() {
	h := handlers.NewVerifyHandler("my-token")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/whatsapp/verify?hub.mode=subscribe&hub.verify_token=my-token", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req)

	s.Equal(http.StatusOK, w.Code)
	s.Equal("", w.Body.String())
}

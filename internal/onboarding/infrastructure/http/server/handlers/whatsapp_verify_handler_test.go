package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
)

type WhatsAppVerifyHandlerSuite struct {
	suite.Suite
	handler *handlers.WhatsAppVerifyHandler
}

func TestWhatsAppVerifyHandler(t *testing.T) {
	suite.Run(t, new(WhatsAppVerifyHandlerSuite))
}

func (s *WhatsAppVerifyHandlerSuite) SetupTest() {
	s.handler = handlers.NewWhatsAppVerifyHandler("my-verify-token", noop.NewProvider())
}

func (s *WhatsAppVerifyHandlerSuite) TestHandle_ValidToken() {
	req := httptest.NewRequest(http.MethodGet, "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=my-verify-token&hub.challenge=challenge123", nil)
	rr := httptest.NewRecorder()

	s.handler.Handle(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Equal(s.T(), "challenge123", rr.Body.String())
}

func (s *WhatsAppVerifyHandlerSuite) TestHandle_InvalidToken() {
	req := httptest.NewRequest(http.MethodGet, "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=wrong-token&hub.challenge=challenge123", nil)
	rr := httptest.NewRecorder()

	s.handler.Handle(rr, req)

	assert.Equal(s.T(), http.StatusForbidden, rr.Code)
}

func (s *WhatsAppVerifyHandlerSuite) TestHandle_WrongMode() {
	req := httptest.NewRequest(http.MethodGet, "/webhooks/whatsapp?hub.mode=other&hub.verify_token=my-verify-token&hub.challenge=challenge123", nil)
	rr := httptest.NewRecorder()

	s.handler.Handle(rr, req)

	assert.Equal(s.T(), http.StatusForbidden, rr.Code)
}

func (s *WhatsAppVerifyHandlerSuite) TestHandle_MissingChallenge() {
	req := httptest.NewRequest(http.MethodGet, "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=my-verify-token", nil)
	rr := httptest.NewRecorder()

	s.handler.Handle(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Equal(s.T(), "", rr.Body.String())
}

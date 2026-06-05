package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
)

type mockUpsertUseCase struct {
	result output.UpsertUserByWhatsApp
	err    error
}

func (m *mockUpsertUseCase) Execute(_ context.Context, _ input.UpsertUserByWhatsApp) (output.UpsertUserByWhatsApp, error) {
	return m.result, m.err
}

type UpsertHandlerSuite struct {
	suite.Suite
}

func TestUpsertHandler(t *testing.T) {
	suite.Run(t, new(UpsertHandlerSuite))
}

func (s *UpsertHandlerSuite) newHandler(uc *mockUpsertUseCase) *handlers.UpsertUserByWhatsAppHandler {
	return handlers.NewUpsertUserByWhatsAppHandler(uc, noop.NewProvider())
}

func (s *UpsertHandlerSuite) post(handler *handlers.UpsertUserByWhatsAppHandler, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		s.T().Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users", &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Handle(rr, req)
	return rr
}

func (s *UpsertHandlerSuite) postRaw(handler *handlers.UpsertUserByWhatsAppHandler, raw string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users", bytes.NewBufferString(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Handle(rr, req)
	return rr
}

func (s *UpsertHandlerSuite) TestPayloadValido_200() {
	uc := &mockUpsertUseCase{
		result: output.UpsertUserByWhatsApp{
			ID:             "some-uuid",
			WhatsAppNumber: "+5511987654321",
			Email:          "user@example.com",
			DisplayName:    "Test User",
			Status:         "active",
		},
	}
	rr := s.post(s.newHandler(uc), map[string]string{
		"whatsapp":     "+5511987654321",
		"email":        "user@example.com",
		"display_name": "Test User",
	})
	s.Equal(http.StatusOK, rr.Code)
	var resp map[string]any
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("some-uuid", resp["id"])
}

func (s *UpsertHandlerSuite) TestJSONMalformado_400_InvalidPayload() {
	uc := &mockUpsertUseCase{}
	rr := s.postRaw(s.newHandler(uc), `{not valid json`)
	s.Equal(http.StatusBadRequest, rr.Code)
	var resp map[string]any
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	details, ok := resp["details"].(map[string]any)
	s.Require().True(ok)
	s.Equal("invalid_payload", details["code"])
}

func (s *UpsertHandlerSuite) TestWhatsAppInvalido_400_InvalidWhatsapp() {
	uc := &mockUpsertUseCase{}
	rr := s.post(s.newHandler(uc), map[string]string{
		"whatsapp": "123",
		"email":    "user@example.com",
	})
	s.Equal(http.StatusBadRequest, rr.Code)
	var resp map[string]any
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	details, ok := resp["details"].(map[string]any)
	s.Require().True(ok)
	s.Equal("invalid_whatsapp", details["code"])
}

func (s *UpsertHandlerSuite) TestEmailInvalido_400_InvalidEmail() {
	uc := &mockUpsertUseCase{}
	rr := s.post(s.newHandler(uc), map[string]string{
		"whatsapp": "+5511987654321",
		"email":    "not-an-email",
	})
	s.Equal(http.StatusBadRequest, rr.Code)
	var resp map[string]any
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	details, ok := resp["details"].(map[string]any)
	s.Require().True(ok)
	s.Equal("invalid_email", details["code"])
}

func (s *UpsertHandlerSuite) TestWhatsAppEmUso_409_WhatsappInUse() {
	uc := &mockUpsertUseCase{err: application.ErrWhatsAppNumberInUse}
	rr := s.post(s.newHandler(uc), map[string]string{
		"whatsapp": "+5511987654321",
	})
	s.Equal(http.StatusConflict, rr.Code)
	var resp map[string]any
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	details, ok := resp["details"].(map[string]any)
	s.Require().True(ok)
	s.Equal("whatsapp_in_use", details["code"])
}

func (s *UpsertHandlerSuite) TestEmailEmUso_409_EmailInUse() {
	uc := &mockUpsertUseCase{err: application.ErrEmailInUse}
	rr := s.post(s.newHandler(uc), map[string]string{
		"whatsapp": "+5511987654321",
		"email":    "taken@example.com",
	})
	s.Equal(http.StatusConflict, rr.Code)
	var resp map[string]any
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	details, ok := resp["details"].(map[string]any)
	s.Require().True(ok)
	s.Equal("email_in_use", details["code"])
}

func (s *UpsertHandlerSuite) TestErroInterno_500() {
	uc := &mockUpsertUseCase{err: errors.New("unexpected db error")}
	rr := s.post(s.newHandler(uc), map[string]string{
		"whatsapp": "+5511987654321",
	})
	s.Equal(http.StatusInternalServerError, rr.Code)
	var resp map[string]any
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("erro inesperado", resp["message"])
}

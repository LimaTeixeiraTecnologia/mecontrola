package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
)

type WhatsAppVerifyHandlerSuite struct {
	suite.Suite
	handler *handlers.WhatsAppVerifyHandler
}

func TestWhatsAppVerifyHandlerSuite(t *testing.T) {
	suite.Run(t, new(WhatsAppVerifyHandlerSuite))
}

func (s *WhatsAppVerifyHandlerSuite) SetupTest() {
	s.handler = handlers.NewWhatsAppVerifyHandler("my-verify-token", noop.NewProvider())
}

func (s *WhatsAppVerifyHandlerSuite) TestHandle() {
	type args struct {
		url string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(*httptest.ResponseRecorder)
	}{
		{
			name: "deve responder challenge com token valido",
			args: args{url: "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=my-verify-token&hub.challenge=challenge123"},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, recorder.Code)
				s.Equal("challenge123", recorder.Body.String())
			},
		},
		{
			name: "deve rejeitar token invalido",
			args: args{url: "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=wrong-token&hub.challenge=challenge123"},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "deve rejeitar modo invalido",
			args: args{url: "/webhooks/whatsapp?hub.mode=other&hub.verify_token=my-verify-token&hub.challenge=challenge123"},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "deve responder vazio sem challenge",
			args: args{url: "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=my-verify-token"},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, recorder.Code)
				s.Empty(recorder.Body.String())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			request := httptest.NewRequest(http.MethodGet, scenario.args.url, nil)
			recorder := httptest.NewRecorder()
			s.handler.Handle(recorder, request)
			scenario.expect(recorder)
		})
	}
}

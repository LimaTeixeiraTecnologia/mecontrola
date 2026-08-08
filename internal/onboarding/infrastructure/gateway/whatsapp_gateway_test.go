package gateway_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	devkitfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/gateway"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/client/meta"
)

type WhatsAppGatewaySuite struct {
	suite.Suite
}

func TestWhatsAppGatewaySuite(t *testing.T) {
	suite.Run(t, new(WhatsAppGatewaySuite))
}

func (s *WhatsAppGatewaySuite) TestSendTypingIndicator() {
	scenarios := []struct {
		name       string
		statusCode int
		response   map[string]any
		expectErr  error
	}{
		{
			name:       "deve delegar envio do indicador de digitação ao client meta",
			statusCode: http.StatusOK,
			response:   map[string]any{"success": true},
			expectErr:  nil,
		},
		{
			name:       "deve classificar bad request do client como erro de cliente",
			statusCode: http.StatusBadRequest,
			response: map[string]any{
				"error": map[string]any{"message": "message_id inválido", "code": 100},
			},
			expectErr: application.ErrWhatsAppClientError,
		},
		{
			name:       "deve classificar erro de servidor do client como erro de servidor",
			statusCode: http.StatusInternalServerError,
			response: map[string]any{
				"error": map[string]any{"message": "internal error", "code": 2},
			},
			expectErr: application.ErrWhatsAppServerError,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var capturedBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				s.Require().NoError(json.NewDecoder(r.Body).Decode(&capturedBody))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.statusCode)
				s.Require().NoError(json.NewEncoder(w).Encode(scenario.response))
			}))
			defer server.Close()

			client, err := meta.NewClient(devkitfake.NewProvider(), meta.Config{
				PhoneNumberID: "123",
				AccessToken:   "token",
				BaseURL:       server.URL,
			})
			s.Require().NoError(err)

			whatsAppGateway := gateway.NewWhatsAppGateway(client)
			err = whatsAppGateway.SendTypingIndicator(context.Background(), "wamid.inbound")

			s.Equal("wamid.inbound", capturedBody["message_id"])
			if scenario.expectErr == nil {
				s.Require().NoError(err)
				return
			}
			s.Require().Error(err)
			s.ErrorIs(err, scenario.expectErr)
		})
	}
}

func (s *WhatsAppGatewaySuite) TestSendActivationTemplate() {
	type args struct {
		statusCode int
		response   map[string]any
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(map[string]any, string, error)
	}{
		{
			name: "deve enviar template com comando ativar",
			args: args{
				statusCode: http.StatusOK,
				response: map[string]any{
					"messages": []map[string]string{{"id": "wamid.test"}},
				},
			},
			expect: func(body map[string]any, wamid string, err error) {
				s.Require().NoError(err)
				s.Equal("wamid.test", wamid)
				template := body["template"].(map[string]any)
				components := template["components"].([]any)
				parameters := components[0].(map[string]any)["parameters"].([]any)
				s.Equal("ATIVAR abc-token", parameters[0].(map[string]any)["text"])
			},
		},
		{
			name: "deve propagar erro do client meta",
			args: args{
				statusCode: http.StatusBadRequest,
				response: map[string]any{
					"error": map[string]any{"message": "template not found"},
				},
			},
			expect: func(body map[string]any, wamid string, err error) {
				s.NotNil(body)
				s.Empty(wamid)
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var capturedBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				s.Require().NoError(json.NewDecoder(r.Body).Decode(&capturedBody))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.args.statusCode)
				s.Require().NoError(json.NewEncoder(w).Encode(scenario.args.response))
			}))
			defer server.Close()

			client, err := meta.NewClient(devkitfake.NewProvider(), meta.Config{
				PhoneNumberID: "123",
				AccessToken:   "token",
				BaseURL:       server.URL,
			})
			s.Require().NoError(err)

			whatsAppGateway := gateway.NewWhatsAppGateway(client)
			wamid, err := whatsAppGateway.SendActivationTemplate(context.Background(), "+5511999990000", "activation_reminder", "abc-token")
			scenario.expect(capturedBody, wamid, err)
		})
	}
}

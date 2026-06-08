package meta_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	devkitfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/client/meta"
)

type ClientSuite struct {
	suite.Suite
	cfg meta.Config
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupTest() {
	s.cfg = meta.Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test-access-token",
		HTTPTimeout:   5 * time.Second,
	}
}

func (s *ClientSuite) newClientWithServer(baseURL string) *meta.Client {
	cfg := s.cfg
	cfg.BaseURL = baseURL
	c, err := meta.NewClient(devkitfake.NewProvider(), cfg)
	s.Require().NoError(err)
	return c
}

func (s *ClientSuite) TestClient_SendTemplate_Scenarios() {
	scenarios := []struct {
		name             string
		toE164           string
		templateName     string
		serverHandler    http.HandlerFunc
		expectErr        bool
		expectErrType    error
		expectedWAMID    string
		validateBody     bool
		expectedTo       string
		expectedType     string
		expectedProduct  string
		checkAuthHeader  bool
		checkContentType bool
	}{
		{
			name:         "Success sends template correctly",
			toE164:       "+5511999998888",
			templateName: "activation_reminder",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				s.Equal(http.MethodPost, r.Method)
				s.Equal("/123456789/messages", r.URL.Path)
				s.Equal("Bearer test-access-token", r.Header.Get("Authorization"))
				s.Equal("application/json", r.Header.Get("Content-Type"))

				var body map[string]any
				s.NoError(json.NewDecoder(r.Body).Decode(&body))
				s.Equal("whatsapp", body["messaging_product"])
				s.Equal("5511999998888", body["to"])
				s.Equal("template", body["type"])

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"messages": []map[string]string{{"id": "wamid.abc123"}},
				})
			},
			expectErr:     false,
			expectedWAMID: "wamid.abc123",
		},
		{
			name:         "Strips plus prefix from phone number",
			toE164:       "+5511999990000",
			templateName: "tmpl",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				s.NoError(json.NewDecoder(r.Body).Decode(&body))
				capturedTo, _ := body["to"].(string)
				s.Equal("5511999990000", capturedTo)

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"messages": []map[string]string{{"id": "wamid.xyz"}},
				})
			},
			expectErr:     false,
			expectedWAMID: "wamid.xyz",
		},
		{
			name:         "400 returns bad request error",
			toE164:       "+5511999990000",
			templateName: "tmpl",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"message": "template not found",
						"type":    "OAuthException",
						"code":    131051,
					},
				})
			},
			expectErr:     true,
			expectErrType: meta.ErrMetaBadRequest,
		},
		{
			name:         "401 returns auth error",
			toE164:       "+5511999990000",
			templateName: "tmpl",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"message": "Invalid OAuth access token",
						"type":    "OAuthException",
						"code":    190,
					},
				})
			},
			expectErr:     true,
			expectErrType: meta.ErrMetaAuth,
		},
		{
			name:         "5xx returns server error",
			toE164:       "+5511999990000",
			templateName: "tmpl",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectErr:     true,
			expectErrType: meta.ErrMetaServer,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			srv := httptest.NewServer(scenario.serverHandler)
			defer srv.Close()

			client := s.newClientWithServer(srv.URL)
			wamid, err := client.SendTemplate(context.Background(), scenario.toE164, scenario.templateName, "pt_BR", nil)

			if scenario.expectErr {
				s.Error(err)
				if scenario.expectErrType != nil {
					s.ErrorIs(err, scenario.expectErrType)
				}
			} else {
				s.NoError(err)
				s.Equal(scenario.expectedWAMID, wamid)
			}
		})
	}
}

func (s *ClientSuite) TestClient_SendText_Scenarios() {
	scenarios := []struct {
		name          string
		toE164        string
		text          string
		serverHandler http.HandlerFunc
		expectErr     bool
		expectErrType error
		expectedWAMID string
	}{
		{
			name:   "Success sends text message",
			toE164: "+5511999990000",
			text:   "Olá, bem-vindo!",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				s.NoError(json.NewDecoder(r.Body).Decode(&body))
				s.Equal("text", body["type"])
				textObj, ok := body["text"].(map[string]any)
				s.True(ok)
				s.Equal("Olá, bem-vindo!", textObj["body"])

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"messages": []map[string]string{{"id": "wamid.text123"}},
				})
			},
			expectErr:     false,
			expectedWAMID: "wamid.text123",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			srv := httptest.NewServer(scenario.serverHandler)
			defer srv.Close()

			client := s.newClientWithServer(srv.URL)
			wamid, err := client.SendText(context.Background(), scenario.toE164, scenario.text)

			if scenario.expectErr {
				s.Error(err)
				if scenario.expectErrType != nil {
					s.ErrorIs(err, scenario.expectErrType)
				}
			} else {
				s.NoError(err)
				s.Equal(scenario.expectedWAMID, wamid)
			}
		})
	}
}

func (s *ClientSuite) TestNewClient_ValidationErrors() {
	scenarios := []struct {
		name   string
		setup  func() error
		expect func(error)
	}{
		{
			name: "deve retornar erro sem observabilidade",
			setup: func() error {
				_, err := meta.NewClient(nil, meta.Config{PhoneNumberID: "x", AccessToken: "y"})
				return err
			},
			expect: func(err error) {
				s.Error(err)
			},
		},
		{
			name: "deve retornar erro sem phone number id",
			setup: func() error {
				_, err := meta.NewClient(devkitfake.NewProvider(), meta.Config{AccessToken: "y"})
				return err
			},
			expect: func(err error) {
				s.Error(err)
			},
		},
		{
			name: "deve retornar erro sem access token",
			setup: func() error {
				_, err := meta.NewClient(devkitfake.NewProvider(), meta.Config{PhoneNumberID: "x"})
				return err
			},
			expect: func(err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			err := scenario.setup()
			scenario.expect(err)
		})
	}
}

func (s *ClientSuite) TestClient_NoHttpClientDirect() {
	files := []string{"client.go", "models.go"}
	for _, f := range files {
		content, err := os.ReadFile(f)
		s.NoError(err, "ler %s", f)
		s.NotContains(string(content), "&http.Client{}", "arquivo %s não pode instanciar http.Client diretamente", f)
	}
}

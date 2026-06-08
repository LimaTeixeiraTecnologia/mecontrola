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
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/client/meta"
)

func newTestClient(t *testing.T, baseURL string) *meta.Client {
	t.Helper()
	cfg := meta.Config{
		PhoneNumberID: "123456789",
		AccessToken:   "test-access-token",
		BaseURL:       baseURL,
		HTTPTimeout:   5 * time.Second,
	}
	c, err := meta.NewClient(devkitfake.NewProvider(), cfg)
	require.NoError(t, err)
	return c
}

func TestClient_SendTemplate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/123456789/messages", r.URL.Path)
		require.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "whatsapp", body["messaging_product"])
		require.Equal(t, "5511999998888", body["to"])
		require.Equal(t, "template", body["type"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.abc123"}},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	wamid, err := client.SendTemplate(context.Background(), "+5511999998888", "activation_reminder", "pt_BR", nil)
	require.NoError(t, err)
	require.Equal(t, "wamid.abc123", wamid)
}

func TestClient_SendTemplate_StripsPlusPrefix(t *testing.T) {
	var capturedTo string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		capturedTo, _ = body["to"].(string)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.xyz"}},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.SendTemplate(context.Background(), "+5511999990000", "tmpl", "pt_BR", nil)
	require.NoError(t, err)
	require.Equal(t, "5511999990000", capturedTo)
}

func TestClient_SendTemplate_4xx_ReturnsBadRequestError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "template not found",
				"type":    "OAuthException",
				"code":    131051,
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.SendTemplate(context.Background(), "+5511999990000", "tmpl", "pt_BR", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, meta.ErrMetaBadRequest)
}

func TestClient_SendTemplate_401_ReturnsAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid OAuth access token",
				"type":    "OAuthException",
				"code":    190,
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.SendTemplate(context.Background(), "+5511999990000", "tmpl", "pt_BR", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, meta.ErrMetaAuth)
}

func TestClient_SendTemplate_5xx_ReturnsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.SendTemplate(context.Background(), "+5511999990000", "tmpl", "pt_BR", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, meta.ErrMetaServer)
}

func TestClient_SendText_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "text", body["type"])
		textObj, ok := body["text"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "Olá, bem-vindo!", textObj["body"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.text123"}},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	wamid, err := client.SendText(context.Background(), "+5511999990000", "Olá, bem-vindo!")
	require.NoError(t, err)
	require.Equal(t, "wamid.text123", wamid)
}

func TestClient_NoHttpClientDirect(t *testing.T) {
	files := []string{"client.go", "models.go"}
	for _, f := range files {
		content, err := os.ReadFile(f)
		require.NoError(t, err, "ler %s", f)
		require.NotContains(t, string(content), "&http.Client{}", "arquivo %s não pode instanciar http.Client diretamente", f)
	}
}

func TestNewClient_MissingObservability(t *testing.T) {
	_, err := meta.NewClient(nil, meta.Config{PhoneNumberID: "x", AccessToken: "y"})
	require.Error(t, err)
}

func TestNewClient_MissingPhoneNumberID(t *testing.T) {
	_, err := meta.NewClient(devkitfake.NewProvider(), meta.Config{AccessToken: "y"})
	require.Error(t, err)
}

func TestNewClient_MissingAccessToken(t *testing.T) {
	_, err := meta.NewClient(devkitfake.NewProvider(), meta.Config{PhoneNumberID: "x"})
	require.Error(t, err)
}

package gateway_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	devkitfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/gateway"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/client/meta"
)

func TestWhatsAppGateway_SendActivationTemplateIncludesAtivarToken(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.test"}},
		})
	}))
	defer srv.Close()

	client, err := meta.NewClient(devkitfake.NewProvider(), meta.Config{
		PhoneNumberID: "123",
		AccessToken:   "token",
		BaseURL:       srv.URL,
	})
	require.NoError(t, err)
	waGateway := gateway.NewWhatsAppGateway(client)

	_, err = waGateway.SendActivationTemplate(context.Background(), "+5511999990000", "activation_reminder", "abc-token")

	require.NoError(t, err)
	template := body["template"].(map[string]any)
	components := template["components"].([]any)
	firstComponent := components[0].(map[string]any)
	parameters := firstComponent["parameters"].([]any)
	firstParameter := parameters[0].(map[string]any)
	require.Equal(t, "ATIVAR abc-token", firstParameter["text"])
}

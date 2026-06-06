package kiwify_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	devkitfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
)

func newTestClient(t *testing.T, baseURL string, overrides ...func(*kiwify.Config)) *kiwify.Client {
	t.Helper()
	cfg := kiwify.Config{
		AccountID:                  "acct-test",
		ClientID:                   "client-id",
		ClientSecret:               "client-secret",
		APIBaseURL:                 baseURL,
		OAuthTokenSafetyMargin:     5 * time.Minute,
		RateLimitMaxRequestsPerMin: 6000,
		RateLimitBurst:             1000,
		HTTPTimeout:                5 * time.Second,
		HTTPRetryMaxAttempts:       3,
		HTTPRetryBackoff:           5 * time.Millisecond,
	}
	for _, fn := range overrides {
		fn(&cfg)
	}
	c, err := kiwify.NewClient(devkitfake.NewProvider(), cfg)
	require.NoError(t, err)
	return c
}

func TestClient_TokenCacheSingleOAuthCall(t *testing.T) {
	var oauthCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			oauthCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok-cached",
				"token_type":   "Bearer",
				"expires_in":   86400,
			})
			return
		}
		require.Equal(t, "Bearer tok-cached", r.Header.Get("Authorization"))
		require.Equal(t, "acct-test", r.Header.Get("x-kiwify-account-id"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":        []any{},
			"hasMore":     false,
			"totalItems":  0,
			"currentPage": 1,
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	now := time.Now().UTC()

	_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
	require.NoError(t, err)

	_, err = client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
	require.NoError(t, err)

	require.Equal(t, int32(1), oauthCalls.Load(), "deve chamar OAuth apenas uma vez dentro da janela de cache")
}

func TestClient_AccountIDHeaderInEveryRequest(t *testing.T) {
	var gotAccountID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok",
				"token_type":   "Bearer",
				"expires_in":   86400,
			})
			return
		}
		gotAccountID = r.Header.Get("x-kiwify-account-id")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":    []any{},
			"hasMore": false,
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	now := time.Now().UTC()
	_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
	require.NoError(t, err)
	require.Equal(t, "acct-test", gotAccountID)
}

func TestClient_RetryOn5xx(t *testing.T) {
	var oauthCalls atomic.Int32
	var salesCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			oauthCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok",
				"token_type":   "Bearer",
				"expires_in":   86400,
			})
			return
		}
		n := salesCalls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":    []any{},
			"hasMore": false,
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, func(c *kiwify.Config) {
		c.HTTPRetryMaxAttempts = 5
		c.HTTPRetryBackoff = 5 * time.Millisecond
	})
	now := time.Now().UTC()
	_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, salesCalls.Load(), int32(3), "deve ter retentado pelo menos 2 vezes após 5xx")
}

func TestClient_AbortOn4xx(t *testing.T) {
	var salesCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok",
				"token_type":   "Bearer",
				"expires_in":   86400,
			})
			return
		}
		salesCalls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	now := time.Now().UTC()
	_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
	require.Error(t, err)
	require.Equal(t, int32(1), salesCalls.Load(), "não deve retentar em 4xx")
}

func TestClient_GetSale_ReturnsMapping(t *testing.T) {
	refundedAt := time.Now().UTC().Add(-2 * time.Hour)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok",
				"token_type":   "Bearer",
				"expires_in":   86400,
			})
			return
		}
		require.Equal(t, "/v1/sales/sale-42", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":              "sale-42",
			"reference":       "ORD-001",
			"status":          "paid",
			"sale_type":       "subscription",
			"payment_method":  "credit_card",
			"product_id":      "prod-xyz",
			"subscription_id": "sub-001",
			"created_at":      time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
			"updated_at":      time.Now().UTC().Format(time.RFC3339),
			"refunded_at":     refundedAt.Format(time.RFC3339),
			"customer":        map[string]string{"email": "user@example.com"},
			"tracking":        map[string]string{"s1": "funnel-token-123"},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	sale, err := client.GetSale(context.Background(), "sale-42")
	require.NoError(t, err)
	require.Equal(t, "sale-42", sale.ID)
	require.Equal(t, "ORD-001", sale.OrderID)
	require.Equal(t, "prod-xyz", sale.KiwifyProductID)
	require.Equal(t, "funnel-token-123", sale.FunnelToken)
	require.Equal(t, "user@example.com", sale.CustomerEmail)
	require.False(t, sale.RefundedAt.IsZero())
}

func TestClient_RateLimiter_BlocksExcess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping rate limit test in short mode")
	}

	var oauthOnce atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			oauthOnce.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok",
				"token_type":   "Bearer",
				"expires_in":   86400,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":    []any{},
			"hasMore": false,
		})
	}))
	defer srv.Close()

	const totalRequests = 50
	const maxPerMin = 60
	client := newTestClient(t, srv.URL, func(c *kiwify.Config) {
		c.RateLimitMaxRequestsPerMin = maxPerMin
		c.RateLimitBurst = 5
	})

	start := time.Now()
	for i := range totalRequests {
		now := time.Now().UTC()
		_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, i+1)
		require.NoError(t, err)
	}
	elapsed := time.Since(start)

	minExpected := time.Duration(float64(time.Minute) * float64(totalRequests-5) / float64(maxPerMin))
	require.Greater(t, elapsed, minExpected,
		"rate limiter deve ter introduzido delay: elapsed=%v minExpected=%v", elapsed, minExpected)
}

func TestClient_NoHttpClientDirect(t *testing.T) {
	files := []string{
		"client.go",
		"auth.go",
		"ratelimit.go",
		"models.go",
	}
	for _, f := range files {
		content, err := os.ReadFile(f)
		require.NoError(t, err, "ler %s", f)
		require.NotContains(t, string(content), "&http.Client{}", "arquivo %s não pode instanciar http.Client diretamente", f)
	}
}

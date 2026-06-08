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

	"github.com/stretchr/testify/suite"

	devkitfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
)

type ClientSuite struct {
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupTest() {}

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

func (s *ClientSuite) TestClient_ListSalesUpdatedSince() {
	scenarios := []struct {
		name      string
		overrides []func(*kiwify.Config)
		exercise  func(*kiwify.Client) error
		expect    func(*atomic.Int32, *atomic.Int32, string, error)
		handler   func(*atomic.Int32, *atomic.Int32, *string) http.HandlerFunc
	}{
		{
			name: "deve reutilizar o token em cache",
			exercise: func(client *kiwify.Client) error {
				now := time.Now().UTC()
				_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
				require.NoError(s.T(), err)
				_, err = client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
				return err
			},
			expect: func(oauthCalls *atomic.Int32, salesCalls *atomic.Int32, gotAccountID string, err error) {
				require.NoError(s.T(), err)
				require.Equal(s.T(), int32(1), oauthCalls.Load())
			},
			handler: func(oauthCalls *atomic.Int32, salesCalls *atomic.Int32, gotAccountID *string) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/v1/oauth/token" {
						oauthCalls.Add(1)
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-cached", "token_type": "Bearer", "expires_in": 86400})
						return
					}
					require.Equal(s.T(), "Bearer tok-cached", r.Header.Get("Authorization"))
					require.Equal(s.T(), "acct-test", r.Header.Get("x-kiwify-account-id"))
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "hasMore": false, "totalItems": 0, "currentPage": 1})
				}
			},
		},
		{
			name: "deve enviar o header x-kiwify-account-id em toda requisicao",
			exercise: func(client *kiwify.Client) error {
				now := time.Now().UTC()
				_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
				return err
			},
			expect: func(oauthCalls *atomic.Int32, salesCalls *atomic.Int32, gotAccountID string, err error) {
				require.NoError(s.T(), err)
				require.Equal(s.T(), "acct-test", gotAccountID)
			},
			handler: func(oauthCalls *atomic.Int32, salesCalls *atomic.Int32, gotAccountID *string) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/v1/oauth/token" {
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "token_type": "Bearer", "expires_in": 86400})
						return
					}
					*gotAccountID = r.Header.Get("x-kiwify-account-id")
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "hasMore": false})
				}
			},
		},
		{
			name: "deve retentar em erro 5xx",
			overrides: []func(*kiwify.Config){
				func(c *kiwify.Config) {
					c.HTTPRetryMaxAttempts = 5
					c.HTTPRetryBackoff = 5 * time.Millisecond
				},
			},
			exercise: func(client *kiwify.Client) error {
				now := time.Now().UTC()
				_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
				return err
			},
			expect: func(oauthCalls *atomic.Int32, salesCalls *atomic.Int32, gotAccountID string, err error) {
				require.NoError(s.T(), err)
				require.GreaterOrEqual(s.T(), salesCalls.Load(), int32(3))
			},
			handler: func(oauthCalls *atomic.Int32, salesCalls *atomic.Int32, gotAccountID *string) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/v1/oauth/token" {
						oauthCalls.Add(1)
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "token_type": "Bearer", "expires_in": 86400})
						return
					}
					n := salesCalls.Add(1)
					if n < 3 {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "hasMore": false})
				}
			},
		},
		{
			name: "deve abortar em erro 4xx",
			exercise: func(client *kiwify.Client) error {
				now := time.Now().UTC()
				_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, 1)
				return err
			},
			expect: func(oauthCalls *atomic.Int32, salesCalls *atomic.Int32, gotAccountID string, err error) {
				require.Error(s.T(), err)
				require.Equal(s.T(), int32(1), salesCalls.Load())
			},
			handler: func(oauthCalls *atomic.Int32, salesCalls *atomic.Int32, gotAccountID *string) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/v1/oauth/token" {
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "token_type": "Bearer", "expires_in": 86400})
						return
					}
					salesCalls.Add(1)
					w.WriteHeader(http.StatusBadRequest)
				}
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var oauthCalls atomic.Int32
			var salesCalls atomic.Int32
			var gotAccountID string
			srv := httptest.NewServer(scenario.handler(&oauthCalls, &salesCalls, &gotAccountID))
			defer srv.Close()

			client := newTestClient(s.T(), srv.URL, scenario.overrides...)
			err := scenario.exercise(client)
			scenario.expect(&oauthCalls, &salesCalls, gotAccountID, err)
		})
	}
}

func (s *ClientSuite) TestClient_GetSale_ReturnsMapping() {
	scenarios := []struct {
		name   string
		expect func(sale interface{}, err error)
	}{
		{
			name: "deve mapear a venda retornada pela API",
			expect: func(rawSale interface{}, err error) {
				sale := rawSale.(interfaces.KiwifySale)
				require.NoError(s.T(), err)
				require.Equal(s.T(), "sale-42", sale.ID)
				require.Equal(s.T(), "ORD-001", sale.OrderID)
				require.Equal(s.T(), "prod-xyz", sale.KiwifyProductID)
				require.Equal(s.T(), "funnel-token-123", sale.FunnelToken)
				require.Equal(s.T(), "user@example.com", sale.CustomerEmail)
				require.False(s.T(), sale.RefundedAt.IsZero())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			refundedAt := time.Now().UTC().Add(-2 * time.Hour)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/oauth/token" {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "token_type": "Bearer", "expires_in": 86400})
					return
				}
				require.Equal(s.T(), "/v1/sales/sale-42", r.URL.Path)
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

			client := newTestClient(s.T(), srv.URL)
			sale, err := client.GetSale(context.Background(), "sale-42")
			scenario.expect(sale, err)
		})
	}
}

func (s *ClientSuite) TestClient_RateLimiter_BlocksExcess() {
	scenarios := []struct {
		name string
	}{
		{name: "deve bloquear excesso de requisicoes"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			if testing.Short() {
				s.T().Skip("skipping rate limit test in short mode")
			}

			var oauthOnce atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/oauth/token" {
					oauthOnce.Add(1)
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "token_type": "Bearer", "expires_in": 86400})
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "hasMore": false})
			}))
			defer srv.Close()

			const totalRequests = 50
			const maxPerMin = 60
			client := newTestClient(s.T(), srv.URL, func(c *kiwify.Config) {
				c.RateLimitMaxRequestsPerMin = maxPerMin
				c.RateLimitBurst = 5
			})

			start := time.Now()
			for i := range totalRequests {
				now := time.Now().UTC()
				_, err := client.ListSalesUpdatedSince(context.Background(), now.Add(-time.Hour), now, i+1)
				require.NoError(s.T(), err)
			}
			elapsed := time.Since(start)
			minExpected := time.Duration(float64(time.Minute) * float64(totalRequests-5) / float64(maxPerMin))
			require.Greater(s.T(), elapsed, minExpected)
		})
	}
}

func (s *ClientSuite) TestClient_NoHttpClientDirect() {
	scenarios := []struct {
		name  string
		files []string
	}{
		{
			name:  "nao deve instanciar http client diretamente",
			files: []string{"client.go", "auth.go", "ratelimit.go", "models.go"},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			for _, f := range scenario.files {
				content, err := os.ReadFile(f)
				require.NoError(s.T(), err, "ler %s", f)
				require.NotContains(s.T(), string(content), "&http.Client{}", "arquivo %s não pode instanciar http.Client diretamente", f)
			}
		})
	}
}

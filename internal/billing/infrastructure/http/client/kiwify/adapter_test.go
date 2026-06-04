package kiwify_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
	platformhttpclientfakes "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient/fakes"
)

type KiwifyAdapterSuite struct {
	suite.Suite
	ctx      context.Context
	registry *kiwify.BillingPlansRegistry
	now      time.Time
}

func TestKiwifyAdapter(t *testing.T) {
	suite.Run(t, new(KiwifyAdapterSuite))
}

func (s *KiwifyAdapterSuite) SetupSuite() {
	s.now = time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	s.registry = kiwify.NewBillingPlansRegistryFromMap(map[string]valueobjects.PlanCode{
		"prod-monthly": valueobjects.PlanCodeMonthly,
	})
}

func (s *KiwifyAdapterSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *KiwifyAdapterSuite) buildTestServer(oauthToken string, saleHandler http.HandlerFunc) (*httptest.Server, func()) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": oauthToken,
			"expires_in":   86400,
		})
	})
	if saleHandler != nil {
		mux.HandleFunc("/v1/sales/", saleHandler)
	}
	srv := httptest.NewServer(mux)
	return srv, srv.Close
}

func (s *KiwifyAdapterSuite) buildAdapter(srv *httptest.Server) *kiwify.KiwifyAdapter {
	platformClient := platformhttpclientfakes.NewClient(srv.URL, "kiwify")
	c := kiwify.NewClient(platformClient, 100, 10)
	oauth := kiwify.NewOAuthClient(platformClient, "client-id", "client-secret", 5*time.Minute)
	verifier := kiwify.NewTokenSignatureVerifier("secret-token", "X-Kiwify-Webhook-Token")
	mapper := kiwify.NewPayloadMapper(s.registry, nil)
	return kiwify.NewKiwifyAdapter(c, oauth, verifier, mapper, s.registry)
}

func (s *KiwifyAdapterSuite) TestVerifySignature_Delegates() {
	srv, cleanup := s.buildTestServer("token-abc", nil)
	defer cleanup()
	adapter := s.buildAdapter(srv)

	err := adapter.VerifySignature([]byte("payload"), map[string]string{"X-Kiwify-Webhook-Token": "secret-token"})
	s.NoError(err)
}

func (s *KiwifyAdapterSuite) TestVerifySignature_Invalid() {
	srv, cleanup := s.buildTestServer("token-abc", nil)
	defer cleanup()
	adapter := s.buildAdapter(srv)

	err := adapter.VerifySignature([]byte("payload"), map[string]string{"X-Kiwify-Webhook-Token": "wrong-token"})
	s.ErrorIs(err, kiwify.ErrInvalidSignature)
}

func (s *KiwifyAdapterSuite) TestFetchSubscription_Happy() {
	expectedPeriodEnd := s.now.Add(30 * 24 * time.Hour)
	saleHandler := func(w http.ResponseWriter, r *http.Request) {
		s.Equal("Bearer valid-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "sub-001",
			"status":  "active",
			"product": map[string]any{"id": "prod-monthly"},
			"subscription": map[string]any{
				"current_period_start": s.now.Format(time.RFC3339),
				"current_period_end":   expectedPeriodEnd.Format(time.RFC3339),
			},
		})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "valid-token", "expires_in": 86400})
	})
	mux.HandleFunc("/v1/sales/", saleHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adapter := s.buildAdapter(srv)
	result, err := adapter.FetchSubscription(s.ctx, "sub-001")
	s.Require().NoError(err)
	s.Equal("sub-001", result.ExternalID)
	s.Equal(valueobjects.SubscriptionStatusActive, result.Status)
	s.Equal(valueobjects.PlanCodeMonthly, result.PlanCode)
}

func (s *KiwifyAdapterSuite) TestFetchSubscription_401Retry() {
	var requestCount int
	saleHandler := func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "sub-001",
			"status":  "active",
			"product": map[string]any{"id": "prod-monthly"},
			"subscription": map[string]any{
				"current_period_start": s.now.Format(time.RFC3339),
				"current_period_end":   s.now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
			},
		})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "refreshed-token", "expires_in": 86400})
	})
	mux.HandleFunc("/v1/sales/", saleHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adapter := s.buildAdapter(srv)
	result, err := adapter.FetchSubscription(s.ctx, "sub-001")
	s.Require().NoError(err, "deve ter sucesso após retry em 401")
	s.Equal("sub-001", result.ExternalID)
	s.Equal(2, requestCount, "deve ter feito 2 requests ao endpoint de sales")
}

func (s *KiwifyAdapterSuite) TestParseEvent_Delegates() {
	srv, cleanup := s.buildTestServer("token-abc", nil)
	defer cleanup()
	adapter := s.buildAdapter(srv)

	raw := []byte(`{
		"id":"event-001",
		"webhook_event_type":"compra_aprovada",
		"updated_at":"2024-06-01T00:00:00Z",
		"customer":{"mobile":"11999990000","email":"a@b.com"},
		"product":{"id":"prod-monthly"},
		"subscription":{"id":"sub-001","current_period_start":"2024-06-01T00:00:00Z","current_period_end":"2024-07-01T00:00:00Z"},
		"refund":{"amount_cents":0},
		"tracking":{"src":"token-abc"}
	}`)

	event, err := adapter.ParseEvent(raw)
	s.Require().NoError(err)
	s.Equal("event-001", event.ExternalEventID)
	s.Equal("token-abc", event.SignupToken)
}

func (s *KiwifyAdapterSuite) TestFetchSubscription_AllStatuses() {
	statuses := []struct {
		raw      string
		expected valueobjects.SubscriptionStatus
	}{
		{"active", valueobjects.SubscriptionStatusActive},
		{"trialing", valueobjects.SubscriptionStatusTrialing},
		{"past_due", valueobjects.SubscriptionStatusPastDue},
		{"late", valueobjects.SubscriptionStatusPastDue},
		{"canceled", valueobjects.SubscriptionStatusCanceledPending},
		{"canceled_pending", valueobjects.SubscriptionStatusCanceledPending},
		{"expired", valueobjects.SubscriptionStatusExpired},
		{"refunded", valueobjects.SubscriptionStatusRefunded},
		{"chargeback", valueobjects.SubscriptionStatusRefunded},
		{"unknown_status", valueobjects.SubscriptionStatusUnknown},
	}

	for _, st := range statuses {
		s.Run(st.raw, func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 86400})
			})
			mux.HandleFunc("/v1/sales/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":      "sub-001",
					"status":  st.raw,
					"product": map[string]any{"id": "prod-monthly"},
					"subscription": map[string]any{
						"current_period_start": s.now.Format(time.RFC3339),
						"current_period_end":   s.now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
					},
				})
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()
			adapter := s.buildAdapter(srv)
			result, err := adapter.FetchSubscription(s.ctx, "sub-001")
			s.Require().NoError(err)
			s.Equal(st.expected, result.Status)
		})
	}
}

func (s *KiwifyAdapterSuite) TestFetchSubscription_5xxError() {
	saleHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token", "expires_in": 86400})
	})
	mux.HandleFunc("/v1/sales/", saleHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adapter := s.buildAdapter(srv)
	_, err := adapter.FetchSubscription(s.ctx, "sub-001")
	s.Require().Error(err)
	s.True(errors.Is(err, kiwify.ErrFetchSubscriptionFailed), "deve retornar ErrFetchSubscriptionFailed em 5xx")
}

func (s *KiwifyAdapterSuite) TestFetchSubscription_429Retry() {
	var requestCount int
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token", "expires_in": 86400})
	})
	mux.HandleFunc("/v1/sales/", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "sub-001",
			"status":  "active",
			"product": map[string]any{"id": "prod-monthly"},
			"subscription": map[string]any{
				"current_period_start": s.now.Format(time.RFC3339),
				"current_period_end":   s.now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	platformClient := platformhttpclientfakes.NewClient(srv.URL, "kiwify")
	client := kiwify.NewClientWithBackoffs(platformClient, 100, 10, []time.Duration{0})
	oauth := kiwify.NewOAuthClient(platformClient, "client-id", "client-secret", 5*time.Minute)
	verifier := kiwify.NewTokenSignatureVerifier("secret-token", "X-Kiwify-Webhook-Token")
	mapper := kiwify.NewPayloadMapper(s.registry, nil)
	adapter := kiwify.NewKiwifyAdapter(client, oauth, verifier, mapper, s.registry)

	result, err := adapter.FetchSubscription(s.ctx, "sub-001")
	s.Require().NoError(err)
	s.Equal("sub-001", result.ExternalID)
	s.Equal(2, requestCount)
}

func (s *KiwifyAdapterSuite) TestFetchSubscription_404Error() {
	saleHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token", "expires_in": 86400})
	})
	mux.HandleFunc("/v1/sales/", saleHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adapter := s.buildAdapter(srv)
	_, err := adapter.FetchSubscription(s.ctx, "sub-001")
	s.Require().Error(err)
	s.ErrorIs(err, kiwify.ErrFetchSubscriptionFailed)
}

func (s *KiwifyAdapterSuite) TestFetchSubscription_BadJSON() {
	saleHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json`))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token", "expires_in": 86400})
	})
	mux.HandleFunc("/v1/sales/", saleHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adapter := s.buildAdapter(srv)
	_, err := adapter.FetchSubscription(s.ctx, "sub-001")
	s.Require().Error(err)
	s.ErrorIs(err, kiwify.ErrFetchSubscriptionFailed)
}

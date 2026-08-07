package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

type CreditsSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCreditsSuite(t *testing.T) {
	suite.Run(t, new(CreditsSuite))
}

func (s *CreditsSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CreditsSuite) buildClient(server *httptest.Server) *CreditsClient {
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(server.URL),
		httpclient.WithTarget("credits_test"),
		httpclient.WithTimeout(5*time.Second),
	)
	s.Require().NoError(err)
	return NewCreditsClient(client, "test-key")
}

func (s *CreditsSuite) TestFetch_HappyPath() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("Bearer test-key", r.Header.Get("Authorization"))
		s.Equal(http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case endpointCredits:
			_, _ = w.Write([]byte(`{"data":{"total_credits":30,"total_usage":22.5}}`))
		case endpointAuthKey:
			_, _ = w.Write([]byte(`{"data":{"usage_daily":1.25,"usage_weekly":15.5,"usage_monthly":16.0}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	snap, err := s.buildClient(server).Fetch(s.ctx)
	s.Require().NoError(err)
	s.Equal(30.0, snap.TotalCreditsUSD)
	s.Equal(22.5, snap.TotalUsageUSD)
	s.Equal(7.5, snap.RemainingUSD)
	s.Equal(1.25, snap.UsageDailyUSD)
	s.Equal(15.5, snap.UsageWeeklyUSD)
	s.Equal(16.0, snap.UsageMonthlyUSD)
}

func (s *CreditsSuite) TestFetch_RemainingNeverNegativeInPayload() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case endpointCredits:
			_, _ = w.Write([]byte(`{"data":{"total_credits":10,"total_usage":12.5}}`))
		case endpointAuthKey:
			_, _ = w.Write([]byte(`{"data":{"usage_daily":0,"usage_weekly":0,"usage_monthly":0}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	snap, err := s.buildClient(server).Fetch(s.ctx)
	s.Require().NoError(err)
	s.Equal(-2.5, snap.RemainingUSD)
}

func (s *CreditsSuite) TestFetch_CreditsEndpointFails() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := s.buildClient(server).Fetch(s.ctx)
	s.Require().Error(err)
	s.ErrorIs(err, ErrProviderUpstream)
}

func (s *CreditsSuite) TestFetch_AuthKeyEndpointFails() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == endpointCredits {
			_, _ = w.Write([]byte(`{"data":{"total_credits":30,"total_usage":22.5}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := s.buildClient(server).Fetch(s.ctx)
	s.Require().Error(err)
	s.ErrorIs(err, ErrProviderUpstream)
}

func (s *CreditsSuite) TestFetch_InvalidJSON() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	_, err := s.buildClient(server).Fetch(s.ctx)
	s.Require().Error(err)
}

func (s *CreditsSuite) TestFetch_ContextCanceled() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	_, err := s.buildClient(server).Fetch(ctx)
	s.Require().Error(err)
	s.True(errors.Is(err, context.Canceled))
}

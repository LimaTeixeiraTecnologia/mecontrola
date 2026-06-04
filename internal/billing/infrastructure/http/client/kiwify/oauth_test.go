package kiwify_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
	platformhttpclientfakes "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient/fakes"
)

func newOAuthTestClient(serverURL string) *kiwify.OAuthClient {
	platformClient := platformhttpclientfakes.NewClient(serverURL, "kiwify")
	return kiwify.NewOAuthClient(platformClient, "client-id", "client-secret", 5*time.Minute)
}

type OAuthClientSuite struct {
	suite.Suite
	ctx context.Context
}

func TestOAuthClient(t *testing.T) {
	suite.Run(t, new(OAuthClientSuite))
}

func (s *OAuthClientSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *OAuthClientSuite) TestCacheHit() {
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		s.Equal("/v1/oauth/token", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-abc",
			"expires_in":   86400,
		})
	}))
	defer srv.Close()

	client := newOAuthTestClient(srv.URL)

	token1, err := client.Token(s.ctx)
	s.Require().NoError(err)
	s.Equal("token-abc", token1)

	token2, err := client.Token(s.ctx)
	s.Require().NoError(err)
	s.Equal("token-abc", token2)

	s.EqualValues(1, atomic.LoadInt32(&requestCount), "deve fazer apenas 1 request ao endpoint oauth")
}

func (s *OAuthClientSuite) TestCacheMiss_RefreshOnExpired() {
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		token := "token-first"
		if count > 1 {
			token = "token-second"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": token,
			"expires_in":   60,
		})
	}))
	defer srv.Close()

	// expires_in 60s - safetyMargin 5min < 0 → token nasce expirado, segundo Token() força refresh.
	oauthClient := newOAuthTestClient(srv.URL)

	token1, err := oauthClient.Token(s.ctx)
	s.Require().NoError(err)
	s.Equal("token-first", token1)

	token2, err := oauthClient.Token(s.ctx)
	s.Require().NoError(err)
	s.Equal("token-second", token2)
	s.EqualValues(2, atomic.LoadInt32(&requestCount))
}

func (s *OAuthClientSuite) TestConcurrentRefresh_SingleRequest() {
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "shared-token",
			"expires_in":   86400,
		})
	}))
	defer srv.Close()

	oauthClient := newOAuthTestClient(srv.URL)

	const goroutines = 10
	tokens := make([]string, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			tokens[idx], errs[idx] = oauthClient.Token(s.ctx)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		s.Require().NoError(err, "goroutine %d retornou erro", i)
		s.Equal("shared-token", tokens[i])
	}
	s.EqualValues(1, atomic.LoadInt32(&requestCount), "apenas 1 request HTTP esperado para 10 goroutines concorrentes")
}

func (s *OAuthClientSuite) TestForceRefresh() {
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		token := "token-original"
		if count > 1 {
			token = "token-forced"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": token,
			"expires_in":   86400,
		})
	}))
	defer srv.Close()

	oauthClient := newOAuthTestClient(srv.URL)

	_, err := oauthClient.Token(s.ctx)
	s.Require().NoError(err)

	forced, err := oauthClient.ForceRefresh(s.ctx)
	s.Require().NoError(err)
	s.Equal("token-forced", forced)
	s.EqualValues(2, atomic.LoadInt32(&requestCount))
}

func (s *OAuthClientSuite) TestAuthError_4xx() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	oauthClient := newOAuthTestClient(srv.URL)

	_, err := oauthClient.Token(s.ctx)
	s.Require().Error(err)
	s.ErrorIs(err, kiwify.ErrOAuthFailed)
}

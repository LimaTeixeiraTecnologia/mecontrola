package kiwify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

type tokenProvider struct {
	mu           sync.Mutex
	httpClient   *httpclient.Client
	clientID     string
	clientSecret string
	safetyMargin time.Duration

	cachedToken string
	expiresAt   time.Time
}

func newTokenProvider(
	httpClient *httpclient.Client,
	clientID string,
	clientSecret string,
	safetyMargin time.Duration,
) *tokenProvider {
	return &tokenProvider{
		httpClient:   httpClient,
		clientID:     clientID,
		clientSecret: clientSecret,
		safetyMargin: safetyMargin,
	}
}

func (p *tokenProvider) token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cachedToken != "" && time.Now().UTC().Before(p.expiresAt) {
		return p.cachedToken, nil
	}

	tok, expiresIn, err := p.fetchToken(ctx)
	if err != nil {
		return "", err
	}

	margin := p.safetyMargin
	if margin <= 0 {
		margin = 600 * time.Second
	}
	p.cachedToken = tok
	p.expiresAt = time.Now().UTC().Add(time.Duration(expiresIn)*time.Second - margin)
	return p.cachedToken, nil
}

func (p *tokenProvider) fetchToken(ctx context.Context) (string, int, error) {
	form := url.Values{}
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	form.Set("grant_type", "client_credentials")

	resp, err := p.httpClient.Post(
		ctx,
		"/v1/oauth/token",
		strings.NewReader(form.Encode()),
		httpclient.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		httpclient.WithoutRetry(),
	)
	if err != nil {
		return "", 0, fmt.Errorf("billing/kiwify: %w: %w", ErrKiwifyAuth, err)
	}

	body, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err != nil {
		return "", 0, errors.Join(
			fmt.Errorf("billing/kiwify: ler resposta OAuth: %w", err),
			p.wrapBodyCloseError(closeErr),
		)
	}
	if closeErr != nil {
		return "", 0, p.wrapBodyCloseError(closeErr)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("billing/kiwify: %w: status %d", ErrKiwifyAuth, resp.StatusCode)
	}

	var result oauthTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("billing/kiwify: deserializar resposta OAuth: %w", err)
	}

	if result.AccessToken == "" {
		return "", 0, fmt.Errorf("billing/kiwify: %w: access_token vazio", ErrKiwifyAuth)
	}

	return result.AccessToken, result.ExpiresIn, nil
}

func (p *tokenProvider) wrapBodyCloseError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("billing/kiwify: fechar resposta OAuth: %w", err)
}

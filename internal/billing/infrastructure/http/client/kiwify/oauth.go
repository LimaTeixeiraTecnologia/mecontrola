package kiwify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	platformhttpclient "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

// ErrOAuthFailed indica falha na autenticação OAuth com a Kiwify.
var ErrOAuthFailed = errors.New("kiwify oauth: autenticação falhou")

type oauthCache struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
}

// OAuthClient obtém e cacheia token de acesso da API Kiwify.
// Cache in-memory com TTL = expires_in − safetyMargin (ADR-008).
// Re-autenticação proativa ou sob 401 (sem refresh_token — D-05 do PRD).
type OAuthClient struct {
	httpClient   *platformhttpclient.Client
	clientID     string
	clientSecret string
	safetyMargin time.Duration
	cache        oauthCache
}

// NewOAuthClient cria um OAuthClient usando o cliente outbound padronizado da plataforma.
// safetyMargin é subtraído de expires_in para re-autenticação proativa (padrão 5min).
func NewOAuthClient(
	httpClient *platformhttpclient.Client,
	clientID string,
	clientSecret string,
	safetyMargin time.Duration,
) *OAuthClient {
	return &OAuthClient{
		httpClient:   httpClient,
		clientID:     clientID,
		clientSecret: clientSecret,
		safetyMargin: safetyMargin,
	}
}

// Token retorna o token cacheado válido ou obtém um novo.
// Garante que apenas um request ao endpoint OAuth seja feito em caso de expiração concorrente.
func (c *OAuthClient) Token(ctx context.Context) (string, error) {
	if cached, ok := c.cachedToken(); ok {
		return cached, nil
	}
	return c.refresh(ctx)
}

// ForceRefresh ignora o cache e obtém um novo token.
// Deve ser chamado após receber 401 de um endpoint protegido (ADR-008).
func (c *OAuthClient) ForceRefresh(ctx context.Context) (string, error) {
	return c.forceRefreshLocked(ctx)
}

func (c *OAuthClient) forceRefreshLocked(ctx context.Context) (string, error) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	c.cache.token = ""
	c.cache.expiresAt = time.Time{}
	return c.doRefresh(ctx)
}

func (c *OAuthClient) cachedToken() (string, bool) {
	c.cache.mu.RLock()
	defer c.cache.mu.RUnlock()
	if c.cache.token == "" {
		return "", false
	}
	if time.Now().UTC().After(c.cache.expiresAt) {
		return "", false
	}
	return c.cache.token, true
}

func (c *OAuthClient) refresh(ctx context.Context) (string, error) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	if c.cache.token != "" && time.Now().UTC().Before(c.cache.expiresAt) {
		return c.cache.token, nil
	}
	return c.doRefresh(ctx)
}

// doRefresh executa o request HTTP de autenticação via cliente outbound da plataforma.
// Deve ser chamado com c.cache.mu mantido (Lock).
func (c *OAuthClient) doRefresh(ctx context.Context) (string, error) {
	body := url.Values{}
	body.Set("client_id", c.clientID)
	body.Set("client_secret", c.clientSecret)
	resp, err := c.httpClient.Post(
		ctx,
		"/v1/oauth/token",
		strings.NewReader(body.Encode()),
		platformhttpclient.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
	)
	if err != nil {
		return "", fmt.Errorf("kiwify oauth: executar request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kiwify oauth: status %d: %w", resp.StatusCode, ErrOAuthFailed)
	}
	var decoded struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("kiwify oauth: decodificar resposta: %w", err)
	}
	if decoded.AccessToken == "" {
		return "", fmt.Errorf("kiwify oauth: access_token vazio: %w", ErrOAuthFailed)
	}
	c.cache.token = decoded.AccessToken
	c.cache.expiresAt = time.Now().UTC().Add(time.Duration(decoded.ExpiresIn)*time.Second - c.safetyMargin)
	return c.cache.token, nil
}

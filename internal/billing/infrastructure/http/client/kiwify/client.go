package kiwify

import (
	"context"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	platformhttpclient "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

const (
	rateLimitInterval = 60 * time.Second
)

// Client é o cliente HTTP base para a API Kiwify, combinando o cliente outbound
// padronizado da plataforma (timeouts + retry default + observabilidade automática)
// com rate limiter local (100 req/min com burst, RF-31b).
//
// O retry de 5xx/erros de rede é responsabilidade do platform httpclient; o controle
// adicional de 429 (rate limit) com backoff específico permanece em camada superior
// (KiwifyAdapter) porque depende de fluxo OAuth (refresh em 401).
type Client struct {
	httpClient  *platformhttpclient.Client
	rateLimiter *rate.Limiter
	backoffs    []time.Duration
}

// NewClient cria um Client com retries de 429 controlados pelo caller (3 tentativas com 1s/2s/4s).
// httpClient é o cliente outbound padronizado, já configurado com BaseURL apontando para Kiwify.
func NewClient(httpClient *platformhttpclient.Client, rateLimitPerMin int, burst int) *Client {
	return NewClientWithBackoffs(httpClient, rateLimitPerMin, burst, []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
	})
}

// NewClientWithBackoffs permite customizar o cronograma de backoffs usado pelo adapter
// em respostas 429. Útil em testes para reduzir o tempo de espera.
func NewClientWithBackoffs(
	httpClient *platformhttpclient.Client,
	rateLimitPerMin int,
	burst int,
	backoffs []time.Duration,
) *Client {
	tokensPerSecond := rate.Limit(float64(rateLimitPerMin) / rateLimitInterval.Seconds())
	return &Client{
		httpClient:  httpClient,
		rateLimiter: rate.NewLimiter(tokensPerSecond, burst),
		backoffs:    backoffs,
	}
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	return c.httpClient.Do(ctx, req)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if body != nil {
		return http.NewRequestWithContext(ctx, method, path, body)
	}
	return http.NewRequestWithContext(ctx, method, path, nil)
}

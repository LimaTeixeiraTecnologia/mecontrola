package kiwify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	devkithttp "github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	devkitobs "github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

type Config struct {
	AccountID                  string
	ClientID                   string
	ClientSecret               string
	APIBaseURL                 string
	OAuthTokenSafetyMargin     time.Duration
	RateLimitMaxRequestsPerMin int
	RateLimitBurst             int
	HTTPTimeout                time.Duration
	HTTPRetryMaxAttempts       int
	HTTPRetryBackoff           time.Duration
}

type Client struct {
	httpClient    *httpclient.Client
	tokenProvider *tokenProvider
	rateLimiter   *rateLimiter
	accountID     string
	retryMax      int
	retryBackoff  time.Duration
}

func NewClient(o11y devkitobs.Observability, cfg Config) (*Client, error) {
	if o11y == nil {
		return nil, fmt.Errorf("billing/kiwify: observability é obrigatório")
	}

	baseURL := cfg.APIBaseURL
	if baseURL == "" {
		baseURL = "https://public-api.kiwify.com"
	}

	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	httpClient, err := httpclient.NewClient(
		o11y,
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("kiwify"),
		httpclient.WithTimeout(timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("billing/kiwify: criar http client: %w", err)
	}

	safetyMargin := cfg.OAuthTokenSafetyMargin
	if safetyMargin <= 0 {
		safetyMargin = 600 * time.Second
	}

	maxRequests := cfg.RateLimitMaxRequestsPerMin
	if maxRequests <= 0 {
		maxRequests = 100
	}

	burst := cfg.RateLimitBurst
	if burst <= 0 {
		burst = 10
	}

	retryMax := cfg.HTTPRetryMaxAttempts
	if retryMax <= 0 {
		retryMax = 3
	}

	retryBackoff := cfg.HTTPRetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = time.Second
	}

	return &Client{
		httpClient:    httpClient,
		tokenProvider: newTokenProvider(httpClient, cfg.ClientID, cfg.ClientSecret, safetyMargin),
		rateLimiter:   newRateLimiter(maxRequests, burst),
		accountID:     cfg.AccountID,
		retryMax:      retryMax,
		retryBackoff:  retryBackoff,
	}, nil
}

func (c *Client) ListSalesUpdatedSince(ctx context.Context, windowStart time.Time, windowEnd time.Time, page int) (interfaces.KiwifySalePage, error) {
	if err := c.rateLimiter.wait(ctx); err != nil {
		return interfaces.KiwifySalePage{}, err
	}

	token, err := c.tokenProvider.token(ctx)
	if err != nil {
		return interfaces.KiwifySalePage{}, err
	}

	startStr := windowStart.UTC().Format(time.RFC3339)
	endStr := windowEnd.UTC().Format(time.RFC3339)
	pageStr := strconv.Itoa(page)

	path := fmt.Sprintf(
		"/v1/sales?updated_at_start_date=%s&updated_at_end_date=%s&page_number=%s",
		startStr,
		endStr,
		pageStr,
	)

	resp, err := c.httpClient.Get(ctx, path,
		httpclient.WithHeader("Authorization", "Bearer "+token),
		httpclient.WithHeader("x-kiwify-account-id", c.accountID),
		httpclient.WithRetry(c.retryMax, c.retryBackoff, devkithttp.IdempotentRetryPolicy),
	)
	if err != nil {
		return interfaces.KiwifySalePage{}, fmt.Errorf("billing/kiwify: listar vendas: %w", err)
	}

	statusErr := c.checkStatus(resp)
	body, bodyErr := c.readResponseBody(resp, "list sales")
	if statusErr != nil {
		return interfaces.KiwifySalePage{}, errors.Join(statusErr, bodyErr)
	}
	if bodyErr != nil {
		return interfaces.KiwifySalePage{}, bodyErr
	}

	var result salesListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return interfaces.KiwifySalePage{}, fmt.Errorf("billing/kiwify: deserializar list sales: %w", err)
	}

	sales := make([]interfaces.KiwifySale, 0, len(result.Data))
	for _, s := range result.Data {
		sales = append(sales, mapSale(s))
	}

	return interfaces.KiwifySalePage{
		Sales:   sales,
		HasMore: result.HasMore,
	}, nil
}

func (c *Client) GetSale(ctx context.Context, saleID string) (interfaces.KiwifySale, error) {
	if err := c.rateLimiter.wait(ctx); err != nil {
		return interfaces.KiwifySale{}, err
	}

	token, err := c.tokenProvider.token(ctx)
	if err != nil {
		return interfaces.KiwifySale{}, err
	}

	path := fmt.Sprintf("/v1/sales/%s", saleID)

	resp, err := c.httpClient.Get(ctx, path,
		httpclient.WithHeader("Authorization", "Bearer "+token),
		httpclient.WithHeader("x-kiwify-account-id", c.accountID),
		httpclient.WithRetry(c.retryMax, c.retryBackoff, devkithttp.IdempotentRetryPolicy),
	)
	if err != nil {
		return interfaces.KiwifySale{}, fmt.Errorf("billing/kiwify: obter venda %s: %w", saleID, err)
	}

	statusErr := c.checkStatus(resp)
	body, bodyErr := c.readResponseBody(resp, "get sale")
	if statusErr != nil {
		return interfaces.KiwifySale{}, errors.Join(statusErr, bodyErr)
	}
	if bodyErr != nil {
		return interfaces.KiwifySale{}, bodyErr
	}

	var result saleResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return interfaces.KiwifySale{}, fmt.Errorf("billing/kiwify: deserializar get sale: %w", err)
	}

	return mapSale(result), nil
}

func (c *Client) readResponseBody(resp *http.Response, op string) ([]byte, error) {
	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, errors.Join(
			fmt.Errorf("billing/kiwify: ler resposta %s: %w", op, readErr),
			c.wrapBodyCloseError(op, closeErr),
		)
	}
	if closeErr != nil {
		return nil, c.wrapBodyCloseError(op, closeErr)
	}

	return body, nil
}

func (c *Client) wrapBodyCloseError(op string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("billing/kiwify: fechar resposta %s: %w", op, err)
}

func (c *Client) checkStatus(resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("billing/kiwify: %w: status 429", ErrKiwifyRateLimited)
	case resp.StatusCode >= 500:
		return fmt.Errorf("billing/kiwify: %w: status %d", ErrKiwifyServer, resp.StatusCode)
	case resp.StatusCode >= 400:
		return fmt.Errorf("billing/kiwify: %w: status %d", ErrKiwifyBadRequest, resp.StatusCode)
	}
	return nil
}

func mapSale(s saleResponse) interfaces.KiwifySale {
	funnelToken := s.Tracking.SCK
	if funnelToken == "" {
		funnelToken = s.Tracking.S1
	}
	if funnelToken == "" {
		funnelToken = s.Tracking.Src
	}
	sale := interfaces.KiwifySale{
		ID:                 s.ID,
		KiwifyProductID:    s.ProductID,
		OrderID:            s.Reference,
		SubscriptionID:     s.SubscriptionID,
		ParentOrderID:      s.ParentOrderID,
		Status:             s.Status,
		SaleType:           s.SaleType,
		PaymentMethod:      s.PaymentMethod,
		FunnelToken:        funnelToken,
		CustomerEmail:      s.Customer.Email,
		CustomerMobileE164: s.Customer.Phone,
		OccurredAt:         s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
	}
	if s.RefundedAt != nil {
		sale.RefundedAt = *s.RefundedAt
	}
	return sale
}

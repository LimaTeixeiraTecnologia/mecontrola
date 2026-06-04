package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	devkithttp "github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	devkitobs "github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type Client struct {
	inner               *devkithttp.ObservableClient
	baseURL             *url.URL
	target              string
	defaultRetryEnabled bool
	defaultRetryMax     int
	defaultRetryBackoff time.Duration
}

func NewClient(o11y devkitobs.Observability, opts ...Option) (*Client, error) {
	if o11y == nil {
		return nil, ErrObservabilityRequired
	}

	cfg := clientOptions{}
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	devkitOpts := make([]devkithttp.ClientOption, 0, 2)
	if cfg.timeout > 0 {
		devkitOpts = append(devkitOpts, devkithttp.WithClientTimeout(cfg.timeout))
	}
	if cfg.maxBodySize != nil {
		devkitOpts = append(devkitOpts, devkithttp.WithMaxBodySize(*cfg.maxBodySize))
	}

	inner, err := devkithttp.NewObservableClient(o11y, devkitOpts...)
	if err != nil {
		return nil, fmt.Errorf("httpclient: %w", err)
	}

	return &Client{
		inner:               inner,
		baseURL:             cfg.baseURL,
		target:              cfg.target,
		defaultRetryEnabled: cfg.defaultRetryEnabled,
		defaultRetryMax:     cfg.defaultRetryMax,
		defaultRetryBackoff: cfg.defaultRetryBackoff,
	}, nil
}

func (c *Client) Target() string {
	return c.target
}

func (c *Client) Get(ctx context.Context, path string, opts ...RequestOption) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

func (c *Client) Head(ctx context.Context, path string, opts ...RequestOption) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodHead, path, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

func (c *Client) Post(ctx context.Context, path string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

func (c *Client) Put(ctx context.Context, path string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

func (c *Client) Patch(ctx context.Context, path string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

func (c *Client) Delete(ctx context.Context, path string, opts ...RequestOption) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, opts...)
}

func (c *Client) Do(ctx context.Context, req *http.Request, opts ...RequestOption) (*http.Response, error) {
	if req == nil {
		return nil, ErrNilRequest
	}

	if c.baseURL != nil && req.URL != nil && !req.URL.IsAbs() {
		resolved := c.baseURL.ResolveReference(req.URL)
		req.URL = resolved
		req.Host = resolved.Host
	}

	state := requestState{}
	for _, opt := range opts {
		opt(&state)
	}

	devkitOpts := state.devkit
	if c.defaultRetryEnabled && !state.overrideRetry && isSafeMethod(req.Method) {
		retryOpt := devkithttp.WithRetry(
			c.defaultRetryMax,
			c.defaultRetryBackoff,
			devkithttp.DefaultNewRetryPolicy,
		)
		devkitOpts = append([]devkithttp.RequestOption{retryOpt}, devkitOpts...)
	}

	return c.inner.Do(ctx, req, devkitOpts...)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	parsed, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("httpclient: path inválido %q: %w", path, err)
	}

	var fullURL string
	switch {
	case parsed.IsAbs():
		fullURL = parsed.String()
	case c.baseURL != nil:
		fullURL = c.baseURL.ResolveReference(parsed).String()
	default:
		return nil, ErrBaseURLRequired
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("httpclient: construir request %s %s: %w", method, fullURL, err)
	}
	return req, nil
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

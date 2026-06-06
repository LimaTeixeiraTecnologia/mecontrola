package httpclient

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	devkithttp "github.com/JailtonJunior94/devkit-go/pkg/httpclient"
)

type Option func(*clientOptions) error

type clientOptions struct {
	timeout             time.Duration
	baseURL             *url.URL
	target              string
	defaultRetryEnabled bool
	defaultRetryMax     int
	defaultRetryBackoff time.Duration
	maxBodySize         *int64
}

func WithTimeout(d time.Duration) Option {
	return func(o *clientOptions) error {
		if d <= 0 {
			return fmt.Errorf("httpclient: WithTimeout requer duração positiva, recebido %v", d)
		}
		o.timeout = d
		return nil
	}
}

func WithBaseURL(raw string) Option {
	return func(o *clientOptions) error {
		if raw == "" {
			return ErrBaseURLRequired
		}
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("httpclient: BaseURL inválido %q: %w", raw, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("httpclient: BaseURL %q deve incluir scheme e host", raw)
		}
		u.Path = strings.TrimRight(u.Path, "/")
		o.baseURL = u
		return nil
	}
}

func WithTarget(name string) Option {
	return func(o *clientOptions) error {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return fmt.Errorf("httpclient: WithTarget requer nome não-vazio")
		}
		o.target = trimmed
		return nil
	}
}

func WithDefaultRetry(maxAttempts int, backoff time.Duration) Option {
	return func(o *clientOptions) error {
		if maxAttempts <= 0 {
			o.defaultRetryEnabled = false
			return nil
		}
		if backoff <= 0 {
			return fmt.Errorf("httpclient: WithDefaultRetry requer backoff positivo, recebido %v", backoff)
		}
		o.defaultRetryEnabled = true
		o.defaultRetryMax = maxAttempts
		o.defaultRetryBackoff = backoff
		return nil
	}
}

func WithMaxBodySize(size int64) Option {
	return func(o *clientOptions) error {
		if size < 0 {
			return fmt.Errorf("httpclient: WithMaxBodySize não pode ser negativo, recebido %d", size)
		}
		s := size
		o.maxBodySize = &s
		return nil
	}
}

type RequestOption func(*requestState)

type requestState struct {
	overrideRetry bool
	devkit        []devkithttp.RequestOption
}

func WithHeader(key, value string) RequestOption {
	return func(s *requestState) {
		s.devkit = append(s.devkit, devkithttp.WithHeader(key, value))
	}
}

func WithHeaders(headers map[string]string) RequestOption {
	return func(s *requestState) {
		s.devkit = append(s.devkit, devkithttp.WithHeaders(headers))
	}
}

func WithRetry(maxAttempts int, backoff time.Duration, policy devkithttp.RetryPolicy) RequestOption {
	return func(s *requestState) {
		s.overrideRetry = true
		s.devkit = append(s.devkit, devkithttp.WithRetry(maxAttempts, backoff, policy))
	}
}

func WithoutRetry() RequestOption {
	return func(s *requestState) {
		s.overrideRetry = true
	}
}

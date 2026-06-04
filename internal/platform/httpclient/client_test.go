package httpclient_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	devkithttp "github.com/JailtonJunior94/devkit-go/pkg/httpclient"
	devkitfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

func newTestClient(t *testing.T, baseURL string, opts ...httpclient.Option) *httpclient.Client {
	t.Helper()
	all := append([]httpclient.Option{
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("test"),
	}, opts...)
	c, err := httpclient.NewClient(devkitfake.NewProvider(), all...)
	require.NoError(t, err)
	return c
}

func TestNewClient_RequiresObservability(t *testing.T) {
	_, err := httpclient.NewClient(nil)
	require.ErrorIs(t, err, httpclient.ErrObservabilityRequired)
}

func TestNewClient_RejectsInvalidBaseURL(t *testing.T) {
	cases := map[string]string{
		"vazio":      "",
		"sem_scheme": "api.example.com",
		"sem_host":   "https://",
		"malformado": "://bad",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := httpclient.NewClient(devkitfake.NewProvider(), httpclient.WithBaseURL(raw))
			require.Error(t, err)
		})
	}
}

func TestNewClient_RejectsInvalidTimeout(t *testing.T) {
	_, err := httpclient.NewClient(devkitfake.NewProvider(), httpclient.WithTimeout(0))
	require.Error(t, err)
}

func TestNewClient_RejectsNegativeMaxBodySize(t *testing.T) {
	_, err := httpclient.NewClient(devkitfake.NewProvider(), httpclient.WithMaxBodySize(-1))
	require.Error(t, err)
}

func TestNewClient_DefaultRetryRequiresPositiveBackoffWhenAttemptsSet(t *testing.T) {
	_, err := httpclient.NewClient(devkitfake.NewProvider(), httpclient.WithDefaultRetry(3, 0))
	require.Error(t, err)
}

func TestNewClient_DefaultRetryAttemptsZeroDisables(t *testing.T) {
	_, err := httpclient.NewClient(devkitfake.NewProvider(), httpclient.WithDefaultRetry(0, 0))
	require.NoError(t, err)
}

func TestClient_Get_ResolvesRelativePathAgainstBaseURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	resp, err := client.Get(context.Background(), "/v1/sales/abc")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, "/v1/sales/abc", gotPath)
}

func TestClient_Get_RetriesOn503AndReturnsSuccess(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, httpclient.WithDefaultRetry(3, 5*time.Millisecond))
	resp, err := client.Get(context.Background(), "/")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.GreaterOrEqual(t, attempts.Load(), int32(3))
}

func TestClient_Post_DoesNotRetryByDefault(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, httpclient.WithDefaultRetry(5, 5*time.Millisecond))
	resp, err := client.Post(context.Background(), "/", strings.NewReader(`{}`))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, int32(1), attempts.Load(), "POST não pode retentar automaticamente")
}

func TestClient_Get_WithoutRetryOptOut(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL, httpclient.WithDefaultRetry(5, 5*time.Millisecond))
	resp, err := client.Get(context.Background(), "/", httpclient.WithoutRetry())
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, int32(1), attempts.Load())
}

func TestClient_Post_WithExplicitRetryRetries(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	resp, err := client.Post(
		context.Background(),
		"/",
		strings.NewReader(`{}`),
		httpclient.WithRetry(3, 5*time.Millisecond, devkithttp.DefaultNewRetryPolicy),
	)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClient_Get_HeadersPropagate(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	resp, err := client.Get(context.Background(), "/", httpclient.WithHeader("Authorization", "Bearer abc"))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, "Bearer abc", gotAuth)
}

func TestClient_Do_ContextCancelStopsRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, "/")
	require.Error(t, err)
	require.True(
		t,
		errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context"),
		"erro esperado relacionado a context, recebido: %v",
		err,
	)
}

func TestClient_Get_RequiresBaseURLForRelativePath(t *testing.T) {
	c, err := httpclient.NewClient(devkitfake.NewProvider(), httpclient.WithTarget("noop"))
	require.NoError(t, err)

	_, err = c.Get(context.Background(), "/relative")
	require.ErrorIs(t, err, httpclient.ErrBaseURLRequired)
}

func TestClient_Do_NilRequestReturnsError(t *testing.T) {
	c := newTestClient(t, "http://example.com")
	_, err := c.Do(context.Background(), nil)
	require.ErrorIs(t, err, httpclient.ErrNilRequest)
}

func TestClient_Get_AbsolutePathOverridesBaseURL(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	client := newTestClient(t, "http://placeholder.invalid")
	resp, err := client.Get(context.Background(), srv.URL+"/abs")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, int32(1), hits.Load())
}

func TestClient_Target_Reflects_WithTarget(t *testing.T) {
	c, err := httpclient.NewClient(
		devkitfake.NewProvider(),
		httpclient.WithBaseURL("http://example.com"),
		httpclient.WithTarget("kiwify"),
	)
	require.NoError(t, err)
	require.Equal(t, "kiwify", c.Target())
}

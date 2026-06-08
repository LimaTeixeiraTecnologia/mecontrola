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
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

type ClientSuite struct {
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupTest() {}

func (s *ClientSuite) newTestClient(baseURL string, opts ...httpclient.Option) *httpclient.Client {
	all := append([]httpclient.Option{
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("test"),
	}, opts...)
	client, err := httpclient.NewClient(devkitfake.NewProvider(), all...)
	s.Require().NoError(err)
	return client
}

func (s *ClientSuite) TestNewClient() {
	type args struct {
		useNilObservability bool
		opts                []httpclient.Option
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(*httpclient.Client, error)
	}{
		{
			name:  "deve exigir observability",
			args:  args{useNilObservability: true},
			setup: func() {},
			expect: func(_ *httpclient.Client, err error) {
				s.ErrorIs(err, httpclient.ErrObservabilityRequired)
			},
		},
		{
			name:  "deve rejeitar base url vazia",
			args:  args{opts: []httpclient.Option{httpclient.WithBaseURL("")}},
			setup: func() {},
			expect: func(_ *httpclient.Client, err error) {
				s.Error(err)
			},
		},
		{
			name:  "deve rejeitar base url sem scheme",
			args:  args{opts: []httpclient.Option{httpclient.WithBaseURL("api.example.com")}},
			setup: func() {},
			expect: func(_ *httpclient.Client, err error) {
				s.Error(err)
			},
		},
		{
			name:  "deve rejeitar timeout invalido",
			args:  args{opts: []httpclient.Option{httpclient.WithTimeout(0)}},
			setup: func() {},
			expect: func(_ *httpclient.Client, err error) {
				s.Error(err)
			},
		},
		{
			name:  "deve rejeitar max body size negativo",
			args:  args{opts: []httpclient.Option{httpclient.WithMaxBodySize(-1)}},
			setup: func() {},
			expect: func(_ *httpclient.Client, err error) {
				s.Error(err)
			},
		},
		{
			name:  "deve exigir backoff positivo quando retry padrao tiver tentativas",
			args:  args{opts: []httpclient.Option{httpclient.WithDefaultRetry(3, 0)}},
			setup: func() {},
			expect: func(_ *httpclient.Client, err error) {
				s.Error(err)
			},
		},
		{
			name:  "deve aceitar retry padrao desabilitado",
			args:  args{opts: []httpclient.Option{httpclient.WithDefaultRetry(0, 0)}},
			setup: func() {},
			expect: func(client *httpclient.Client, err error) {
				s.NoError(err)
				s.NotNil(client)
			},
		},
		{
			name: "deve refletir target configurado",
			args: args{opts: []httpclient.Option{
				httpclient.WithBaseURL("http://example.com"),
				httpclient.WithTarget("kiwify"),
			}},
			setup: func() {},
			expect: func(client *httpclient.Client, err error) {
				s.NoError(err)
				s.Equal("kiwify", client.Target())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			if scenario.args.useNilObservability {
				client, err := httpclient.NewClient(nil, scenario.args.opts...)
				scenario.expect(client, err)
				return
			}

			client, err := httpclient.NewClient(devkitfake.NewProvider(), scenario.args.opts...)

			scenario.expect(client, err)
		})
	}
}

func (s *ClientSuite) TestGet() {
	type args struct {
		ctx  context.Context
		path string
		opts []httpclient.RequestOption
	}

	type observed struct {
		attempts atomic.Int32
		gotPath  string
		gotAuth  string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(*observed) *httpclient.Client
		expect func(*http.Response, error, *observed)
	}{
		{
			name: "deve resolver caminho relativo a partir da base url",
			args: args{ctx: context.Background(), path: "/v1/sales/abc"},
			setup: func(state *observed) *httpclient.Client {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					state.gotPath = r.URL.Path
					w.WriteHeader(http.StatusOK)
				}))
				s.T().Cleanup(server.Close)
				return s.newTestClient(server.URL)
			},
			expect: func(response *http.Response, err error, state *observed) {
				s.NoError(err)
				s.Require().NotNil(response)
				s.NoError(response.Body.Close())
				s.Equal("/v1/sales/abc", state.gotPath)
			},
		},
		{
			name: "deve retentar get em 503 e retornar sucesso",
			args: args{ctx: context.Background(), path: "/"},
			setup: func(state *observed) *httpclient.Client {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if state.attempts.Add(1) < 3 {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					w.WriteHeader(http.StatusOK)
				}))
				s.T().Cleanup(server.Close)
				return s.newTestClient(server.URL, httpclient.WithDefaultRetry(3, 5*time.Millisecond))
			},
			expect: func(response *http.Response, err error, state *observed) {
				s.NoError(err)
				s.Require().NotNil(response)
				s.NoError(response.Body.Close())
				s.Equal(http.StatusOK, response.StatusCode)
				s.GreaterOrEqual(state.attempts.Load(), int32(3))
			},
		},
		{
			name: "deve respeitar opt out de retry",
			args: args{ctx: context.Background(), path: "/", opts: []httpclient.RequestOption{httpclient.WithoutRetry()}},
			setup: func(state *observed) *httpclient.Client {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					state.attempts.Add(1)
					w.WriteHeader(http.StatusServiceUnavailable)
				}))
				s.T().Cleanup(server.Close)
				return s.newTestClient(server.URL, httpclient.WithDefaultRetry(5, 5*time.Millisecond))
			},
			expect: func(response *http.Response, err error, state *observed) {
				s.NoError(err)
				s.Require().NotNil(response)
				s.NoError(response.Body.Close())
				s.Equal(int32(1), state.attempts.Load())
			},
		},
		{
			name: "deve propagar headers para requisicao",
			args: args{
				ctx:  context.Background(),
				path: "/",
				opts: []httpclient.RequestOption{httpclient.WithHeader("Authorization", "Bearer abc")},
			},
			setup: func(state *observed) *httpclient.Client {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					state.gotAuth = r.Header.Get("Authorization")
					w.WriteHeader(http.StatusOK)
				}))
				s.T().Cleanup(server.Close)
				return s.newTestClient(server.URL)
			},
			expect: func(response *http.Response, err error, state *observed) {
				s.NoError(err)
				s.Require().NotNil(response)
				s.NoError(response.Body.Close())
				s.Equal("Bearer abc", state.gotAuth)
			},
		},
		{
			name: "deve exigir base url para caminho relativo",
			args: args{ctx: context.Background(), path: "/relative"},
			setup: func(*observed) *httpclient.Client {
				client, err := httpclient.NewClient(devkitfake.NewProvider(), httpclient.WithTarget("noop"))
				s.Require().NoError(err)
				return client
			},
			expect: func(_ *http.Response, err error, _ *observed) {
				s.ErrorIs(err, httpclient.ErrBaseURLRequired)
			},
		},
		{
			name: "deve permitir caminho absoluto sobrepor base url",
			args: args{ctx: context.Background(), path: ""},
			setup: func(state *observed) *httpclient.Client {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					state.attempts.Add(1)
					_, _ = io.WriteString(w, "ok")
				}))
				s.T().Cleanup(server.Close)
				client := s.newTestClient("http://placeholder.invalid")
				state.gotPath = server.URL + "/abs"
				return client
			},
			expect: func(response *http.Response, err error, state *observed) {
				s.NoError(err)
				s.Require().NotNil(response)
				s.NoError(response.Body.Close())
				s.Equal(int32(1), state.attempts.Load())
			},
		},
		{
			name: "deve cancelar requisicao quando contexto expirar",
			args: func() args {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				s.T().Cleanup(cancel)
				return args{ctx: ctx, path: "/"}
			}(),
			setup: func(*observed) *httpclient.Client {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					time.Sleep(200 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				}))
				s.T().Cleanup(server.Close)
				return s.newTestClient(server.URL)
			},
			expect: func(_ *http.Response, err error, _ *observed) {
				s.Error(err)
				s.True(errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context"))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := &observed{}
			client := scenario.setup(state)
			path := scenario.args.path
			if scenario.name == "deve permitir caminho absoluto sobrepor base url" {
				path = state.gotPath
			}

			response, err := client.Get(scenario.args.ctx, path, scenario.args.opts...)

			scenario.expect(response, err, state)
		})
	}
}

func (s *ClientSuite) TestPost() {
	type args struct {
		ctx  context.Context
		body string
		opts []httpclient.RequestOption
	}

	type observed struct {
		attempts atomic.Int32
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(*observed) *httpclient.Client
		expect func(*http.Response, error, *observed)
	}{
		{
			name: "deve nao retentar post por padrao",
			args: args{ctx: context.Background(), body: `{}`},
			setup: func(state *observed) *httpclient.Client {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					state.attempts.Add(1)
					w.WriteHeader(http.StatusServiceUnavailable)
				}))
				s.T().Cleanup(server.Close)
				return s.newTestClient(server.URL, httpclient.WithDefaultRetry(5, 5*time.Millisecond))
			},
			expect: func(response *http.Response, err error, state *observed) {
				s.NoError(err)
				s.Require().NotNil(response)
				s.NoError(response.Body.Close())
				s.Equal(http.StatusServiceUnavailable, response.StatusCode)
				s.Equal(int32(1), state.attempts.Load())
			},
		},
		{
			name: "deve retentar post quando configurado explicitamente",
			args: args{
				ctx:  context.Background(),
				body: `{}`,
				opts: []httpclient.RequestOption{httpclient.WithRetry(3, 5*time.Millisecond, devkithttp.DefaultRetryPolicy)},
			},
			setup: func(state *observed) *httpclient.Client {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if state.attempts.Add(1) < 2 {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					w.WriteHeader(http.StatusOK)
				}))
				s.T().Cleanup(server.Close)
				return s.newTestClient(server.URL)
			},
			expect: func(response *http.Response, err error, state *observed) {
				s.NoError(err)
				s.Require().NotNil(response)
				s.NoError(response.Body.Close())
				s.Equal(http.StatusOK, response.StatusCode)
				s.GreaterOrEqual(state.attempts.Load(), int32(2))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			state := &observed{}
			client := scenario.setup(state)

			response, err := client.Post(scenario.args.ctx, "/", strings.NewReader(scenario.args.body), scenario.args.opts...)

			scenario.expect(response, err, state)
		})
	}
}

func (s *ClientSuite) TestDo() {
	type args struct {
		ctx context.Context
		req *http.Request
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func() *httpclient.Client
		expect func(*http.Response, error)
	}{
		{
			name:  "deve retornar erro para request nil",
			args:  args{ctx: context.Background()},
			setup: func() *httpclient.Client { return s.newTestClient("http://example.com") },
			expect: func(_ *http.Response, err error) {
				s.ErrorIs(err, httpclient.ErrNilRequest)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			client := scenario.setup()
			response, err := client.Do(scenario.args.ctx, scenario.args.req)
			scenario.expect(response, err)
		})
	}
}

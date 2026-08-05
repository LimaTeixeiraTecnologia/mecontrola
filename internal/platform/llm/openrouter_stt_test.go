package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

type STTSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSTTSuite(t *testing.T) {
	suite.Run(t, new(STTSuite))
}

func (s *STTSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *STTSuite) buildTranscriber(server *httptest.Server, cfg Config) Transcriber {
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(server.URL),
		httpclient.WithTarget("stt_test"),
		httpclient.WithTimeout(5*time.Second),
	)
	s.Require().NoError(err)

	cfg.BaseURL = server.URL
	if cfg.APIKey == "" {
		cfg.APIKey = "test-key"
	}
	if cfg.HTTPReferer == "" {
		cfg.HTTPReferer = "https://example.com"
	}
	if cfg.XTitle == "" {
		cfg.XTitle = "TestApp"
	}
	return NewOpenRouterTranscriber(client, cfg, fake.NewProvider())
}

func (s *STTSuite) baseRequest() TranscriptionRequest {
	return TranscriptionRequest{
		Model:           "openai/whisper-large-v3",
		Audio:           []byte{0x01, 0x02, 0x03},
		Format:          AudioFormatOGG,
		Language:        "pt",
		Temperature:     0,
		DurationMs:      5000,
		MaxCostMicrousd: 2000,
	}
}

func (s *STTSuite) TestTranscribe_HappyPath() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodPost, r.Method)
		s.Equal(endpointAudioTranscriptions, r.URL.Path)
		s.Equal("Bearer test-key", r.Header.Get("Authorization"))
		s.Equal("application/json", r.Header.Get("Content-Type"))

		_, _ = w.Write([]byte(`{"text":"compra de cinquenta reais no mercado","language":"pt","usage":{"seconds":5.0,"cost":0.0001}}`))
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{})
	resp, err := sut.Transcribe(s.ctx, s.baseRequest())

	s.NoError(err)
	s.Equal("compra de cinquenta reais no mercado", resp.Text)
	s.Equal("pt", resp.Language)
	s.False(resp.Truncated)
	s.Require().NotNil(resp.CostMicrousd)
	s.Equal(int64(100), *resp.CostMicrousd)
	s.Equal("openrouter", resp.Provider)
	s.Require().NotNil(resp.DurationMs)
	s.Equal(5000, *resp.DurationMs)
}

func (s *STTSuite) TestTranscribe_ProviderOmitsLanguageFallsBackToRequested() {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"text":"cinquenta reais no mercado","usage":{"seconds":4.713375,"cost":0.000117834375}}`))
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{})
	resp, err := sut.Transcribe(s.ctx, s.baseRequest())

	s.NoError(err)
	s.Equal("pt", resp.Language,
		"nenhum modelo STT do OpenRouter retorna language; sem fallback o gate de idioma rejeitaria 100% dos audios")

	var sent sttRequestWire
	s.Require().NoError(json.Unmarshal(body, &sent))
	s.Equal("pt", sent.Language, "o fallback de idioma depende de enviarmos language no request")
	s.Equal("ogg", sent.InputAudio.Format)
	s.Equal(float64(0), sent.Temperature)
	s.Equal("openai/whisper-large-v3", sent.Model)
}

func (s *STTSuite) TestTranscribe_EmptyText() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"text":"   ","usage":{"seconds":1.0,"cost":0.00001}}`))
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{})
	_, err := sut.Transcribe(s.ctx, s.baseRequest())

	s.Error(err)
	s.True(errors.Is(err, ErrSTTEmptyText))
}

func (s *STTSuite) TestTranscribe_InvalidJSON() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{})
	_, err := sut.Transcribe(s.ctx, s.baseRequest())

	s.Error(err)
	s.True(errors.Is(err, ErrSTTInvalidResponse))
}

func (s *STTSuite) TestTranscribe_PostflightCostExceeded() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"text":"algum texto valido","usage":{"seconds":5.0,"cost":0.5}}`))
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{})
	_, err := sut.Transcribe(s.ctx, s.baseRequest())

	s.Error(err)
	s.True(errors.Is(err, ErrSTTCostExceeded))
}

func (s *STTSuite) TestTranscribe_HTTPStatusCodes_Table() {
	cases := []struct {
		name       string
		statusCode int
		expectErr  error
	}{
		{"bad_request", http.StatusBadRequest, ErrSTTUpstream},
		{"unauthorized", http.StatusUnauthorized, ErrSTTUpstream},
		{"forbidden", http.StatusForbidden, ErrSTTUpstream},
		{"payment_required", http.StatusPaymentRequired, ErrSTTUpstream},
		{"model_not_found", http.StatusNotFound, ErrSTTModelUnavailable},
		{"too_many_requests", http.StatusTooManyRequests, ErrSTTUpstream},
		{"server_error", http.StatusInternalServerError, ErrSTTUpstream},
		{"bad_gateway", http.StatusBadGateway, ErrSTTUpstream},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
			}))
			defer server.Close()

			sut := s.buildTranscriber(server, Config{})
			_, err := sut.Transcribe(s.ctx, s.baseRequest())

			s.Error(err)
			s.True(errors.Is(err, tc.expectErr), "expected %v got %v", tc.expectErr, err)
		})
	}
}

func (s *STTSuite) TestTranscribe_Timeout() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"text":"ok","usage":{"seconds":1.0,"cost":0.00001}}`))
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{STTTimeout: 20 * time.Millisecond})
	_, err := sut.Transcribe(s.ctx, s.baseRequest())

	s.Error(err)
	s.True(errors.Is(err, ErrSTTTimeout), "expected ErrSTTTimeout got %v", err)
}

func (s *STTSuite) TestTranscribe_PreflightCostExceeded_NoHTTPCall() {
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		_, _ = w.Write([]byte(`{"text":"nunca deveria chegar aqui","usage":{"seconds":1.0,"cost":0.00001}}`))
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{})
	req := s.baseRequest()
	req.DurationMs = 60_000
	req.MaxCostMicrousd = 100

	_, err := sut.Transcribe(s.ctx, req)

	s.Error(err)
	s.True(errors.Is(err, ErrSTTCostPreflightExceeded))
	s.False(called.Load(), "http call must not happen when preflight rejects")
}

func (s *STTSuite) TestTranscribe_UnsupportedFormat_NoHTTPCall() {
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{})
	req := s.baseRequest()
	req.Format = AudioFormat("wav")

	_, err := sut.Transcribe(s.ctx, req)

	s.Error(err)
	s.True(errors.Is(err, ErrSTTFormatUnsupported))
	s.False(called.Load())
}

func (s *STTSuite) TestTranscribe_MissingMaxCost_NoHTTPCall() {
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{})
	req := s.baseRequest()
	req.MaxCostMicrousd = 0

	_, err := sut.Transcribe(s.ctx, req)

	s.Error(err)
	s.True(errors.Is(err, ErrSTTMaxCostRequired), "expected ErrSTTMaxCostRequired got %v", err)
	s.False(called.Load(), "http call must not happen when max cost is unset (fail closed)")
}

func (s *STTSuite) TestTranscribe_MissingDuration_NoHTTPCall() {
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
	}))
	defer server.Close()

	sut := s.buildTranscriber(server, Config{})
	req := s.baseRequest()
	req.DurationMs = 0

	_, err := sut.Transcribe(s.ctx, req)

	s.Error(err)
	s.True(errors.Is(err, ErrSTTDurationRequired))
	s.False(called.Load())
}

func TestEstimateSTTPreflightCostMicrousd(t *testing.T) {
	cases := []struct {
		name       string
		durationMs int
		rate       int64
		expect     int64
	}{
		{"zero_duration", 0, 25, 0},
		{"default_rate_when_non_positive", 1000, 0, defaultSTTPreflightRateMicrousdPerSecond},
		{"one_second_custom_rate", 1000, 10, 10},
		{"rounds_up_partial_second", 1500, 10, 20},
		{"sixty_seconds_default_rate", 60_000, defaultSTTPreflightRateMicrousdPerSecond, 1500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EstimateSTTPreflightCostMicrousd(tc.durationMs, tc.rate)
			if got != tc.expect {
				t.Fatalf("expected %d got %d", tc.expect, got)
			}
		})
	}
}

func TestDecideSTTPreflightCost(t *testing.T) {
	t.Run("rejects_zero_duration", func(t *testing.T) {
		err := DecideSTTPreflightCost(0, 2000, 34)
		if !errors.Is(err, ErrSTTDurationRequired) {
			t.Fatalf("expected ErrSTTDurationRequired got %v", err)
		}
	})

	t.Run("fail_closed_when_max_zero", func(t *testing.T) {
		err := DecideSTTPreflightCost(60_000, 0, 34)
		if !errors.Is(err, ErrSTTCostPreflightExceeded) {
			t.Fatalf("expected ErrSTTCostPreflightExceeded got %v", err)
		}
	})

	t.Run("approves_within_budget", func(t *testing.T) {
		err := DecideSTTPreflightCost(5000, 2000, 25)
		if err != nil {
			t.Fatalf("expected nil got %v", err)
		}
	})

	t.Run("approves_max_duration_audio_under_techspec_budget", func(t *testing.T) {
		err := DecideSTTPreflightCost(60_000, 2000, defaultSTTPreflightRateMicrousdPerSecond)
		if err != nil {
			t.Fatalf("audio de 60s deve caber em AGENT_AUDIO_MAX_COST_MICROUSD=2000 (RF-07): %v", err)
		}
	})

	t.Run("rejects_over_budget", func(t *testing.T) {
		err := DecideSTTPreflightCost(60_000, 100, 34)
		if !errors.Is(err, ErrSTTCostPreflightExceeded) {
			t.Fatalf("expected ErrSTTCostPreflightExceeded got %v", err)
		}
	})
}

func TestDecideSTTPostflightCost(t *testing.T) {
	t.Run("nil_cost_is_unknown_and_approved", func(t *testing.T) {
		got, err := DecideSTTPostflightCost(nil, 2000)
		if err != nil {
			t.Fatalf("expected nil err got %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil cost got %v", *got)
		}
	})

	t.Run("under_budget_is_approved", func(t *testing.T) {
		cost := 0.0001
		got, err := DecideSTTPostflightCost(&cost, 2000)
		if err != nil {
			t.Fatalf("expected nil err got %v", err)
		}
		if got == nil || *got != 100 {
			t.Fatalf("expected 100 microusd got %v", got)
		}
	})

	t.Run("over_budget_is_rejected", func(t *testing.T) {
		cost := 0.5
		got, err := DecideSTTPostflightCost(&cost, 2000)
		if !errors.Is(err, ErrSTTCostExceeded) {
			t.Fatalf("expected ErrSTTCostExceeded got %v", err)
		}
		if got == nil || *got != 500000 {
			t.Fatalf("expected 500000 microusd got %v", got)
		}
	})
}

func TestAudioFormat_IsValid(t *testing.T) {
	if !AudioFormatOGG.IsValid() {
		t.Fatal("expected ogg valid")
	}
	if !AudioFormatM4A.IsValid() {
		t.Fatal("expected m4a valid")
	}
	if AudioFormat("wav").IsValid() {
		t.Fatal("expected wav invalid")
	}
}

package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

const (
	endpointAudioTranscriptions = "/api/v1/audio/transcriptions"
	defaultSTTTimeout           = 20 * time.Second
	defaultSTTMaxTextLength     = 4000
)

type sttInputAudioWire struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

type sttRequestWire struct {
	Model       string            `json:"model"`
	InputAudio  sttInputAudioWire `json:"input_audio"`
	Language    string            `json:"language,omitempty"`
	Temperature float64           `json:"temperature"`
}

type sttUsageWire struct {
	Seconds      float64  `json:"seconds"`
	Cost         *float64 `json:"cost"`
	TotalTokens  *int     `json:"total_tokens"`
	InputTokens  *int     `json:"input_tokens"`
	OutputTokens *int     `json:"output_tokens"`
}

type sttResponseWire struct {
	Text         string       `json:"text"`
	Language     string       `json:"language"`
	FinishReason string       `json:"finish_reason"`
	Usage        sttUsageWire `json:"usage"`
	Error        *chatError   `json:"error,omitempty"`
}

func (p *openrouterProvider) Transcribe(ctx context.Context, req TranscriptionRequest) (TranscriptionResponse, error) {
	ctx, span := p.o11y.Tracer().Start(ctx, "llm.stt.transcribe")
	defer span.End()

	if err := validateTranscriptionRequest(req); err != nil {
		span.RecordError(err)
		return TranscriptionResponse{}, err
	}

	rate := p.cfg.STTPreflightRateMicrousdPerSec
	if err := DecideSTTPreflightCost(req.DurationMs, req.MaxCostMicrousd, rate); err != nil {
		p.recordSTTError(ctx, req.Model, "preflight_cost_exceeded")
		span.RecordError(err)
		return TranscriptionResponse{}, err
	}

	timeout := p.cfg.STTTimeout
	if timeout <= 0 {
		timeout = defaultSTTTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	encoded, err := encodeSTTRequest(req)
	if err != nil {
		span.RecordError(err)
		return TranscriptionResponse{}, err
	}

	rawBody, err := p.sttSend(callCtx, encoded, req.Model)
	if err != nil {
		span.RecordError(err)
		return TranscriptionResponse{}, err
	}

	parsed, err := p.decodeSTTResponse(ctx, rawBody, req.Model)
	if err != nil {
		span.RecordError(err)
		return TranscriptionResponse{}, err
	}

	costMicrousd, err := p.applySTTPostflightCost(ctx, parsed.Usage.Cost, req.MaxCostMicrousd, req.Model)
	if err != nil {
		span.RecordError(err)
		return TranscriptionResponse{}, err
	}

	p.sttCallTotal.Add(ctx, 1,
		observability.String("model", req.Model),
		observability.String("status", "ok"),
	)

	return buildTranscriptionResponse(req, parsed, costMicrousd), nil
}

func encodeSTTRequest(req TranscriptionRequest) ([]byte, error) {
	wire := sttRequestWire{
		Model: req.Model,
		InputAudio: sttInputAudioWire{
			Data:   base64.StdEncoding.EncodeToString(req.Audio),
			Format: req.Format.String(),
		},
		Language:    req.Language,
		Temperature: req.Temperature,
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("llm.stt: marshal request: %w", err)
	}
	return encoded, nil
}

func (p *openrouterProvider) recordSTTError(ctx context.Context, model, reason string) {
	p.sttCallError.Add(ctx, 1,
		observability.String("model", model),
		observability.String("reason", reason),
	)
}

func (p *openrouterProvider) decodeSTTResponse(ctx context.Context, rawBody []byte, model string) (sttResponseWire, error) {
	var parsed sttResponseWire
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		p.recordSTTError(ctx, model, "decode")
		return sttResponseWire{}, fmt.Errorf("%w: decode: %v", ErrSTTInvalidResponse, err)
	}
	if parsed.Error != nil {
		p.recordSTTError(ctx, model, "upstream_error")
		return sttResponseWire{}, fmt.Errorf("%w: %s", ErrSTTUpstream, parsed.Error.Message)
	}
	if isTranscriptionEmpty(parsed.Text) {
		p.recordSTTError(ctx, model, "empty_text")
		return sttResponseWire{}, ErrSTTEmptyText
	}
	return parsed, nil
}

func (p *openrouterProvider) applySTTPostflightCost(ctx context.Context, costUSD *float64, maxCostMicrousd int64, model string) (*int64, error) {
	costMicrousd, costErr := DecideSTTPostflightCost(costUSD, maxCostMicrousd)
	if costMicrousd != nil {
		p.sttCostMicrousd.Add(ctx, *costMicrousd, observability.String("model", model))
	}
	if costErr != nil {
		p.recordSTTError(ctx, model, "postflight_cost_exceeded")
		return costMicrousd, costErr
	}
	return costMicrousd, nil
}

func buildTranscriptionResponse(req TranscriptionRequest, parsed sttResponseWire, costMicrousd *int64) TranscriptionResponse {
	maxTextLength := req.MaxTextLength
	if maxTextLength <= 0 {
		maxTextLength = defaultSTTMaxTextLength
	}
	truncated := parsed.FinishReason == finishReasonLength || len(parsed.Text) > maxTextLength

	language := parsed.Language
	if strings.TrimSpace(language) == "" {
		language = req.Language
	}

	resp := TranscriptionResponse{
		Text:         parsed.Text,
		Language:     language,
		Model:        req.Model,
		Provider:     "openrouter",
		Truncated:    truncated,
		CostMicrousd: costMicrousd,
		Usage: TranscriptionUsage{
			Seconds:      parsed.Usage.Seconds,
			CostUSD:      parsed.Usage.Cost,
			TotalTokens:  parsed.Usage.TotalTokens,
			InputTokens:  parsed.Usage.InputTokens,
			OutputTokens: parsed.Usage.OutputTokens,
		},
	}
	if req.DurationMs > 0 {
		resp.DurationMs = &req.DurationMs
	}
	return resp
}

func (p *openrouterProvider) sttSend(ctx context.Context, body []byte, model string) ([]byte, error) {
	start := time.Now()
	resp, err := p.client.Post(ctx, endpointAudioTranscriptions, bytes.NewReader(body),
		httpclient.WithHeader("Content-Type", "application/json"),
		httpclient.WithHeader("Authorization", "Bearer "+p.cfg.APIKey),
		httpclient.WithHeader("HTTP-Referer", p.cfg.HTTPReferer),
		httpclient.WithHeader("X-Title", p.cfg.XTitle),
		httpclient.WithoutRetry(),
	)
	p.sttLatency.Record(ctx, time.Since(start).Seconds(), observability.String("model", model))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			p.sttCallError.Add(ctx, 1,
				observability.String("model", model),
				observability.String("reason", "timeout"),
			)
			return nil, fmt.Errorf("%w: %v", ErrSTTTimeout, err)
		}
		p.sttCallError.Add(ctx, 1,
			observability.String("model", model),
			observability.String("reason", "transport"),
		)
		return nil, fmt.Errorf("%w: post: %v", ErrSTTUpstream, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			p.o11y.Logger().Warn(ctx, "llm.stt.close_body_failed", observability.Error(closeErr))
		}
	}()

	rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		p.sttCallError.Add(ctx, 1,
			observability.String("model", model),
			observability.String("reason", "read_body"),
		)
		return nil, fmt.Errorf("%w: read body: %v", ErrSTTUpstream, readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		reason := classifySTTStatus(resp.StatusCode)
		p.sttCallError.Add(ctx, 1,
			observability.String("model", model),
			observability.String("reason", reason),
		)
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: status=%d body=%s", ErrSTTModelUnavailable, resp.StatusCode, truncatePreview(rawBody))
		}
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrSTTUpstream, resp.StatusCode, truncatePreview(rawBody))
	}
	return rawBody, nil
}

func classifySTTStatus(code int) string {
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return "unauthorized"
	case code == http.StatusPaymentRequired:
		return "no_credit"
	case code == http.StatusNotFound:
		return "model_unavailable"
	case code == http.StatusTooManyRequests:
		return "rate_limited"
	case code == http.StatusRequestTimeout:
		return "timeout"
	case code >= 500:
		return "upstream_5xx"
	default:
		return "client_4xx"
	}
}

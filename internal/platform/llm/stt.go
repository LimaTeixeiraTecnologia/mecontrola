package llm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
)

type AudioFormat string

const (
	AudioFormatOGG AudioFormat = "ogg"
	AudioFormatM4A AudioFormat = "m4a"
)

func (f AudioFormat) String() string { return string(f) }

func (f AudioFormat) IsValid() bool {
	switch f {
	case AudioFormatOGG, AudioFormatM4A:
		return true
	default:
		return false
	}
}

var (
	ErrSTTFormatUnsupported     = errors.New("llm.stt: audio format unsupported")
	ErrSTTModelRequired         = errors.New("llm.stt: model required")
	ErrSTTAudioRequired         = errors.New("llm.stt: audio bytes required")
	ErrSTTDurationRequired      = errors.New("llm.stt: duration_ms required")
	ErrSTTCostPreflightExceeded = errors.New("llm.stt: preflight cost exceeds max_cost_microusd")
	ErrSTTCostExceeded          = errors.New("llm.stt: usage cost exceeds max_cost_microusd")
	ErrSTTMaxCostRequired       = errors.New("llm.stt: max_cost_microusd required")
	ErrSTTModelUnavailable      = errors.New("llm.stt: model unavailable")
	ErrSTTUpstream              = errors.New("llm.stt: upstream error")
	ErrSTTTimeout               = errors.New("llm.stt: request timeout")
	ErrSTTInvalidResponse       = errors.New("llm.stt: invalid response")
	ErrSTTEmptyText             = errors.New("llm.stt: transcription text empty")
)

type Transcriber interface {
	Transcribe(ctx context.Context, req TranscriptionRequest) (TranscriptionResponse, error)
}

type TranscriptionRequest struct {
	Model           string
	Audio           []byte
	Format          AudioFormat
	Language        string
	Temperature     float64
	DurationMs      int
	MaxCostMicrousd int64
	MaxTextLength   int
}

type TranscriptionUsage struct {
	Seconds      float64
	CostUSD      *float64
	TotalTokens  *int
	InputTokens  *int
	OutputTokens *int
}

type TranscriptionResponse struct {
	Text         string
	Language     string
	DurationMs   *int
	Model        string
	Provider     string
	Usage        TranscriptionUsage
	Truncated    bool
	Confidence   *float64
	CostMicrousd *int64
}

const defaultSTTPreflightRateMicrousdPerSecond int64 = 25

func EstimateSTTPreflightCostMicrousd(durationMs int, rateMicrousdPerSecond int64) int64 {
	if rateMicrousdPerSecond <= 0 {
		rateMicrousdPerSecond = defaultSTTPreflightRateMicrousdPerSecond
	}
	if durationMs <= 0 {
		return 0
	}
	seconds := math.Ceil(float64(durationMs) / 1000.0)
	return int64(seconds) * rateMicrousdPerSecond
}

func DecideSTTPreflightCost(durationMs int, maxCostMicrousd int64, rateMicrousdPerSecond int64) error {
	if durationMs <= 0 {
		return ErrSTTDurationRequired
	}
	estimated := EstimateSTTPreflightCostMicrousd(durationMs, rateMicrousdPerSecond)
	if estimated > maxCostMicrousd {
		return fmt.Errorf("%w: estimated_microusd=%d max_microusd=%d duration_ms=%d",
			ErrSTTCostPreflightExceeded, estimated, maxCostMicrousd, durationMs)
	}
	return nil
}

func DecideSTTPostflightCost(costUSD *float64, maxCostMicrousd int64) (*int64, error) {
	if costUSD == nil {
		return nil, nil
	}
	costMicrousd := int64(math.Round(*costUSD * 1_000_000))
	if maxCostMicrousd > 0 && costMicrousd > maxCostMicrousd {
		return &costMicrousd, fmt.Errorf("%w: cost_microusd=%d max_microusd=%d",
			ErrSTTCostExceeded, costMicrousd, maxCostMicrousd)
	}
	return &costMicrousd, nil
}

func validateTranscriptionRequest(req TranscriptionRequest) error {
	if strings.TrimSpace(req.Model) == "" {
		return ErrSTTModelRequired
	}
	if len(req.Audio) == 0 {
		return ErrSTTAudioRequired
	}
	if !req.Format.IsValid() {
		return fmt.Errorf("%w: format=%q", ErrSTTFormatUnsupported, req.Format)
	}
	if req.DurationMs <= 0 {
		return ErrSTTDurationRequired
	}
	if req.MaxCostMicrousd <= 0 {
		return ErrSTTMaxCostRequired
	}
	return nil
}

func isTranscriptionEmpty(text string) bool {
	return strings.TrimSpace(text) == ""
}

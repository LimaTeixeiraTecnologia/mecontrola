package scorer

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ToolCallRecord struct {
	ID      string
	Name    string
	Args    map[string]any
	Outcome string
}

type RunSample struct {
	Input          string
	Output         string
	ExpectedOutput string
	ToolCalls      []ToolCallRecord
	Metadata       map[string]any
}

type ScoreResult struct {
	Score    float64
	Reason   string
	Metadata map[string]any
}

type ScorerResult struct {
	ID        uuid.UUID
	RunID     uuid.UUID
	ScorerID  string
	Kind      ScorerKind
	Score     float64
	Reason    string
	Metadata  map[string]any
	Sampled   bool
	CreatedAt time.Time
}

type Sampling struct {
	Type  SamplingType
	Ratio float64
}

func AlwaysSample() Sampling {
	return Sampling{Type: SamplingTypeAlways, Ratio: 1.0}
}

func NeverSample() Sampling {
	return Sampling{Type: SamplingTypeNever, Ratio: 0}
}

func RatioSample(ratio float64) Sampling {
	return Sampling{Type: SamplingTypeRatio, Ratio: ratio}
}

type Scorer interface {
	ID() string
	Kind() ScorerKind
	Score(ctx context.Context, s RunSample) (ScoreResult, error)
}

type ScorerRunner interface {
	Observe(ctx context.Context, runID uuid.UUID, s RunSample)
	Shutdown(ctx context.Context)
}

type ResultStore interface {
	Insert(ctx context.Context, r ScorerResult) error
}

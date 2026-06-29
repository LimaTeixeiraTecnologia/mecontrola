package scorer

import (
	"errors"
	"fmt"
)

type ScorerKind int

const (
	ScorerKindCodeBased ScorerKind = iota + 1
	ScorerKindLLMJudged
)

func (k ScorerKind) String() string {
	switch k {
	case ScorerKindCodeBased:
		return "code_based"
	case ScorerKindLLMJudged:
		return "llm_judged"
	default:
		return "unknown"
	}
}

func (k ScorerKind) IsValid() bool {
	return k >= ScorerKindCodeBased && k <= ScorerKindLLMJudged
}

var errInvalidScorerKind = errors.New("scorer: invalid scorer kind")

func ParseScorerKind(s string) (ScorerKind, error) {
	switch s {
	case "code_based":
		return ScorerKindCodeBased, nil
	case "llm_judged":
		return ScorerKindLLMJudged, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidScorerKind, s)
	}
}

type SamplingType int

const (
	SamplingTypeRatio SamplingType = iota + 1
	SamplingTypeAlways
	SamplingTypeNever
)

func (t SamplingType) String() string {
	switch t {
	case SamplingTypeRatio:
		return "ratio"
	case SamplingTypeAlways:
		return "always"
	case SamplingTypeNever:
		return "never"
	default:
		return "unknown"
	}
}

func (t SamplingType) IsValid() bool {
	return t >= SamplingTypeRatio && t <= SamplingTypeNever
}

var errInvalidSamplingType = errors.New("scorer: invalid sampling type")

func ParseSamplingType(s string) (SamplingType, error) {
	switch s {
	case "ratio":
		return SamplingTypeRatio, nil
	case "always":
		return SamplingTypeAlways, nil
	case "never":
		return SamplingTypeNever, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidSamplingType, s)
	}
}

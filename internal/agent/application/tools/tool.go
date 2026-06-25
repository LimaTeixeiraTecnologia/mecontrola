package tools

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type ToolOutcome int

const (
	OutcomeRouted ToolOutcome = iota + 1
	OutcomeFallback
	OutcomeParseError
	OutcomeUsecaseError
	OutcomeMissingResolver
	OutcomeReplyFailed
	OutcomeEmptyText
	OutcomeAuthzDenied
	OutcomeClarify
	OutcomePolicyBlocked
	OutcomeReplay
)

var ErrToolOutcomeUnknown = errors.New("agent.application.tools: tool outcome not allowed")

func (o ToolOutcome) String() string {
	switch o {
	case OutcomeRouted:
		return "routed"
	case OutcomeFallback:
		return "fallback"
	case OutcomeParseError:
		return "parse_error"
	case OutcomeUsecaseError:
		return "usecase_error"
	case OutcomeMissingResolver:
		return "missing_resolver"
	case OutcomeReplyFailed:
		return "reply_failed"
	case OutcomeEmptyText:
		return "empty_text"
	case OutcomeAuthzDenied:
		return "authz_denied"
	case OutcomeClarify:
		return "clarify"
	case OutcomePolicyBlocked:
		return "policy_blocked"
	case OutcomeReplay:
		return "replay"
	default:
		return "usecase_error"
	}
}

func ParseOutcome(raw string) (ToolOutcome, error) {
	switch raw {
	case "routed":
		return OutcomeRouted, nil
	case "fallback":
		return OutcomeFallback, nil
	case "parse_error":
		return OutcomeParseError, nil
	case "usecase_error":
		return OutcomeUsecaseError, nil
	case "missing_resolver":
		return OutcomeMissingResolver, nil
	case "reply_failed":
		return OutcomeReplyFailed, nil
	case "empty_text":
		return OutcomeEmptyText, nil
	case "authz_denied":
		return OutcomeAuthzDenied, nil
	case "clarify":
		return OutcomeClarify, nil
	case "policy_blocked":
		return OutcomePolicyBlocked, nil
	case "replay":
		return OutcomeReplay, nil
	default:
		return 0, ErrToolOutcomeUnknown
	}
}

type ToolInput struct {
	UserID       uuid.UUID
	Channel      string
	Intent       intent.Intent
	StepIndex    int
	MessageID    string
	Text         string
	Confidence   valueobjects.Confidence
	Parsed       any
	LLMModel     string
	PromptSHA256 string
	DirectReply  string
	RawResponse  string
}

type ToolResult struct {
	Reply   string
	Outcome ToolOutcome
	Kind    intent.Kind
}

type Tool interface {
	Name() string
	Descriptor() ToolSpec
	Execute(ctx context.Context, in ToolInput) (ToolResult, error)
}

type ExecuteFunc func(ctx context.Context, in ToolInput) (ToolResult, error)

type funcTool struct {
	spec ToolSpec
	exec ExecuteFunc
}

func NewTool(spec ToolSpec, exec ExecuteFunc) Tool {
	return &funcTool{spec: spec, exec: exec}
}

func (t *funcTool) Name() string {
	return t.spec.Name
}

func (t *funcTool) Descriptor() ToolSpec {
	return t.spec
}

func (t *funcTool) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
	return t.exec(ctx, in)
}

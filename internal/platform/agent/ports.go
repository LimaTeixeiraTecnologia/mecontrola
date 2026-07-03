package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type Request struct {
	AgentID     string
	ResourceID  string
	ThreadID    string
	Messages    []llm.Message
	Schema      *llm.Schema
	Decoder     StructuredDecoder
	Tools       []tool.ToolHandle
	MaxTokens   int
	Temperature float64
}

type ToolCallOutcome int

const (
	ToolCallOutcomeSuccess ToolCallOutcome = iota + 1
	ToolCallOutcomeError
)

func (o ToolCallOutcome) String() string {
	switch o {
	case ToolCallOutcomeSuccess:
		return "success"
	case ToolCallOutcomeError:
		return "error"
	default:
		return "unknown"
	}
}

func (o ToolCallOutcome) IsValid() bool {
	return o >= ToolCallOutcomeSuccess && o <= ToolCallOutcomeError
}

type ToolCallRecord struct {
	Tool    string
	Outcome ToolCallOutcome
	Content string
}

type Result struct {
	Content           string
	RawJSON           []byte
	Mode              ExecutionMode
	ToolOutcome       ToolOutcome
	ToolCalls         []ToolCallRecord
	TruncatedByLength bool
}

type InboundRequest struct {
	ResourceID string
	ThreadID   string
	AgentID    string
	Message    string
	MessageID  string
}

func (i *InboundRequest) Validate() error {
	var errs []error
	if i.AgentID == "" {
		errs = append(errs, fmt.Errorf("agent_id: %w", ErrEmptyAgentID))
	}
	if i.ResourceID == "" {
		errs = append(errs, fmt.Errorf("resource_id: %w", ErrEmptyResourceID))
	}
	if i.ThreadID == "" {
		errs = append(errs, fmt.Errorf("thread_id: %w", ErrEmptyThreadID))
	}
	if i.Message == "" {
		errs = append(errs, fmt.Errorf("message: %w", ErrEmptyMessage))
	}
	return errors.Join(errs...)
}

type Outcome struct {
	RunID   uuid.UUID
	Content string
	Status  RunStatus
	Outcome ToolOutcome
	Mode    ExecutionMode
}

type Run struct {
	ID             uuid.UUID
	ThreadPK       uuid.UUID
	ResourceID     string
	ThreadID       string
	AgentID        string
	Workflow       string
	CorrelationKey string
	Status         RunStatus
	Outcome        ToolOutcome
	Error          string
	StartedAt      time.Time
	EndedAt        *time.Time
	DurationMs     int64
}

type Agent interface {
	ID() string
	Instructions() string
	Execute(ctx context.Context, in Request) (Result, error)
	Stream(ctx context.Context, in Request) (ResultStream, error)
}

type ResultStream interface {
	Deltas() <-chan string
	Result(ctx context.Context) (Result, error)
}

type AgentRuntime interface {
	Execute(ctx context.Context, in InboundRequest) (Outcome, error)
}

type AgentRegistry interface {
	Register(a Agent)
	Resolve(id string) (Agent, error)
}

type WorkflowRegistry[S any] interface {
	Resolve(agentID string) (workflow.Definition[S], bool)
}

type MutableWorkflowRegistry[S any] interface {
	WorkflowRegistry[S]
	Register(def workflow.Definition[S])
}

type RunStore interface {
	Insert(ctx context.Context, run Run) error
	Update(ctx context.Context, run Run) error
	Load(ctx context.Context, id uuid.UUID) (Run, error)
}

type Hooks interface {
	BeforeExecute(ctx context.Context, agentID string, in Request) context.Context
	AfterExecute(ctx context.Context, agentID string, result Result, err error)
	BeforeTool(ctx context.Context, agentID, toolID string) context.Context
	AfterTool(ctx context.Context, agentID, toolID string, resultBytes []byte, err error)
}

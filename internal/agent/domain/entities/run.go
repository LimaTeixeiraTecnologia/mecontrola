package entities

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RunStatus int

const (
	RunStatusRunning RunStatus = iota + 1
	RunStatusSucceeded
	RunStatusFailed
)

func (s RunStatus) String() string {
	switch s {
	case RunStatusRunning:
		return "running"
	case RunStatusSucceeded:
		return "succeeded"
	case RunStatusFailed:
		return "failed"
	default:
		return ""
	}
}

func (s RunStatus) IsValid() bool {
	switch s {
	case RunStatusRunning, RunStatusSucceeded, RunStatusFailed:
		return true
	default:
		return false
	}
}

func ParseRunStatus(raw string) (RunStatus, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "running":
		return RunStatusRunning, nil
	case "succeeded":
		return RunStatusSucceeded, nil
	case "failed":
		return RunStatusFailed, nil
	default:
		return 0, ErrRunStatusInvalid
	}
}

var (
	ErrRunThreadRequired  = errors.New("agent.run: thread_id é obrigatório")
	ErrRunUserRequired    = errors.New("agent.run: user_id é obrigatório")
	ErrRunChannelRequired = errors.New("agent.run: channel é obrigatório")
	ErrRunStatusInvalid   = errors.New("agent.run: status inválido")
)

type StartRunParams struct {
	ThreadID      uuid.UUID
	UserID        uuid.UUID
	Channel       string
	MessageID     string
	AgentID       string
	Workflow      string
	ToolName      string
	IntentKind    string
	SchemaVersion string
	DecisionID    uuid.UUID
}

type Run struct {
	id            uuid.UUID
	threadID      uuid.UUID
	userID        uuid.UUID
	channel       string
	messageID     string
	agentID       string
	workflow      string
	toolName      string
	intentKind    string
	schemaVersion string
	outcome       string
	status        RunStatus
	errText       string
	decisionID    uuid.UUID
	startedAt     time.Time
	endedAt       *time.Time
}

const runSchemaVersionDefault = "v1"

func StartRun(params StartRunParams) (Run, error) {
	if params.ThreadID == uuid.Nil {
		return Run{}, ErrRunThreadRequired
	}
	if params.UserID == uuid.Nil {
		return Run{}, ErrRunUserRequired
	}
	channel := strings.TrimSpace(params.Channel)
	if channel == "" {
		return Run{}, ErrRunChannelRequired
	}
	schemaVersion := strings.TrimSpace(params.SchemaVersion)
	if schemaVersion == "" {
		schemaVersion = runSchemaVersionDefault
	}
	return Run{
		id:            uuid.New(),
		threadID:      params.ThreadID,
		userID:        params.UserID,
		channel:       channel,
		messageID:     strings.TrimSpace(params.MessageID),
		agentID:       strings.TrimSpace(params.AgentID),
		workflow:      strings.TrimSpace(params.Workflow),
		toolName:      strings.TrimSpace(params.ToolName),
		intentKind:    strings.TrimSpace(params.IntentKind),
		schemaVersion: schemaVersion,
		status:        RunStatusRunning,
		decisionID:    params.DecisionID,
		startedAt:     time.Now().UTC(),
	}, nil
}

func (r Run) Finish(outcome string, ok bool, errText string) Run {
	next := r
	endedAt := time.Now().UTC()
	next.endedAt = &endedAt
	next.outcome = strings.TrimSpace(outcome)
	if ok {
		next.status = RunStatusSucceeded
		next.errText = ""
		return next
	}
	next.status = RunStatusFailed
	next.errText = strings.TrimSpace(errText)
	return next
}

type RunResolution struct {
	Workflow   string
	ToolName   string
	IntentKind string
}

func (r Run) Resolve(res RunResolution) Run {
	next := r
	next.workflow = strings.TrimSpace(res.Workflow)
	next.toolName = strings.TrimSpace(res.ToolName)
	next.intentKind = strings.TrimSpace(res.IntentKind)
	return next
}

func (r Run) DurationMs() int64 {
	if r.endedAt == nil {
		return 0
	}
	return r.endedAt.Sub(r.startedAt).Milliseconds()
}

func (r Run) ID() uuid.UUID         { return r.id }
func (r Run) ThreadID() uuid.UUID   { return r.threadID }
func (r Run) UserID() uuid.UUID     { return r.userID }
func (r Run) Channel() string       { return r.channel }
func (r Run) MessageID() string     { return r.messageID }
func (r Run) AgentID() string       { return r.agentID }
func (r Run) Workflow() string      { return r.workflow }
func (r Run) ToolName() string      { return r.toolName }
func (r Run) IntentKind() string    { return r.intentKind }
func (r Run) SchemaVersion() string { return r.schemaVersion }
func (r Run) Outcome() string       { return r.outcome }
func (r Run) Status() RunStatus     { return r.status }
func (r Run) ErrText() string       { return r.errText }
func (r Run) StartedAt() time.Time  { return r.startedAt }

func (r Run) EndedAt() (time.Time, bool) {
	if r.endedAt == nil {
		return time.Time{}, false
	}
	return *r.endedAt, true
}

func (r Run) DecisionID() (uuid.UUID, bool) {
	if r.decisionID == uuid.Nil {
		return uuid.Nil, false
	}
	return r.decisionID, true
}

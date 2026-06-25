package entities

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

var (
	ErrAgentDecisionIDRequired         = errors.New("agent.decision: id é obrigatório")
	ErrAgentDecisionUserRequired       = errors.New("agent.decision: user_id é obrigatório")
	ErrAgentDecisionChannelRequired    = errors.New("agent.decision: channel é obrigatório")
	ErrAgentDecisionMessageIDRequired  = errors.New("agent.decision: message_id é obrigatório")
	ErrAgentDecisionIntentKindRequired = errors.New("agent.decision: intent_kind é obrigatório")
	ErrAgentDecisionLLMModelRequired   = errors.New("agent.decision: llm_model é obrigatório")
	ErrAgentDecisionActionRequired     = errors.New("agent.decision: decided_action é obrigatório")
	ErrAgentDecisionPromptDigest       = errors.New("agent.decision: prompt_sha256 deve ser 64 caracteres hexadecimais")
	ErrAgentDecisionAlreadySettled     = errors.New("agent.decision: decisão já liquidada")
	ErrAgentDecisionEventRequired      = errors.New("agent.decision: resulting_event_id é obrigatório para executed")
)

const promptDigestHexLen = 64

type AgentDecisionParams struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Channel          string
	MessageID        string
	IntentKind       string
	PromptSHA256     string
	LLMModel         string
	RedactedResponse json.RawMessage
	TraceID          string
	DecidedAction    string
	CreatedAt        time.Time
	StepIndex        int
}

type AgentDecision struct {
	id               uuid.UUID
	userID           uuid.UUID
	channel          string
	messageID        string
	intentKind       string
	promptSHA256     string
	llmModel         string
	redactedResponse json.RawMessage
	traceID          string
	decidedAction    string
	status           valueobjects.DecisionStatus
	resultingEventID uuid.UUID
	createdAt        time.Time
	settledAt        time.Time
	stepIndex        int
}

func NewPendingDecision(params AgentDecisionParams) (AgentDecision, error) {
	return newDecision(params, valueobjects.DecisionStatusPending)
}

func NewAwaitingConfirmationDecision(params AgentDecisionParams) (AgentDecision, error) {
	return newDecision(params, valueobjects.DecisionStatusAwaitingConfirmation)
}

func newDecision(params AgentDecisionParams, status valueobjects.DecisionStatus) (AgentDecision, error) {
	if params.ID == uuid.Nil {
		return AgentDecision{}, ErrAgentDecisionIDRequired
	}
	if params.UserID == uuid.Nil {
		return AgentDecision{}, ErrAgentDecisionUserRequired
	}
	channel := strings.TrimSpace(params.Channel)
	if channel == "" {
		return AgentDecision{}, ErrAgentDecisionChannelRequired
	}
	messageID := strings.TrimSpace(params.MessageID)
	if messageID == "" {
		return AgentDecision{}, ErrAgentDecisionMessageIDRequired
	}
	intentKind := strings.TrimSpace(params.IntentKind)
	if intentKind == "" {
		return AgentDecision{}, ErrAgentDecisionIntentKindRequired
	}
	llmModel := strings.TrimSpace(params.LLMModel)
	if llmModel == "" {
		return AgentDecision{}, ErrAgentDecisionLLMModelRequired
	}
	decidedAction := strings.TrimSpace(params.DecidedAction)
	if decidedAction == "" {
		return AgentDecision{}, ErrAgentDecisionActionRequired
	}
	digest, err := normalizePromptDigest(params.PromptSHA256)
	if err != nil {
		return AgentDecision{}, err
	}
	createdAt := params.CreatedAt
	if createdAt.IsZero() {
		return AgentDecision{}, errors.New("agent.decision: created_at é obrigatório")
	}
	return AgentDecision{
		id:               params.ID,
		userID:           params.UserID,
		channel:          channel,
		messageID:        messageID,
		intentKind:       intentKind,
		promptSHA256:     digest,
		llmModel:         llmModel,
		redactedResponse: params.RedactedResponse,
		traceID:          strings.TrimSpace(params.TraceID),
		decidedAction:    decidedAction,
		status:           status,
		createdAt:        createdAt.UTC(),
		stepIndex:        params.StepIndex,
	}, nil
}

func normalizePromptDigest(raw string) (string, error) {
	digest := strings.ToLower(strings.TrimSpace(raw))
	if len(digest) != promptDigestHexLen {
		return "", ErrAgentDecisionPromptDigest
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", ErrAgentDecisionPromptDigest
	}
	return digest, nil
}

func (d AgentDecision) Execute(eventID uuid.UUID, settledAt time.Time) (AgentDecision, error) {
	if d.status.IsSettled() {
		return AgentDecision{}, ErrAgentDecisionAlreadySettled
	}
	if eventID == uuid.Nil {
		return AgentDecision{}, ErrAgentDecisionEventRequired
	}
	next := d
	next.status = valueobjects.DecisionStatusExecuted
	next.resultingEventID = eventID
	next.settledAt = settledAt.UTC()
	return next, nil
}

func (d AgentDecision) Reject(settledAt time.Time) (AgentDecision, error) {
	if d.status.IsSettled() {
		return AgentDecision{}, ErrAgentDecisionAlreadySettled
	}
	next := d
	next.status = valueobjects.DecisionStatusRejected
	next.resultingEventID = uuid.Nil
	next.settledAt = settledAt.UTC()
	return next, nil
}

func (d AgentDecision) ID() uuid.UUID                       { return d.id }
func (d AgentDecision) UserID() uuid.UUID                   { return d.userID }
func (d AgentDecision) Channel() string                     { return d.channel }
func (d AgentDecision) MessageID() string                   { return d.messageID }
func (d AgentDecision) IntentKind() string                  { return d.intentKind }
func (d AgentDecision) PromptSHA256() string                { return d.promptSHA256 }
func (d AgentDecision) LLMModel() string                    { return d.llmModel }
func (d AgentDecision) RedactedResponse() json.RawMessage   { return d.redactedResponse }
func (d AgentDecision) TraceID() string                     { return d.traceID }
func (d AgentDecision) DecidedAction() string               { return d.decidedAction }
func (d AgentDecision) Status() valueobjects.DecisionStatus { return d.status }
func (d AgentDecision) CreatedAt() time.Time                { return d.createdAt }
func (d AgentDecision) StepIndex() int                      { return d.stepIndex }

func (d AgentDecision) ResultingEventID() (uuid.UUID, bool) {
	if d.resultingEventID == uuid.Nil {
		return uuid.Nil, false
	}
	return d.resultingEventID, true
}

func (d AgentDecision) SettledAt() (time.Time, bool) {
	if d.settledAt.IsZero() {
		return time.Time{}, false
	}
	return d.settledAt, true
}

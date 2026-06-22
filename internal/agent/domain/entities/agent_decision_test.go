package entities_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type AgentDecisionSuite struct {
	suite.Suite
}

func TestAgentDecisionSuite(t *testing.T) {
	suite.Run(t, new(AgentDecisionSuite))
}

func validDigest() string {
	sum := sha256.Sum256([]byte("prompt sanitizado"))
	return hex.EncodeToString(sum[:])
}

func baseParams() entities.AgentDecisionParams {
	return entities.AgentDecisionParams{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		Channel:          "whatsapp",
		MessageID:        "wamid.ABC123",
		IntentKind:       "log_expense",
		PromptSHA256:     validDigest(),
		LLMModel:         "openai/gpt-4o-mini",
		RedactedResponse: json.RawMessage(`{"kind":"log_expense"}`),
		TraceID:          "0af7651916cd43dd8448eb211c80319c",
		DecidedAction:    "log_expense",
		CreatedAt:        time.Now().UTC(),
	}
}

func (s *AgentDecisionSuite) TestNewPendingDecisionValidation() {
	cases := []struct {
		name    string
		mutate  func(*entities.AgentDecisionParams)
		wantErr error
	}{
		{
			name:    "valido",
			mutate:  func(*entities.AgentDecisionParams) {},
			wantErr: nil,
		},
		{
			name:    "id ausente",
			mutate:  func(p *entities.AgentDecisionParams) { p.ID = uuid.Nil },
			wantErr: entities.ErrAgentDecisionIDRequired,
		},
		{
			name:    "user ausente",
			mutate:  func(p *entities.AgentDecisionParams) { p.UserID = uuid.Nil },
			wantErr: entities.ErrAgentDecisionUserRequired,
		},
		{
			name:    "channel vazio",
			mutate:  func(p *entities.AgentDecisionParams) { p.Channel = "  " },
			wantErr: entities.ErrAgentDecisionChannelRequired,
		},
		{
			name:    "message id vazio",
			mutate:  func(p *entities.AgentDecisionParams) { p.MessageID = "" },
			wantErr: entities.ErrAgentDecisionMessageIDRequired,
		},
		{
			name:    "intent kind vazio",
			mutate:  func(p *entities.AgentDecisionParams) { p.IntentKind = "" },
			wantErr: entities.ErrAgentDecisionIntentKindRequired,
		},
		{
			name:    "llm model vazio",
			mutate:  func(p *entities.AgentDecisionParams) { p.LLMModel = "" },
			wantErr: entities.ErrAgentDecisionLLMModelRequired,
		},
		{
			name:    "decided action vazio",
			mutate:  func(p *entities.AgentDecisionParams) { p.DecidedAction = "" },
			wantErr: entities.ErrAgentDecisionActionRequired,
		},
		{
			name:    "digest curto",
			mutate:  func(p *entities.AgentDecisionParams) { p.PromptSHA256 = "abcdef" },
			wantErr: entities.ErrAgentDecisionPromptDigest,
		},
		{
			name:    "digest nao hex",
			mutate:  func(p *entities.AgentDecisionParams) { p.PromptSHA256 = strings.Repeat("z", 64) },
			wantErr: entities.ErrAgentDecisionPromptDigest,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			params := baseParams()
			tc.mutate(&params)
			decision, err := entities.NewPendingDecision(params)
			if tc.wantErr != nil {
				s.Require().ErrorIs(err, tc.wantErr)
				return
			}
			s.Require().NoError(err)
			s.Equal(valueobjects.DecisionStatusPending, decision.Status())
			s.Equal(params.ID, decision.ID())
			s.Equal("whatsapp", decision.Channel())
			s.Equal(params.PromptSHA256, decision.PromptSHA256())
			_, settled := decision.SettledAt()
			s.False(settled)
			_, hasEvent := decision.ResultingEventID()
			s.False(hasEvent)
		})
	}
}

func (s *AgentDecisionSuite) TestNewAwaitingConfirmationDecision() {
	decision, err := entities.NewAwaitingConfirmationDecision(baseParams())
	s.Require().NoError(err)
	s.Equal(valueobjects.DecisionStatusAwaitingConfirmation, decision.Status())
}

func (s *AgentDecisionSuite) TestExecuteTransition() {
	pending, err := entities.NewPendingDecision(baseParams())
	s.Require().NoError(err)

	eventID := uuid.New()
	settledAt := time.Now().UTC()
	executed, err := pending.Execute(eventID, settledAt)
	s.Require().NoError(err)

	s.Equal(valueobjects.DecisionStatusExecuted, executed.Status())
	gotEvent, ok := executed.ResultingEventID()
	s.True(ok)
	s.Equal(eventID, gotEvent)
	gotSettled, ok := executed.SettledAt()
	s.True(ok)
	s.WithinDuration(settledAt, gotSettled, time.Second)

	s.Equal(valueobjects.DecisionStatusPending, pending.Status())
	_, stillSettled := pending.SettledAt()
	s.False(stillSettled)
}

func (s *AgentDecisionSuite) TestExecuteRequiresEventID() {
	pending, err := entities.NewPendingDecision(baseParams())
	s.Require().NoError(err)

	_, err = pending.Execute(uuid.Nil, time.Now().UTC())
	s.Require().ErrorIs(err, entities.ErrAgentDecisionEventRequired)
}

func (s *AgentDecisionSuite) TestRejectTransition() {
	pending, err := entities.NewPendingDecision(baseParams())
	s.Require().NoError(err)

	settledAt := time.Now().UTC()
	rejected, err := pending.Reject(settledAt)
	s.Require().NoError(err)

	s.Equal(valueobjects.DecisionStatusRejected, rejected.Status())
	_, hasEvent := rejected.ResultingEventID()
	s.False(hasEvent)
	gotSettled, ok := rejected.SettledAt()
	s.True(ok)
	s.WithinDuration(settledAt, gotSettled, time.Second)
}

func (s *AgentDecisionSuite) TestRefusesReSettle() {
	pending, err := entities.NewPendingDecision(baseParams())
	s.Require().NoError(err)

	executed, err := pending.Execute(uuid.New(), time.Now().UTC())
	s.Require().NoError(err)

	_, err = executed.Execute(uuid.New(), time.Now().UTC())
	s.Require().ErrorIs(err, entities.ErrAgentDecisionAlreadySettled)

	_, err = executed.Reject(time.Now().UTC())
	s.Require().ErrorIs(err, entities.ErrAgentDecisionAlreadySettled)
}

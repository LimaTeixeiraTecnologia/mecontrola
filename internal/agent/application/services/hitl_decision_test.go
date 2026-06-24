package services

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
)

type HITLDecisionSuite struct {
	suite.Suite
	ctx context.Context
}

func TestHITLDecisionSuite(t *testing.T) {
	suite.Run(t, new(HITLDecisionSuite))
}

func (s *HITLDecisionSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *HITLDecisionSuite) newReplayFunc(repo interfaces.AgentDecisionRepository) func(context.Context, confirmation.ConfirmState) (string, bool) {
	return NewConfirmReplayFunc(
		noop.NewProvider(),
		DecisionAuditDeps{Factory: &stubDecisionFactory{repo: repo}, UoW: &stubUoW{}},
		nil,
	)
}

func (s *HITLDecisionSuite) newAuditBeginFunc(repo interfaces.AgentDecisionRepository) func(context.Context, confirmation.ConfirmState) steps.ConfirmAuditBeginResult {
	return NewConfirmAuditBeginFunc(
		noop.NewProvider(),
		DecisionAuditDeps{Factory: &stubDecisionFactory{repo: repo}, UoW: &stubUoW{}},
		nil,
	)
}

func (s *HITLDecisionSuite) TestReplayFunc_Found() {
	repo := &stubDecisionRepo{
		found: true,
		snapshot: interfaces.AgentDecisionSnapshot{
			RedactedResponse: []byte(`{"redacted":"ok"}`),
		},
	}
	replay := s.newReplayFunc(repo)
	state := confirmation.ConfirmState{
		UserID:          uuid.New().String(),
		Channel:         "whatsapp",
		ResumeMessageID: "wamid.replay",
	}

	reply, found := replay(s.ctx, state)

	s.True(found)
	s.Equal("ok", reply)
	s.Equal(1, repo.findCalls)
}

func (s *HITLDecisionSuite) TestReplayFunc_MissingMessageID() {
	repo := &stubDecisionRepo{found: true}
	replay := s.newReplayFunc(repo)
	state := confirmation.ConfirmState{UserID: uuid.New().String(), Channel: "whatsapp"}

	_, found := replay(s.ctx, state)

	s.False(found)
	s.Zero(repo.findCalls)
}

func (s *HITLDecisionSuite) TestReplayFunc_InvalidUserID() {
	repo := &stubDecisionRepo{found: true}
	replay := s.newReplayFunc(repo)
	state := confirmation.ConfirmState{UserID: "invalid", ResumeMessageID: "x"}

	_, found := replay(s.ctx, state)

	s.False(found)
	s.Zero(repo.findCalls)
}

func (s *HITLDecisionSuite) TestAuditBeginFunc_UsesOriginalMessageID() {
	repo := &stubDecisionRepo{}
	begin := s.newAuditBeginFunc(repo)
	uid := uuid.New()
	state := confirmation.ConfirmState{
		UserID:    uid.String(),
		Channel:   "whatsapp",
		MessageID: "wamid.original",
	}

	result := begin(s.ctx, state)

	s.NotEqual(uuid.Nil, result.DecisionID)
	s.NotNil(result.Settle)
}

func (s *HITLDecisionSuite) TestAuditBeginFunc_UsesResumeMessageID() {
	repo := &stubDecisionRepo{}
	begin := s.newAuditBeginFunc(repo)
	uid := uuid.New()
	state := confirmation.ConfirmState{
		UserID:          uid.String(),
		Channel:         "whatsapp",
		MessageID:       "wamid.original",
		ResumeMessageID: "wamid.resume",
	}

	result := begin(s.ctx, state)

	s.NotEqual(uuid.Nil, result.DecisionID)
	s.NotNil(result.Settle)
}

func (s *HITLDecisionSuite) TestAuditBeginFunc_NoDecisionRepo() {
	begin := NewConfirmAuditBeginFunc(noop.NewProvider(), DecisionAuditDeps{}, nil)
	state := confirmation.ConfirmState{UserID: uuid.New().String(), MessageID: "x"}

	result := begin(s.ctx, state)

	s.Equal(uuid.Nil, result.DecisionID)
	s.Nil(result.Settle)
}

func (s *HITLDecisionSuite) TestAuditBeginFunc_RecordsDecisionForSettle() {
	repo := &stubDecisionRepo{}
	begin := s.newAuditBeginFunc(repo)
	uid := uuid.New()
	state := confirmation.ConfirmState{
		UserID:    uid.String(),
		Channel:   "whatsapp",
		MessageID: "wamid.settle",
	}

	result := begin(s.ctx, state)
	result.Settle(s.ctx, true)

	s.NotEqual(uuid.Nil, result.DecisionID)
}

func (s *HITLDecisionSuite) TestAuditBeginFunc_RecordsDecisionForReject() {
	repo := &stubDecisionRepo{}
	begin := s.newAuditBeginFunc(repo)
	uid := uuid.New()
	state := confirmation.ConfirmState{
		UserID:    uid.String(),
		Channel:   "whatsapp",
		MessageID: "wamid.reject",
	}

	result := begin(s.ctx, state)
	result.Settle(s.ctx, false)

	s.NotEqual(uuid.Nil, result.DecisionID)
}

func (s *HITLDecisionSuite) TestAuditBeginFunc_Conflict() {
	repo := &stubDecisionRepo{insertErr: interfaces.ErrAgentDecisionConflict}
	begin := s.newAuditBeginFunc(repo)
	uid := uuid.New()
	state := confirmation.ConfirmState{
		UserID:    uid.String(),
		Channel:   "whatsapp",
		MessageID: "wamid.conflict",
	}

	result := begin(s.ctx, state)

	s.True(result.Conflicted)
}

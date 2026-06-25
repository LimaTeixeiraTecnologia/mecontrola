package services

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type stubDecisionRepo struct {
	insertErr error
	snapshot  interfaces.AgentDecisionSnapshot
	found     bool
	findErr   error
	findCalls int
}

func (r *stubDecisionRepo) Insert(_ context.Context, _ entities.AgentDecision) error {
	return r.insertErr
}

func (r *stubDecisionRepo) UpdateSettlement(_ context.Context, _ entities.AgentDecision) error {
	return nil
}

func (r *stubDecisionRepo) FindByMessage(_ context.Context, _ uuid.UUID, _, _ string, _ int) (interfaces.AgentDecisionSnapshot, bool, error) {
	r.findCalls++
	return r.snapshot, r.found, r.findErr
}

type stubDecisionFactory struct {
	repo interfaces.AgentDecisionRepository
}

func (f *stubDecisionFactory) AgentDecisionRepository(_ database.DBTX) interfaces.AgentDecisionRepository {
	return f.repo
}

type stubUoW struct {
	db database.DBTX
}

func (u *stubUoW) DBTX() database.DBTX {
	return u.db
}

func (u *stubUoW) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, u.db)
}

type stubRedactor struct {
	err error
}

func (r *stubRedactor) Clean(raw string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return raw, nil
}

type DecisionAuditSuite struct {
	suite.Suite
	ctx context.Context
}

func TestDecisionAuditSuite(t *testing.T) {
	suite.Run(t, new(DecisionAuditSuite))
}

func (s *DecisionAuditSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *DecisionAuditSuite) newTestAuditor(repo interfaces.AgentDecisionRepository, redactor DecisionRedactor) *decisionAuditor {
	return newDecisionAuditor(
		noop.NewProvider(),
		DecisionAuditDeps{
			Factory: &stubDecisionFactory{repo: repo},
			UoW:     &stubUoW{},
		},
		redactor,
	)
}

func (s *DecisionAuditSuite) validInput() decisionRecordInput {
	return decisionRecordInput{
		UserID:       uuid.New(),
		Channel:      "whatsapp",
		MessageID:    "wamid.abc123",
		IntentKind:   "record_expense",
		PromptSHA256: "a3f1e9b2c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1",
		LLMModel:     "openai/gpt-4o-mini",
		DirectReply:  "gasto registrado",
	}
}

func (s *DecisionAuditSuite) TestBeginSkipped_WhenLLMModelMissing() {
	a := s.newTestAuditor(&stubDecisionRepo{}, nil)
	in := s.validInput()
	in.LLMModel = ""
	dc := a.begin(s.ctx, in)
	s.False(dc.recorded, "esperava decisionContext nao gravado quando LLMModel ausente")
}

func (s *DecisionAuditSuite) TestBeginSkipped_WhenMessageIDMissing() {
	a := s.newTestAuditor(&stubDecisionRepo{}, nil)
	in := s.validInput()
	in.MessageID = ""
	dc := a.begin(s.ctx, in)
	s.False(dc.recorded, "esperava decisionContext nao gravado quando MessageID ausente")
}

func (s *DecisionAuditSuite) TestBeginSkipped_WhenPromptSHA256Missing() {
	a := s.newTestAuditor(&stubDecisionRepo{}, nil)
	in := s.validInput()
	in.PromptSHA256 = ""
	dc := a.begin(s.ctx, in)
	s.False(dc.recorded, "esperava decisionContext nao gravado quando PromptSHA256 ausente")
}

func (s *DecisionAuditSuite) TestRedactorFailure_ReturnsEmptyJSON() {
	redactor := &stubRedactor{err: errors.New("sanitizer: empty after normalization")}
	a := s.newTestAuditor(&stubDecisionRepo{}, redactor)
	result := a.redactResponse(s.ctx, "texto com pii", nil)
	s.Equal("{}", string(result), "esperava '{}' quando redactor falha")
}

func (s *DecisionAuditSuite) TestBegin_Recorded_WhenValid() {
	a := s.newTestAuditor(&stubDecisionRepo{}, &stubRedactor{})
	dc := a.begin(s.ctx, s.validInput())
	s.True(dc.recorded, "esperava decisionContext gravado com input valido")
	s.NotNil(dc.auditor, "esperava auditor nao nil no decisionContext")
}

func (s *DecisionAuditSuite) TestBegin_NotRecorded_WhenInsertFails() {
	repo := &stubDecisionRepo{insertErr: errors.New("db: connection refused")}
	a := s.newTestAuditor(repo, &stubRedactor{})
	dc := a.begin(s.ctx, s.validInput())
	s.False(dc.recorded, "esperava decisionContext nao gravado quando Insert falha")
	s.True(dc.failed, "esperava decisionContext failed quando Insert falha com erro nao-conflito")
}

func (s *DecisionAuditSuite) TestSettle_NoopWhenNotRecorded() {
	dc := decisionContext{}
	dc.settle(s.ctx, true)
}

func (s *DecisionAuditSuite) TestDisabledAuditor_BeginReturnsEmpty() {
	a := newDecisionAuditor(noop.NewProvider(), DecisionAuditDeps{}, nil)
	dc := a.begin(s.ctx, s.validInput())
	s.False(dc.recorded, "auditor desabilitado nao deve gravar")
}

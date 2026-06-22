package services

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type stubDecisionRepo struct {
	insertErr error
}

func (r *stubDecisionRepo) Insert(_ context.Context, _ entities.AgentDecision) error {
	return r.insertErr
}

func (r *stubDecisionRepo) UpdateSettlement(_ context.Context, _ entities.AgentDecision) error {
	return nil
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

func newTestAuditor(repo interfaces.AgentDecisionRepository, redactor DecisionRedactor) *decisionAuditor {
	return newDecisionAuditor(
		noop.NewProvider(),
		DecisionAuditDeps{
			Factory: &stubDecisionFactory{repo: repo},
			UoW:     &stubUoW{},
		},
		redactor,
	)
}

func validInput() decisionRecordInput {
	return decisionRecordInput{
		UserID:       uuid.New(),
		Channel:      "whatsapp",
		MessageID:    "wamid.abc123",
		IntentKind:   "log_expense",
		PromptSHA256: "a3f1e9b2c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1",
		LLMModel:     "openai/gpt-4o-mini",
		DirectReply:  "gasto registrado",
	}
}

func TestDecisionAudit_BeginSkipped_WhenLLMModelMissing(t *testing.T) {
	t.Parallel()
	a := newTestAuditor(&stubDecisionRepo{}, nil)
	in := validInput()
	in.LLMModel = ""
	dc := a.begin(context.Background(), in)
	if dc.recorded {
		t.Fatal("esperava decisionContext nao gravado quando LLMModel ausente")
	}
}

func TestDecisionAudit_BeginSkipped_WhenMessageIDMissing(t *testing.T) {
	t.Parallel()
	a := newTestAuditor(&stubDecisionRepo{}, nil)
	in := validInput()
	in.MessageID = ""
	dc := a.begin(context.Background(), in)
	if dc.recorded {
		t.Fatal("esperava decisionContext nao gravado quando MessageID ausente")
	}
}

func TestDecisionAudit_BeginSkipped_WhenPromptSHA256Missing(t *testing.T) {
	t.Parallel()
	a := newTestAuditor(&stubDecisionRepo{}, nil)
	in := validInput()
	in.PromptSHA256 = ""
	dc := a.begin(context.Background(), in)
	if dc.recorded {
		t.Fatal("esperava decisionContext nao gravado quando PromptSHA256 ausente")
	}
}

func TestDecisionAudit_RedactorFailure_ReturnsEmptyJSON(t *testing.T) {
	t.Parallel()
	redactor := &stubRedactor{err: errors.New("sanitizer: empty after normalization")}
	a := newTestAuditor(&stubDecisionRepo{}, redactor)
	result := a.redactResponse(context.Background(), "texto com pii", nil)
	if string(result) != "{}" {
		t.Fatalf("esperava '{}' quando redactor falha, obteve: %s", string(result))
	}
}

func TestDecisionAudit_Begin_Recorded_WhenValid(t *testing.T) {
	t.Parallel()
	a := newTestAuditor(&stubDecisionRepo{}, &stubRedactor{})
	dc := a.begin(context.Background(), validInput())
	if !dc.recorded {
		t.Fatal("esperava decisionContext gravado com input valido")
	}
	if dc.auditor == nil {
		t.Fatal("esperava auditor nao nil no decisionContext")
	}
}

func TestDecisionAudit_Begin_NotRecorded_WhenInsertFails(t *testing.T) {
	t.Parallel()
	repo := &stubDecisionRepo{insertErr: errors.New("db: connection refused")}
	a := newTestAuditor(repo, &stubRedactor{})
	dc := a.begin(context.Background(), validInput())
	if dc.recorded {
		t.Fatal("esperava decisionContext nao gravado quando Insert falha")
	}
}

func TestDecisionAudit_Settle_NoopWhenNotRecorded(t *testing.T) {
	t.Parallel()
	dc := decisionContext{}
	dc.settle(context.Background(), true)
}

func TestDecisionAudit_DisabledAuditor_BeginReturnsEmpty(t *testing.T) {
	t.Parallel()
	a := newDecisionAuditor(noop.NewProvider(), DecisionAuditDeps{}, nil)
	dc := a.begin(context.Background(), validInput())
	if dc.recorded {
		t.Fatal("auditor desabilitado nao deve gravar")
	}
}

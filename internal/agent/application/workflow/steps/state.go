package steps

import (
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
)

type ExpenseState struct {
	UserID          uuid.UUID
	Channel         string
	MessageID       string
	StepIndex       int
	Confidence      float64
	Kind            intent.Kind
	TransactionKind pendingexpense.TransactionKind

	AmountCents   int64
	Merchant      string
	CategoryHint  string
	PaymentMethod string
	Direction     string
	OccurredAt    string
	CategoryID    string
	SubcategoryID string
	CategoryPath  string
	Candidates    []string
	CandidateRefs []pendingexpense.CandidateRef
	AwaitingKind  pendingexpense.AwaitingKind
	Installments  int
	CardHint      string
	ForceCategory *string
	CardName      string

	ResumeText string

	Outcome      tools.ToolOutcome
	Reply        string
	DecisionID   uuid.UUID
	ShortCircuit bool
	LLMModel     string
	PromptSHA256 string
	DirectReply  string
	RawResponse  string
}

func (s ExpenseState) IsDone() bool {
	return s.ShortCircuit
}

func (s ExpenseState) ToDraft() pendingexpense.Draft {
	return pendingexpense.Draft{
		AmountCents:     s.AmountCents,
		Merchant:        s.Merchant,
		PaymentMethod:   s.PaymentMethod,
		Direction:       s.Direction,
		OccurredAt:      s.OccurredAt,
		CategoryID:      s.CategoryID,
		SubcategoryID:   s.SubcategoryID,
		CategoryPath:    s.CategoryPath,
		Candidates:      s.Candidates,
		CandidateRefs:   s.CandidateRefs,
		AwaitingKind:    s.AwaitingKind,
		TransactionKind: s.TransactionKind,
		Installments:    s.Installments,
		CardHint:        s.CardHint,
	}
}

func StateFromDraft(draft pendingexpense.Draft, userID uuid.UUID, channel, messageID string, decisionID uuid.UUID, resumeText string) ExpenseState {
	return ExpenseState{
		UserID:          userID,
		Channel:         channel,
		MessageID:       messageID,
		Kind:            resolveKindFromDraft(draft),
		TransactionKind: draft.TransactionKind,
		AmountCents:     draft.AmountCents,
		Merchant:        draft.Merchant,
		PaymentMethod:   draft.PaymentMethod,
		Direction:       draft.Direction,
		OccurredAt:      draft.OccurredAt,
		CategoryID:      draft.CategoryID,
		SubcategoryID:   draft.SubcategoryID,
		CategoryPath:    draft.CategoryPath,
		Candidates:      draft.Candidates,
		CandidateRefs:   draft.CandidateRefs,
		AwaitingKind:    draft.AwaitingKind,
		Installments:    draft.Installments,
		CardHint:        draft.CardHint,
		DecisionID:      decisionID,
		ResumeText:      resumeText,
	}
}

func resolveKindFromDraft(draft pendingexpense.Draft) intent.Kind {
	switch draft.TransactionKind {
	case pendingexpense.TransactionKindIncome:
		return intent.KindRecordIncome
	case pendingexpense.TransactionKindCardPurchase:
		return intent.KindRecordCardPurchase
	default:
		return intent.KindRecordExpense
	}
}

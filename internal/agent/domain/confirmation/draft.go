package confirmation

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
)

type OperationKind int

const (
	OperationDeleteLast OperationKind = iota + 1
	OperationEditLast
	OperationDeleteCard
	OperationBudgetCommit
	OperationDeleteByRef
	OperationEditByRef
)

func (o OperationKind) String() string {
	switch o {
	case OperationDeleteLast:
		return "delete_last"
	case OperationEditLast:
		return "edit_last"
	case OperationDeleteCard:
		return "delete_card"
	case OperationBudgetCommit:
		return "budget_commit"
	case OperationDeleteByRef:
		return "delete_by_ref"
	case OperationEditByRef:
		return "edit_by_ref"
	default:
		return "unknown"
	}
}

func (o OperationKind) IsValid() bool {
	return o >= OperationDeleteLast && o <= OperationEditByRef
}

var errInvalidOperationKind = errors.New("confirmation: invalid operation kind")

func ParseOperationKind(s string) (OperationKind, error) {
	switch s {
	case "delete_last":
		return OperationDeleteLast, nil
	case "edit_last":
		return OperationEditLast, nil
	case "delete_card":
		return OperationDeleteCard, nil
	case "budget_commit":
		return OperationBudgetCommit, nil
	case "delete_by_ref":
		return OperationDeleteByRef, nil
	case "edit_by_ref":
		return OperationEditByRef, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidOperationKind, s)
	}
}

type AwaitingApproval int

const (
	AwaitingNone AwaitingApproval = iota
	AwaitingConfirm
	AwaitingSelect
)

func (a AwaitingApproval) String() string {
	switch a {
	case AwaitingNone:
		return "none"
	case AwaitingConfirm:
		return "confirm"
	case AwaitingSelect:
		return "select"
	default:
		return "unknown"
	}
}

func (a AwaitingApproval) IsValid() bool {
	return a >= AwaitingNone && a <= AwaitingSelect
}

var errInvalidAwaitingApproval = errors.New("confirmation: invalid awaiting approval")

func ParseAwaitingApproval(s string) (AwaitingApproval, error) {
	switch s {
	case "none":
		return AwaitingNone, nil
	case "confirm":
		return AwaitingConfirm, nil
	case "select":
		return AwaitingSelect, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidAwaitingApproval, s)
	}
}

type ConfirmState struct {
	OperationKind            OperationKind     `json:"operation_kind"`
	AwaitingApproval         AwaitingApproval  `json:"awaiting_approval"`
	RepromptCount            int               `json:"reprompt_count"`
	MessageID                string            `json:"message_id"`
	StepIndex                int               `json:"step_index"`
	SuspendedAt              time.Time         `json:"suspended_at"`
	ShortCircuit             bool              `json:"short_circuit"`
	Expired                  bool              `json:"expired"`
	ResumeText               string            `json:"resume_text"`
	UserID                   string            `json:"user_id"`
	Channel                  string            `json:"channel"`
	PromptText               string            `json:"prompt_text"`
	Reply                    string            `json:"reply"`
	Outcome                  int               `json:"outcome"`
	NewAmountCents           int64             `json:"new_amount_cents"`
	CardName                 string            `json:"card_name"`
	BudgetDraftJSON          []byte            `json:"budget_draft_json,omitempty"`
	ResumeMessageID          string            `json:"resume_message_id"`
	DecisionID               string            `json:"decision_id"`
	TargetTransactionID      string            `json:"target_transaction_id"`
	TargetTransactionVersion int64             `json:"target_transaction_version"`
	TargetDescription        string            `json:"target_description,omitempty"`
	TargetAmountCents        int64             `json:"target_amount_cents,omitempty"`
	SearchQuery              string            `json:"search_query,omitempty"`
	Candidates               []TargetCandidate `json:"candidates,omitempty"`
	SelectRepromptCount      int               `json:"select_reprompt_count,omitempty"`
}

type TargetCandidate struct {
	TxID        string `json:"tx_id"`
	Version     int64  `json:"version"`
	Description string `json:"description"`
	AmountCents int64  `json:"amount_cents"`
	OccurredAt  string `json:"occurred_at"`
}

func (s ConfirmState) IsDone() bool {
	return s.ShortCircuit
}

type budgetDraftData struct {
	TotalCents  int64          `json:"total_cents"`
	Allocations map[string]int `json:"allocations"`
	Competence  string         `json:"competence"`
}

func (s *ConfirmState) SetBudgetDraft(d budgetdraft.Draft) error {
	data := budgetDraftData{
		TotalCents:  d.TotalCents(),
		Allocations: d.Allocations(),
		Competence:  d.Competence(),
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("confirmation: encode budget draft: %w", err)
	}
	s.BudgetDraftJSON = bytes
	return nil
}

func (s ConfirmState) BudgetDraft() (budgetdraft.Draft, error) {
	if len(s.BudgetDraftJSON) == 0 {
		return budgetdraft.New(""), nil
	}
	var data budgetDraftData
	if err := json.Unmarshal(s.BudgetDraftJSON, &data); err != nil {
		return budgetdraft.Draft{}, fmt.Errorf("confirmation: decode budget draft: %w", err)
	}
	return budgetdraft.Restore(data.TotalCents, data.Allocations, data.Competence)
}

package confirmation

import (
	"encoding/json"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
)

type ConfirmationSuite struct {
	suite.Suite
}

func TestConfirmationSuite(t *testing.T) {
	suite.Run(t, new(ConfirmationSuite))
}

func (s *ConfirmationSuite) TestOperationKindString() {
	cases := []struct {
		kind OperationKind
		want string
	}{
		{OperationDeleteLast, "delete_last"},
		{OperationEditLast, "edit_last"},
		{OperationDeleteCard, "delete_card"},
		{OperationBudgetCommit, "budget_commit"},
		{OperationKind(99), "unknown"},
	}
	for _, c := range cases {
		s.Equal(c.want, c.kind.String())
	}
}

func (s *ConfirmationSuite) TestOperationKindIsValid() {
	s.True(OperationDeleteLast.IsValid())
	s.True(OperationEditLast.IsValid())
	s.True(OperationDeleteCard.IsValid())
	s.True(OperationBudgetCommit.IsValid())
	s.False(OperationKind(0).IsValid())
	s.False(OperationKind(99).IsValid())
}

func (s *ConfirmationSuite) TestParseOperationKind() {
	valids := []struct {
		input string
		want  OperationKind
	}{
		{"delete_last", OperationDeleteLast},
		{"edit_last", OperationEditLast},
		{"delete_card", OperationDeleteCard},
		{"budget_commit", OperationBudgetCommit},
	}
	for _, c := range valids {
		got, err := ParseOperationKind(c.input)
		s.NoError(err)
		s.Equal(c.want, got)
	}

	invalids := []string{"", "DELETE_LAST", "unknown", "routed", "foo"}
	for _, inv := range invalids {
		_, err := ParseOperationKind(inv)
		s.Error(err, "expected error for %q", inv)
	}
}

func (s *ConfirmationSuite) TestAwaitingApprovalString() {
	cases := []struct {
		a    AwaitingApproval
		want string
	}{
		{AwaitingNone, "none"},
		{AwaitingConfirm, "confirm"},
		{AwaitingApproval(99), "unknown"},
	}
	for _, c := range cases {
		s.Equal(c.want, c.a.String())
	}
}

func (s *ConfirmationSuite) TestAwaitingApprovalIsValid() {
	s.True(AwaitingNone.IsValid())
	s.True(AwaitingConfirm.IsValid())
	s.False(AwaitingApproval(99).IsValid())
}

func (s *ConfirmationSuite) TestParseAwaitingApproval() {
	valids := []struct {
		input string
		want  AwaitingApproval
	}{
		{"none", AwaitingNone},
		{"confirm", AwaitingConfirm},
	}
	for _, c := range valids {
		got, err := ParseAwaitingApproval(c.input)
		s.NoError(err)
		s.Equal(c.want, got)
	}

	invalids := []string{"", "NONE", "awaiting", "foo"}
	for _, inv := range invalids {
		_, err := ParseAwaitingApproval(inv)
		s.Error(err, "expected error for %q", inv)
	}
}

func (s *ConfirmationSuite) TestConfirmStateIsDone() {
	s.False(ConfirmState{ShortCircuit: false}.IsDone())
	s.True(ConfirmState{ShortCircuit: true}.IsDone())
}

func (s *ConfirmationSuite) TestConfirmStateJSONRoundTrip() {
	original := ConfirmState{
		OperationKind:    OperationDeleteLast,
		AwaitingApproval: AwaitingConfirm,
		RepromptCount:    1,
		MessageID:        "msg-abc-123",
		SuspendedAt:      time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC),
		ShortCircuit:     false,
		ResumeText:       "sim",
		UserID:           "user-xyz",
		Channel:          "whatsapp",
	}

	data, err := json.Marshal(original)
	s.NoError(err)
	s.NotEmpty(data)

	var restored ConfirmState
	s.NoError(json.Unmarshal(data, &restored))

	s.Equal(original.OperationKind, restored.OperationKind)
	s.Equal(original.AwaitingApproval, restored.AwaitingApproval)
	s.Equal(original.RepromptCount, restored.RepromptCount)
	s.Equal(original.MessageID, restored.MessageID)
	s.True(original.SuspendedAt.Equal(restored.SuspendedAt))
	s.Equal(original.ShortCircuit, restored.ShortCircuit)
	s.Equal(original.ResumeText, restored.ResumeText)
	s.Equal(original.UserID, restored.UserID)
	s.Equal(original.Channel, restored.Channel)
}

func (s *ConfirmationSuite) TestConfirmStateMergePatchCompatible() {
	base := ConfirmState{
		OperationKind:    OperationEditLast,
		AwaitingApproval: AwaitingConfirm,
		RepromptCount:    0,
		MessageID:        "msg-001",
		UserID:           "user-1",
		Channel:          "telegram",
	}

	baseBytes, err := json.Marshal(base)
	s.NoError(err)

	patch := map[string]any{
		"resume_text": "nao",
	}
	patchBytes, err := json.Marshal(patch)
	s.NoError(err)

	var baseMap map[string]any
	s.NoError(json.Unmarshal(baseBytes, &baseMap))
	var patchMap map[string]any
	s.NoError(json.Unmarshal(patchBytes, &patchMap))

	maps.Copy(baseMap, patchMap)

	mergedBytes, err := json.Marshal(baseMap)
	s.NoError(err)

	var merged ConfirmState
	s.NoError(json.Unmarshal(mergedBytes, &merged))

	s.Equal(OperationEditLast, merged.OperationKind)
	s.Equal(AwaitingConfirm, merged.AwaitingApproval)
	s.Equal("msg-001", merged.MessageID)
	s.Equal("nao", merged.ResumeText)
}

func (s *ConfirmationSuite) TestConfirmState_BudgetDraftRoundTrip() {
	draft, err := budgetdraft.Restore(500000, map[string]int{
		budgetdraft.SlugCustoFixo:           3000,
		budgetdraft.SlugConhecimento:        2000,
		budgetdraft.SlugPrazeres:            2000,
		budgetdraft.SlugMetas:               1500,
		budgetdraft.SlugLiberdadeFinanceira: 1500,
	}, "2026-06")
	s.Require().NoError(err)

	state := ConfirmState{}
	s.NoError(state.SetBudgetDraft(draft))
	s.NotEmpty(state.BudgetDraftJSON)

	restored, err := state.BudgetDraft()
	s.NoError(err)
	s.Equal(draft.TotalCents(), restored.TotalCents())
	s.Equal(draft.Competence(), restored.Competence())
	s.Equal(draft.Allocations(), restored.Allocations())
}

func (s *ConfirmationSuite) TestConfirmState_BudgetDraftEmpty() {
	state := ConfirmState{}
	restored, err := state.BudgetDraft()
	s.NoError(err)
	s.Equal(int64(0), restored.TotalCents())
	s.Empty(restored.Competence())
	s.Empty(restored.Allocations())
}

func (s *ConfirmationSuite) TestConfirmState_FieldsRoundTrip() {
	original := ConfirmState{
		OperationKind:  OperationEditLast,
		NewAmountCents: 12345,
		CardName:       "Nubank",
	}

	data, err := json.Marshal(original)
	s.NoError(err)

	var restored ConfirmState
	s.NoError(json.Unmarshal(data, &restored))

	s.Equal(int64(12345), restored.NewAmountCents)
	s.Equal("Nubank", restored.CardName)
}

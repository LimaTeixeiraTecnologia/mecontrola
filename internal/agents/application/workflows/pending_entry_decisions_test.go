package workflows

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func baseState() PendingEntryState {
	return PendingEntryState{
		Status:      PendingStatusActive,
		Awaiting:    AwaitingSlotCategory,
		SuspendedAt: time.Now().UTC(),
	}
}

func TestDecideNewOperationReplacement_G7_01(t *testing.T) {
	state := baseState()
	msg := PendingMessage{Text: "Gastei R$ 150,00 na farmácia hoje, no pix"}
	decision := DecideNewOperationReplacement(state, msg)
	require.Equal(t, PendingActionReplace, decision.Action)
}

func TestDecideNewOperationReplacement_NoReplace(t *testing.T) {
	state := baseState()
	msg := PendingMessage{Text: "supermercado"}
	decision := DecideNewOperationReplacement(state, msg)
	require.Equal(t, PendingActionNone, decision.Action)
}

func TestDecidePendingResume_Expired_G7_08(t *testing.T) {
	state := baseState()
	state.SuspendedAt = time.Now().UTC().Add(-31 * time.Minute)
	msg := PendingMessage{Text: "supermercado"}
	now := time.Now().UTC()
	decision, err := DecidePendingResume(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, PendingActionExpire, decision.Action)
}

func TestDecidePendingResume_Cancel_G7_04(t *testing.T) {
	state := baseState()
	msg := PendingMessage{Text: "cancela"}
	now := time.Now().UTC()
	decision, err := DecidePendingResume(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, PendingActionCancel, decision.Action)
}

func TestDecidePendingResume_Cancel_G7_05(t *testing.T) {
	state := baseState()
	msg := PendingMessage{Text: "deixa pra lá"}
	now := time.Now().UTC()
	decision, err := DecidePendingResume(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, PendingActionCancel, decision.Action)
}

func TestDecidePendingResume_Cancel_G7_06(t *testing.T) {
	state := baseState()
	msg := PendingMessage{Text: "não registra"}
	now := time.Now().UTC()
	decision, err := DecidePendingResume(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, PendingActionCancel, decision.Action)
}

func TestDecidePendingResume_Replace_NewOperation(t *testing.T) {
	state := baseState()
	msg := PendingMessage{Text: "Gastei R$ 150,00 na farmácia hoje, no pix"}
	now := time.Now().UTC()
	decision, err := DecidePendingResume(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, PendingActionReplace, decision.Action)
}

func TestDecidePendingResume_Reprompt_G7_13(t *testing.T) {
	state := baseState()
	state.RepromptCount = 0
	msg := PendingMessage{Text: "tudo bem"}
	now := time.Now().UTC()
	decision, err := DecidePendingResume(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, PendingActionReprompt, decision.Action)
}

func TestDecidePendingResume_Cancel_After2Reprompts_G7_14(t *testing.T) {
	state := baseState()
	state.RepromptCount = 1
	msg := PendingMessage{Text: "ok sim"}
	now := time.Now().UTC()
	decision, err := DecidePendingResume(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, PendingActionCancel, decision.Action)
}

func TestDecidePendingResume_SimNaoPix_G7_07(t *testing.T) {
	state := baseState()
	state.Awaiting = AwaitingSlotCategory
	state.RepromptCount = 0
	msg := PendingMessage{Text: "sim e pix"}
	now := time.Now().UTC()
	decision, err := DecidePendingResume(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, PendingActionReprompt, decision.Action)
}

func TestDecideConfirmation_Accept(t *testing.T) {
	state := baseState()
	state.Awaiting = AwaitingSlotConfirmation
	now := time.Now().UTC()

	for _, text := range []string{"sim", "confirmar", "confirma", "ok", "pode"} {
		msg := PendingMessage{Text: text}
		decision, err := DecideConfirmation(state, msg, now)
		require.NoError(t, err)
		require.Equal(t, ConfirmActionAccept, decision.Action, "text=%q", text)
	}
}

func TestDecideConfirmation_Cancel_CA13_CA14(t *testing.T) {
	state := baseState()
	state.Awaiting = AwaitingSlotConfirmation
	now := time.Now().UTC()

	for _, text := range []string{"não", "nao", "cancela"} {
		msg := PendingMessage{Text: text}
		decision, err := DecideConfirmation(state, msg, now)
		require.NoError(t, err)
		require.Equal(t, ConfirmActionCancel, decision.Action, "text=%q", text)
	}
}

func TestDecideConfirmation_Ambiguous_Reprompt(t *testing.T) {
	state := baseState()
	state.Awaiting = AwaitingSlotConfirmation
	state.ConfirmRepromptCount = 0
	now := time.Now().UTC()
	msg := PendingMessage{Text: "talvez"}
	decision, err := DecideConfirmation(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, ConfirmActionReprompt, decision.Action)
}

func TestDecideConfirmation_Ambiguous_2nd_Cancels(t *testing.T) {
	state := baseState()
	state.Awaiting = AwaitingSlotConfirmation
	state.ConfirmRepromptCount = 1
	now := time.Now().UTC()
	msg := PendingMessage{Text: "sei lá"}
	decision, err := DecideConfirmation(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, ConfirmActionCancel, decision.Action)
}

func TestDecideConfirmation_Expired(t *testing.T) {
	state := baseState()
	state.Awaiting = AwaitingSlotConfirmation
	state.SuspendedAt = time.Now().UTC().Add(-31 * time.Minute)
	now := time.Now().UTC()
	msg := PendingMessage{Text: "sim"}
	decision, err := DecideConfirmation(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, ConfirmActionExpire, decision.Action)
}

func TestDecideConfirmation_Replay(t *testing.T) {
	state := baseState()
	state.Awaiting = AwaitingSlotConfirmation
	state.ProcessedMessageID = "wamid-123"
	now := time.Now().UTC()
	msg := PendingMessage{Text: "sim", MessageID: "wamid-123"}
	decision, err := DecideConfirmation(state, msg, now)
	require.NoError(t, err)
	require.Equal(t, ConfirmActionReplay, decision.Action)
}

func makeCandidate(rootSlug, subSlug string) PendingCategoryCandidate {
	return PendingCategoryCandidate{
		RootCategoryID:  uuid.New(),
		RootSlug:        rootSlug,
		SubcategoryID:   uuid.New(),
		SubcategorySlug: subSlug,
		Path:            rootSlug + " > " + subSlug,
	}
}

func TestDecideCategoryChoice_ByIndex_CA15(t *testing.T) {
	candidates := []PendingCategoryCandidate{
		makeCandidate("custo-fixo", "plano-de-saude"),
		makeCandidate("custo-fixo", "consultas-e-exames"),
		makeCandidate("custo-fixo", "terapia-e-saude-mental"),
	}
	state := baseState()

	decision, err := DecideCategoryChoice(state, candidates, "2")
	require.NoError(t, err)
	require.Equal(t, CategoryChoiceActionSelected, decision.Action)
	require.Equal(t, "consultas-e-exames", decision.Candidate.SubcategorySlug)
}

func TestDecideCategoryChoice_ByName_CA15(t *testing.T) {
	candidates := []PendingCategoryCandidate{
		makeCandidate("custo-fixo", "plano-de-saude"),
		makeCandidate("custo-fixo", "consultas-e-exames"),
		makeCandidate("custo-fixo", "terapia-e-saude-mental"),
	}
	state := baseState()

	decision, err := DecideCategoryChoice(state, candidates, "consultas-e-exames")
	require.NoError(t, err)
	require.Equal(t, CategoryChoiceActionSelected, decision.Action)
	require.Equal(t, "consultas-e-exames", decision.Candidate.SubcategorySlug)
}

func TestDecideCategoryChoice_RootOnly_G7_03(t *testing.T) {
	rootID := uuid.New()
	candidates := []PendingCategoryCandidate{
		{
			RootCategoryID:  rootID,
			RootSlug:        "custo-fixo",
			SubcategoryID:   rootID,
			SubcategorySlug: "custo-fixo",
			Path:            "custo-fixo",
		},
	}
	state := baseState()
	decision, err := DecideCategoryChoice(state, candidates, "1")
	require.NoError(t, err)
	require.Equal(t, CategoryChoiceActionRootOnly, decision.Action)
}

func TestDecideCategoryChoice_Reprompt_Incompatible(t *testing.T) {
	candidates := []PendingCategoryCandidate{
		makeCandidate("custo-fixo", "supermercado"),
	}
	state := baseState()
	decision, err := DecideCategoryChoice(state, candidates, "textocompletoaleatório")
	require.NoError(t, err)
	require.Equal(t, CategoryChoiceActionReprompt, decision.Action)
}

func TestDecidePendingResume_NoIO(t *testing.T) {
	state := baseState()
	msg := PendingMessage{Text: "supermercado"}
	now := time.Now().UTC()
	_, err := DecidePendingResume(state, msg, now)
	require.NoError(t, err)
}

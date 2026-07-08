package workflows

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
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

func TestParseWeekday(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	scenarios := []struct {
		text string
		want string
		ok   bool
	}{
		{"segunda", "2026-07-06", true},
		{"segunda-feira", "2026-07-06", true},
		{"terca", "2026-06-30", true},
		{"terca-feira", "2026-06-30", true},
		{"terça", "2026-06-30", true},
		{"terça-feira", "2026-06-30", true},
		{"quarta", "2026-07-01", true},
		{"quarta-feira", "2026-07-01", true},
		{"quinta", "2026-07-02", true},
		{"quinta-feira", "2026-07-02", true},
		{"sexta", "2026-07-03", true},
		{"sexta-feira", "2026-07-03", true},
		{"sabado", "2026-07-04", true},
		{"sábado", "2026-07-04", true},
		{"domingo", "2026-07-05", true},
		{"segunda passada", "2026-06-29", true},
		{"segunda-feira passada", "2026-06-29", true},
		{"sabado passado", "2026-06-27", true},
		{"sábado passado", "2026-06-27", true},
		{"domingo passado", "2026-06-28", true},
		{"semana passada", "", false},
		{"mes passado", "", false},
		{"mês passado", "", false},
		{"amanha", "", false},
		{"", "", false},
		{"hoje", "", false},
	}

	for _, s := range scenarios {
		got, ok := parseWeekday(s.text, now)
		require.Equal(t, s.ok, ok, "text=%q ok mismatch", s.text)
		require.Equal(t, s.want, got, "text=%q date mismatch", s.text)
	}
}

func TestParseInputDate_Weekday(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	scenarios := []struct {
		text string
		want string
	}{
		{"segunda", "2026-07-06"},
		{"terça-feira", "2026-06-30"},
		{"sábado passado", "2026-06-27"},
		{"semana passada", ""},
		{"mês passado", ""},
		{"hoje", "2026-07-06"},
		{"ontem", "2026-07-05"},
	}

	for _, s := range scenarios {
		got := parseInputDate(s.text, now)
		require.Equal(t, s.want, got, "text=%q", s.text)
	}
}

func TestKnownPaymentMethods_AllValuesParseValid(t *testing.T) {
	for key, val := range knownPaymentMethods {
		_, err := valueobjects.ParsePaymentMethod(val)
		require.NoError(t, err, "key=%q value=%q deve ser aceito por ParsePaymentMethod", key, val)
	}
}

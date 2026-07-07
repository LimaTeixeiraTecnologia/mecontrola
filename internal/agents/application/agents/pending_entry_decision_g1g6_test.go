package agents

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
)

var hLibFinanceiraRootID = uuid.MustParse("35ced21e-b436-5cea-afb9-ffd43f98a124")

func hMsg(text string) workflows.PendingMessage {
	return workflows.PendingMessage{Text: text, MessageID: "wamid-new"}
}

func TestDecidePendingResume_Cancel(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:      workflows.PendingStatusActive,
		Awaiting:    workflows.AwaitingSlotCategory,
		SuspendedAt: time.Now().UTC(),
		MessageID:   "wamid-001",
	}
	cases := []struct {
		text   string
		expect workflows.PendingAction
	}{
		{"cancela", workflows.PendingActionCancel},
		{"cancelar", workflows.PendingActionCancel},
		{"deixa pra lá", workflows.PendingActionCancel},
		{"não registra", workflows.PendingActionCancel},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			decision, err := workflows.DecidePendingResume(state, hMsg(tc.text), time.Now().UTC())
			require.NoError(t, err)
			assert.Equal(t, tc.expect, decision.Action)
		})
	}
}

func TestDecidePendingResume_Replace_NewOperation(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:      workflows.PendingStatusActive,
		Awaiting:    workflows.AwaitingSlotCategory,
		SuspendedAt: time.Now().UTC(),
		MessageID:   "wamid-001",
	}
	cases := []struct {
		name string
		text string
	}{
		{"value_pix", "Gastei R$ 150,00 no mercado, pix"},
		{"value_cartao", "Paguei R$ 50,00 no restaurante, cartão"},
		{"recebi", "Recebi R$ 3000,00 de salário"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := workflows.DecidePendingResume(state, hMsg(tc.text), time.Now().UTC())
			require.NoError(t, err)
			assert.Equal(t, workflows.PendingActionReplace, decision.Action)
		})
	}
}

func TestDecidePendingResume_Expired(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:      workflows.PendingStatusActive,
		Awaiting:    workflows.AwaitingSlotCategory,
		SuspendedAt: time.Now().UTC().Add(-31 * time.Minute),
		MessageID:   "wamid-001",
	}
	decision, err := workflows.DecidePendingResume(state, hMsg("supermercado"), time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, workflows.PendingActionExpire, decision.Action)
}

func TestDecidePendingResume_Reprompt_MaxReached(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotCategory,
		SuspendedAt:   time.Now().UTC(),
		RepromptCount: 1,
		MessageID:     "wamid-001",
	}
	decision, err := workflows.DecidePendingResume(state, hMsg("xpto"), time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, workflows.PendingActionCancel, decision.Action)
}

func TestDecidePendingResume_Reprompt_First(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotCategory,
		SuspendedAt:   time.Now().UTC(),
		RepromptCount: 0,
		MessageID:     "wamid-001",
	}
	decision, err := workflows.DecidePendingResume(state, hMsg("xpto"), time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, workflows.PendingActionReprompt, decision.Action)
}

func TestDecideConfirmation_Accept(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:      workflows.PendingStatusActive,
		Awaiting:    workflows.AwaitingSlotConfirmation,
		SuspendedAt: time.Now().UTC(),
		Candidates:  hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado"),
		MessageID:   "wamid-001",
	}
	cases := []string{"sim", "confirmar", "ok", "pode"}
	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			decision, err := workflows.DecideConfirmation(state, hMsg(text), time.Now().UTC())
			require.NoError(t, err)
			assert.Equal(t, workflows.ConfirmActionAccept, decision.Action)
		})
	}
}

func TestDecideConfirmation_Reject(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:      workflows.PendingStatusActive,
		Awaiting:    workflows.AwaitingSlotConfirmation,
		SuspendedAt: time.Now().UTC(),
		MessageID:   "wamid-001",
	}
	cases := []string{"não", "nao", "cancelar", "cancela"}
	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			decision, err := workflows.DecideConfirmation(state, hMsg(text), time.Now().UTC())
			require.NoError(t, err)
			assert.Equal(t, workflows.ConfirmActionCancel, decision.Action)
		})
	}
}

func TestDecideConfirmation_Ambiguous_Reprompt(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:               workflows.PendingStatusActive,
		Awaiting:             workflows.AwaitingSlotConfirmation,
		SuspendedAt:          time.Now().UTC(),
		ConfirmRepromptCount: 0,
		MessageID:            "wamid-001",
	}
	decision, err := workflows.DecideConfirmation(state, hMsg("talvez"), time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, workflows.ConfirmActionReprompt, decision.Action)
}

func TestDecideConfirmation_Ambiguous_SecondTime_Cancels(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:               workflows.PendingStatusActive,
		Awaiting:             workflows.AwaitingSlotConfirmation,
		SuspendedAt:          time.Now().UTC(),
		ConfirmRepromptCount: 1,
		MessageID:            "wamid-001",
	}
	decision, err := workflows.DecideConfirmation(state, hMsg("sei lá"), time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, workflows.ConfirmActionCancel, decision.Action)
}

func TestDecideConfirmation_Expired(t *testing.T) {
	state := workflows.PendingEntryState{
		Status:      workflows.PendingStatusActive,
		Awaiting:    workflows.AwaitingSlotConfirmation,
		SuspendedAt: time.Now().UTC().Add(-31 * time.Minute),
		MessageID:   "wamid-001",
	}
	decision, err := workflows.DecideConfirmation(state, hMsg("sim"), time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, workflows.ConfirmActionExpire, decision.Action)
}

func TestDecideCategoryChoice_SingleCandidate_Selected(t *testing.T) {
	state := workflows.PendingEntryState{
		Awaiting: workflows.AwaitingSlotCategory,
		Kind:     ifaces.CategoryKindExpense,
	}
	candidates := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")

	decision, err := workflows.DecideCategoryChoice(state, candidates, "1")
	require.NoError(t, err)
	assert.Equal(t, workflows.CategoryChoiceActionSelected, decision.Action)
	assert.Equal(t, hSupermercadoSubID, decision.Candidate.SubcategoryID)
}

func TestDecideCategoryChoice_RootOnly_Rejected(t *testing.T) {
	state := workflows.PendingEntryState{
		Awaiting: workflows.AwaitingSlotCategory,
		Kind:     ifaces.CategoryKindExpense,
	}
	candidates := []workflows.PendingCategoryCandidate{{
		RootCategoryID: hCustoFixoRootID,
		SubcategoryID:  hCustoFixoRootID,
		RootSlug:       "custo-fixo",
		Path:           "Custo Fixo",
	}}

	decision, err := workflows.DecideCategoryChoice(state, candidates, "1")
	require.NoError(t, err)
	assert.Equal(t, workflows.CategoryChoiceActionRootOnly, decision.Action)
}

func TestDecideCategoryChoice_ByIndexAndName_CA15(t *testing.T) {
	state := workflows.PendingEntryState{
		Awaiting: workflows.AwaitingSlotCategory,
		Kind:     ifaces.CategoryKindExpense,
	}
	candidates := []workflows.PendingCategoryCandidate{
		{RootCategoryID: hCustoFixoRootID, SubcategoryID: hPlanoSaudeSubID, SubcategorySlug: "plano-de-saude", Path: "Custo Fixo > Plano de Saúde"},
		{RootCategoryID: hCustoFixoRootID, SubcategoryID: hConsultasSubID, SubcategorySlug: "consultas-e-exames", Path: "Custo Fixo > Consultas e Exames"},
	}

	byIdx, errIdx := workflows.DecideCategoryChoice(state, candidates, "2")
	require.NoError(t, errIdx)
	assert.Equal(t, workflows.CategoryChoiceActionSelected, byIdx.Action)
	assert.Equal(t, hConsultasSubID, byIdx.Candidate.SubcategoryID)

	byName, errName := workflows.DecideCategoryChoice(state, candidates, "consultas-e-exames")
	require.NoError(t, errName)
	assert.Equal(t, workflows.CategoryChoiceActionSelected, byName.Action)
	assert.Equal(t, hConsultasSubID, byName.Candidate.SubcategoryID)
}

func TestCategoryPairs_G1_CustoFixo(t *testing.T) {
	assert.Equal(t, uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62"), hCustoFixoRootID)
	assert.Equal(t, uuid.MustParse("97fa4b86-d43c-5ad5-a99b-c88c8427fb30"), hSupermercadoSubID)
}

func TestCategoryPairs_G2_Prazeres(t *testing.T) {
	assert.Equal(t, uuid.MustParse("ac535261-4060-56ef-b2e8-57c8cc7032d1"), hPrazeresRootID)
	assert.Equal(t, uuid.MustParse("d539672d-961f-5553-b807-0e0156a63163"), hRestaurantesSubID)
}

func TestCategoryPairs_G3_Vendas(t *testing.T) {
	assert.Equal(t, uuid.MustParse("8dba4d69-834f-5bdb-8c8c-9f86a9b56858"), hVendasRootID)
}

func TestCategoryPairs_G4_Metas(t *testing.T) {
	assert.Equal(t, uuid.MustParse("f133508e-7dc3-58a3-96db-199d8fbd2987"), hMetasRootID)
	assert.Equal(t, uuid.MustParse("3ff5e6b5-b958-5848-9092-73eb541598fc"), hTecnologiaSubID)
}

func TestCategoryPairs_G5_CustoFixoSaude(t *testing.T) {
	assert.Equal(t, uuid.MustParse("af5619e0-3683-5b8c-b9fc-0b3ddfbd2075"), hConsultasSubID)
	assert.Equal(t, uuid.MustParse("c8f579ea-952b-5e24-beed-ef22fb845a4b"), hPlanoSaudeSubID)
	assert.Equal(t, uuid.MustParse("3ca95dd5-c630-5c03-bd47-071777bde81c"), hFarmaciaSubID)
}

func TestCategoryPairs_G6_LibFinanceira(t *testing.T) {
	assert.Equal(t, uuid.MustParse("35ced21e-b436-5cea-afb9-ffd43f98a124"), hLibFinanceiraRootID)
}

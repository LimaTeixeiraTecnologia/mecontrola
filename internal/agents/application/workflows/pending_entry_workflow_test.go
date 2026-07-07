package workflows

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	ifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type PendingEntryWorkflowSuite struct {
	suite.Suite
	ctx    context.Context
	store  *wfStore
	engine workflow.Engine[PendingEntryState]
	def    workflow.Definition[PendingEntryState]
	ledger *imocks.TransactionsLedger
	userID uuid.UUID
}

func TestPendingEntryWorkflowSuite(t *testing.T) {
	suite.Run(t, new(PendingEntryWorkflowSuite))
}

func (s *PendingEntryWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = newWfStore()
	s.ledger = imocks.NewTransactionsLedger(s.T())
	s.engine = workflow.NewEngine[PendingEntryState](s.store, fake.NewProvider())
	s.def = BuildPendingEntryWorkflow(s.ledger, nil, nil)
	s.userID = uuid.New()
}

func (s *PendingEntryWorkflowSuite) newState(awaiting AwaitingSlot) PendingEntryState {
	return PendingEntryState{
		Status:        PendingStatusActive,
		Awaiting:      awaiting,
		OperationKind: PendingOpRegisterExpense,
		UserID:        s.userID,
		ResourceID:    s.userID,
		ThreadID:      "thr-001",
		MessageID:     "wamid-001",
		OriginalText:  "Gastei R$ 150,00 no mercado, pix",
		AmountCents:   15000,
		Description:   "mercado",
		PaymentMethod: "pix",
		Candidates: []PendingCategoryCandidate{{
			RootCategoryID:  uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62"),
			RootSlug:        "custo-fixo",
			SubcategoryID:   uuid.MustParse("c2fda6a3-c329-52c8-81ea-771b6ea4f365"),
			SubcategorySlug: "aluguel",
			Path:            "Custo Fixo > Aluguel",
		}},
		CategoryVersion: 1,
	}
}

func (s *PendingEntryWorkflowSuite) key(suffix string) string {
	return PendingEntryKey(s.userID.String(), suffix)
}

func (s *PendingEntryWorkflowSuite) resumePayload(text string) []byte {
	b, _ := json.Marshal(map[string]string{"resumeText": text})
	return b
}

func (s *PendingEntryWorkflowSuite) insertSuspended(k string, state PendingEntryState) {
	codec := workflow.NewCodec[PendingEntryState]()
	encoded, err := codec.Encode(state)
	s.Require().NoError(err)
	snap := workflow.Snapshot{
		RunID:          uuid.New(),
		Workflow:       PendingEntryWorkflowID,
		CorrelationKey: k,
		Status:         workflow.RunStatusSuspended,
		Version:        1,
		State:          encoded,
	}
	s.Require().NoError(s.store.Insert(s.ctx, snap))
}

func (s *PendingEntryWorkflowSuite) TestStart_SuspendsWithCategoryPrompt_Subtask2_2() {
	state := s.newState(AwaitingSlotCategory)
	k := s.key("thr-start-cat")

	result, err := s.engine.Start(s.ctx, s.def, k, state)

	s.NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(PendingStatusActive, result.State.Status)
	s.Equal(AwaitingSlotCategory, result.State.Awaiting)
	s.NotEmpty(result.State.ResponseText)
	s.False(result.State.SuspendedAt.IsZero())
}

func (s *PendingEntryWorkflowSuite) TestStart_FullySpecified_SuspendsWithConfirmation_RF_38() {
	state := s.newState(AwaitingSlotConfirmation)
	k := s.key("thr-start-confirm")

	result, err := s.engine.Start(s.ctx, s.def, k, state)

	s.NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(AwaitingSlotConfirmation, result.State.Awaiting)
	s.Contains(result.State.ResponseText, "Confirma")
	s.False(result.State.SuspendedAt.IsZero())
}

func (s *PendingEntryWorkflowSuite) TestResume_Cancel_Explicit_G7_04() {
	state := s.newState(AwaitingSlotCategory)
	k := s.key("thr-cancel-explicit")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("cancela"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCancelled, result.State.Status)
	s.NotEmpty(result.State.ResponseText)
}

func (s *PendingEntryWorkflowSuite) TestResume_Cancel_DeixaPraLa_G7_05() {
	state := s.newState(AwaitingSlotCategory)
	k := s.key("thr-cancel-deixa")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("deixa pra lá"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Cancel_NaoRegistra_G7_06() {
	state := s.newState(AwaitingSlotCategory)
	k := s.key("thr-cancel-naoreg")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("não registra"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Expired_G7_08() {
	state := s.newState(AwaitingSlotCategory)
	state.SuspendedAt = time.Now().UTC().Add(-31 * time.Minute)
	k := s.key("thr-expired")

	s.insertSuspended(k, state)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("supermercado"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusExpired, result.State.Status)
	s.Contains(result.State.ResponseText, "expirou")
}

func (s *PendingEntryWorkflowSuite) TestResume_Replace_NewCompleteOperation_G7_01() {
	state := s.newState(AwaitingSlotCategory)
	k := s.key("thr-replace")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("Gastei R$ 150,00 na farmácia hoje, no pix"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusReplaced, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Reprompt_AmbiguousText_G7_07() {
	state := s.newState(AwaitingSlotCategory)
	k := s.key("thr-reprompt")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("talvez"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(1, result.State.RepromptCount)
	s.Equal(PendingStatusActive, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Reprompt_MaxReached_Cancels() {
	state := s.newState(AwaitingSlotCategory)
	state.RepromptCount = maxReprompts
	k := s.key("thr-reprompt-max")

	s.insertSuspended(k, state)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("xpto"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Confirmation_Accept_Sim() {
	state := s.newState(AwaitingSlotConfirmation)
	k := s.key("thr-confirm-accept")

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCompleted, result.State.Status)
	s.NotContains(result.State.ResponseText, "Não consegui registrar")
}

func (s *PendingEntryWorkflowSuite) newCategoryState() PendingEntryState {
	state := s.newState(AwaitingSlotCategory)
	state.Candidates = []PendingCategoryCandidate{
		{
			RootCategoryID:  uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62"),
			RootSlug:        "custo-fixo",
			SubcategoryID:   uuid.MustParse("c2fda6a3-c329-52c8-81ea-771b6ea4f365"),
			SubcategorySlug: "aluguel",
			Path:            "Custo Fixo > Aluguel",
		},
		{
			RootCategoryID:  uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			RootSlug:        "moradia",
			SubcategoryID:   uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			SubcategorySlug: "condominio",
			Path:            "Moradia > Condominio",
		},
	}
	return state
}

func (s *PendingEntryWorkflowSuite) TestResume_CategorySelectionByNumber_MovesToConfirmation_CA15() {
	state := s.newCategoryState()
	k := s.key("thr-cat-number")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("2"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(AwaitingSlotConfirmation, result.State.Awaiting)
	s.Require().Len(result.State.Candidates, 1)
	s.Equal("condominio", result.State.Candidates[0].SubcategorySlug)
}

func (s *PendingEntryWorkflowSuite) TestResume_CategorySelectionByName_MovesToConfirmation_CA01() {
	state := s.newCategoryState()
	k := s.key("thr-cat-name")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("aluguel"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(AwaitingSlotConfirmation, result.State.Awaiting)
	s.Require().Len(result.State.Candidates, 1)
	s.Equal("aluguel", result.State.Candidates[0].SubcategorySlug)
}

func (s *PendingEntryWorkflowSuite) TestResume_TwoPendencies_IsolatedByThread_M06() {
	stateA := s.newState(AwaitingSlotConfirmation)
	stateB := s.newState(AwaitingSlotConfirmation)
	kA := s.key("thr-M06-A")
	kB := s.key("thr-M06-B")

	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.Anything).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, s.def, kA, stateA)
	s.Require().NoError(err)
	_, err = s.engine.Start(s.ctx, s.def, kB, stateB)
	s.Require().NoError(err)

	resultA, err := s.engine.Resume(s.ctx, s.def, kA, s.resumePayload("sim"))
	s.NoError(err)
	s.Equal(PendingStatusCompleted, resultA.State.Status)

	loaded, found, loadErr := s.store.Load(s.ctx, PendingEntryWorkflowID, kB)
	s.Require().NoError(loadErr)
	s.Require().True(found)
	s.Equal(workflow.RunStatusSuspended, loaded.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Confirmation_CancelExplicit() {
	state := s.newState(AwaitingSlotConfirmation)
	k := s.key("thr-confirm-cancel")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("não"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Confirmation_Reprompt_CA14() {
	state := s.newState(AwaitingSlotConfirmation)
	k := s.key("thr-confirm-reprompt")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("talvez"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(1, result.State.ConfirmRepromptCount)
}

func (s *PendingEntryWorkflowSuite) TestResume_Confirmation_2ndAmbiguous_Cancels_CA14() {
	state := s.newState(AwaitingSlotConfirmation)
	state.ConfirmRepromptCount = maxReprompts
	k := s.key("thr-confirm-2nd-ambig")

	s.insertSuspended(k, state)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("talvez sim"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestResume_Confirmation_Expired_CA08() {
	state := s.newState(AwaitingSlotConfirmation)
	state.SuspendedAt = time.Now().UTC().Add(-31 * time.Minute)
	k := s.key("thr-confirm-expired")

	s.insertSuspended(k, state)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("sim"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusExpired, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestErrRunAlreadyExists_NoDoubleWrite() {
	state := s.newState(AwaitingSlotCategory)
	k := s.key("thr-dedup")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	_, err2 := s.engine.Start(s.ctx, s.def, k, state)
	s.Error(err2)
	s.ErrorIs(err2, workflow.ErrRunAlreadyExists)
}

func (s *PendingEntryWorkflowSuite) TestMergePatch_StatePreserved_R_WF_KERNEL_001_7() {
	state := s.newState(AwaitingSlotCategory)
	state.Description = "mercado-original"
	state.AmountCents = 15000
	k := s.key("thr-mergepatch")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, s.def, k, s.resumePayload("cancela"))

	s.NoError(err)
	s.Equal("mercado-original", result.State.Description)
	s.Equal(int64(15000), result.State.AmountCents)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryWorkflowSuite) TestPendingEntryKey_Format() {
	k := PendingEntryKey("res-123", "thr-456")
	s.Equal("res-123:thr-456:pending-entry", k)
}

func (s *PendingEntryWorkflowSuite) TestBuildPendingEntryReaper_NotNil() {
	reaper := BuildPendingEntryReaper(s.store, fake.NewProvider())
	s.NotNil(reaper)
}

func (s *PendingEntryWorkflowSuite) TestStart_PaymentMethodSlot_Suspends() {
	state := s.newState(AwaitingSlotPaymentMethod)
	k := s.key("thr-start-payment")

	result, err := s.engine.Start(s.ctx, s.def, k, state)

	s.NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(AwaitingSlotPaymentMethod, result.State.Awaiting)
	s.NotEmpty(result.State.ResponseText)
}

func (s *PendingEntryWorkflowSuite) TestResume_MergePatchDelta_OnlyUpdatesResumeText() {
	state := s.newState(AwaitingSlotCategory)
	state.AmountCents = 32000
	k := s.key("thr-delta-patch")

	_, err := s.engine.Start(s.ctx, s.def, k, state)
	s.Require().NoError(err)

	patchBytes, _ := json.Marshal(map[string]string{"resumeText": "cancela"})
	result, err := s.engine.Resume(s.ctx, s.def, k, patchBytes)

	s.NoError(err)
	s.Equal(int64(32000), result.State.AmountCents)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

package workflows

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	ifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type PendingEntryCardSuite struct {
	suite.Suite
	ctx    context.Context
	store  *wfStore
	engine workflow.Engine[PendingEntryState]
	ledger *imocks.TransactionsLedger
	cards  *imocks.CardManager
	userID uuid.UUID
	cardID uuid.UUID
}

func TestPendingEntryCardSuite(t *testing.T) {
	suite.Run(t, new(PendingEntryCardSuite))
}

func (s *PendingEntryCardSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = newWfStore()
	s.ledger = imocks.NewTransactionsLedger(s.T())
	s.cards = imocks.NewCardManager(s.T())
	s.engine = workflow.NewEngine[PendingEntryState](s.store, fake.NewProvider())
	s.userID = uuid.New()
	s.cardID = uuid.New()
}

func (s *PendingEntryCardSuite) buildDef() workflow.Definition[PendingEntryState] {
	return BuildPendingEntryWorkflow(s.ledger, s.cards, nil)
}

func (s *PendingEntryCardSuite) cardState() PendingEntryState {
	return PendingEntryState{
		Status:        PendingStatusActive,
		Awaiting:      AwaitingSlotCard,
		OperationKind: PendingOpRegisterExpense,
		UserID:        s.userID,
		ResourceID:    s.userID,
		ThreadID:      "thr-card-001",
		MessageID:     "wamid-card-001",
		AmountCents:   20000,
		Description:   "compra no cartão",
		PaymentMethod: "credit_card",
		Kind:          ifaces.CategoryKindExpense,
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

func (s *PendingEntryCardSuite) resume(text string) []byte {
	b, _ := json.Marshal(map[string]string{"resumeText": text})
	return b
}

func (s *PendingEntryCardSuite) TestAwaitCard_Resolved_MovesToConfirmation_CA10_G7_16() {
	def := s.buildDef()
	k := s.userID.String() + ":thr-card-001:pending-entry"

	s.cards.EXPECT().
		ResolveCardByNickname(mock.Anything, s.userID, "nubank").
		Return(ifaces.Card{ID: s.cardID.String(), Nickname: "nubank"}, nil).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, s.cardState())
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resume("nubank"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(AwaitingSlotConfirmation, result.State.Awaiting)
	s.NotNil(result.State.CardID)
	s.Equal(s.cardID, *result.State.CardID)
	s.Contains(result.State.ResponseText, "Confirma")
}

func (s *PendingEntryCardSuite) TestAwaitCard_NotFound_Reprompt() {
	def := s.buildDef()
	k := s.userID.String() + ":thr-card-002:pending-entry"
	state := s.cardState()
	state.ThreadID = "thr-card-002"

	s.cards.EXPECT().
		ResolveCardByNickname(mock.Anything, s.userID, "cartaodesconhecido").
		Return(ifaces.Card{}, ifaces.ErrCardNotFound).
		Once()

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resume("cartaodesconhecido"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Equal(AwaitingSlotCard, result.State.Awaiting)
	s.Equal(1, result.State.RepromptCount)
}

func (s *PendingEntryCardSuite) TestAwaitCard_NotFound_MaxReprompts_Cancels() {
	def := s.buildDef()
	k := s.userID.String() + ":thr-card-003:pending-entry"
	state := s.cardState()
	state.ThreadID = "thr-card-003"
	state.RepromptCount = maxReprompts

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

	s.cards.EXPECT().
		ResolveCardByNickname(mock.Anything, s.userID, "xpto").
		Return(ifaces.Card{}, ifaces.ErrCardNotFound).
		Once()

	result, err := s.engine.Resume(s.ctx, def, k, s.resume("xpto"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryCardSuite) TestAwaitCard_Cancel_Explicit() {
	def := s.buildDef()
	k := s.userID.String() + ":thr-card-cancel:pending-entry"
	state := s.cardState()
	state.ThreadID = "thr-card-cancel"

	_, err := s.engine.Start(s.ctx, def, k, state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resume("cancela"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCancelled, result.State.Status)
}

func (s *PendingEntryCardSuite) TestAwaitCard_Resolved_ThenConfirm_FullFlow_G7_16() {
	def := s.buildDef()
	k := s.userID.String() + ":thr-card-full:pending-entry"

	s.cards.EXPECT().
		ResolveCardByNickname(mock.Anything, s.userID, "itau").
		Return(ifaces.Card{ID: s.cardID.String(), Nickname: "itau"}, nil).
		Once()
	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.MatchedBy(func(in ifaces.RawTransaction) bool {
			return in.PaymentMethod == "credit_card" && in.CardID != nil && *in.CardID == s.cardID
		})).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	state := s.cardState()
	state.ThreadID = "thr-card-full"

	_, err := s.engine.Start(s.ctx, def, s.userID.String()+":thr-card-full:pending-entry", state)
	s.Require().NoError(err)

	result, err := s.engine.Resume(s.ctx, def, k, s.resume("itau"))
	s.Require().NoError(err)
	s.Equal(AwaitingSlotConfirmation, result.State.Awaiting)

	result, err = s.engine.Resume(s.ctx, def, k, s.resume("sim"))

	s.NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCompleted, result.State.Status)
}

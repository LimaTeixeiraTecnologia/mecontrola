package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
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

type wfStore struct {
	mu   sync.RWMutex
	data map[string]workflow.Snapshot
}

func newWfStore() *wfStore {
	return &wfStore{data: make(map[string]workflow.Snapshot)}
}

func (s *wfStore) key(wid, ck string) string { return wid + "::" + ck }

func (s *wfStore) Insert(_ context.Context, snap workflow.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(snap.Workflow, snap.CorrelationKey)
	if ex, ok := s.data[k]; ok && (ex.Status == workflow.RunStatusRunning || ex.Status == workflow.RunStatusSuspended) {
		return workflow.ErrRunAlreadyExists
	}
	s.data[k] = snap
	return nil
}

func (s *wfStore) Load(_ context.Context, wid, key string) (workflow.Snapshot, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.data[s.key(wid, key)]
	return snap, ok, nil
}

func (s *wfStore) LoadLatest(_ context.Context, wid, key string) (workflow.Snapshot, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.data[s.key(wid, key)]
	return snap, ok, nil
}

func (s *wfStore) Save(_ context.Context, snap workflow.Snapshot, expected int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(snap.Workflow, snap.CorrelationKey)
	if ex, ok := s.data[k]; ok && ex.Version != expected {
		return workflow.ErrVersionConflict
	}
	s.data[k] = snap
	return nil
}

func (s *wfStore) AppendStep(_ context.Context, _ workflow.StepRecord) error { return nil }

func (s *wfStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

func (s *wfStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]workflow.Snapshot, error) {
	return nil, nil
}

type DestructiveConfirmSuite struct {
	suite.Suite
	ctx         context.Context
	store       *wfStore
	engine      workflow.Engine[ConfirmState]
	def         workflow.Definition[ConfirmState]
	ledger      *imocks.TransactionsLedger
	cards       *imocks.CardManager
	categories  *imocks.CategoriesReader
	recurrences *imocks.RecurrenceManager
	targetID    uuid.UUID
	userID      string
	key         string
}

func TestDestructiveConfirmSuite(t *testing.T) {
	suite.Run(t, new(DestructiveConfirmSuite))
}

func (s *DestructiveConfirmSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = newWfStore()
	s.ledger = imocks.NewTransactionsLedger(s.T())
	s.cards = imocks.NewCardManager(s.T())
	s.categories = imocks.NewCategoriesReader(s.T())
	s.recurrences = imocks.NewRecurrenceManager(s.T())
	s.engine = workflow.NewEngine[ConfirmState](s.store, fake.NewProvider())
	s.def = BuildDestructiveConfirmWorkflow(s.ledger, s.cards, s.categories, s.recurrences)
	s.targetID = uuid.New()
	s.userID = uuid.New().String()
	s.key = DestructiveConfirmKey(s.userID)
}

func (s *DestructiveConfirmSuite) startPendingDelete() {
	state := ConfirmState{
		Awaiting:    AwaitingConfirm,
		Operation:   OpDeleteEntry,
		TargetRef:   s.targetID.String(),
		TargetKind:  "transaction",
		ImpactNote:  "Será removido permanentemente.",
		SuspendedAt: time.Now().UTC(),
	}
	result, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
}

func (s *DestructiveConfirmSuite) TestNoSuspendedRun_NotHandled() {
	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.NoError(err)
	s.False(handled)
	s.Empty(response)
}

func (s *DestructiveConfirmSuite) TestConfirm_Sim_ExecutesDelete() {
	s.ledger.EXPECT().
		DeleteTransaction(mock.Anything, mock.AnythingOfType("interfaces.EntryRef"), int64(0)).
		Return(nil).
		Once()

	s.startPendingDelete()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
}

func (s *DestructiveConfirmSuite) TestConfirm_Confirmo_ExecutesDelete() {
	s.ledger.EXPECT().
		DeleteTransaction(mock.Anything, mock.AnythingOfType("interfaces.EntryRef"), int64(0)).
		Return(nil).
		Once()

	s.startPendingDelete()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "confirmo")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
}

func (s *DestructiveConfirmSuite) TestCancel_Nao_Discards() {
	s.startPendingDelete()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "não")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "cancelada")
}

func (s *DestructiveConfirmSuite) TestCancel_Cancelar_Discards() {
	s.startPendingDelete()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "cancelar")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "cancelada")
}

func (s *DestructiveConfirmSuite) TestAmbiguous_FirstTime_Reprompts() {
	s.startPendingDelete()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "talvez")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "sim")
}

func (s *DestructiveConfirmSuite) TestAmbiguous_SecondTime_Cancels() {
	s.startPendingDelete()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "talvez")
	s.Require().NoError(err)
	s.Require().True(handled)
	s.Contains(response, "sim")

	handled2, response2, err2 := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "quem sabe")
	s.NoError(err2)
	s.True(handled2)
	s.Contains(response2, "cancelad")
}

func (s *DestructiveConfirmSuite) TestTTL_Expired_Cancels() {
	state := ConfirmState{
		Awaiting:    AwaitingConfirm,
		Operation:   OpDeleteEntry,
		TargetRef:   s.targetID.String(),
		TargetKind:  "transaction",
		ImpactNote:  "Será removido.",
		SuspendedAt: time.Now().UTC().Add(-10 * time.Minute),
	}
	result, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.NoError(err)
	s.False(handled)
	s.Empty(response)
}

func (s *DestructiveConfirmSuite) TestDeleteCard_WithOpenInstallments_ImpactNote() {
	cardID := uuid.New()
	userID := uuid.New()
	s.cards.EXPECT().
		HasOpenInstallments(mock.Anything, cardID, userID).
		Return(true, nil).
		Once()

	note := BuildImpactNote(s.ctx, cardID.String(), "card", userID, s.cards)
	s.Contains(note, "parcelas")
}

func (s *DestructiveConfirmSuite) TestDeleteCard_NoOpenInstallments_SimpleNote() {
	cardID := uuid.New()
	userID := uuid.New()
	s.cards.EXPECT().
		HasOpenInstallments(mock.Anything, cardID, userID).
		Return(false, nil).
		Once()

	note := BuildImpactNote(s.ctx, cardID.String(), "card", userID, s.cards)
	s.Contains(note, "permanente")
}

func (s *DestructiveConfirmSuite) TestDeleteCard_Confirm_CallsSoftDelete() {
	cardID := uuid.New()
	s.cards.EXPECT().
		SoftDeleteCard(mock.Anything, cardID, mock.AnythingOfType("uuid.UUID")).
		Return(nil).
		Once()

	state := ConfirmState{
		Awaiting:    AwaitingConfirm,
		Operation:   OpDeleteCard,
		TargetRef:   cardID.String(),
		TargetKind:  "card",
		ImpactNote:  "Cartão será removido.",
		SuspendedAt: time.Now().UTC(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
}

func (s *DestructiveConfirmSuite) TestEditEntry_Confirm_CallsUpdateTransaction() {
	entryID := uuid.New()
	s.ledger.EXPECT().
		UpdateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawUpdateTransaction")).
		Return(ifaces.EntryRef{ID: entryID, Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	upd := map[string]any{"amountCents": int64(5000), "description": "Almoço"}
	payload, _ := json.Marshal(upd)

	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpEditEntry,
		TargetRef:     entryID.String(),
		TargetKind:    "transaction",
		ImpactNote:    "Lançamento será atualizado.",
		SuspendedAt:   time.Now().UTC(),
		UpdatePayload: string(payload),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
}

func (s *DestructiveConfirmSuite) TestConfirmState_TypesClosed() {
	s.True(AwaitingNone.IsValid())
	s.True(AwaitingConfirm.IsValid())
	s.False(AwaitingKind(0).IsValid())
	s.False(AwaitingKind(99).IsValid())

	s.True(OpDeleteEntry.IsValid())
	s.True(OpEditEntry.IsValid())
	s.True(OpDeleteCard.IsValid())
	s.False(OperationKind(0).IsValid())
	s.False(OperationKind(99).IsValid())
}

func (s *DestructiveConfirmSuite) TestParseAwaitingKind_RoundTrip() {
	for _, k := range []AwaitingKind{AwaitingNone, AwaitingConfirm} {
		parsed, err := ParseAwaitingKind(k.String())
		s.NoError(err)
		s.Equal(k, parsed)
	}
	_, err := ParseAwaitingKind("invalid")
	s.Error(err)
}

func (s *DestructiveConfirmSuite) TestParseOperationKind_RoundTrip() {
	for _, o := range []OperationKind{OpDeleteEntry, OpEditEntry, OpDeleteCard} {
		parsed, err := ParseOperationKind(o.String())
		s.NoError(err)
		s.Equal(o, parsed)
	}
	_, err := ParseOperationKind("invalid")
	s.Error(err)
}

func (s *DestructiveConfirmSuite) TestResumeBeforeParse_OrderDeterministic() {
	s.ledger.EXPECT().
		DeleteTransaction(mock.Anything, mock.AnythingOfType("interfaces.EntryRef"), int64(0)).
		Return(nil).
		Once()

	s.startPendingDelete()

	handled, _, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.NoError(err)
	s.True(handled, "resume deve ocorrer antes do parse (retornou handled=true)")

	handledAgain, _, err2 := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "outra mensagem")
	s.NoError(err2)
	s.False(handledAgain, "run já completado não deve ser retomado")
}

func (s *DestructiveConfirmSuite) TestDeterministicCleanup_NoOrphanRun() {
	s.ledger.EXPECT().
		DeleteTransaction(mock.Anything, mock.AnythingOfType("interfaces.EntryRef"), int64(0)).
		Return(nil).
		Once()

	s.startPendingDelete()

	_, _, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.Require().NoError(err)

	snap, found, err := s.store.Load(s.ctx, DestructiveConfirmWorkflowID, s.key)
	s.NoError(err)
	s.True(found)
	s.Equal(workflow.RunStatusSucceeded, snap.Status, "run deve estar Succeeded, nunca Suspended após confirmação")
}

func (s *DestructiveConfirmSuite) TestCancel_DeterministicCleanup() {
	s.startPendingDelete()

	_, _, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "não")
	s.Require().NoError(err)

	snap, found, err := s.store.Load(s.ctx, DestructiveConfirmWorkflowID, s.key)
	s.NoError(err)
	s.True(found)
	s.Equal(workflow.RunStatusSucceeded, snap.Status, "run deve estar Succeeded após cancelamento")
}

func (s *DestructiveConfirmSuite) TestDeleteEntry_CreditCardKind_RoutesToDeleteTransaction() {
	s.ledger.EXPECT().
		DeleteTransaction(mock.Anything, mock.AnythingOfType("interfaces.EntryRef"), int64(0)).
		Return(nil).
		Once()

	entryID := uuid.New()
	state := ConfirmState{
		Awaiting:    AwaitingConfirm,
		Operation:   OpDeleteEntry,
		TargetRef:   entryID.String(),
		TargetKind:  "transaction",
		ImpactNote:  "⚠️ Todas as parcelas serão removidas.",
		SuspendedAt: time.Now().UTC(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "ok")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
}

func (s *DestructiveConfirmSuite) TestBindingError_ReturnsError() {
	s.ledger.EXPECT().
		DeleteTransaction(mock.Anything, mock.AnythingOfType("interfaces.EntryRef"), int64(0)).
		Return(errors.New("banco indisponível")).
		Once()

	s.startPendingDelete()

	handled, _, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.True(handled)
	s.Error(err)
}

func (s *DestructiveConfirmSuite) TestOpConfirmRegister_StringIsValid() {
	s.Equal("confirm_register", OpConfirmRegister.String())
	s.True(OpConfirmRegister.IsValid())
	op, err := ParseOperationKind("confirm_register")
	s.NoError(err)
	s.Equal(OpConfirmRegister, op)
}

func (s *DestructiveConfirmSuite) TestOpConfirmRegister_SuspendsAndAskCategory() {
	draft := ifaces.RawTransaction{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   5000,
		Description:   "padaria",
		OccurredAt:    "2026-07-02",
	}
	payload, _ := json.Marshal(draft)
	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpConfirmRegister,
		TargetKind:    "transaction",
		UpdatePayload: string(payload),
		SuspendedAt:   time.Now().UTC(),
		UserID:        uuid.New(),
	}
	result, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)
	s.Contains(result.State.ResponseText, "categoria")
}

func (s *DestructiveConfirmSuite) TestOpConfirmRegister_CategoryResolved_RegistersAndCompletes() {
	rootID := uuid.New()
	leafID := uuid.New()
	draft := ifaces.RawTransaction{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   5000,
		Description:   "padaria",
		OccurredAt:    "2026-07-02",
	}
	payload, _ := json.Marshal(draft)
	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpConfirmRegister,
		TargetKind:    "transaction",
		UpdatePayload: string(payload),
		SuspendedAt:   time.Now().UTC(),
		UserID:        uuid.New(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "alimentação", "expense").
		Return(ifaces.CategorySearchResult{
			Outcome: ifaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []ifaces.CategoryCandidate{{
				CategoryID:     leafID,
				RootCategoryID: rootID,
				Score:          0.9,
				Confidence:     "high",
				MatchQuality:   "exact",
				SignalType:     "alias",
				MatchedTerm:    "alimentação",
				MatchReason:    "alias match",
			}},
		}, nil).
		Once()
	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.MatchedBy(func(r ifaces.RawTransaction) bool {
			return r.CategoryID == rootID &&
				r.SubcategoryID != nil && *r.SubcategoryID == leafID &&
				r.Description == "padaria" &&
				r.CategorySource == "user_selected_candidate"
		})).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "alimentação")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
	s.Contains(response, "registrado")
}

func (s *DestructiveConfirmSuite) TestOpConfirmRegister_CategoryNotFound_Reprompts() {
	draft := ifaces.RawTransaction{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   5000,
		Description:   "padaria",
		OccurredAt:    "2026-07-02",
	}
	payload, _ := json.Marshal(draft)
	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpConfirmRegister,
		TargetKind:    "transaction",
		UpdatePayload: string(payload),
		SuspendedAt:   time.Now().UTC(),
		UserID:        uuid.New(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "xyzxyz", "expense").
		Return(ifaces.CategorySearchResult{}, errors.New("não encontrado")).
		Once()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "xyzxyz")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "categoria")

	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "xyzxyz", "expense").
		Return(ifaces.CategorySearchResult{}, errors.New("não encontrado")).
		Once()

	handled2, response2, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "xyzxyz")
	s.NoError(err)
	s.True(handled2)
	s.Contains(response2, "cancelado")
}

func (s *DestructiveConfirmSuite) TestParseOperationKind_NewKinds() {
	cases := []struct {
		str string
		op  OperationKind
	}{
		{"update_recurrence", OpUpdateRecurrence},
		{"delete_recurrence", OpDeleteRecurrence},
		{"update_card", OpUpdateCard},
	}
	for _, c := range cases {
		parsed, err := ParseOperationKind(c.str)
		s.NoError(err)
		s.Equal(c.op, parsed)
		s.Equal(c.str, c.op.String())
		s.True(c.op.IsValid())
	}
	_, err := ParseOperationKind("invalid_kind")
	s.Error(err)
}

func (s *DestructiveConfirmSuite) TestOpUpdateRecurrence_Confirm_CallsUpdateRecurrence() {
	templateID := uuid.New().String()
	upd := ifaces.RawUpdateRecurrence{
		AmountCents: func() *int64 { v := int64(8000); return &v }(),
		Version:     1,
	}
	payload, _ := json.Marshal(upd)

	s.recurrences.EXPECT().
		UpdateRecurrence(mock.Anything, templateID, mock.AnythingOfType("interfaces.RawUpdateRecurrence")).
		Return(ifaces.EntryRef{}, nil).
		Once()

	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpUpdateRecurrence,
		TargetRef:     templateID,
		TargetKind:    "recurring_template",
		ImpactNote:    "Esta recorrência será atualizada.",
		SuspendedAt:   time.Now().UTC(),
		UpdatePayload: string(payload),
		Version:       1,
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
	s.Contains(response, "Recorrência atualizada")
}

func (s *DestructiveConfirmSuite) TestOpDeleteRecurrence_Confirm_CallsDeleteRecurrence() {
	templateID := uuid.New().String()

	s.recurrences.EXPECT().
		DeleteRecurrence(mock.Anything, templateID, int64(2)).
		Return(nil).
		Once()

	state := ConfirmState{
		Awaiting:    AwaitingConfirm,
		Operation:   OpDeleteRecurrence,
		TargetRef:   templateID,
		TargetKind:  "recurring_template",
		ImpactNote:  "Esta recorrência será removida permanentemente.",
		SuspendedAt: time.Now().UTC(),
		Version:     2,
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
	s.Contains(response, "Recorrência removida")
}

func (s *DestructiveConfirmSuite) TestOpUpdateCard_Confirm_CallsUpdateCard() {
	cardID := uuid.New()
	dueDay := 10
	upd := ifaces.CardUpdate{
		DueDay: &dueDay,
	}
	payload, _ := json.Marshal(upd)

	s.cards.EXPECT().
		UpdateCard(mock.Anything, mock.AnythingOfType("interfaces.CardUpdate")).
		Return(ifaces.Card{}, nil).
		Once()

	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpUpdateCard,
		TargetRef:     cardID.String(),
		TargetKind:    "card",
		ImpactNote:    "A alteração do dia de vencimento pode impactar parcelas em aberto.",
		SuspendedAt:   time.Now().UTC(),
		UpdatePayload: string(payload),
		UserID:        uuid.New(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "sim")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
	s.Contains(response, "Cartão atualizado")
}

func (s *DestructiveConfirmSuite) TestOpUpdateRecurrence_Cancel_NoEffect() {
	templateID := uuid.New().String()
	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpUpdateRecurrence,
		TargetRef:     templateID,
		TargetKind:    "recurring_template",
		ImpactNote:    "Esta recorrência será atualizada.",
		SuspendedAt:   time.Now().UTC(),
		UpdatePayload: `{"version":1}`,
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "não")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "cancelada")
}

func (s *DestructiveConfirmSuite) TestOpConfirmRegister_IncomeDirection_UsesIncomeKind() {
	rootID := uuid.New()
	leafID := uuid.New()
	draft := ifaces.RawTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   100000,
		Description:   "salário",
		OccurredAt:    "2026-07-06",
	}
	payload, _ := json.Marshal(draft)
	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpConfirmRegister,
		TargetKind:    "transaction",
		UpdatePayload: string(payload),
		SuspendedAt:   time.Now().UTC(),
		UserID:        uuid.New(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "emprego", "income").
		Return(ifaces.CategorySearchResult{
			Outcome: ifaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []ifaces.CategoryCandidate{{
				CategoryID:     leafID,
				RootCategoryID: rootID,
				Score:          0.95,
				Confidence:     "high",
				MatchQuality:   "exact",
				SignalType:     "canonical_name",
				MatchedTerm:    "emprego",
				MatchReason:    "canonical match",
			}},
		}, nil).
		Once()
	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.MatchedBy(func(r ifaces.RawTransaction) bool {
			return r.CategoryID == rootID &&
				r.SubcategoryID != nil && *r.SubcategoryID == leafID &&
				r.CategorySource == "user_selected_candidate"
		})).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "emprego")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
}

func (s *DestructiveConfirmSuite) TestOpConfirmRegister_AmbiguousResult_Reprompts() {
	leafID := uuid.New()
	rootID := uuid.New()
	draft := ifaces.RawTransaction{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   3000,
		Description:   "farmácia",
		OccurredAt:    "2026-07-06",
	}
	payload, _ := json.Marshal(draft)
	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpConfirmRegister,
		TargetKind:    "transaction",
		UpdatePayload: string(payload),
		SuspendedAt:   time.Now().UTC(),
		UserID:        uuid.New(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "saúde", "expense").
		Return(ifaces.CategorySearchResult{
			Outcome: ifaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []ifaces.CategoryCandidate{
				{CategoryID: leafID, RootCategoryID: rootID, IsAmbiguous: true, Score: 0.6, Confidence: "low", MatchQuality: "fuzzy"},
			},
		}, nil).
		Once()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "saúde")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "categoria")
	s.NotContains(response, "✅")
}

func (s *DestructiveConfirmSuite) TestOpConfirmRegister_MultipleCandidate_Reprompts() {
	draft := ifaces.RawTransaction{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   1500,
		Description:   "mercado",
		OccurredAt:    "2026-07-06",
	}
	payload, _ := json.Marshal(draft)
	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpConfirmRegister,
		TargetKind:    "transaction",
		UpdatePayload: string(payload),
		SuspendedAt:   time.Now().UTC(),
		UserID:        uuid.New(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "alimentação", "expense").
		Return(ifaces.CategorySearchResult{
			Outcome: ifaces.ClassifyOutcomeAmbiguous,
			Version: 1,
			Candidates: []ifaces.CategoryCandidate{
				{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Score: 0.8, Confidence: "medium", MatchQuality: "token"},
				{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Score: 0.75, Confidence: "low", MatchQuality: "fuzzy"},
			},
		}, nil).
		Once()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "alimentação")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "categoria")
	s.NotContains(response, "✅")
}

func (s *DestructiveConfirmSuite) TestOpConfirmRegister_RootEqualsLeaf_Reprompts() {
	sameID := uuid.New()
	draft := ifaces.RawTransaction{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   2000,
		Description:   "compra",
		OccurredAt:    "2026-07-06",
	}
	payload, _ := json.Marshal(draft)
	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpConfirmRegister,
		TargetKind:    "transaction",
		UpdatePayload: string(payload),
		SuspendedAt:   time.Now().UTC(),
		UserID:        uuid.New(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "geral", "expense").
		Return(ifaces.CategorySearchResult{
			Outcome: ifaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []ifaces.CategoryCandidate{
				{CategoryID: sameID, RootCategoryID: sameID, Score: 0.9, Confidence: "high", MatchQuality: "exact"},
			},
		}, nil).
		Once()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "geral")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "categoria")
	s.NotContains(response, "✅")
}

func (s *DestructiveConfirmSuite) TestOpConfirmRegister_FreetextDispatchesNewResolution() {
	rootID := uuid.New()
	leafID := uuid.New()
	draft := ifaces.RawTransaction{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   5000,
		Description:   "supermercado",
		OccurredAt:    "2026-07-06",
	}
	payload, _ := json.Marshal(draft)
	state := ConfirmState{
		Awaiting:      AwaitingConfirm,
		Operation:     OpConfirmRegister,
		TargetKind:    "transaction",
		UpdatePayload: string(payload),
		SuspendedAt:   time.Now().UTC(),
		UserID:        uuid.New(),
	}
	_, err := s.engine.Start(s.ctx, s.def, s.key, state)
	s.Require().NoError(err)

	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "texto livre qualquer", "expense").
		Return(ifaces.CategorySearchResult{
			Outcome: ifaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []ifaces.CategoryCandidate{{
				CategoryID:     leafID,
				RootCategoryID: rootID,
				Score:          0.85,
				Confidence:     "high",
				MatchQuality:   "exact",
				SignalType:     "phrase",
				MatchedTerm:    "texto livre qualquer",
				MatchReason:    "phrase match",
			}},
		}, nil).
		Once()
	s.ledger.EXPECT().
		CreateTransaction(mock.Anything, mock.MatchedBy(func(r ifaces.RawTransaction) bool {
			return r.CategoryID == rootID && r.SubcategoryID != nil && *r.SubcategoryID == leafID
		})).
		Return(ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil).
		Once()

	handled, response, err := ContinueDestructiveConfirm(s.ctx, s.engine, s.def, s.key, "texto livre qualquer")
	s.NoError(err)
	s.True(handled)
	s.Contains(response, "✅")
}

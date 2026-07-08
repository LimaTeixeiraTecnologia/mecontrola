package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

// ─── in-memory workflow store ────────────────────────────────────────────────

type harnessStore struct {
	mu   sync.RWMutex
	data map[string]workflow.Snapshot
}

func newHarnessStore() *harnessStore {
	return &harnessStore{data: make(map[string]workflow.Snapshot)}
}

func (s *harnessStore) storeKey(wid, ck string) string { return wid + "::" + ck }

func (s *harnessStore) Insert(_ context.Context, snap workflow.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.storeKey(snap.Workflow, snap.CorrelationKey)
	if ex, ok := s.data[k]; ok {
		if ex.Status == workflow.RunStatusRunning || ex.Status == workflow.RunStatusSuspended {
			return workflow.ErrRunAlreadyExists
		}
	}
	s.data[k] = snap
	return nil
}

func (s *harnessStore) Load(_ context.Context, wid, key string) (workflow.Snapshot, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.data[s.storeKey(wid, key)]
	return snap, ok, nil
}

func (s *harnessStore) Save(_ context.Context, snap workflow.Snapshot, expected int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.storeKey(snap.Workflow, snap.CorrelationKey)
	if ex, ok := s.data[k]; ok && ex.Version != expected {
		return workflow.ErrVersionConflict
	}
	s.data[k] = snap
	return nil
}

func (s *harnessStore) AppendStep(_ context.Context, _ workflow.StepRecord) error { return nil }

func (s *harnessStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

func (s *harnessStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]workflow.Snapshot, error) {
	return nil, nil
}

// ─── fake TransactionsLedger ─────────────────────────────────────────────────

type hFakeTxLedger struct {
	mu          sync.Mutex
	calls       []ifaces.RawTransaction
	updateCalls []ifaces.RawUpdateTransaction
	recurCalls  []ifaces.RawRecurringTemplate
	forceErr    error
}

func (f *hFakeTxLedger) CreateTransaction(_ context.Context, in ifaces.RawTransaction) (ifaces.EntryRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceErr != nil {
		return ifaces.EntryRef{}, f.forceErr
	}
	f.calls = append(f.calls, in)
	return ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil
}

func (f *hFakeTxLedger) UpdateTransaction(_ context.Context, in ifaces.RawUpdateTransaction) (ifaces.EntryRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceErr != nil {
		return ifaces.EntryRef{}, f.forceErr
	}
	f.updateCalls = append(f.updateCalls, in)
	return ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindTransaction}, nil
}

func (f *hFakeTxLedger) CreateRecurringTemplate(_ context.Context, in ifaces.RawRecurringTemplate) (ifaces.EntryRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceErr != nil {
		return ifaces.EntryRef{}, f.forceErr
	}
	f.recurCalls = append(f.recurCalls, in)
	return ifaces.EntryRef{ID: uuid.New(), Kind: ifaces.EntryKindRecurringTemplate}, nil
}

func (f *hFakeTxLedger) DeleteTransaction(_ context.Context, _ ifaces.EntryRef, _ int64) error {
	return nil
}

func (f *hFakeTxLedger) ListMonthlyEntries(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]ifaces.MonthlyEntry, error) {
	return nil, nil
}

func (f *hFakeTxLedger) GetMonthlySummary(_ context.Context, _ uuid.UUID, _ string) (ifaces.MonthlySummary, error) {
	return ifaces.MonthlySummary{}, nil
}

func (f *hFakeTxLedger) GetCardInvoice(_ context.Context, _ uuid.UUID, _ string) (ifaces.CardInvoice, error) {
	return ifaces.CardInvoice{}, nil
}

func (f *hFakeTxLedger) SearchTransactions(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]ifaces.Entry, error) {
	return nil, nil
}

func (f *hFakeTxLedger) GetTransaction(_ context.Context, _ string) (ifaces.Entry, error) {
	return ifaces.Entry{}, nil
}

func (f *hFakeTxLedger) totalWrites() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls) + len(f.updateCalls) + len(f.recurCalls)
}

type hFakeIdempotentWriter struct {
	mu      sync.Mutex
	written map[string]uuid.UUID
}

func newHFakeIdempotentWriter() *hFakeIdempotentWriter {
	return &hFakeIdempotentWriter{written: make(map[string]uuid.UUID)}
}

func (f *hFakeIdempotentWriter) Execute(ctx context.Context, _ uuid.UUID, wamid string, itemSeq int, operation, _ string, write workflows.IdempotentWriteFn) (uuid.UUID, agent.ToolOutcome, error) {
	key := fmt.Sprintf("%s:%d:%s", wamid, itemSeq, operation)
	f.mu.Lock()
	if rid, ok := f.written[key]; ok {
		f.mu.Unlock()
		return rid, agent.ToolOutcomeReplay, nil
	}
	f.mu.Unlock()
	rid, _, err := write(ctx)
	if err != nil {
		return uuid.Nil, agent.ToolOutcomeMissingResolver, err
	}
	f.mu.Lock()
	f.written[key] = rid
	f.mu.Unlock()
	return rid, agent.ToolOutcomeRouted, nil
}

// ─── fake CategoriesReader ───────────────────────────────────────────────────

type hFakeCatEntry struct {
	rootID   uuid.UUID
	rootSlug string
	subID    uuid.UUID
	subSlug  string
	path     string
	rootOnly bool
}

type hFakeCatReader struct {
	entries map[string][]hFakeCatEntry
	version int64
}

func newHFakeCatReader() *hFakeCatReader {
	return &hFakeCatReader{entries: make(map[string][]hFakeCatEntry), version: 1}
}

func (f *hFakeCatReader) addLeaf(term string, rootID uuid.UUID, rootSlug string, subID uuid.UUID, subSlug, path string) {
	f.entries[term] = append(f.entries[term], hFakeCatEntry{
		rootID: rootID, rootSlug: rootSlug,
		subID: subID, subSlug: subSlug,
		path: path, rootOnly: false,
	})
}

func (f *hFakeCatReader) addRoot(term string, rootID uuid.UUID, rootSlug string) {
	f.entries[term] = append(f.entries[term], hFakeCatEntry{
		rootID: rootID, rootSlug: rootSlug,
		subID: rootID, subSlug: rootSlug,
		path: rootSlug, rootOnly: true,
	})
}

func (f *hFakeCatReader) SearchDictionary(_ context.Context, term, _ string) (ifaces.CategorySearchResult, error) {
	entries, ok := f.entries[term]
	if !ok || len(entries) == 0 {
		return ifaces.CategorySearchResult{Outcome: ifaces.ClassifyOutcomeNoMatch, Version: f.version}, nil
	}
	var candidates []ifaces.CategoryCandidate
	for _, e := range entries {
		catID := e.subID
		if e.rootOnly {
			catID = e.rootID
		}
		candidates = append(candidates, ifaces.CategoryCandidate{
			CategoryID:     catID,
			RootCategoryID: e.rootID,
			Path:           e.path,
			MatchedTerm:    term,
			SignalType:     "exact",
			Confidence:     "high",
			MatchQuality:   "exact",
			Score:          1.0,
			IsAmbiguous:    len(entries) > 1,
		})
	}
	outcome := ifaces.ClassifyOutcomeMatched
	if len(candidates) > 1 {
		outcome = ifaces.ClassifyOutcomeAmbiguous
	}
	return ifaces.CategorySearchResult{Outcome: outcome, Version: f.version, Candidates: candidates}, nil
}

func (f *hFakeCatReader) ResolveForWrite(_ context.Context, in ifaces.CategoryWriteRequest) (ifaces.CategoryWriteDecision, error) {
	for _, entries := range f.entries {
		for _, e := range entries {
			if e.rootID == in.RootCategoryID && e.subID == in.SubcategoryID && !e.rootOnly {
				return ifaces.CategoryWriteDecision{
					RootCategoryID:   e.rootID,
					SubcategoryID:    e.subID,
					Kind:             in.Kind,
					Path:             e.path,
					RootSlug:         e.rootSlug,
					SubcategorySlug:  e.subSlug,
					EditorialVersion: f.version,
				}, nil
			}
		}
	}
	return ifaces.CategoryWriteDecision{}, errors.New("fake: category not found or invalid")
}

func (f *hFakeCatReader) ListCategories(_ context.Context, _ uuid.UUID) ([]ifaces.Category, error) {
	return nil, nil
}

// ─── fake CardManager ────────────────────────────────────────────────────────

type hFakeCardMgr struct {
	cards map[string]ifaces.Card
}

func newHFakeCardMgr() *hFakeCardMgr {
	return &hFakeCardMgr{cards: make(map[string]ifaces.Card)}
}

func (f *hFakeCardMgr) addCard(nickname string, c ifaces.Card) {
	f.cards[nickname] = c
}

func (f *hFakeCardMgr) ResolveCardByNickname(_ context.Context, _ uuid.UUID, nickname string) (ifaces.Card, error) {
	if c, ok := f.cards[nickname]; ok {
		return c, nil
	}
	return ifaces.Card{}, fmt.Errorf("fake: card %q not found", nickname)
}

func (f *hFakeCardMgr) CreateCard(_ context.Context, _ ifaces.NewCard) (ifaces.CardRef, error) {
	return ifaces.CardRef{}, nil
}

func (f *hFakeCardMgr) ListCards(_ context.Context, _ uuid.UUID) ([]ifaces.Card, error) {
	return nil, nil
}

func (f *hFakeCardMgr) GetCard(_ context.Context, _, _ uuid.UUID) (ifaces.Card, error) {
	return ifaces.Card{}, nil
}

func (f *hFakeCardMgr) CountCards(_ context.Context, _ uuid.UUID) (int64, error) { return 0, nil }

func (f *hFakeCardMgr) BestPurchaseDay(_ context.Context, _ string, _ int) (ifaces.BestPurchaseDay, error) {
	return ifaces.BestPurchaseDay{}, nil
}

func (f *hFakeCardMgr) UpdateCard(_ context.Context, _ ifaces.CardUpdate) (ifaces.Card, error) {
	return ifaces.Card{}, nil
}

func (f *hFakeCardMgr) SoftDeleteCard(_ context.Context, _, _ uuid.UUID) error { return nil }

func (f *hFakeCardMgr) HasOpenInstallments(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

// ─── category UUIDs from migrations/000001 ──────────────────────────────────

var (
	hCustoFixoRootID   = uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62")
	hSupermercadoSubID = uuid.MustParse("97fa4b86-d43c-5ad5-a99b-c88c8427fb30")
	hPrazeresRootID    = uuid.MustParse("ac535261-4060-56ef-b2e8-57c8cc7032d1")
	hRestaurantesSubID = uuid.MustParse("d539672d-961f-5553-b807-0e0156a63163")
	hConsultasSubID    = uuid.MustParse("af5619e0-3683-5b8c-b9fc-0b3ddfbd2075")
	hPlanoSaudeSubID   = uuid.MustParse("c8f579ea-952b-5e24-beed-ef22fb845a4b")
	hFarmaciaSubID     = uuid.MustParse("3ca95dd5-c630-5c03-bd47-071777bde81c")
	hVendasRootID      = uuid.MustParse("8dba4d69-834f-5bdb-8c8c-9f86a9b56858")
	hMetasRootID       = uuid.MustParse("f133508e-7dc3-58a3-96db-199d8fbd2987")
	hTecnologiaSubID   = uuid.MustParse("3ff5e6b5-b958-5848-9092-73eb541598fc")
)

// ─── harness types ───────────────────────────────────────────────────────────

type harnessWrite struct {
	amountCents    int64
	paymentMethod  string
	rootCategoryID uuid.UUID
	subcategoryID  uuid.UUID
	categorySource string
}

type harnessAgentStep struct {
	expectPendingStatus      *workflows.PendingStatus
	expectAwaitingSlot       *workflows.AwaitingSlot
	expectWrite              *harnessWrite
	expectNoWrite            bool
	expectRunStatus          *workflow.RunStatus
	expectConfirmBeforeWrite bool
}

func hPtr[T any](v T) *T { return &v }

type pendingEntryHarness struct {
	t           *testing.T
	engine      workflow.Engine[workflows.PendingEntryState]
	def         workflow.Definition[workflows.PendingEntryState]
	cats        *hFakeCatReader
	ledger      *hFakeTxLedger
	key         string
	last        *workflow.RunResult[workflows.PendingEntryState]
	confirmSeen bool
}

func newPEHarness(t *testing.T, userID uuid.UUID, ledger *hFakeTxLedger, cats *hFakeCatReader, cards ifaces.CardManager) *pendingEntryHarness {
	t.Helper()
	store := newHarnessStore()
	eng := workflow.NewEngine[workflows.PendingEntryState](store, fake.NewProvider())
	def := workflows.BuildPendingEntryWorkflow(ledger, cards, cats, newHFakeIdempotentWriter())
	return &pendingEntryHarness{
		t:      t,
		engine: eng,
		def:    def,
		cats:   cats,
		ledger: ledger,
		key:    fmt.Sprintf("%s:thread-001:%s", userID, workflows.PendingEntryWorkflowID),
	}
}

func (h *pendingEntryHarness) start(state workflows.PendingEntryState) {
	h.t.Helper()
	result, err := h.engine.Start(context.Background(), h.def, h.key, state)
	require.NoError(h.t, err)
	h.last = &result
}

func (h *pendingEntryHarness) sendUser(text, messageID string) {
	h.t.Helper()

	var patch []byte
	var err error

	if h.last != nil && h.last.State.Awaiting == workflows.AwaitingSlotCategory && h.cats != nil {
		patch, err = h.buildCategoryPatch(text, messageID)
	} else {
		patch, err = json.Marshal(map[string]string{
			"resumeText":        text,
			"incomingMessageId": messageID,
		})
	}
	require.NoError(h.t, err)

	result, err := h.engine.Resume(context.Background(), h.def, h.key, patch)
	require.NoError(h.t, err)
	h.last = &result
}

func (h *pendingEntryHarness) buildCategoryPatch(text, messageID string) ([]byte, error) {
	if hIsCancelOrNewOp(text) {
		return json.Marshal(map[string]string{"resumeText": text, "incomingMessageId": messageID})
	}

	entries, ok := h.cats.entries[text]
	if !ok || len(entries) == 0 {
		return json.Marshal(map[string]string{"resumeText": text, "incomingMessageId": messageID})
	}

	var leaves []hFakeCatEntry
	for _, e := range entries {
		if !e.rootOnly && e.subID != (uuid.UUID{}) && e.subID != e.rootID {
			leaves = append(leaves, e)
		}
	}

	if len(leaves) == 1 {
		e := leaves[0]
		candidates := []workflows.PendingCategoryCandidate{{
			RootCategoryID:  e.rootID,
			RootSlug:        e.rootSlug,
			SubcategoryID:   e.subID,
			SubcategorySlug: e.subSlug,
			Path:            e.path,
			Score:           1.0,
			Confidence:      "high",
			MatchQuality:    "exact",
		}}
		return json.Marshal(map[string]interface{}{
			"awaiting":          int(workflows.AwaitingSlotConfirmation),
			"candidates":        candidates,
			"incomingMessageId": messageID,
			"suspendedAt":       time.Now().UTC(),
			"repromptCount":     0,
		})
	}

	if len(leaves) > 1 {
		var candidates []workflows.PendingCategoryCandidate
		for _, e := range leaves {
			candidates = append(candidates, workflows.PendingCategoryCandidate{
				RootCategoryID:  e.rootID,
				RootSlug:        e.rootSlug,
				SubcategoryID:   e.subID,
				SubcategorySlug: e.subSlug,
				Path:            e.path,
				Score:           1.0,
			})
		}
		return json.Marshal(map[string]interface{}{
			"candidates":        candidates,
			"incomingMessageId": messageID,
		})
	}

	return json.Marshal(map[string]string{"resumeText": text, "incomingMessageId": messageID})
}

func hIsCancelOrNewOp(text string) bool {
	cancels := []string{"cancela", "cancelar", "deixa pra lá", "não registra", "nao registra"}
	for _, c := range cancels {
		if strings.EqualFold(text, c) {
			return true
		}
	}
	lower := strings.ToLower(text)
	newOps := []string{"gastei", "paguei", "comprei", "recebi", "ganhei"}
	for _, op := range newOps {
		if strings.Contains(lower, op) && strings.Contains(lower, "r$") {
			return true
		}
	}
	return false
}

func (h *pendingEntryHarness) assertAgent(step harnessAgentStep) {
	h.t.Helper()
	require.NotNil(h.t, h.last)
	state := h.last.State

	if step.expectPendingStatus != nil {
		assert.Equal(h.t, *step.expectPendingStatus, state.Status, "pending status mismatch")
	}
	if step.expectAwaitingSlot != nil {
		assert.Equal(h.t, *step.expectAwaitingSlot, state.Awaiting, "awaiting slot mismatch")
		if *step.expectAwaitingSlot == workflows.AwaitingSlotConfirmation {
			h.confirmSeen = true
		}
	}
	if step.expectRunStatus != nil {
		assert.Equal(h.t, *step.expectRunStatus, h.last.Status, "run status mismatch")
	}
	if step.expectNoWrite {
		assert.Equal(h.t, 0, h.ledger.totalWrites(), "expected no write but ledger was called")
	}
	if step.expectWrite != nil {
		h.assertWrite(step)
	}
}

func (h *pendingEntryHarness) assertWrite(step harnessAgentStep) {
	h.t.Helper()
	if step.expectConfirmBeforeWrite {
		assert.True(h.t, h.confirmSeen, "M-07: write without prior confirmation turn")
	}
	assert.Greater(h.t, h.ledger.totalWrites(), 0, "expected write but ledger was not called")

	h.ledger.mu.Lock()
	defer h.ledger.mu.Unlock()

	if len(h.ledger.calls) > 0 {
		last := h.ledger.calls[len(h.ledger.calls)-1]
		h.assertRawTransaction(last, step.expectWrite)
		return
	}
	if len(h.ledger.updateCalls) > 0 {
		last := h.ledger.updateCalls[len(h.ledger.updateCalls)-1]
		if step.expectWrite.amountCents > 0 {
			assert.Equal(h.t, step.expectWrite.amountCents, last.AmountCents, "amountCents (update) mismatch")
		}
		return
	}
	if len(h.ledger.recurCalls) > 0 {
		last := h.ledger.recurCalls[len(h.ledger.recurCalls)-1]
		if step.expectWrite.amountCents > 0 {
			assert.Equal(h.t, step.expectWrite.amountCents, last.AmountCents, "amountCents (recur) mismatch")
		}
	}
}

func (h *pendingEntryHarness) assertRawTransaction(last ifaces.RawTransaction, expect *harnessWrite) {
	h.t.Helper()
	if expect.amountCents > 0 {
		assert.Equal(h.t, expect.amountCents, last.AmountCents, "amountCents mismatch")
	}
	if expect.paymentMethod != "" {
		assert.Equal(h.t, expect.paymentMethod, last.PaymentMethod, "paymentMethod mismatch")
	}
	if expect.rootCategoryID != (uuid.UUID{}) {
		assert.Equal(h.t, expect.rootCategoryID, last.CategoryID, "rootCategoryID mismatch")
	}
	if expect.subcategoryID != (uuid.UUID{}) && last.SubcategoryID != nil {
		assert.Equal(h.t, expect.subcategoryID, *last.SubcategoryID, "subcategoryID mismatch")
	}
	if expect.categorySource != "" {
		assert.Equal(h.t, expect.categorySource, last.CategorySource, "categorySource mismatch")
	}
}

// ─── state builders ───────────────────────────────────────────────────────────

func hNewExpenseState(userID uuid.UUID, awaiting workflows.AwaitingSlot, amountCents int64, description, paymentMethod string, candidates []workflows.PendingCategoryCandidate) workflows.PendingEntryState {
	return workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      awaiting,
		OperationKind: workflows.PendingOpRegisterExpense,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      "thread-001",
		MessageID:     "wamid-001",
		AmountCents:   amountCents,
		Description:   description,
		PaymentMethod: paymentMethod,
		Kind:          ifaces.CategoryKindExpense,
		Candidates:    candidates,
		OccurredAt:    time.Now().UTC().Format("2006-01-02"),
		SuspendedAt:   time.Now().UTC(),
	}
}

func hNewIncomeState(userID uuid.UUID, awaiting workflows.AwaitingSlot, amountCents int64, description string, candidates []workflows.PendingCategoryCandidate) workflows.PendingEntryState {
	return workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      awaiting,
		OperationKind: workflows.PendingOpRegisterIncome,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      "thread-001",
		MessageID:     "wamid-001",
		AmountCents:   amountCents,
		Description:   description,
		PaymentMethod: "pix",
		Kind:          ifaces.CategoryKindIncome,
		Candidates:    candidates,
		OccurredAt:    time.Now().UTC().Format("2006-01-02"),
		SuspendedAt:   time.Now().UTC(),
	}
}

func hSingleCandidate(rootID uuid.UUID, rootSlug string, subID uuid.UUID, subSlug, path string) []workflows.PendingCategoryCandidate {
	return []workflows.PendingCategoryCandidate{{
		RootCategoryID:  rootID,
		RootSlug:        rootSlug,
		SubcategoryID:   subID,
		SubcategorySlug: subSlug,
		Path:            path,
		Score:           1.0,
		Confidence:      "high",
	}}
}

func hInsertSuspended(t *testing.T, store *harnessStore, key string, state workflows.PendingEntryState) {
	t.Helper()
	codec := workflow.NewCodec[workflows.PendingEntryState]()
	encoded, err := codec.Encode(state)
	require.NoError(t, err)
	snap := workflow.Snapshot{
		RunID:          uuid.New(),
		Workflow:       workflows.PendingEntryWorkflowID,
		CorrelationKey: key,
		Status:         workflow.RunStatusSuspended,
		Version:        1,
		State:          encoded,
	}
	require.NoError(t, store.Insert(context.Background(), snap))
}

// ─── G7-01: Substituição de pendência por nova frase completa ────────────────

func TestG7_01_SubstituicaoPorNovaFraseCompleta(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 15000, "mercado", "pix", nil))
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotCategory),
	})

	h.sendUser("Gastei R$ 150,00 na farmácia hoje, no pix", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusReplaced),
		expectNoWrite:       true,
		expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
	})

	assert.Equal(t, 0, ledger.totalWrites(), "M-07: zero writes on substitution")
}

// ─── G7-02: Substituição — CA-11 ─────────────────────────────────────────────

func TestG7_02_SubstituicaoCA11(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 15000, "mercado", "pix", nil))

	h.sendUser("Gastei R$ 50,00 na farmácia, cartão", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusReplaced),
		expectNoWrite:       true,
		expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
	})
	assert.Equal(t, 0, ledger.totalWrites(), "zero writes: CA-11 substituição")
}

// ─── G7-03: Raiz sem folha bloqueia ──────────────────────────────────────────

func TestG7_03_RaizSemFolhaBloqueiaEscrita(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addRoot("custo fixo", hCustoFixoRootID, "custo-fixo")
	h := newPEHarness(t, userID, ledger, cats, nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 20000, "loja", "pix", nil))
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotCategory),
	})

	h.sendUser("custo fixo", "wamid-002")
	assert.Equal(t, workflows.AwaitingSlotCategory, h.last.State.Awaiting, "must remain on category slot when root-only")
	assert.Equal(t, 0, ledger.totalWrites(), "no write: root without leaf, M-04=0")
}

// ─── G7-04: Cancelamento explícito ───────────────────────────────────────────

func TestG7_04_CancelamentoExplicito(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 30000, "material", "pix", nil))
	h.sendUser("cancela", "wamid-002")

	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusCancelled),
		expectNoWrite:       true,
		expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
	})
}

// ─── G7-05: Cancelamento "deixa pra lá" ──────────────────────────────────────

func TestG7_05_CancelamentoDeixaPraLa(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 9000, "bar", "pix", nil))
	h.sendUser("deixa pra lá", "wamid-002")

	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusCancelled),
		expectNoWrite:       true,
		expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
	})
}

// ─── G7-06: Cancelamento "não registra" ──────────────────────────────────────

func TestG7_06_CancelamentoNaoRegistra(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 5000, "uber", "pix", nil))
	h.sendUser("não registra", "wamid-002")

	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusCancelled),
		expectNoWrite:       true,
		expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
	})
}

// ─── G7-07: "sim e pix" não é categoria — reprompt ───────────────────────────

func TestG7_07_SimEPixNaoECategoriaReprompt(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 15000, "mercado", "pix", nil))

	h.sendUser("sim e pix", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotCategory),
		expectNoWrite:       true,
	})
	assert.Equal(t, 1, h.last.State.RepromptCount, "reprompt count should be 1")
}

// ─── G7-08: Expiração de pendência ───────────────────────────────────────────

func TestG7_08_ExpiracaoDePendencia(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	store := newHarnessStore()
	eng := workflow.NewEngine[workflows.PendingEntryState](store, fake.NewProvider())
	def := workflows.BuildPendingEntryWorkflow(ledger, nil, nil, newHFakeIdempotentWriter())
	key := fmt.Sprintf("%s:thread-001:%s", userID, workflows.PendingEntryWorkflowID)

	state := hNewExpenseState(userID, workflows.AwaitingSlotCategory, 20000, "supermercado", "debito", nil)
	state.SuspendedAt = time.Now().UTC().Add(-31 * time.Minute)
	hInsertSuspended(t, store, key, state)

	patch, _ := json.Marshal(map[string]string{"resumeText": "supermercado"})
	result, err := eng.Resume(context.Background(), def, key, patch)
	require.NoError(t, err)

	assert.Equal(t, workflow.RunStatusSucceeded, result.Status, "G7-08: run succeeded")
	assert.Equal(t, workflows.PendingStatusExpired, result.State.Status, "G7-08: status expired")
	assert.Contains(t, result.State.ResponseText, "expirou")
	assert.Equal(t, 0, ledger.totalWrites(), "no write on expiration")
}

// ─── G7-09: Replay idempotente ────────────────────────────────────────────────

func TestG7_09_ReplayIdempotente(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("farmacia", hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
	h := newPEHarness(t, userID, ledger, cats, nil)

	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
	state := hNewExpenseState(userID, workflows.AwaitingSlotConfirmation, 8000, "farmácia", "pix", candidato)
	h.start(state)
	h.confirmSeen = true

	h.sendUser("sim", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
		expectWrite:              &harnessWrite{amountCents: 8000, paymentMethod: "pix"},
		expectConfirmBeforeWrite: true,
	})

	assert.Equal(t, 1, ledger.totalWrites(), "exactly 1 write: G7-09 idempotência")
}

// ─── G7-10: Múltiplos candidatos — escolha ───────────────────────────────────

func TestG7_10_MultiplosCandidatosEscolha(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("consultas", hCustoFixoRootID, "custo-fixo", hConsultasSubID, "consultas-e-exames", "Custo Fixo > Consultas e Exames")
	h := newPEHarness(t, userID, ledger, cats, nil)

	candidates := []workflows.PendingCategoryCandidate{
		{RootCategoryID: hCustoFixoRootID, RootSlug: "custo-fixo", SubcategoryID: hPlanoSaudeSubID, SubcategorySlug: "plano-de-saude", Path: "Custo Fixo > Plano de Saúde", Score: 0.9},
		{RootCategoryID: hCustoFixoRootID, RootSlug: "custo-fixo", SubcategoryID: hConsultasSubID, SubcategorySlug: "consultas-e-exames", Path: "Custo Fixo > Consultas e Exames", Score: 0.8},
	}
	state := hNewExpenseState(userID, workflows.AwaitingSlotCategory, 20000, "saúde", "pix", candidates)
	h.start(state)
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotCategory),
	})

	patch, _ := json.Marshal(map[string]interface{}{
		"awaiting":      int(workflows.AwaitingSlotConfirmation),
		"candidates":    hSingleCandidate(hCustoFixoRootID, "custo-fixo", hConsultasSubID, "consultas-e-exames", "Custo Fixo > Consultas e Exames"),
		"suspendedAt":   time.Now().UTC(),
		"repromptCount": 0,
	})
	result, err := h.engine.Resume(context.Background(), h.def, h.key, patch)
	require.NoError(t, err)
	h.last = &result
	h.confirmSeen = true

	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotConfirmation),
		expectNoWrite:       true,
	})

	h.sendUser("sim", "wamid-003")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
		expectWrite:              &harnessWrite{amountCents: 20000, subcategoryID: hConsultasSubID, categorySource: "user_selected_candidate"},
		expectConfirmBeforeWrite: true,
	})
}

// ─── G7-11: ResolveForWrite rejeita candidato inválido ───────────────────────

func TestG7_11_ResolveForWriteRejeitaCandidatoInvalido(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	invalidSubID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	cats.addLeaf("medicamento", hCustoFixoRootID, "custo-fixo", invalidSubID, "invalido", "Custo Fixo > Inválido")
	h := newPEHarness(t, userID, ledger, cats, nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 8500, "medicamento", "pix", nil))

	h.sendUser("medicamento", "wamid-002")
	assert.Equal(t, 0, ledger.totalWrites(), "M-04=0: ResolveForWrite rejects invalid ID")
}

// ─── G7-12: Correção de descrição ────────────────────────────────────────────

func TestG7_12_CorrecaoDescricao(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("farmácia", hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
	h := newPEHarness(t, userID, ledger, cats, nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 15000, "mercado", "pix", nil))

	h.sendUser("farmácia", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotConfirmation),
		expectNoWrite:       true,
	})
	h.confirmSeen = true

	h.sendUser("sim", "wamid-003")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
		expectWrite:              &harnessWrite{amountCents: 15000, paymentMethod: "pix"},
		expectConfirmBeforeWrite: true,
	})
	assert.Equal(t, 1, ledger.totalWrites(), "exactly one write: G7-12")
}

// ─── G7-13: Resposta curta ambígua — reprompt ─────────────────────────────────

func TestG7_13_RespostaCurtaAmbiguaReprompt(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 20000, "loja", "pix", nil))

	h.sendUser("tudo bem", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectNoWrite:       true,
	})
	assert.Equal(t, 1, h.last.State.RepromptCount, "RepromptCount=1")
}

// ─── G7-14: Resposta ambígua segunda vez — cancela ────────────────────────────

func TestG7_14_RespostaAmbiguaSegundaVezCancela(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	store := newHarnessStore()
	eng := workflow.NewEngine[workflows.PendingEntryState](store, fake.NewProvider())
	def := workflows.BuildPendingEntryWorkflow(ledger, nil, nil, newHFakeIdempotentWriter())
	key := fmt.Sprintf("%s:thread-001:%s", userID, workflows.PendingEntryWorkflowID)

	state := hNewExpenseState(userID, workflows.AwaitingSlotCategory, 20000, "loja", "pix", nil)
	state.RepromptCount = 1
	hInsertSuspended(t, store, key, state)

	patch, _ := json.Marshal(map[string]string{"resumeText": "ok sim"})
	result, err := eng.Resume(context.Background(), def, key, patch)
	require.NoError(t, err)

	assert.Equal(t, workflow.RunStatusSucceeded, result.Status)
	assert.Equal(t, workflows.PendingStatusCancelled, result.State.Status, "G7-14: segunda ambiguidade cancela")
	assert.Equal(t, 0, ledger.totalWrites())
}

// ─── G7-15: Erro de ledger — sem sucesso simulado (M-03=0) ───────────────────

func TestG7_15_ErroLedgerSemSucessoSimulado(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{forceErr: errors.New("db error 500")}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	state := hNewExpenseState(userID, workflows.AwaitingSlotConfirmation, 30000, "supermercado", "pix", candidato)
	h.start(state)
	h.confirmSeen = true

	h.sendUser("sim", "wamid-002")
	assert.NotEqual(t, workflows.PendingStatusCompleted, h.last.State.Status, "status must not be completed on ledger error")

	resp := h.last.State.ResponseText
	assert.NotContains(t, resp, "registrei", "M-03=0: no false success")
	assert.NotContains(t, resp, "anotei", "M-03=0: no false success")
	assert.NotContains(t, resp, "salvo", "M-03=0: no false success")
}

// ─── G7-16: Cartão de crédito com resolução de nickname ──────────────────────

func TestG7_16_CartaoCreditoNicknameResolvido(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cards := newHFakeCardMgr()
	nubank := ifaces.Card{ID: uuid.New().String(), UserID: userID.String(), Nickname: "Nubank"}
	cards.addCard("Nubank", nubank)
	cats := newHFakeCatReader()
	cats.addLeaf("tecnologia", hMetasRootID, "metas", hTecnologiaSubID, "tecnologia", "Metas > Tecnologia")
	h := newPEHarness(t, userID, ledger, cats, cards)

	candidato := hSingleCandidate(hMetasRootID, "metas", hTecnologiaSubID, "tecnologia", "Metas > Tecnologia")
	state := hNewExpenseState(userID, workflows.AwaitingSlotCard, 32000, "tênis", "credit_card", candidato)
	state.Installments = 1
	h.start(state)
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotCard),
	})

	h.sendUser("Nubank", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotConfirmation),
		expectNoWrite:       true,
	})
	h.confirmSeen = true

	h.sendUser("sim", "wamid-003")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectWrite:              &harnessWrite{amountCents: 32000, paymentMethod: "credit_card"},
		expectConfirmBeforeWrite: true,
	})

	h.ledger.mu.Lock()
	defer h.ledger.mu.Unlock()
	require.Greater(t, len(h.ledger.calls), 0)
	assert.NotNil(t, h.ledger.calls[0].CardID, "cardId must be set: CA-10")
}

// ─── G7-17: Cartão de crédito com parcelas ───────────────────────────────────

func TestG7_17_CartaoCreditoComParcelas(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cards := newHFakeCardMgr()
	itau := ifaces.Card{ID: uuid.New().String(), UserID: userID.String(), Nickname: "Itaú"}
	cards.addCard("Itaú", itau)
	cats := newHFakeCatReader()
	cats.addLeaf("tecnologia", hMetasRootID, "metas", hTecnologiaSubID, "tecnologia", "Metas > Tecnologia")
	h := newPEHarness(t, userID, ledger, cats, cards)

	candidato := hSingleCandidate(hMetasRootID, "metas", hTecnologiaSubID, "tecnologia", "Metas > Tecnologia")
	state := hNewExpenseState(userID, workflows.AwaitingSlotCard, 320000, "geladeira", "credit_card", candidato)
	state.Installments = 10
	h.start(state)
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotCard),
	})

	h.sendUser("Itaú", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectAwaitingSlot: hPtr(workflows.AwaitingSlotConfirmation),
		expectNoWrite:      true,
	})
	h.confirmSeen = true

	h.sendUser("sim", "wamid-003")
	h.ledger.mu.Lock()
	defer h.ledger.mu.Unlock()
	assert.Equal(t, 1, len(h.ledger.calls))
	assert.Equal(t, 10, h.ledger.calls[0].Installments, "installments=10: G7-17")
	assert.NotNil(t, h.ledger.calls[0].CardID)
}

// ─── G7-18: Pendência de pagamento ───────────────────────────────────────────

func TestG7_18_PendenciaPaymentMethod(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("restaurantes", hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
	h := newPEHarness(t, userID, ledger, cats, nil)

	candidato := hSingleCandidate(hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
	state := hNewExpenseState(userID, workflows.AwaitingSlotPaymentMethod, 18000, "restaurante", "", candidato)
	h.start(state)
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotPaymentMethod),
	})

	h.sendUser("pix", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotConfirmation),
		expectNoWrite:       true,
	})
	h.confirmSeen = true

	h.sendUser("sim", "wamid-003")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectWrite:              &harnessWrite{amountCents: 18000},
		expectConfirmBeforeWrite: true,
	})
}

// ─── G7-19: Pendência de data ─────────────────────────────────────────────────

func TestG7_19_PendenciaData(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("restaurantes", hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
	h := newPEHarness(t, userID, ledger, cats, nil)

	candidato := hSingleCandidate(hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
	state := hNewExpenseState(userID, workflows.AwaitingSlotDate, 16000, "academia", "pix", candidato)
	h.start(state)
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotDate),
	})

	h.sendUser("ontem", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotConfirmation),
		expectNoWrite:       true,
	})
	h.confirmSeen = true

	h.sendUser("sim", "wamid-003")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectWrite:              &harnessWrite{amountCents: 16000},
		expectConfirmBeforeWrite: true,
	})
}

// ─── G7-20: Fluxo completo confirmado (CA-01..CA-05) ────────────────────────

func TestG7_20_FluxoCompletoConfirmado(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	h := newPEHarness(t, userID, ledger, cats, nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 15000, "mercado", "pix", nil))
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotCategory),
		expectNoWrite:       true,
	})

	h.sendUser("supermercado", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotConfirmation),
		expectNoWrite:       true,
	})
	h.confirmSeen = true

	h.sendUser("sim", "wamid-003")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
		expectWrite:              &harnessWrite{amountCents: 15000, paymentMethod: "pix", rootCategoryID: hCustoFixoRootID, subcategoryID: hSupermercadoSubID, categorySource: "user_selected_candidate"},
		expectConfirmBeforeWrite: true,
	})
	assert.Equal(t, 1, ledger.totalWrites(), "M-01=100%, M-02=100%, M-03=0, M-07=0")
}

// ─── G10-01: Raiz sem folha — income ─────────────────────────────────────────

func TestG10_01_RaizSemFolhaIncome(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addRoot("vendas", hVendasRootID, "vendas")
	h := newPEHarness(t, userID, ledger, cats, nil)

	h.start(hNewIncomeState(userID, workflows.AwaitingSlotCategory, 50000, "vendas", nil))

	h.sendUser("vendas", "wamid-002")
	assert.Equal(t, workflows.AwaitingSlotCategory, h.last.State.Awaiting, "G10-01: root-only income stays on category slot")
	assert.Equal(t, 0, ledger.totalWrites(), "M-04=0: no write for root-only income")
}

// ─── G10-02: ID inválido — ResolveForWrite rejeita ───────────────────────────

func TestG10_02_IDInvalidoResolveForWriteRejeita(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	invalidID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	candidatesWithInvalid := hSingleCandidate(hCustoFixoRootID, "custo-fixo", invalidID, "invalido", "Custo Fixo > Inválido")
	state := hNewExpenseState(userID, workflows.AwaitingSlotConfirmation, 10000, "mercado", "pix", candidatesWithInvalid)
	h.start(state)
	h.confirmSeen = true

	h.sendUser("sim", "wamid-002")
	assert.Equal(t, 0, ledger.totalWrites(), "G10-02: invalid sub ID → no write")
}

// ─── G10-03: Sucesso simulado proibido (M-03=0) ───────────────────────────────

func TestG10_03_SucessoSimuladoProibido(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{forceErr: errors.New("db unavailable")}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	state := hNewExpenseState(userID, workflows.AwaitingSlotConfirmation, 10000, "mercado", "pix", candidato)
	h.start(state)
	h.confirmSeen = true

	h.sendUser("sim", "wamid-002")
	assert.NotEqual(t, workflows.PendingStatusCompleted, h.last.State.Status, "status must not be completed on ledger error")

	resp := h.last.State.ResponseText
	assert.NotContains(t, resp, "registrei", "M-03=0: no false 'registrei'")
	assert.NotContains(t, resp, "anotei", "M-03=0: no false 'anotei'")
	assert.NotContains(t, resp, "salvo", "M-03=0: no false 'salvo'")
}

// ─── G10-04: Dados preservados após resposta curta (M-02=100%) ───────────────

func TestG10_04_DadosPreservadosAposRespostaCurta(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	h := newPEHarness(t, userID, ledger, cats, nil)

	h.start(hNewExpenseState(userID, workflows.AwaitingSlotCategory, 34000, "supermercado", "debit", nil))

	h.sendUser("supermercado", "wamid-002")
	assert.Equal(t, int64(34000), h.last.State.AmountCents, "M-02=100%: amountCents preserved")
	assert.Equal(t, "debit", h.last.State.PaymentMethod, "M-02=100%: paymentMethod preserved")
}

// ─── G12-01: Caminho inequívoco exige confirmação (CA-13) ────────────────────

func TestG12_01_CaminhoInequivocoClarificacaoObrigatoria(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	h := newPEHarness(t, userID, ledger, cats, nil)

	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	state := hNewExpenseState(userID, workflows.AwaitingSlotConfirmation, 32000, "supermercado", "pix", candidato)
	h.start(state)

	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotConfirmation),
		expectNoWrite:       true,
	})
	h.confirmSeen = true

	h.sendUser("sim", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
		expectWrite:              &harnessWrite{amountCents: 32000, paymentMethod: "pix", rootCategoryID: hCustoFixoRootID, subcategoryID: hSupermercadoSubID, categorySource: "user_selected_candidate"},
		expectConfirmBeforeWrite: true,
	})
	assert.Equal(t, 1, ledger.totalWrites(), "CA-13: write only after confirmation")
}

// ─── G12-02: Recusa no turno de confirmação (CA-05) ──────────────────────────

func TestG12_02_RecusaNoGateDeConfirmacao(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	state := hNewExpenseState(userID, workflows.AwaitingSlotConfirmation, 32000, "supermercado", "pix", candidato)
	h.start(state)

	h.sendUser("não", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusCancelled),
		expectNoWrite:       true,
		expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
	})
}

// ─── G12-03: Confirmação ambígua → reprompt único → cancela (CA-14) ──────────

func TestG12_03_ConfirmacaoAmbiguaRepromptUnicoCancela(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	h := newPEHarness(t, userID, ledger, newHFakeCatReader(), nil)

	candidato := hSingleCandidate(hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
	state := hNewExpenseState(userID, workflows.AwaitingSlotConfirmation, 18000, "restaurante", "pix", candidato)
	h.start(state)

	h.sendUser("talvez", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusActive),
		expectAwaitingSlot:  hPtr(workflows.AwaitingSlotConfirmation),
		expectNoWrite:       true,
	})
	assert.Equal(t, 1, h.last.State.ConfirmRepromptCount, "CA-14: ConfirmRepromptCount=1 after 1st ambiguity")

	h.sendUser("sei lá", "wamid-003")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(workflows.PendingStatusCancelled),
		expectNoWrite:       true,
		expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
	})
}

// ─── G12-04: Múltiplos candidatos — escolha por número e por nome (CA-15) ────

func TestG12_04_MultiplosCandidatosEscolhaPorNumeroENome(t *testing.T) {
	candidates := []workflows.PendingCategoryCandidate{
		{RootCategoryID: hCustoFixoRootID, RootSlug: "custo-fixo", SubcategoryID: hPlanoSaudeSubID, SubcategorySlug: "plano-de-saude", Path: "Custo Fixo > Plano de Saúde", Score: 0.9},
		{RootCategoryID: hCustoFixoRootID, RootSlug: "custo-fixo", SubcategoryID: hConsultasSubID, SubcategorySlug: "consultas-e-exames", Path: "Custo Fixo > Consultas e Exames", Score: 0.8},
	}
	state := hNewExpenseState(uuid.New(), workflows.AwaitingSlotCategory, 20000, "saúde", "pix", candidates)

	byIdx, errIdx := workflows.DecideCategoryChoice(state, candidates, "2")
	require.NoError(t, errIdx)
	assert.Equal(t, workflows.CategoryChoiceActionSelected, byIdx.Action)
	assert.Equal(t, hConsultasSubID, byIdx.Candidate.SubcategoryID, "CA-15: index 2 resolves consultas-e-exames")

	byName, errName := workflows.DecideCategoryChoice(state, candidates, "consultas-e-exames")
	require.NoError(t, errName)
	assert.Equal(t, workflows.CategoryChoiceActionSelected, byName.Action)
	assert.Equal(t, hConsultasSubID, byName.Candidate.SubcategoryID, "CA-15: name also resolves")
}

// ─── G12-05: Edição com confirmação e TargetVersion (CA-17) ──────────────────

func TestG12_05_EdicaoComConfirmacaoCA17(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	h := newPEHarness(t, userID, ledger, cats, nil)

	targetID := uuid.New()
	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	state := workflows.PendingEntryState{
		Status:              workflows.PendingStatusActive,
		Awaiting:            workflows.AwaitingSlotConfirmation,
		OperationKind:       workflows.PendingOpEditEntry,
		UserID:              userID,
		ResourceID:          userID,
		ThreadID:            "thread-001",
		MessageID:           "wamid-001",
		AmountCents:         17500,
		Description:         "supermercado",
		PaymentMethod:       "pix",
		Kind:                ifaces.CategoryKindExpense,
		Candidates:          candidato,
		OccurredAt:          time.Now().UTC().Format("2006-01-02"),
		SuspendedAt:         time.Now().UTC(),
		TargetTransactionID: &targetID,
		TargetVersion:       1,
	}
	h.start(state)
	h.confirmSeen = true

	h.sendUser("sim", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
		expectConfirmBeforeWrite: true,
	})

	h.ledger.mu.Lock()
	defer h.ledger.mu.Unlock()
	assert.Equal(t, 0, len(h.ledger.calls), "no CreateTransaction on edit")
	assert.Equal(t, 1, len(h.ledger.updateCalls), "UpdateTransaction called: CA-17")
	assert.Equal(t, targetID, h.ledger.updateCalls[0].ID, "target transaction ID preserved")
	assert.Equal(t, int64(1), h.ledger.updateCalls[0].Version, "TargetVersion=1 respected")
	assert.Equal(t, int64(17500), h.ledger.updateCalls[0].AmountCents)
}

// ─── G12-06: Recorrência via CreateRecurringTemplate (CA-16) ─────────────────

func TestG12_06_RecorrenciaViaCreateRecurringTemplate(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	aluguelSubID := uuid.MustParse("c2fda6a3-0000-0000-0000-000000000003")
	cats := newHFakeCatReader()
	cats.addLeaf("aluguel", hCustoFixoRootID, "custo-fixo", aluguelSubID, "aluguel", "Custo Fixo > Aluguel")
	h := newPEHarness(t, userID, ledger, cats, nil)

	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", aluguelSubID, "aluguel", "Custo Fixo > Aluguel")
	state := workflows.PendingEntryState{
		Status:               workflows.PendingStatusActive,
		Awaiting:             workflows.AwaitingSlotConfirmation,
		OperationKind:        workflows.PendingOpCreateRecurrence,
		UserID:               userID,
		ResourceID:           userID,
		ThreadID:             "thread-001",
		MessageID:            "wamid-001",
		AmountCents:          180000,
		Description:          "aluguel",
		PaymentMethod:        "boleto",
		Kind:                 ifaces.CategoryKindExpense,
		Candidates:           candidato,
		Frequency:            "monthly",
		RecurrenceDayOfMonth: 1,
		OccurredAt:           time.Now().UTC().Format("2006-01-02"),
		SuspendedAt:          time.Now().UTC(),
	}
	h.start(state)
	h.confirmSeen = true

	h.sendUser("sim", "wamid-002")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
		expectConfirmBeforeWrite: true,
	})

	h.ledger.mu.Lock()
	defer h.ledger.mu.Unlock()
	assert.Equal(t, 0, len(h.ledger.calls), "no CreateTransaction on recurrence")
	assert.Equal(t, 1, len(h.ledger.recurCalls), "CreateRecurringTemplate called: CA-16")
	assert.Equal(t, "monthly", h.ledger.recurCalls[0].Frequency)
	assert.Equal(t, int64(180000), h.ledger.recurCalls[0].AmountCents)
	assert.Equal(t, "boleto", h.ledger.recurCalls[0].PaymentMethod)
}

func TestIdempotentWriter_ReplayNaoFaz2oInsert(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("farmacia", hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
	idem := newHFakeIdempotentWriter()
	store := newHarnessStore()
	eng := workflow.NewEngine[workflows.PendingEntryState](store, fake.NewProvider())
	def := workflows.BuildPendingEntryWorkflow(ledger, nil, cats, idem)
	key := fmt.Sprintf("%s:thread-001:%s", userID, workflows.PendingEntryWorkflowID)

	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotConfirmation,
		UserID:        userID,
		ThreadID:      "thread-001",
		MessageID:     "wamid-original",
		AmountCents:   5000,
		Description:   "farmácia",
		PaymentMethod: "pix",
		Kind:          ifaces.CategoryKindExpense,
		Candidates:    candidato,
		SuspendedAt:   time.Now().UTC(),
		OperationKind: workflows.PendingOpRegisterExpense,
	}

	result1, err := eng.Start(context.Background(), def, key, state)
	require.NoError(t, err)
	require.Equal(t, workflow.RunStatusSuspended, result1.Status)

	patch1, _ := json.Marshal(map[string]string{"resumeText": "sim", "incomingMessageId": "wamid-002"})
	result2, err := eng.Resume(context.Background(), def, key, patch1)
	require.NoError(t, err)
	assert.Equal(t, workflow.RunStatusSucceeded, result2.Status)
	assert.Equal(t, workflows.PendingStatusCompleted, result2.State.Status)
	assert.Equal(t, 1, ledger.totalWrites(), "primeira escrita: 1 insert")

	idem2 := idem
	store2 := newHarnessStore()
	eng2 := workflow.NewEngine[workflows.PendingEntryState](store2, fake.NewProvider())
	def2 := workflows.BuildPendingEntryWorkflow(ledger, nil, cats, idem2)
	key2 := fmt.Sprintf("%s:thread-002:%s", userID, workflows.PendingEntryWorkflowID)

	state2 := state
	state2.ThreadID = "thread-002"
	result3, err := eng2.Start(context.Background(), def2, key2, state2)
	require.NoError(t, err)
	require.Equal(t, workflow.RunStatusSuspended, result3.Status)

	patch2, _ := json.Marshal(map[string]string{"resumeText": "sim", "incomingMessageId": "wamid-003"})
	result4, err := eng2.Resume(context.Background(), def2, key2, patch2)
	require.NoError(t, err)
	assert.Equal(t, workflow.RunStatusSucceeded, result4.Status)
	assert.Equal(t, workflows.PendingStatusCompleted, result4.State.Status)
	assert.Equal(t, 1, ledger.totalWrites(), "replay: sem segundo INSERT")
	assert.NotEmpty(t, result4.State.ResponseText, "replay retorna texto de sucesso")
}

func TestHarness_OccurredAt_DiaDaSemana(t *testing.T) {
	userID := uuid.New()
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	h := newPEHarness(t, userID, ledger, cats, nil)

	tuesdayDate := "2026-07-07"
	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotConfirmation,
		OperationKind: workflows.PendingOpRegisterExpense,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      "thread-001",
		MessageID:     "wamid-terca",
		AmountCents:   8000,
		Description:   "mercado na terça",
		PaymentMethod: "pix",
		Kind:          ifaces.CategoryKindExpense,
		Candidates:    candidato,
		OccurredAt:    tuesdayDate,
		SuspendedAt:   time.Now().UTC(),
	}
	h.start(state)
	h.confirmSeen = true

	h.sendUser("sim", "wamid-terca-conf")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(workflows.PendingStatusCompleted),
		expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
		expectWrite:              &harnessWrite{amountCents: 8000, paymentMethod: "pix"},
		expectConfirmBeforeWrite: true,
	})

	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	require.Len(t, ledger.calls, 1, "8.2: exatamente 1 write para data de dia da semana")
	assert.Equal(t, tuesdayDate, ledger.calls[0].OccurredAt, "8.2: OccurredAt derivado de terça preservado na escrita")
}

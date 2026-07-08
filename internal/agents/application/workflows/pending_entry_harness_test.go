package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	ifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

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

func (f *hFakeIdempotentWriter) Execute(ctx context.Context, _ uuid.UUID, wamid string, itemSeq int, operation, _ string, write IdempotentWriteFn) (uuid.UUID, agent.ToolOutcome, error) {
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

var (
	hCustoFixoRootID     = uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62")
	hSupermercadoSubID   = uuid.MustParse("97fa4b86-d43c-5ad5-a99b-c88c8427fb30")
	hPrazeresRootID      = uuid.MustParse("ac535261-4060-56ef-b2e8-57c8cc7032d1")
	hRestaurantesSubID   = uuid.MustParse("d539672d-961f-5553-b807-0e0156a63163")
	hConsultasSubID      = uuid.MustParse("af5619e0-3683-5b8c-b9fc-0b3ddfbd2075")
	hPlanoSaudeSubID     = uuid.MustParse("c8f579ea-952b-5e24-beed-ef22fb845a4b")
	hFarmaciaSubID       = uuid.MustParse("3ca95dd5-c630-5c03-bd47-071777bde81c")
	hVendasRootID        = uuid.MustParse("8dba4d69-834f-5bdb-8c8c-9f86a9b56858")
	hMetasRootID         = uuid.MustParse("f133508e-7dc3-58a3-96db-199d8fbd2987")
	hTecnologiaSubID     = uuid.MustParse("3ff5e6b5-b958-5848-9092-73eb541598fc")
	hLibFinanceiraRootID = uuid.MustParse("35ced21e-b436-5cea-afb9-ffd43f98a124")
)

type harnessWrite struct {
	amountCents    int64
	paymentMethod  string
	rootCategoryID uuid.UUID
	subcategoryID  uuid.UUID
	categorySource string
}

type harnessAgentStep struct {
	expectPendingStatus      *PendingStatus
	expectAwaitingSlot       *AwaitingSlot
	expectWrite              *harnessWrite
	expectNoWrite            bool
	expectRunStatus          *workflow.RunStatus
	expectConfirmBeforeWrite bool
}

func hPtr[T any](v T) *T { return &v }

type pendingEntryHarness struct {
	t           *testing.T
	engine      workflow.Engine[PendingEntryState]
	def         workflow.Definition[PendingEntryState]
	store       *harnessStore
	cats        *hFakeCatReader
	ledger      *hFakeTxLedger
	key         string
	last        *workflow.RunResult[PendingEntryState]
	confirmSeen bool
}

func newPEHarness(t *testing.T, obs observability.Observability, userID uuid.UUID, ledger *hFakeTxLedger, cats *hFakeCatReader, cards ifaces.CardManager) *pendingEntryHarness {
	t.Helper()
	store := newHarnessStore()
	eng := workflow.NewEngine[PendingEntryState](store, obs)
	def := BuildPendingEntryWorkflow(ledger, cards, cats, newHFakeIdempotentWriter())
	return &pendingEntryHarness{
		t:      t,
		engine: eng,
		def:    def,
		store:  store,
		cats:   cats,
		ledger: ledger,
		key:    fmt.Sprintf("%s:thread-001:%s", userID, PendingEntryWorkflowID),
	}
}

func (h *pendingEntryHarness) start(state PendingEntryState) {
	h.t.Helper()
	result, err := h.engine.Start(context.Background(), h.def, h.key, state)
	if err != nil {
		h.t.Fatalf("start: %v", err)
	}
	h.last = &result
}

func (h *pendingEntryHarness) sendUser(text, messageID string) {
	h.t.Helper()

	var patch []byte
	var err error

	if h.last != nil && h.last.State.Awaiting == AwaitingSlotCategory && h.cats != nil {
		patch, err = h.buildCategoryPatch(text, messageID)
	} else {
		patch, err = json.Marshal(map[string]string{
			"resumeText":        text,
			"incomingMessageId": messageID,
		})
	}
	if err != nil {
		h.t.Fatalf("marshal patch: %v", err)
	}

	result, err := h.engine.Resume(context.Background(), h.def, h.key, patch)
	if err != nil {
		h.t.Fatalf("resume: %v", err)
	}
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
		candidates := []PendingCategoryCandidate{{
			RootCategoryID:  e.rootID,
			RootSlug:        e.rootSlug,
			SubcategoryID:   e.subID,
			SubcategorySlug: e.subSlug,
			Path:            e.path,
			Score:           1.0,
			Confidence:      "high",
			MatchQuality:    "exact",
		}}
		return json.Marshal(map[string]any{
			"awaiting":          int(AwaitingSlotConfirmation),
			"candidates":        candidates,
			"incomingMessageId": messageID,
			"suspendedAt":       time.Now().UTC(),
			"repromptCount":     0,
		})
	}

	if len(leaves) > 1 {
		var candidates []PendingCategoryCandidate
		for _, e := range leaves {
			candidates = append(candidates, PendingCategoryCandidate{
				RootCategoryID:  e.rootID,
				RootSlug:        e.rootSlug,
				SubcategoryID:   e.subID,
				SubcategorySlug: e.subSlug,
				Path:            e.path,
				Score:           1.0,
			})
		}
		return json.Marshal(map[string]any{
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
	if h.last == nil {
		h.t.Fatal("no last result")
	}
	state := h.last.State

	if step.expectPendingStatus != nil && *step.expectPendingStatus != state.Status {
		h.t.Errorf("pending status mismatch: want %v got %v", *step.expectPendingStatus, state.Status)
	}
	if step.expectAwaitingSlot != nil {
		if *step.expectAwaitingSlot != state.Awaiting {
			h.t.Errorf("awaiting slot mismatch: want %v got %v", *step.expectAwaitingSlot, state.Awaiting)
		}
		if *step.expectAwaitingSlot == AwaitingSlotConfirmation {
			h.confirmSeen = true
		}
	}
	if step.expectRunStatus != nil && *step.expectRunStatus != h.last.Status {
		h.t.Errorf("run status mismatch: want %v got %v", *step.expectRunStatus, h.last.Status)
	}
	if step.expectNoWrite && h.ledger.totalWrites() != 0 {
		h.t.Errorf("expected no write but ledger was called")
	}
	if step.expectWrite != nil {
		h.assertWrite(step)
	}
}

func (h *pendingEntryHarness) assertWrite(step harnessAgentStep) {
	h.t.Helper()
	if step.expectConfirmBeforeWrite && !h.confirmSeen {
		h.t.Errorf("M-07: write without prior confirmation turn")
	}
	if h.ledger.totalWrites() == 0 {
		h.t.Errorf("expected write but ledger was not called")
	}

	h.ledger.mu.Lock()
	defer h.ledger.mu.Unlock()

	if len(h.ledger.calls) > 0 {
		last := h.ledger.calls[len(h.ledger.calls)-1]
		h.assertRawTransaction(last, step.expectWrite)
		return
	}
	if len(h.ledger.updateCalls) > 0 {
		last := h.ledger.updateCalls[len(h.ledger.updateCalls)-1]
		if step.expectWrite.amountCents > 0 && step.expectWrite.amountCents != last.AmountCents {
			h.t.Errorf("amountCents (update) mismatch: want %d got %d", step.expectWrite.amountCents, last.AmountCents)
		}
		return
	}
	if len(h.ledger.recurCalls) > 0 {
		last := h.ledger.recurCalls[len(h.ledger.recurCalls)-1]
		if step.expectWrite.amountCents > 0 && step.expectWrite.amountCents != last.AmountCents {
			h.t.Errorf("amountCents (recur) mismatch: want %d got %d", step.expectWrite.amountCents, last.AmountCents)
		}
	}
}

func (h *pendingEntryHarness) assertRawTransaction(last ifaces.RawTransaction, expect *harnessWrite) {
	h.t.Helper()
	if expect.amountCents > 0 && expect.amountCents != last.AmountCents {
		h.t.Errorf("amountCents mismatch: want %d got %d", expect.amountCents, last.AmountCents)
	}
	if expect.paymentMethod != "" && expect.paymentMethod != last.PaymentMethod {
		h.t.Errorf("paymentMethod mismatch: want %q got %q", expect.paymentMethod, last.PaymentMethod)
	}
	if expect.rootCategoryID != (uuid.UUID{}) && expect.rootCategoryID != last.CategoryID {
		h.t.Errorf("rootCategoryID mismatch")
	}
	if expect.subcategoryID != (uuid.UUID{}) && last.SubcategoryID != nil && expect.subcategoryID != *last.SubcategoryID {
		h.t.Errorf("subcategoryID mismatch")
	}
	if expect.categorySource != "" && expect.categorySource != last.CategorySource {
		h.t.Errorf("categorySource mismatch: want %q got %q", expect.categorySource, last.CategorySource)
	}
}

func hNewExpenseState(userID uuid.UUID, awaiting AwaitingSlot, amountCents int64, description, paymentMethod string, candidates []PendingCategoryCandidate) PendingEntryState {
	return PendingEntryState{
		Status:        PendingStatusActive,
		Awaiting:      awaiting,
		OperationKind: PendingOpRegisterExpense,
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

func hNewIncomeState(userID uuid.UUID, awaiting AwaitingSlot, amountCents int64, description string, candidates []PendingCategoryCandidate) PendingEntryState {
	return PendingEntryState{
		Status:        PendingStatusActive,
		Awaiting:      awaiting,
		OperationKind: PendingOpRegisterIncome,
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

func hSingleCandidate(rootID uuid.UUID, rootSlug string, subID uuid.UUID, subSlug, path string) []PendingCategoryCandidate {
	return []PendingCategoryCandidate{{
		RootCategoryID:  rootID,
		RootSlug:        rootSlug,
		SubcategoryID:   subID,
		SubcategorySlug: subSlug,
		Path:            path,
		Score:           1.0,
		Confidence:      "high",
	}}
}

func hInsertSuspended(t *testing.T, store *harnessStore, key string, state PendingEntryState) {
	t.Helper()
	codec := workflow.NewCodec[PendingEntryState]()
	encoded, err := codec.Encode(state)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	snap := workflow.Snapshot{
		RunID:          uuid.New(),
		Workflow:       PendingEntryWorkflowID,
		CorrelationKey: key,
		Status:         workflow.RunStatusSuspended,
		Version:        1,
		State:          encoded,
	}
	if err := store.Insert(context.Background(), snap); err != nil {
		t.Fatalf("insert suspended: %v", err)
	}
}

type PendingEntryHarnessSuite struct {
	suite.Suite
	ctx    context.Context
	obs    observability.Observability
	userID uuid.UUID
}

func TestPendingEntryHarnessSuite(t *testing.T) {
	suite.Run(t, new(PendingEntryHarnessSuite))
}

func (s *PendingEntryHarnessSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.userID = uuid.New()
}

func (s *PendingEntryHarnessSuite) TestPendingEntryScenarios() {
	type dependencies struct {
		ledger *hFakeTxLedger
		cats   *hFakeCatReader
		cards  ifaces.CardManager
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		exec         func(h *pendingEntryHarness, ledger *hFakeTxLedger)
	}{
		{
			name:         "G7-01 substituicao por nova frase completa",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 15000, "mercado", "pix", nil))
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotCategory),
				})
				h.sendUser("Gastei R$ 150,00 na farmácia hoje, no pix", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusReplaced),
					expectNoWrite:       true,
					expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
				})
				s.Equal(0, ledger.totalWrites(), "M-07: zero writes on substitution")
			},
		},
		{
			name:         "G7-02 substituicao CA-11",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 15000, "mercado", "pix", nil))
				h.sendUser("Gastei R$ 50,00 na farmácia, cartão", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusReplaced),
					expectNoWrite:       true,
					expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
				})
				s.Equal(0, ledger.totalWrites(), "zero writes: CA-11 substituição")
			},
		},
		{
			name: "G7-03 raiz sem folha bloqueia escrita",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addRoot("custo fixo", hCustoFixoRootID, "custo-fixo")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 20000, "loja", "pix", nil))
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotCategory),
				})
				h.sendUser("custo fixo", "wamid-002")
				s.Equal(AwaitingSlotCategory, h.last.State.Awaiting, "must remain on category slot when root-only")
				s.Equal(0, ledger.totalWrites(), "no write: root without leaf, M-04=0")
			},
		},
		{
			name:         "G7-04 cancelamento explicito",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 30000, "material", "pix", nil))
				h.sendUser("cancela", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusCancelled),
					expectNoWrite:       true,
					expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
				})
			},
		},
		{
			name:         "G7-05 cancelamento deixa pra la",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 9000, "bar", "pix", nil))
				h.sendUser("deixa pra lá", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusCancelled),
					expectNoWrite:       true,
					expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
				})
			},
		},
		{
			name:         "G7-06 cancelamento nao registra",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 5000, "uber", "pix", nil))
				h.sendUser("não registra", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusCancelled),
					expectNoWrite:       true,
					expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
				})
			},
		},
		{
			name:         "G7-07 sim e pix nao e categoria reprompt",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 15000, "mercado", "pix", nil))
				h.sendUser("sim e pix", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotCategory),
					expectNoWrite:       true,
				})
				s.Equal(1, h.last.State.RepromptCount, "reprompt count should be 1")
			},
		},
		{
			name: "G7-09 replay idempotente",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("farmacia", hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
				state := hNewExpenseState(s.userID, AwaitingSlotConfirmation, 8000, "farmácia", "pix", candidato)
				h.start(state)
				h.confirmSeen = true
				h.sendUser("sim", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
					expectWrite:              &harnessWrite{amountCents: 8000, paymentMethod: "pix"},
					expectConfirmBeforeWrite: true,
				})
				s.Equal(1, ledger.totalWrites(), "exactly 1 write: G7-09 idempotência")
			},
		},
		{
			name: "G7-11 resolveforwrite rejeita candidato invalido",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					invalidSubID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
					cats.addLeaf("medicamento", hCustoFixoRootID, "custo-fixo", invalidSubID, "invalido", "Custo Fixo > Inválido")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 8500, "medicamento", "pix", nil))
				h.sendUser("medicamento", "wamid-002")
				s.Equal(0, ledger.totalWrites(), "M-04=0: ResolveForWrite rejects invalid ID")
			},
		},
		{
			name: "G7-12 correcao descricao",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("farmácia", hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 15000, "mercado", "pix", nil))
				h.sendUser("farmácia", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotConfirmation),
					expectNoWrite:       true,
				})
				h.confirmSeen = true
				h.sendUser("sim", "wamid-003")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
					expectWrite:              &harnessWrite{amountCents: 15000, paymentMethod: "pix"},
					expectConfirmBeforeWrite: true,
				})
				s.Equal(1, ledger.totalWrites(), "exactly one write: G7-12")
			},
		},
		{
			name:         "G7-13 resposta curta ambigua reprompt",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 20000, "loja", "pix", nil))
				h.sendUser("tudo bem", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectNoWrite:       true,
				})
				s.Equal(1, h.last.State.RepromptCount, "RepromptCount=1")
			},
		},
		{
			name: "G7-16 cartao credito nickname resolvido",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("tecnologia", hMetasRootID, "metas", hTecnologiaSubID, "tecnologia", "Metas > Tecnologia")
					return cats
				}(),
				cards: func() ifaces.CardManager {
					cards := newHFakeCardMgr()
					cards.addCard("Nubank", ifaces.Card{ID: uuid.New().String(), UserID: uuid.New().String(), Nickname: "Nubank"})
					return cards
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				candidato := hSingleCandidate(hMetasRootID, "metas", hTecnologiaSubID, "tecnologia", "Metas > Tecnologia")
				state := hNewExpenseState(s.userID, AwaitingSlotCard, 32000, "tênis", "credit_card", candidato)
				state.Installments = 1
				h.start(state)
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotCard),
				})
				h.sendUser("Nubank", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotConfirmation),
					expectNoWrite:       true,
				})
				h.confirmSeen = true
				h.sendUser("sim", "wamid-003")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectWrite:              &harnessWrite{amountCents: 32000, paymentMethod: "credit_card"},
					expectConfirmBeforeWrite: true,
				})
				h.ledger.mu.Lock()
				defer h.ledger.mu.Unlock()
				s.Greater(len(h.ledger.calls), 0)
				s.NotNil(h.ledger.calls[0].CardID, "cardId must be set: CA-10")
			},
		},
		{
			name: "G7-17 cartao credito com parcelas",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("tecnologia", hMetasRootID, "metas", hTecnologiaSubID, "tecnologia", "Metas > Tecnologia")
					return cats
				}(),
				cards: func() ifaces.CardManager {
					cards := newHFakeCardMgr()
					cards.addCard("Itaú", ifaces.Card{ID: uuid.New().String(), UserID: uuid.New().String(), Nickname: "Itaú"})
					return cards
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				candidato := hSingleCandidate(hMetasRootID, "metas", hTecnologiaSubID, "tecnologia", "Metas > Tecnologia")
				state := hNewExpenseState(s.userID, AwaitingSlotCard, 320000, "geladeira", "credit_card", candidato)
				state.Installments = 10
				h.start(state)
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotCard),
				})
				h.sendUser("Itaú", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectAwaitingSlot: hPtr(AwaitingSlotConfirmation),
					expectNoWrite:      true,
				})
				h.confirmSeen = true
				h.sendUser("sim", "wamid-003")
				h.ledger.mu.Lock()
				defer h.ledger.mu.Unlock()
				s.Equal(1, len(h.ledger.calls))
				s.Equal(10, h.ledger.calls[0].Installments, "installments=10: G7-17")
				s.NotNil(h.ledger.calls[0].CardID)
			},
		},
		{
			name: "G7-18 pendencia payment method",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("restaurantes", hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				candidato := hSingleCandidate(hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
				state := hNewExpenseState(s.userID, AwaitingSlotPaymentMethod, 18000, "restaurante", "", candidato)
				h.start(state)
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotPaymentMethod),
				})
				h.sendUser("pix", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotConfirmation),
					expectNoWrite:       true,
				})
				h.confirmSeen = true
				h.sendUser("sim", "wamid-003")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectWrite:              &harnessWrite{amountCents: 18000},
					expectConfirmBeforeWrite: true,
				})
			},
		},
		{
			name: "G7-19 pendencia data",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("restaurantes", hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				candidato := hSingleCandidate(hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
				state := hNewExpenseState(s.userID, AwaitingSlotDate, 16000, "academia", "pix", candidato)
				h.start(state)
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotDate),
				})
				h.sendUser("ontem", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotConfirmation),
					expectNoWrite:       true,
				})
				h.confirmSeen = true
				h.sendUser("sim", "wamid-003")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectWrite:              &harnessWrite{amountCents: 16000},
					expectConfirmBeforeWrite: true,
				})
			},
		},
		{
			name: "G7-20 fluxo completo confirmado",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 15000, "mercado", "pix", nil))
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotCategory),
					expectNoWrite:       true,
				})
				h.sendUser("supermercado", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotConfirmation),
					expectNoWrite:       true,
				})
				h.confirmSeen = true
				h.sendUser("sim", "wamid-003")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
					expectWrite:              &harnessWrite{amountCents: 15000, paymentMethod: "pix", rootCategoryID: hCustoFixoRootID, subcategoryID: hSupermercadoSubID, categorySource: "user_selected_candidate"},
					expectConfirmBeforeWrite: true,
				})
				s.Equal(1, ledger.totalWrites(), "M-01=100%, M-02=100%, M-03=0, M-07=0")
			},
		},
		{
			name: "G10-01 raiz sem folha income",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addRoot("vendas", hVendasRootID, "vendas")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewIncomeState(s.userID, AwaitingSlotCategory, 50000, "vendas", nil))
				h.sendUser("vendas", "wamid-002")
				s.Equal(AwaitingSlotCategory, h.last.State.Awaiting, "G10-01: root-only income stays on category slot")
				s.Equal(0, ledger.totalWrites(), "M-04=0: no write for root-only income")
			},
		},
		{
			name:         "G10-02 id invalido resolveforwrite rejeita",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				invalidID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
				candidatesWithInvalid := hSingleCandidate(hCustoFixoRootID, "custo-fixo", invalidID, "invalido", "Custo Fixo > Inválido")
				state := hNewExpenseState(s.userID, AwaitingSlotConfirmation, 10000, "mercado", "pix", candidatesWithInvalid)
				h.start(state)
				h.confirmSeen = true
				h.sendUser("sim", "wamid-002")
				s.Equal(0, ledger.totalWrites(), "G10-02: invalid sub ID → no write")
			},
		},
		{
			name:         "G10-03 sucesso simulado proibido",
			dependencies: dependencies{ledger: &hFakeTxLedger{forceErr: errors.New("db unavailable")}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
				state := hNewExpenseState(s.userID, AwaitingSlotConfirmation, 10000, "mercado", "pix", candidato)
				h.start(state)
				h.confirmSeen = true
				h.sendUser("sim", "wamid-002")
				s.NotEqual(PendingStatusCompleted, h.last.State.Status, "status must not be completed on ledger error")
				resp := h.last.State.ResponseText
				s.NotContains(resp, "registrei", "M-03=0: no false 'registrei'")
				s.NotContains(resp, "anotei", "M-03=0: no false 'anotei'")
				s.NotContains(resp, "salvo", "M-03=0: no false 'salvo'")
			},
		},
		{
			name: "G10-04 dados preservados apos resposta curta",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				h.start(hNewExpenseState(s.userID, AwaitingSlotCategory, 34000, "supermercado", "debit", nil))
				h.sendUser("supermercado", "wamid-002")
				s.Equal(int64(34000), h.last.State.AmountCents, "M-02=100%: amountCents preserved")
				s.Equal("debit", h.last.State.PaymentMethod, "M-02=100%: paymentMethod preserved")
			},
		},
		{
			name: "G12-01 caminho inequivoco clarificacao obrigatoria",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
				state := hNewExpenseState(s.userID, AwaitingSlotConfirmation, 32000, "supermercado", "pix", candidato)
				h.start(state)
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotConfirmation),
					expectNoWrite:       true,
				})
				h.confirmSeen = true
				h.sendUser("sim", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
					expectWrite:              &harnessWrite{amountCents: 32000, paymentMethod: "pix", rootCategoryID: hCustoFixoRootID, subcategoryID: hSupermercadoSubID, categorySource: "user_selected_candidate"},
					expectConfirmBeforeWrite: true,
				})
				s.Equal(1, ledger.totalWrites(), "CA-13: write only after confirmation")
			},
		},
		{
			name:         "G12-02 recusa no gate de confirmacao",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
				state := hNewExpenseState(s.userID, AwaitingSlotConfirmation, 32000, "supermercado", "pix", candidato)
				h.start(state)
				h.sendUser("não", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusCancelled),
					expectNoWrite:       true,
					expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
				})
			},
		},
		{
			name:         "G12-03 confirmacao ambigua reprompt unico cancela",
			dependencies: dependencies{ledger: &hFakeTxLedger{}, cats: newHFakeCatReader()},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				candidato := hSingleCandidate(hPrazeresRootID, "prazeres", hRestaurantesSubID, "restaurantes", "Prazeres > Restaurantes")
				state := hNewExpenseState(s.userID, AwaitingSlotConfirmation, 18000, "restaurante", "pix", candidato)
				h.start(state)
				h.sendUser("talvez", "wamid-002")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusActive),
					expectAwaitingSlot:  hPtr(AwaitingSlotConfirmation),
					expectNoWrite:       true,
				})
				s.Equal(1, h.last.State.ConfirmRepromptCount, "CA-14: ConfirmRepromptCount=1 after 1st ambiguity")
				h.sendUser("sei lá", "wamid-003")
				h.assertAgent(harnessAgentStep{
					expectPendingStatus: hPtr(PendingStatusCancelled),
					expectNoWrite:       true,
					expectRunStatus:     hPtr(workflow.RunStatusSucceeded),
				})
			},
		},
		{
			name: "G12-05 edicao com confirmacao CA-17",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				targetID := uuid.New()
				candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
				state := PendingEntryState{
					Status:              PendingStatusActive,
					Awaiting:            AwaitingSlotConfirmation,
					OperationKind:       PendingOpEditEntry,
					UserID:              s.userID,
					ResourceID:          s.userID,
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
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
					expectConfirmBeforeWrite: true,
				})
				h.ledger.mu.Lock()
				defer h.ledger.mu.Unlock()
				s.Equal(0, len(h.ledger.calls), "no CreateTransaction on edit")
				s.Equal(1, len(h.ledger.updateCalls), "UpdateTransaction called: CA-17")
				s.Equal(targetID, h.ledger.updateCalls[0].ID, "target transaction ID preserved")
				s.Equal(int64(1), h.ledger.updateCalls[0].Version, "TargetVersion=1 respected")
				s.Equal(int64(17500), h.ledger.updateCalls[0].AmountCents)
			},
		},
		{
			name: "G12-06 recorrencia via createrecurringtemplate",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					aluguelSubID := uuid.MustParse("c2fda6a3-0000-0000-0000-000000000003")
					cats.addLeaf("aluguel", hCustoFixoRootID, "custo-fixo", aluguelSubID, "aluguel", "Custo Fixo > Aluguel")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				aluguelSubID := uuid.MustParse("c2fda6a3-0000-0000-0000-000000000003")
				candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", aluguelSubID, "aluguel", "Custo Fixo > Aluguel")
				state := PendingEntryState{
					Status:               PendingStatusActive,
					Awaiting:             AwaitingSlotConfirmation,
					OperationKind:        PendingOpCreateRecurrence,
					UserID:               s.userID,
					ResourceID:           s.userID,
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
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
					expectConfirmBeforeWrite: true,
				})
				h.ledger.mu.Lock()
				defer h.ledger.mu.Unlock()
				s.Equal(0, len(h.ledger.calls), "no CreateTransaction on recurrence")
				s.Equal(1, len(h.ledger.recurCalls), "CreateRecurringTemplate called: CA-16")
				s.Equal("monthly", h.ledger.recurCalls[0].Frequency)
				s.Equal(int64(180000), h.ledger.recurCalls[0].AmountCents)
				s.Equal("boleto", h.ledger.recurCalls[0].PaymentMethod)
			},
		},
		{
			name: "harness occurredAt dia da semana",
			dependencies: dependencies{
				ledger: &hFakeTxLedger{},
				cats: func() *hFakeCatReader {
					cats := newHFakeCatReader()
					cats.addLeaf("supermercado", hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
					return cats
				}(),
			},
			exec: func(h *pendingEntryHarness, ledger *hFakeTxLedger) {
				tuesdayDate := "2026-07-07"
				candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
				state := PendingEntryState{
					Status:        PendingStatusActive,
					Awaiting:      AwaitingSlotConfirmation,
					OperationKind: PendingOpRegisterExpense,
					UserID:        s.userID,
					ResourceID:    s.userID,
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
					expectPendingStatus:      hPtr(PendingStatusCompleted),
					expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
					expectWrite:              &harnessWrite{amountCents: 8000, paymentMethod: "pix"},
					expectConfirmBeforeWrite: true,
				})
				ledger.mu.Lock()
				defer ledger.mu.Unlock()
				s.Len(ledger.calls, 1, "8.2: exatamente 1 write para data de dia da semana")
				s.Equal(tuesdayDate, ledger.calls[0].OccurredAt, "8.2: OccurredAt derivado de terça preservado na escrita")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			h := newPEHarness(s.T(), s.obs, s.userID, scenario.dependencies.ledger, scenario.dependencies.cats, scenario.dependencies.cards)
			scenario.exec(h, scenario.dependencies.ledger)
		})
	}
}

func (s *PendingEntryHarnessSuite) TestCategoryPairs() {
	s.Equal(uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62"), hCustoFixoRootID)
	s.Equal(uuid.MustParse("97fa4b86-d43c-5ad5-a99b-c88c8427fb30"), hSupermercadoSubID)
	s.Equal(uuid.MustParse("ac535261-4060-56ef-b2e8-57c8cc7032d1"), hPrazeresRootID)
	s.Equal(uuid.MustParse("d539672d-961f-5553-b807-0e0156a63163"), hRestaurantesSubID)
	s.Equal(uuid.MustParse("8dba4d69-834f-5bdb-8c8c-9f86a9b56858"), hVendasRootID)
	s.Equal(uuid.MustParse("f133508e-7dc3-58a3-96db-199d8fbd2987"), hMetasRootID)
	s.Equal(uuid.MustParse("3ff5e6b5-b958-5848-9092-73eb541598fc"), hTecnologiaSubID)
	s.Equal(uuid.MustParse("af5619e0-3683-5b8c-b9fc-0b3ddfbd2075"), hConsultasSubID)
	s.Equal(uuid.MustParse("c8f579ea-952b-5e24-beed-ef22fb845a4b"), hPlanoSaudeSubID)
	s.Equal(uuid.MustParse("3ca95dd5-c630-5c03-bd47-071777bde81c"), hFarmaciaSubID)
	s.Equal(uuid.MustParse("35ced21e-b436-5cea-afb9-ffd43f98a124"), hLibFinanceiraRootID)
}

func (s *PendingEntryHarnessSuite) TestG7_08_ExpiracaoDePendencia() {
	ledger := &hFakeTxLedger{}
	store := newHarnessStore()
	eng := workflow.NewEngine[PendingEntryState](store, s.obs)
	def := BuildPendingEntryWorkflow(ledger, nil, nil, newHFakeIdempotentWriter())
	key := fmt.Sprintf("%s:thread-001:%s", s.userID, PendingEntryWorkflowID)

	state := hNewExpenseState(s.userID, AwaitingSlotCategory, 20000, "supermercado", "debito", nil)
	state.SuspendedAt = time.Now().UTC().Add(-31 * time.Minute)
	hInsertSuspended(s.T(), store, key, state)

	patch, err := json.Marshal(map[string]string{"resumeText": "supermercado"})
	s.Require().NoError(err)
	result, err := eng.Resume(s.ctx, def, key, patch)
	s.Require().NoError(err)

	s.Equal(workflow.RunStatusSucceeded, result.Status, "G7-08: run succeeded")
	s.Equal(PendingStatusExpired, result.State.Status, "G7-08: status expired")
	s.Contains(result.State.ResponseText, "expirou")
	s.Equal(0, ledger.totalWrites(), "no write on expiration")
}

func (s *PendingEntryHarnessSuite) TestG7_10_MultiplosCandidatosEscolha() {
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("consultas", hCustoFixoRootID, "custo-fixo", hConsultasSubID, "consultas-e-exames", "Custo Fixo > Consultas e Exames")
	h := newPEHarness(s.T(), s.obs, s.userID, ledger, cats, nil)

	candidates := []PendingCategoryCandidate{
		{RootCategoryID: hCustoFixoRootID, RootSlug: "custo-fixo", SubcategoryID: hPlanoSaudeSubID, SubcategorySlug: "plano-de-saude", Path: "Custo Fixo > Plano de Saúde", Score: 0.9},
		{RootCategoryID: hCustoFixoRootID, RootSlug: "custo-fixo", SubcategoryID: hConsultasSubID, SubcategorySlug: "consultas-e-exames", Path: "Custo Fixo > Consultas e Exames", Score: 0.8},
	}
	state := hNewExpenseState(s.userID, AwaitingSlotCategory, 20000, "saúde", "pix", candidates)
	h.start(state)
	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(PendingStatusActive),
		expectAwaitingSlot:  hPtr(AwaitingSlotCategory),
	})

	patch, err := json.Marshal(map[string]any{
		"awaiting":      int(AwaitingSlotConfirmation),
		"candidates":    hSingleCandidate(hCustoFixoRootID, "custo-fixo", hConsultasSubID, "consultas-e-exames", "Custo Fixo > Consultas e Exames"),
		"suspendedAt":   time.Now().UTC(),
		"repromptCount": 0,
	})
	s.Require().NoError(err)
	result, err := h.engine.Resume(s.ctx, h.def, h.key, patch)
	s.Require().NoError(err)
	h.last = &result
	h.confirmSeen = true

	h.assertAgent(harnessAgentStep{
		expectPendingStatus: hPtr(PendingStatusActive),
		expectAwaitingSlot:  hPtr(AwaitingSlotConfirmation),
		expectNoWrite:       true,
	})

	h.sendUser("sim", "wamid-003")
	h.assertAgent(harnessAgentStep{
		expectPendingStatus:      hPtr(PendingStatusCompleted),
		expectRunStatus:          hPtr(workflow.RunStatusSucceeded),
		expectWrite:              &harnessWrite{amountCents: 20000, subcategoryID: hConsultasSubID, categorySource: "user_selected_candidate"},
		expectConfirmBeforeWrite: true,
	})
}

func (s *PendingEntryHarnessSuite) TestG7_14_RespostaAmbiguaSegundaVezCancela() {
	ledger := &hFakeTxLedger{}
	store := newHarnessStore()
	eng := workflow.NewEngine[PendingEntryState](store, s.obs)
	def := BuildPendingEntryWorkflow(ledger, nil, nil, newHFakeIdempotentWriter())
	key := fmt.Sprintf("%s:thread-001:%s", s.userID, PendingEntryWorkflowID)

	state := hNewExpenseState(s.userID, AwaitingSlotCategory, 20000, "loja", "pix", nil)
	state.RepromptCount = 1
	hInsertSuspended(s.T(), store, key, state)

	patch, err := json.Marshal(map[string]string{"resumeText": "ok sim"})
	s.Require().NoError(err)
	result, err := eng.Resume(s.ctx, def, key, patch)
	s.Require().NoError(err)

	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(PendingStatusCancelled, result.State.Status, "G7-14: segunda ambiguidade cancela")
	s.Equal(0, ledger.totalWrites())
}

func (s *PendingEntryHarnessSuite) TestG7_15_ErroLedgerSemSucessoSimulado() {
	ledger := &hFakeTxLedger{forceErr: errors.New("db error 500")}
	h := newPEHarness(s.T(), s.obs, s.userID, ledger, newHFakeCatReader(), nil)

	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hSupermercadoSubID, "supermercado", "Custo Fixo > Supermercado")
	state := hNewExpenseState(s.userID, AwaitingSlotConfirmation, 30000, "supermercado", "pix", candidato)
	h.start(state)
	h.confirmSeen = true

	h.sendUser("sim", "wamid-002")
	s.NotEqual(PendingStatusCompleted, h.last.State.Status, "status must not be completed on ledger error")

	resp := h.last.State.ResponseText
	s.NotContains(resp, "registrei", "M-03=0: no false success")
	s.NotContains(resp, "anotei", "M-03=0: no false success")
	s.NotContains(resp, "salvo", "M-03=0: no false success")
}

func (s *PendingEntryHarnessSuite) TestG12_04_MultiplosCandidatosEscolhaPorNumeroENome() {
	candidates := []PendingCategoryCandidate{
		{RootCategoryID: hCustoFixoRootID, RootSlug: "custo-fixo", SubcategoryID: hPlanoSaudeSubID, SubcategorySlug: "plano-de-saude", Path: "Custo Fixo > Plano de Saúde", Score: 0.9},
		{RootCategoryID: hCustoFixoRootID, RootSlug: "custo-fixo", SubcategoryID: hConsultasSubID, SubcategorySlug: "consultas-e-exames", Path: "Custo Fixo > Consultas e Exames", Score: 0.8},
	}
	state := hNewExpenseState(s.userID, AwaitingSlotCategory, 20000, "saúde", "pix", candidates)

	byIdx, errIdx := DecideCategoryChoice(state, candidates, "2")
	s.Require().NoError(errIdx)
	s.Equal(CategoryChoiceActionSelected, byIdx.Action)
	s.Equal(hConsultasSubID, byIdx.Candidate.SubcategoryID, "CA-15: index 2 resolves consultas-e-exames")

	byName, errName := DecideCategoryChoice(state, candidates, "consultas-e-exames")
	s.Require().NoError(errName)
	s.Equal(CategoryChoiceActionSelected, byName.Action)
	s.Equal(hConsultasSubID, byName.Candidate.SubcategoryID, "CA-15: name also resolves")
}

func (s *PendingEntryHarnessSuite) TestIdempotentWriter_ReplayNaoFaz2oInsert() {
	ledger := &hFakeTxLedger{}
	cats := newHFakeCatReader()
	cats.addLeaf("farmacia", hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
	idem := newHFakeIdempotentWriter()
	store := newHarnessStore()
	eng := workflow.NewEngine[PendingEntryState](store, s.obs)
	def := BuildPendingEntryWorkflow(ledger, nil, cats, idem)
	key := fmt.Sprintf("%s:thread-001:%s", s.userID, PendingEntryWorkflowID)

	candidato := hSingleCandidate(hCustoFixoRootID, "custo-fixo", hFarmaciaSubID, "medicamentos-e-farmacia", "Custo Fixo > Medicamentos e Farmácia")
	state := PendingEntryState{
		Status:        PendingStatusActive,
		Awaiting:      AwaitingSlotConfirmation,
		UserID:        s.userID,
		ThreadID:      "thread-001",
		MessageID:     "wamid-original",
		AmountCents:   5000,
		Description:   "farmácia",
		PaymentMethod: "pix",
		Kind:          ifaces.CategoryKindExpense,
		Candidates:    candidato,
		SuspendedAt:   time.Now().UTC(),
		OperationKind: PendingOpRegisterExpense,
	}

	result1, err := eng.Start(s.ctx, def, key, state)
	s.Require().NoError(err)
	s.Require().Equal(workflow.RunStatusSuspended, result1.Status)

	patch1, err := json.Marshal(map[string]string{"resumeText": "sim", "incomingMessageId": "wamid-002"})
	s.Require().NoError(err)
	result2, err := eng.Resume(s.ctx, def, key, patch1)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result2.Status)
	s.Equal(PendingStatusCompleted, result2.State.Status)
	s.Equal(1, ledger.totalWrites(), "primeira escrita: 1 insert")

	idem2 := idem
	store2 := newHarnessStore()
	eng2 := workflow.NewEngine[PendingEntryState](store2, s.obs)
	def2 := BuildPendingEntryWorkflow(ledger, nil, cats, idem2)
	key2 := fmt.Sprintf("%s:thread-002:%s", s.userID, PendingEntryWorkflowID)

	state2 := state
	state2.ThreadID = "thread-002"
	result3, err := eng2.Start(s.ctx, def2, key2, state2)
	s.Require().NoError(err)
	s.Require().Equal(workflow.RunStatusSuspended, result3.Status)

	patch2, err := json.Marshal(map[string]string{"resumeText": "sim", "incomingMessageId": "wamid-003"})
	s.Require().NoError(err)
	result4, err := eng2.Resume(s.ctx, def2, key2, patch2)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result4.Status)
	s.Equal(PendingStatusCompleted, result4.State.Status)
	s.Equal(1, ledger.totalWrites(), "replay: sem segundo INSERT")
	s.NotEmpty(result4.State.ResponseText, "replay retorna texto de sucesso")
}

package binding

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	transactionsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type capturingTransactionCreateUC struct {
	last transactionsinput.RawCreateTransaction
	out  transactionsoutput.Transaction
}

func (f *capturingTransactionCreateUC) Execute(_ context.Context, raw transactionsinput.RawCreateTransaction) (transactionsoutput.Transaction, error) {
	f.last = raw
	return f.out, nil
}

type capturingCardPurchaseCreateUC struct {
	last transactionsinput.RawCreateCardPurchase
}

func (f *capturingCardPurchaseCreateUC) Execute(_ context.Context, raw transactionsinput.RawCreateCardPurchase) (transactionsoutput.CardPurchase, error) {
	f.last = raw
	return transactionsoutput.CardPurchase{}, nil
}

type stubCardLister struct {
	list cardoutput.CardList
}

func (s *stubCardLister) Execute(_ context.Context, _ cardinput.ListCards) (cardoutput.CardList, error) {
	return s.list, nil
}

type stubCategorySearch struct {
	out *categoriesoutput.DictionarySearchOutput
}

func (s *stubCategorySearch) Execute(_ context.Context, _ *categoriesinput.SearchDictionaryInput) (*categoriesoutput.DictionarySearchOutput, error) {
	return s.out, nil
}

type reproStore struct {
	mu   sync.Mutex
	runs map[string]platform.Snapshot
}

func newReproStore() *reproStore { return &reproStore{runs: make(map[string]platform.Snapshot)} }

func (s *reproStore) key(workflow, correlationKey string) string {
	return workflow + ":" + correlationKey
}

func (s *reproStore) Insert(_ context.Context, snap platform.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *reproStore) Load(_ context.Context, workflow, key string) (platform.Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.runs[s.key(workflow, key)]
	return snap, ok, nil
}

func (s *reproStore) Save(_ context.Context, snap platform.Snapshot, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *reproStore) AppendStep(_ context.Context, _ platform.StepRecord) error { return nil }

func (s *reproStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

type ForcedCategoryPersistReproSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestForcedCategoryPersistReproSuite(t *testing.T) {
	suite.Run(t, new(ForcedCategoryPersistReproSuite))
}

func (s *ForcedCategoryPersistReproSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
}

func (s *ForcedCategoryPersistReproSuite) buildDefinition(resolver steps.CategoryResolverFunc, persist steps.PersistFunc) platform.Definition[steps.ExpenseState] {
	return workflow.NewTransactionsWriteDefinition(workflow.TransactionsWriteDeps{
		Authorize: func(_ context.Context, _ steps.ExpenseState) bool { return true },
		Replay:    func(_ context.Context, _ steps.ExpenseState) (string, bool) { return "", false },
		Policy:    func(_ context.Context, _ steps.ExpenseState) (bool, string) { return false, "" },
		AuditBegin: func(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
			return steps.AuditBeginResult{Settle: func(_ context.Context, _ bool) {}}
		},
		OnSettle:       nil,
		Resolver:       resolver,
		Persist:        persist,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha auditoria",
	})
}

func (s *ForcedCategoryPersistReproSuite) TestExpenseAmbiguousChoiceResolvesUUIDThroughRealPersist() {
	rootRestaurante := uuid.New()
	subRestaurante := uuid.New()
	rootMercado := uuid.New()
	subMercado := uuid.New()

	resolver := NewKernelCategoryResolver(&stubCategorySearch{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: subRestaurante, RootCategoryID: rootRestaurante, Path: "Alimentação > Restaurante", Score: 0.9, IsAmbiguous: true},
				{CategoryID: subMercado, RootCategoryID: rootMercado, Path: "Alimentação > Mercado", Score: 0.85},
			},
		},
	})

	capturing := &capturingTransactionCreateUC{out: transactionsoutput.Transaction{AmountCents: 8000, Direction: "outcome"}}
	realCreator := NewTransactionCreatorAdapter(capturing)
	recorder := NewTransactionLoggerAdapter(usecases.NewRecordTransactionFromAgent(&stubCategorySearch{}, realCreator, s.obs))
	persist := NewKernelPersistFunc(recorder, nil)

	def := s.buildDefinition(resolver, persist)
	store := newReproStore()
	eng := platform.NewEngine[steps.ExpenseState](store, s.obs)
	key := "user-ambig:whatsapp"

	initial := steps.ExpenseState{
		UserID:          uuid.New(),
		Channel:         "whatsapp",
		MessageID:       "msg-1",
		Kind:            intent.KindRecordExpense,
		TransactionKind: pendingexpense.TransactionKindExpense,
		AmountCents:     8000,
		Merchant:        "Lugar X",
		CategoryHint:    "comida",
		PaymentMethod:   "debit",
		Direction:       "outcome",
	}

	startResult, err := eng.Start(s.ctx, def, key, initial)
	s.Require().NoError(err)
	s.Require().Equal(platform.RunStatusSuspended, startResult.Status, "categoria ambígua deve suspender")
	s.Equal(tools.OutcomeClarify, startResult.State.Outcome)

	resumeResult, err := eng.Resume(s.ctx, def, key, []byte(`{"ResumeText":"2"}`))
	s.Require().NoError(err)
	s.Equal(tools.OutcomeRouted, resumeResult.State.Outcome, "escolha 2 deve rotear e persistir")

	s.Equal(rootMercado, capturing.last.CategoryID, "creator deve receber o root UUID do candidate escolhido, não o display path")
	s.Require().NotNil(capturing.last.SubcategoryID, "saída deve carregar a subcategoria resolvida")
	s.Equal(subMercado, *capturing.last.SubcategoryID)
}

func (s *ForcedCategoryPersistReproSuite) TestCardPurchaseNeedsConfirmResolvesUUIDThroughRealPersist() {
	rootMercado := uuid.New()
	subMercado := uuid.New()
	cardID := uuid.New()

	resolver := NewKernelCategoryResolver(&stubCategorySearch{
		out: &categoriesoutput.DictionarySearchOutput{
			Candidates: []categoriesoutput.CandidateOutput{
				{CategoryID: subMercado, RootCategoryID: rootMercado, Path: "Alimentação > Mercado", Score: categoriesvo.ScoreConfirmThreshold},
			},
		},
	})

	capturing := &capturingCardPurchaseCreateUC{}
	cardLister := &stubCardLister{list: cardoutput.CardList{Items: []cardoutput.Card{{ID: cardID.String(), Name: "Nubank", Nickname: "nubank"}}}}
	cardCreator := NewCardPurchaseCreatorAdapter(cardLister, capturing)
	logger := NewCardPurchaseLoggerAdapter(usecases.NewRecordCardPurchaseFromAgent(&stubCategorySearch{}, cardCreator, s.obs))
	persist := NewKernelPersistFunc(nil, logger)

	def := s.buildDefinition(resolver, persist)
	store := newReproStore()
	eng := platform.NewEngine[steps.ExpenseState](store, s.obs)
	key := "user-confirm:whatsapp"

	initial := steps.ExpenseState{
		UserID:          uuid.New(),
		Channel:         "whatsapp",
		MessageID:       "msg-2",
		Kind:            intent.KindRecordCardPurchase,
		TransactionKind: pendingexpense.TransactionKindCardPurchase,
		AmountCents:     30000,
		Merchant:        "Compra Z",
		CategoryHint:    "mercado",
		CardHint:        "nubank",
		Installments:    3,
	}

	startResult, err := eng.Start(s.ctx, def, key, initial)
	s.Require().NoError(err)
	s.Require().Equal(platform.RunStatusSuspended, startResult.Status, "categoria que precisa de confirmação deve suspender")

	resumeResult, err := eng.Resume(s.ctx, def, key, []byte(`{"ResumeText":"sim"}`))
	s.Require().NoError(err)
	s.Equal(tools.OutcomeRouted, resumeResult.State.Outcome, "confirmação deve rotear e persistir a compra parcelada")

	s.Equal(rootMercado, capturing.last.CategoryID, "card purchase creator deve receber o root UUID confirmado, não o display path")
	s.Equal(cardID, capturing.last.CardID)
	s.Require().NotNil(capturing.last.SubcategoryID, "saída deve carregar a subcategoria resolvida")
	s.Equal(subMercado, *capturing.last.SubcategoryID)
}

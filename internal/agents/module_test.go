package agents

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

type fakeDB struct{}

func (f *fakeDB) PrepareContext(_ context.Context, _ string) (*sql.Stmt, error)  { return nil, nil }
func (f *fakeDB) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row { return nil }
func (f *fakeDB) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	return nil, nil
}
func (f *fakeDB) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, nil
}

func testBuildFinancialToolsArgs() []tool.ToolHandle {
	return buildFinancialTools(
		nil, nil, nil, nil, nil, nil,
		nil, workflow.Definition[workflows.DestructiveManageState]{},
		nil, workflow.Definition[workflows.CardManageState]{},
		nil, workflow.Definition[workflows.BudgetManageState]{},
		nil, workflow.Definition[workflows.GoalEditState]{},
	)
}

func TestBuildFinancialTools_ReturnsExactly30Tools(t *testing.T) {
	tools := testBuildFinancialToolsArgs()
	assert.Len(t, tools, 30)
}

func TestBuildFinancialTools_RegistersCreateCardTool(t *testing.T) {
	tools := testBuildFinancialToolsArgs()
	ids := make([]string, 0, len(tools))
	for _, tl := range tools {
		ids = append(ids, tl.ID())
	}
	assert.Contains(t, ids, "create_card")
}

func TestBuildFinancialTools_RegistersCreateBudgetTool(t *testing.T) {
	tools := testBuildFinancialToolsArgs()
	ids := make([]string, 0, len(tools))
	for _, tl := range tools {
		ids = append(ids, tl.ID())
	}
	assert.Contains(t, ids, "create_budget")
}

func TestBuildFinancialTools_RegistersNewTools(t *testing.T) {
	tools := testBuildFinancialToolsArgs()
	ids := make([]string, 0, len(tools))
	for _, tl := range tools {
		ids = append(ids, tl.ID())
	}
	assert.Contains(t, ids, "edit_budget_total")
	assert.Contains(t, ids, "category_detail")
	assert.Contains(t, ids, "cancel_plan_info")
	assert.Contains(t, ids, "support_info")
	assert.Contains(t, ids, "edit_goal")
}

func TestNewModule_RequiredDepsValidation(t *testing.T) {
	o11y := fake.NewProvider()
	validLLM := LLMConfig{APIKey: "key", Model: "openai/gpt-4o-mini"}

	scenarios := []struct {
		name    string
		deps    Deps
		wantErr string
	}{
		{
			name:    "db ausente",
			deps:    Deps{DB: nil, O11y: o11y, LLM: validLLM},
			wantErr: "agents.module: db is required",
		},
		{
			name:    "o11y ausente",
			deps:    Deps{DB: &fakeDB{}, O11y: nil, LLM: validLLM},
			wantErr: "agents.module: o11y is required",
		},
		{
			name:    "llm api_key ausente",
			deps:    Deps{DB: &fakeDB{}, O11y: o11y, LLM: LLMConfig{APIKey: ""}},
			wantErr: "agents.module: llm api_key is required",
		},
		{
			name: "deps validas constroem modulo sem erro",
			deps: Deps{
				DB:   &fakeDB{},
				O11y: o11y,
				LLM: LLMConfig{
					APIKey:  "key",
					Model:   "openai/gpt-4o-mini",
					BaseURL: "https://openrouter.ai",
				},
				CategoriesModule:   &categories.CategoriesModule{},
				CardModule:         card.CardModule{},
				BudgetsModule:      &budgets.BudgetsModule{},
				TransactionsModule: transactions.TransactionsModule{},
			},
			wantErr: "",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			_, err := NewModule(sc.deps)
			if sc.wantErr != "" {
				assert.ErrorContains(t, err, sc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type WhatsAppAgentRouteSuite struct {
	suite.Suite
	ctx context.Context
}

func TestWhatsAppAgentRouteSuite(t *testing.T) {
	suite.Run(t, new(WhatsAppAgentRouteSuite))
}

func (s *WhatsAppAgentRouteSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *WhatsAppAgentRouteSuite) ctxWithPrincipal() context.Context {
	principal := auth.Principal{
		UserID: uuid.MustParse("a0a0a0a0-0000-0000-0000-000000000001"),
		Source: auth.SourceWhatsApp,
	}
	return auth.WithPrincipal(s.ctx, principal)
}

func (s *WhatsAppAgentRouteSuite) TestBuildWhatsAppAgentRoute_ValidTimestamp_UsesMetaTimestamp() {
	o11y := fake.NewProvider()
	publisherMock := outboxmocks.NewPublisher(s.T())

	var capturedEvent outbox.Event
	publisherMock.On("Publish", mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
		capturedEvent = evt
		return true
	})).Return(nil).Once()

	route := buildWhatsAppAgentRoute(publisherMock, o11y)
	ctx := s.ctxWithPrincipal()

	msg := wapayload.Message{
		From:      "+5511999999999",
		WAMID:     "wamid-valid-ts",
		Timestamp: "1686000000",
		Text:      "oi",
	}

	outcome := route(ctx, msg)

	s.Equal("agent", string(outcome))
	expectedTS := time.Unix(1686000000, 0).UTC()
	s.Equal(expectedTS, capturedEvent.OccurredAt, "OccurredAt deve refletir o timestamp da Meta")
	s.Equal("wamid-valid-ts", capturedEvent.AggregateID)
}

func (s *WhatsAppAgentRouteSuite) TestBuildWhatsAppAgentRoute_EmptyTimestamp_UsesFallback() {
	o11y := fake.NewProvider()
	publisherMock := outboxmocks.NewPublisher(s.T())

	before := time.Now().UTC()
	var capturedEvent outbox.Event
	publisherMock.On("Publish", mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
		capturedEvent = evt
		return true
	})).Return(nil).Once()

	route := buildWhatsAppAgentRoute(publisherMock, o11y)
	ctx := s.ctxWithPrincipal()

	msg := wapayload.Message{
		From:      "+5511999999999",
		WAMID:     "wamid-no-ts",
		Timestamp: "",
		Text:      "oi",
	}

	outcome := route(ctx, msg)
	after := time.Now().UTC()

	s.Equal("agent", string(outcome))
	s.True(!capturedEvent.OccurredAt.Before(before), "OccurredAt fallback deve ser >= before")
	s.True(!capturedEvent.OccurredAt.After(after), "OccurredAt fallback deve ser <= after")
}

func (s *WhatsAppAgentRouteSuite) TestBuildWhatsAppAgentRoute_InvalidTimestamp_UsesFallback() {
	o11y := fake.NewProvider()
	publisherMock := outboxmocks.NewPublisher(s.T())

	before := time.Now().UTC()
	var capturedEvent outbox.Event
	publisherMock.On("Publish", mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
		capturedEvent = evt
		return true
	})).Return(nil).Once()

	route := buildWhatsAppAgentRoute(publisherMock, o11y)
	ctx := s.ctxWithPrincipal()

	msg := wapayload.Message{
		From:      "+5511999999999",
		WAMID:     "wamid-bad-ts",
		Timestamp: "not-a-number",
		Text:      "oi",
	}

	outcome := route(ctx, msg)
	after := time.Now().UTC()

	s.Equal("agent", string(outcome))
	s.True(!capturedEvent.OccurredAt.Before(before), "OccurredAt fallback deve ser >= before")
	s.True(!capturedEvent.OccurredAt.After(after), "OccurredAt fallback deve ser <= after")
}

func (s *WhatsAppAgentRouteSuite) TestBuildWhatsAppAgentRoute_NoPrincipal_ReturnsInvalid() {
	o11y := fake.NewProvider()
	publisherMock := outboxmocks.NewPublisher(s.T())

	route := buildWhatsAppAgentRoute(publisherMock, o11y)

	msg := wapayload.Message{
		From:      "+5511999999999",
		WAMID:     "wamid-no-principal",
		Timestamp: "1686000000",
		Text:      "oi",
	}

	outcome := route(s.ctx, msg)

	s.Equal("invalid", string(outcome))
}

type fakeWriteLedger struct {
	entry     usecases.WriteLedgerEntry
	found     bool
	findErr   error
	insertErr error
	inserted  []usecases.WriteLedgerEntry
}

func (f *fakeWriteLedger) FindByKey(_ context.Context, _ string, _ int, _ string) (usecases.WriteLedgerEntry, error) {
	if f.findErr != nil {
		return usecases.WriteLedgerEntry{}, f.findErr
	}
	if !f.found {
		return usecases.WriteLedgerEntry{}, usecases.ErrLedgerEntryNotFound
	}
	return f.entry, nil
}

func (f *fakeWriteLedger) Insert(_ context.Context, entry usecases.WriteLedgerEntry) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, entry)
	return nil
}

func (f *fakeWriteLedger) DeleteBefore(_ context.Context, _ time.Time, _ int) (int64, error) {
	return 0, nil
}

type IdempotentWriterAdapterSuite struct {
	suite.Suite
	ctx    context.Context
	userID uuid.UUID
	wamid  string
}

func TestIdempotentWriterAdapterSuite(t *testing.T) {
	suite.Run(t, new(IdempotentWriterAdapterSuite))
}

func (s *IdempotentWriterAdapterSuite) SetupTest() {
	s.ctx = context.Background()
	s.userID = uuid.New()
	s.wamid = "wamid-test-123"
}

func (s *IdempotentWriterAdapterSuite) TestExecute_NewWrite_ReturnsRoutedOutcome() {
	resourceID := uuid.New()
	ledger := &fakeWriteLedger{}
	uc := usecases.NewIdempotentWrite(ledger, fake.NewProvider())
	adapter := idempotentWriterAdapter{uc: uc}

	writeFn := workflows.IdempotentWriteFn(func(_ context.Context) (uuid.UUID, bool, error) {
		return resourceID, false, nil
	})

	gotID, outcome, err := adapter.Execute(s.ctx, s.userID, s.wamid, 1, "create_transaction", "transaction", writeFn, nil)

	s.NoError(err)
	s.Equal(resourceID, gotID)
	s.Equal(agent.ToolOutcomeRouted, outcome)
}

func (s *IdempotentWriterAdapterSuite) TestExecute_ReplayExisting_ReturnsReplayOutcome() {
	existingID := uuid.New()
	ledger := &fakeWriteLedger{
		found: true,
		entry: usecases.WriteLedgerEntry{
			ResourceID: existingID,
			WAMID:      s.wamid,
			ItemSeq:    1,
			Operation:  "create_transaction",
		},
	}
	uc := usecases.NewIdempotentWrite(ledger, fake.NewProvider())
	adapter := idempotentWriterAdapter{uc: uc}

	writeFn := workflows.IdempotentWriteFn(func(_ context.Context) (uuid.UUID, bool, error) {
		return uuid.New(), false, nil
	})

	gotID, outcome, err := adapter.Execute(s.ctx, s.userID, s.wamid, 1, "create_transaction", "transaction", writeFn, nil)

	s.NoError(err)
	s.Equal(existingID, gotID)
	s.Equal(agent.ToolOutcomeReplay, outcome)
}

func (s *IdempotentWriterAdapterSuite) TestExecute_WriteFnError_ReturnsError() {
	writeErr := errors.New("usecase failed")
	ledger := &fakeWriteLedger{}
	uc := usecases.NewIdempotentWrite(ledger, fake.NewProvider())
	adapter := idempotentWriterAdapter{uc: uc}

	writeFn := workflows.IdempotentWriteFn(func(_ context.Context) (uuid.UUID, bool, error) {
		return uuid.Nil, false, writeErr
	})

	_, _, err := adapter.Execute(s.ctx, s.userID, s.wamid, 1, "create_transaction", "transaction", writeFn, nil)

	s.Error(err)
}

func (s *IdempotentWriterAdapterSuite) TestExecute_ReconciledWrite_ReturnsReconciledOutcome() {
	resourceID := uuid.New()
	ledger := &fakeWriteLedger{}
	uc := usecases.NewIdempotentWrite(ledger, fake.NewProvider())
	adapter := idempotentWriterAdapter{uc: uc}

	writeFn := workflows.IdempotentWriteFn(func(_ context.Context) (uuid.UUID, bool, error) {
		return resourceID, true, nil
	})

	gotID, outcome, err := adapter.Execute(s.ctx, s.userID, s.wamid, 1, "create_transaction", "transaction", writeFn, nil)

	s.NoError(err)
	s.Equal(resourceID, gotID)
	s.Equal(agent.ToolOutcomeReconciled, outcome)
}

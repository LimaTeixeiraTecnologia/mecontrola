package usecases

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	catinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeTransactionWriteEngine struct {
	startResult wf.RunResult[workflows.TransactionWriteState]
	startErr    error
	startCalled bool
	lastState   workflows.TransactionWriteState
	lastKey     string
}

func (f *fakeTransactionWriteEngine) Start(_ context.Context, _ wf.Definition[workflows.TransactionWriteState], key string, state workflows.TransactionWriteState) (wf.RunResult[workflows.TransactionWriteState], error) {
	f.startCalled = true
	f.lastKey = key
	f.lastState = state
	return f.startResult, f.startErr
}

func (f *fakeTransactionWriteEngine) Resume(_ context.Context, _ wf.Definition[workflows.TransactionWriteState], _ string, _ []byte) (wf.RunResult[workflows.TransactionWriteState], error) {
	return wf.RunResult[workflows.TransactionWriteState]{}, nil
}

func (f *fakeTransactionWriteEngine) LoadLatestState(_ context.Context, _ wf.Definition[workflows.TransactionWriteState], _ string) (workflows.TransactionWriteState, wf.Snapshot, bool, error) {
	return workflows.TransactionWriteState{}, wf.Snapshot{}, false, nil
}

type TransactionWriteStarterSuite struct {
	suite.Suite
	ctx        context.Context
	obs        *fake.Provider
	categories *imocks.CategoriesReader
	ledger     *imocks.TransactionsLedger
	engine     *fakeTransactionWriteEngine
	uc         *TransactionWriteStarter
	userID     uuid.UUID
	catRootID  uuid.UUID
	catSubID   uuid.UUID
}

func TestTransactionWriteStarterSuite(t *testing.T) {
	suite.Run(t, new(TransactionWriteStarterSuite))
}

func (s *TransactionWriteStarterSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.categories = imocks.NewCategoriesReader(s.T())
	s.categories.EXPECT().CatalogVersion(mock.Anything).Return(int64(9), nil).Maybe()
	s.ledger = imocks.NewTransactionsLedger(s.T())
	s.engine = &fakeTransactionWriteEngine{
		startResult: wf.RunResult[workflows.TransactionWriteState]{
			Status:  wf.RunStatusSuspended,
			Suspend: &wf.Suspension{Prompt: "Posso registrar?"},
		},
	}
	s.userID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	s.catRootID = uuid.New()
	s.catSubID = uuid.New()

	def := workflows.BuildTransactionWriteWorkflowWithObservability(nil, nil, nil, nil, nil)
	s.uc = NewTransactionWriteStarter(s.categories, s.ledger, s.engine, def, s.obs)
}

func (s *TransactionWriteStarterSuite) TestRegisterExpense_ExplicitCategory_StartsWorkflow() {
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, interfaces.CategoryWriteRequest{
			RootCategoryID:  s.catRootID,
			SubcategoryID:   s.catSubID,
			Kind:            interfaces.CategoryKindExpense,
			ExpectedVersion: 9,
		}).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:  s.catRootID,
			SubcategoryID:   s.catSubID,
			RootSlug:        "custo-fixo",
			SubcategorySlug: "mercado",
			Path:            "Custo Fixo > Mercado",
		}, nil).Once()

	result, err := s.uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
		UserID:        s.userID,
		ThreadID:      "thr-001",
		WAMID:         "wamid-001",
		AmountCents:   15000,
		Description:   "mercado",
		PaymentMethod: "pix",
		CategoryID:    s.catRootID,
		SubcategoryID: s.catSubID,
	})

	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.True(s.engine.startCalled)
	s.Equal(workflows.TransactionOpRegisterExpense, s.engine.lastState.OperationKind)
	s.Equal(s.userID, s.engine.lastState.UserID)
	s.Require().Len(s.engine.lastState.Candidates, 1)
	s.Equal(s.catRootID, s.engine.lastState.Candidates[0].RootCategoryID)
}

func (s *TransactionWriteStarterSuite) TestRegisterIncome_UsesFixedPaymentMethod() {
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.Anything).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:  s.catRootID,
			SubcategoryID:   s.catSubID,
			RootSlug:        "entradas",
			SubcategorySlug: "salario",
		}, nil).Once()

	result, err := s.uc.RegisterIncome(s.ctx, RegisterIncomeCommand{
		UserID:        s.userID,
		ThreadID:      "thr-002",
		WAMID:         "wamid-002",
		AmountCents:   500000,
		Description:   "salário",
		CategoryID:    s.catRootID,
		SubcategoryID: s.catSubID,
	})

	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.Equal(workflows.TransactionOpRegisterIncome, s.engine.lastState.OperationKind)
	s.Equal(registerIncomePaymentMethod, s.engine.lastState.PaymentMethod)
}

func (s *TransactionWriteStarterSuite) TestRegisterExpense_ActiveRunAlreadyExists() {
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.Anything).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:  s.catRootID,
			SubcategoryID:   s.catSubID,
			RootSlug:        "custo-fixo",
			SubcategorySlug: "mercado",
		}, nil).Once()
	s.engine.startErr = wf.ErrRunAlreadyExists

	result, err := s.uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
		UserID:        s.userID,
		ThreadID:      "thr-003",
		WAMID:         "wamid-003",
		AmountCents:   1000,
		Description:   "mercado",
		PaymentMethod: "pix",
		CategoryID:    s.catRootID,
		SubcategoryID: s.catSubID,
	})

	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.NotEmpty(result.Message)
}

func (s *TransactionWriteStarterSuite) TestEditEntry_WithTargetID_FetchesCurrentAndStarts() {
	targetID := uuid.New()
	s.ledger.EXPECT().
		GetTransaction(mock.Anything, targetID.String()).
		Return(interfaces.Entry{
			ID:            targetID.String(),
			Direction:     "outcome",
			CategoryID:    s.catRootID.String(),
			PaymentMethod: "pix",
			AmountCents:   9000,
			Description:   "mercado",
			Version:       3,
		}, nil).Once()

	result, err := s.uc.EditEntry(s.ctx, EditEntryCommand{
		UserID:              s.userID,
		ThreadID:            "thr-004",
		WAMID:               "wamid-004",
		TargetTransactionID: targetID,
		AmountCents:         9500,
	})

	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.Equal(workflows.TransactionOpEditEntry, s.engine.lastState.OperationKind)
	s.Require().NotNil(s.engine.lastState.TargetTransactionID)
	s.Equal(targetID, *s.engine.lastState.TargetTransactionID)
	s.Equal(int64(3), s.engine.lastState.TargetVersion)
}

func (s *TransactionWriteStarterSuite) TestEditEntry_WithoutTargetID_UsesSearchCriteria() {
	result, err := s.uc.EditEntry(s.ctx, EditEntryCommand{
		UserID:            s.userID,
		ThreadID:          "thr-005",
		WAMID:             "wamid-005",
		SearchAmountCents: 9000,
		SearchTerm:        "mercado",
	})

	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.Nil(s.engine.lastState.TargetTransactionID)
	s.Equal(int64(9000), s.engine.lastState.EditSearchAmountCents)
	s.Equal("mercado", s.engine.lastState.EditSearchTerm)
}

func (s *TransactionWriteStarterSuite) TestRegisterExpense_ExplicitCategory_CarriesManualEvidenceContract() {
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.Anything).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:  s.catRootID,
			SubcategoryID:   s.catSubID,
			RootSlug:        "custo-fixo",
			SubcategorySlug: "mercado",
			Path:            "Custo Fixo > Mercado",
		}, nil).Once()

	_, err := s.uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
		UserID:        s.userID,
		ThreadID:      "thr-010",
		WAMID:         "wamid-010",
		AmountCents:   600000,
		Description:   "TV",
		PaymentMethod: "credit_card",
		CategoryID:    s.catRootID,
		SubcategoryID: s.catSubID,
	})

	s.Require().NoError(err)
	s.Require().Len(s.engine.lastState.Candidates, 1)
	candidate := s.engine.lastState.Candidates[0]
	s.Equal("manual_confirmed", candidate.Confidence)
	s.Equal("manual_canonical", candidate.MatchQuality)
	s.Equal("manual_canonical", candidate.SignalType)
	s.Equal("mercado", candidate.MatchedTerm)
	s.Equal("manual canonical id validated", candidate.MatchReason)
}

func (s *TransactionWriteStarterSuite) TestRegisterExpense_ShortTermInvalidQuery_AsksCategoryInsteadOfError() {
	rootID := uuid.New()
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "TV", "expense").
		Return(interfaces.CategorySearchResult{}, fmt.Errorf("agents/binding/categories_reader: buscar dicionário: %w", catinput.ErrInvalidQuery)).
		Once()
	s.categories.EXPECT().
		ListCategories(mock.Anything, s.userID).
		Return([]interfaces.Category{{ID: rootID, Slug: "custo-fixo", Name: "Custo Fixo", Kind: "expense"}}, nil).
		Once()

	result, err := s.uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
		UserID:        s.userID,
		ThreadID:      "thr-011",
		WAMID:         "wamid-011",
		AmountCents:   600000,
		Description:   "TV",
		PaymentMethod: "credit_card",
	})

	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.True(s.engine.startCalled)
	s.Require().Len(s.engine.lastState.Candidates, 1)
	s.Equal(rootID, s.engine.lastState.Candidates[0].RootCategoryID)
	s.Equal(uuid.Nil, s.engine.lastState.Candidates[0].SubcategoryID)
	s.Equal("Custo Fixo", s.engine.lastState.Candidates[0].Path)
}

func (s *TransactionWriteStarterSuite) TestRegisterExpense_CategoryTextLeaf_ResolvesWithCatalogVersion() {
	s.categories.EXPECT().
		ListCategories(mock.Anything, s.userID).
		Return([]interfaces.Category{{
			ID: s.catRootID, Slug: "custo-fixo", Name: "Custo Fixo", Kind: "expense",
			Subcategories: []interfaces.Category{{ID: s.catSubID, Slug: "combustivel", Name: "Combustível"}},
		}}, nil).
		Once()
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, interfaces.CategoryWriteRequest{
			RootCategoryID:  s.catRootID,
			SubcategoryID:   s.catSubID,
			Kind:            interfaces.CategoryKindExpense,
			ExpectedVersion: 9,
		}).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:   s.catRootID,
			SubcategoryID:    s.catSubID,
			RootSlug:         "custo-fixo",
			SubcategorySlug:  "combustivel",
			Path:             "Custo Fixo > Combustível",
			EditorialVersion: 9,
		}, nil).
		Once()

	result, err := s.uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
		UserID:       s.userID,
		ThreadID:     "thr-020",
		WAMID:        "wamid-020",
		AmountCents:  10000,
		Description:  "abastecimento do veículo",
		CategoryText: "combustível",
	})

	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.Require().Len(s.engine.lastState.Candidates, 1)
	s.Equal(s.catSubID, s.engine.lastState.Candidates[0].SubcategoryID)
	s.Equal("Custo Fixo > Combustível", s.engine.lastState.Candidates[0].Path)
	s.Equal(int64(9), s.engine.lastState.CategoryVersion)
}

func (s *TransactionWriteStarterSuite) TestRegisterExpense_CategoryTextRoot_ListsLeavesWithCatalogVersion() {
	leafA := uuid.New()
	leafB := uuid.New()
	s.categories.EXPECT().
		ListCategories(mock.Anything, s.userID).
		Return([]interfaces.Category{{
			ID: s.catRootID, Slug: "custo-fixo", Name: "Custo Fixo", Kind: "expense",
			Subcategories: []interfaces.Category{
				{ID: leafA, Slug: "combustivel", Name: "Combustível"},
				{ID: leafB, Slug: "supermercado", Name: "Supermercado"},
			},
		}}, nil).
		Once()
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "tv nova", "expense").
		Return(interfaces.CategorySearchResult{}, fmt.Errorf("buscar dicionário: %w", catinput.ErrInvalidQuery)).
		Once()

	result, err := s.uc.RegisterExpense(s.ctx, RegisterExpenseCommand{
		UserID:       s.userID,
		ThreadID:     "thr-021",
		WAMID:        "wamid-021",
		AmountCents:  50000,
		Description:  "tv nova",
		CategoryText: "custos fixos",
	})

	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.Require().Len(s.engine.lastState.Candidates, 2)
	s.Equal(leafA, s.engine.lastState.Candidates[0].SubcategoryID)
	s.Equal(int64(9), s.engine.lastState.CategoryVersion,
		"candidatos derivados do catálogo devem carregar a versão editorial vigente para o ResolveForWrite na escrita")
}

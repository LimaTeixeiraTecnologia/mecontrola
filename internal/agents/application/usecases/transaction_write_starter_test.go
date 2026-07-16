package usecases

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
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
			RootCategoryID: s.catRootID,
			SubcategoryID:  s.catSubID,
			Kind:           interfaces.CategoryKindExpense,
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

package usecases

import (
	"context"
	"errors"
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

type fakeEngine struct {
	startResult wf.RunResult[workflows.PendingEntryState]
	startErr    error
	startCalled bool
	lastState   workflows.PendingEntryState
	lastKey     string
}

func (f *fakeEngine) Start(_ context.Context, _ wf.Definition[workflows.PendingEntryState], key string, state workflows.PendingEntryState) (wf.RunResult[workflows.PendingEntryState], error) {
	f.startCalled = true
	f.lastKey = key
	f.lastState = state
	return f.startResult, f.startErr
}

func (f *fakeEngine) Resume(_ context.Context, _ wf.Definition[workflows.PendingEntryState], _ string, _ []byte) (wf.RunResult[workflows.PendingEntryState], error) {
	return wf.RunResult[workflows.PendingEntryState]{}, nil
}

type RegisterAttemptSuite struct {
	suite.Suite
	ctx        context.Context
	obs        *fake.Provider
	categories *imocks.CategoriesReader
	ledger     *imocks.TransactionsLedger
	engine     *fakeEngine
	uc         *RegisterAttempt
	userID     uuid.UUID
	catRootID  uuid.UUID
	catSubID   uuid.UUID
}

func TestRegisterAttemptSuite(t *testing.T) {
	suite.Run(t, new(RegisterAttemptSuite))
}

func (s *RegisterAttemptSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.categories = imocks.NewCategoriesReader(s.T())
	s.engine = &fakeEngine{startResult: wf.RunResult[workflows.PendingEntryState]{
		Status:  wf.RunStatusSuspended,
		Suspend: &wf.Suspension{Prompt: "Confirma? R$ 150,00 em *Custo Fixo > Aluguel* para hoje no pix?"},
	}}
	s.userID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	s.catRootID = uuid.New()
	s.catSubID = uuid.New()

	def := workflows.BuildPendingEntryWorkflow(nil, nil, nil, nil)
	s.ledger = imocks.NewTransactionsLedger(s.T())
	s.uc = NewRegisterAttempt(s.categories, s.ledger, s.engine, def, s.obs)
}

func (s *RegisterAttemptSuite) expenseCmd(paymentMethod string, cardID *uuid.UUID, subcategoryID uuid.UUID) RegisterExpenseCommand {
	return RegisterExpenseCommand{
		UserID:        s.userID,
		ThreadID:      "thr-001",
		WAMID:         "wamid-001",
		AmountCents:   15000,
		Description:   "mercado",
		PaymentMethod: paymentMethod,
		CardID:        cardID,
		Installments:  1,
		SubcategoryID: subcategoryID,
		CategoryID:    s.catRootID,
	}
}

func (s *RegisterAttemptSuite) TestRegisterExpense_ClassifyClarify_EngineStartCalled_G7_20() {
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "mercado", "expense").
		Return(interfaces.CategorySearchResult{
			Outcome:    interfaces.ClassifyOutcomeAmbiguous,
			Version:    1,
			Candidates: []interfaces.CategoryCandidate{},
		}, nil).
		Once()

	result, err := s.uc.RegisterExpense(s.ctx, s.expenseCmd("pix", nil, uuid.Nil))

	s.NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.True(s.engine.startCalled)
	s.Equal(workflows.AwaitingSlotCategory, s.engine.lastState.Awaiting)
}

func (s *RegisterAttemptSuite) TestRegisterExpense_ClassifyResolved_OpensConfirmation_CA13() {
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "mercado", "expense").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []interfaces.CategoryCandidate{
				{RootCategoryID: s.catRootID, CategoryID: s.catSubID, Confidence: "high", MatchQuality: "exact"},
			},
		}, nil).
		Once()
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.AnythingOfType("interfaces.CategoryWriteRequest")).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:   s.catRootID,
			SubcategoryID:    s.catSubID,
			EditorialVersion: 1,
			Path:             "Custo Fixo > Supermercado",
		}, nil).
		Once()

	result, err := s.uc.RegisterExpense(s.ctx, s.expenseCmd("pix", nil, uuid.Nil))

	s.NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.True(s.engine.startCalled)
	s.Equal(workflows.AwaitingSlotConfirmation, s.engine.lastState.Awaiting)
	s.Len(s.engine.lastState.Candidates, 1)
	s.Contains(result.Message, "Confirma?", "o prompt de confirmação deve ser devolvido ao chamador para relay ao usuário")
}

func (s *RegisterAttemptSuite) TestRegisterExpense_CreditCardNoCardID_AwaitingCard_CA10() {
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "mercado", "expense").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []interfaces.CategoryCandidate{
				{RootCategoryID: s.catRootID, CategoryID: s.catSubID, Confidence: "high", MatchQuality: "exact"},
			},
		}, nil).
		Once()
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.AnythingOfType("interfaces.CategoryWriteRequest")).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:   s.catRootID,
			SubcategoryID:    s.catSubID,
			EditorialVersion: 1,
		}, nil).
		Once()

	result, err := s.uc.RegisterExpense(s.ctx, s.expenseCmd("credit_card", nil, uuid.Nil))

	s.NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.True(s.engine.startCalled)
	s.Equal(workflows.AwaitingSlotCard, s.engine.lastState.Awaiting)
}

func (s *RegisterAttemptSuite) TestRegisterExpense_EngineStartFails_ReturnsError() {
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, mock.Anything, mock.Anything).
		Return(interfaces.CategorySearchResult{Outcome: interfaces.ClassifyOutcomeNoMatch, Version: 1}, nil).
		Once()
	s.engine.startErr = errors.New("store unavailable")

	_, err := s.uc.RegisterExpense(s.ctx, s.expenseCmd("pix", nil, uuid.Nil))

	s.Error(err)
	s.True(s.engine.startCalled)
}

func (s *RegisterAttemptSuite) TestRegisterExpense_ExplicitCategory_OpensConfirmation_CA13() {
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.MatchedBy(func(req interfaces.CategoryWriteRequest) bool {
			return req.SubcategoryID == s.catSubID
		})).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:   s.catRootID,
			SubcategoryID:    s.catSubID,
			EditorialVersion: 2,
			Path:             "Custo Fixo > Supermercado",
		}, nil).
		Once()

	result, err := s.uc.RegisterExpense(s.ctx, s.expenseCmd("pix", nil, s.catSubID))

	s.NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.True(s.engine.startCalled)
	s.Equal(workflows.AwaitingSlotConfirmation, s.engine.lastState.Awaiting)
}

func (s *RegisterAttemptSuite) TestRegisterIncome_ClassifyClarify_EngineStartCalled() {
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "salário", "income").
		Return(interfaces.CategorySearchResult{
			Outcome:    interfaces.ClassifyOutcomeNoMatch,
			Version:    1,
			Candidates: []interfaces.CategoryCandidate{},
		}, nil).
		Once()

	incomeCmd := RegisterIncomeCommand{
		UserID:      s.userID,
		ThreadID:    "thr-001",
		WAMID:       "wamid-001",
		AmountCents: 500000,
		Description: "salário",
	}

	result, err := s.uc.RegisterIncome(s.ctx, incomeCmd)

	s.NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.True(s.engine.startCalled)
	s.Equal(workflows.AwaitingSlotCategory, s.engine.lastState.Awaiting)
	s.Equal(workflows.PendingOpRegisterIncome, s.engine.lastState.OperationKind)
}

func (s *RegisterAttemptSuite) TestCreateRecurrence_OpensPendingConfirmation_NoSyncWrite_CA16() {
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.MatchedBy(func(req interfaces.CategoryWriteRequest) bool {
			return req.SubcategoryID == s.catSubID
		})).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:   s.catRootID,
			SubcategoryID:    s.catSubID,
			EditorialVersion: 1,
			Path:             "Custo Fixo > Aluguel",
		}, nil).
		Once()

	cmd := CreateRecurrenceCommand{
		UserID:        s.userID,
		ThreadID:      "thr-001",
		WAMID:         "wamid-recur",
		Direction:     "outcome",
		PaymentMethod: "debit_card",
		AmountCents:   150000,
		Description:   "aluguel",
		CategoryID:    s.catRootID,
		SubcategoryID: s.catSubID,
		Frequency:     "monthly",
		DayOfMonth:    5,
	}

	result, err := s.uc.CreateRecurrence(s.ctx, cmd)

	s.NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.True(s.engine.startCalled)
	s.Equal(workflows.PendingOpCreateRecurrence, s.engine.lastState.OperationKind)
	s.Equal(workflows.AwaitingSlotConfirmation, s.engine.lastState.Awaiting)
	s.Equal("monthly", s.engine.lastState.Frequency)
	s.Equal(5, s.engine.lastState.RecurrenceDayOfMonth)
	s.ledger.AssertNotCalled(s.T(), "CreateRecurringTemplate", mock.Anything, mock.Anything)
}

func (s *RegisterAttemptSuite) TestEditEntry_PreservesKindAndTarget_CA17() {
	targetID := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	subStr := s.catSubID.String()

	s.ledger.EXPECT().
		GetTransaction(mock.Anything, targetID.String()).
		Return(interfaces.Entry{
			ID:            targetID.String(),
			Direction:     "income",
			PaymentMethod: "pix",
			AmountCents:   500000,
			Description:   "salário",
			CategoryID:    s.catRootID.String(),
			SubcategoryID: &subStr,
			Version:       3,
		}, nil).
		Once()
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "salário", "income").
		Return(interfaces.CategorySearchResult{Outcome: interfaces.ClassifyOutcomeMatched, Version: 7}, nil).
		Once()
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.MatchedBy(func(req interfaces.CategoryWriteRequest) bool {
			return req.SubcategoryID == s.catSubID && req.Kind == interfaces.CategoryKindIncome && req.ExpectedVersion == 7
		})).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:   s.catRootID,
			SubcategoryID:    s.catSubID,
			EditorialVersion: 7,
			Path:             "Receitas > Salário",
		}, nil).
		Once()

	cmd := EditEntryCommand{
		UserID:              s.userID,
		ThreadID:            "thr-001",
		WAMID:               "wamid-edit",
		TargetTransactionID: targetID,
		AmountCents:         600000,
	}

	result, err := s.uc.EditEntry(s.ctx, cmd)

	s.NoError(err)
	s.Equal(agent.ToolOutcomeClarify, result.Outcome)
	s.True(s.engine.startCalled)
	s.Equal(workflows.PendingOpEditEntry, s.engine.lastState.OperationKind)
	s.Equal(interfaces.CategoryKindIncome, s.engine.lastState.Kind)
	s.Require().NotNil(s.engine.lastState.TargetTransactionID)
	s.Equal(targetID, *s.engine.lastState.TargetTransactionID)
	s.Equal(int64(3), s.engine.lastState.TargetVersion)
	s.Equal(int64(600000), s.engine.lastState.AmountCents)
}

func (s *RegisterAttemptSuite) TestRegisterExpense_WorkflowKey_ContainsResourceAndThread() {
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, mock.Anything, mock.Anything).
		Return(interfaces.CategorySearchResult{Outcome: interfaces.ClassifyOutcomeNoMatch, Version: 1}, nil).
		Once()

	_, err := s.uc.RegisterExpense(s.ctx, s.expenseCmd("pix", nil, uuid.Nil))

	s.NoError(err)
	s.Contains(s.engine.lastKey, s.userID.String())
	s.Contains(s.engine.lastKey, "thr-001")
	s.Contains(s.engine.lastKey, "pending-entry")
}

func (s *RegisterAttemptSuite) TestRegisterExpense_ItemSeq_PropagatedToState() {
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, mock.Anything, mock.Anything).
		Return(interfaces.CategorySearchResult{Outcome: interfaces.ClassifyOutcomeNoMatch, Version: 1}, nil).
		Once()

	cmd := s.expenseCmd("pix", nil, uuid.Nil)
	cmd.ItemSeq = 3

	_, err := s.uc.RegisterExpense(s.ctx, cmd)

	s.NoError(err)
	s.Equal(3, s.engine.lastState.ItemSeq)
}

func (s *RegisterAttemptSuite) TestRegisterIncome_ItemSeq_PropagatedToState() {
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, mock.Anything, mock.Anything).
		Return(interfaces.CategorySearchResult{Outcome: interfaces.ClassifyOutcomeNoMatch, Version: 1}, nil).
		Once()

	cmd := RegisterIncomeCommand{
		UserID:      s.userID,
		ThreadID:    "thr-001",
		WAMID:       "wamid-001",
		ItemSeq:     7,
		AmountCents: 500000,
		Description: "salário",
	}

	_, err := s.uc.RegisterIncome(s.ctx, cmd)

	s.NoError(err)
	s.Equal(7, s.engine.lastState.ItemSeq)
}

func (s *RegisterAttemptSuite) TestCreateRecurrence_ItemSeq_PropagatedToState() {
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.MatchedBy(func(req interfaces.CategoryWriteRequest) bool {
			return req.SubcategoryID == s.catSubID
		})).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:   s.catRootID,
			SubcategoryID:    s.catSubID,
			EditorialVersion: 1,
			Path:             "Custo Fixo > Aluguel",
		}, nil).
		Once()

	cmd := CreateRecurrenceCommand{
		UserID:        s.userID,
		ThreadID:      "thr-001",
		WAMID:         "wamid-recur",
		ItemSeq:       2,
		Direction:     "outcome",
		PaymentMethod: "debit_card",
		AmountCents:   150000,
		Description:   "aluguel",
		CategoryID:    s.catRootID,
		SubcategoryID: s.catSubID,
		Frequency:     "monthly",
		DayOfMonth:    5,
	}

	_, err := s.uc.CreateRecurrence(s.ctx, cmd)

	s.NoError(err)
	s.Equal(2, s.engine.lastState.ItemSeq)
}

func (s *RegisterAttemptSuite) TestEditEntry_ItemSeq_PropagatedToState() {
	targetID := uuid.MustParse("00000000-0000-0000-0000-0000000000bb")
	subStr := s.catSubID.String()

	s.ledger.EXPECT().
		GetTransaction(mock.Anything, targetID.String()).
		Return(interfaces.Entry{
			ID:            targetID.String(),
			Direction:     "outcome",
			PaymentMethod: "pix",
			AmountCents:   10000,
			Description:   "mercado",
			CategoryID:    s.catRootID.String(),
			SubcategoryID: &subStr,
			Version:       1,
		}, nil).
		Once()
	s.categories.EXPECT().
		SearchDictionary(mock.Anything, "mercado", "expense").
		Return(interfaces.CategorySearchResult{Outcome: interfaces.ClassifyOutcomeMatched, Version: 1}, nil).
		Once()
	s.categories.EXPECT().
		ResolveForWrite(mock.Anything, mock.MatchedBy(func(req interfaces.CategoryWriteRequest) bool {
			return req.SubcategoryID == s.catSubID
		})).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:   s.catRootID,
			SubcategoryID:    s.catSubID,
			EditorialVersion: 1,
			Path:             "Custo Fixo > Mercado",
		}, nil).
		Once()

	cmd := EditEntryCommand{
		UserID:              s.userID,
		ThreadID:            "thr-001",
		WAMID:               "wamid-edit",
		ItemSeq:             5,
		TargetTransactionID: targetID,
	}

	_, err := s.uc.EditEntry(s.ctx, cmd)

	s.NoError(err)
	s.Equal(5, s.engine.lastState.ItemSeq)
}

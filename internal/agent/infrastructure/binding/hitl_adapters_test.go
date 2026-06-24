package binding

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type hitlAdaptersSuite struct {
	suite.Suite
	ctx context.Context
}

func TestHITLAdaptersSuite(t *testing.T) {
	suite.Run(t, new(hitlAdaptersSuite))
}

func (s *hitlAdaptersSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *hitlAdaptersSuite) TestLastTransactionEditorExecutor_UsesTargetFromState() {
	var captured tools.EditTransactionInput
	fakeEditor := &fakeLastTransactionEditor{
		fn: func(_ context.Context, in tools.EditTransactionInput) (tools.EditTransactionResult, error) {
			captured = in
			return tools.EditTransactionResult{}, nil
		},
	}

	executor := NewLastTransactionEditorExecutor(fakeEditor)
	state := confirmation.ConfirmState{
		UserID:                   uuid.New().String(),
		OperationKind:            confirmation.OperationEditLast,
		NewAmountCents:           9999,
		TargetTransactionID:      "tx-1",
		TargetTransactionVersion: 7,
	}

	_, err := executor(s.ctx, state)
	s.NoError(err)
	s.Equal(int64(9999), captured.NewAmount)
	s.Equal("tx-1", captured.Current.ID)
	s.Equal(int64(7), captured.Current.Version)
}

func (s *hitlAdaptersSuite) TestLastTransactionEditorExecutor_RequiresTarget() {
	fakeEditor := &fakeLastTransactionEditor{
		fn: func(_ context.Context, _ tools.EditTransactionInput) (tools.EditTransactionResult, error) {
			return tools.EditTransactionResult{}, nil
		},
	}

	executor := NewLastTransactionEditorExecutor(fakeEditor)
	state := confirmation.ConfirmState{
		UserID:        uuid.New().String(),
		OperationKind: confirmation.OperationEditLast,
	}

	_, err := executor(s.ctx, state)
	s.Error(err)
}

func (s *hitlAdaptersSuite) TestLastTransactionDeleterExecutor_RequiresTarget() {
	fakeDeleter := &fakeLastTransactionDeleter{
		fn: func(_ context.Context, _ string, _ string, _ int64) error {
			return nil
		},
	}

	executor := NewLastTransactionDeleterExecutor(fakeDeleter)
	state := confirmation.ConfirmState{
		UserID:        uuid.New().String(),
		OperationKind: confirmation.OperationDeleteLast,
	}

	_, err := executor(s.ctx, state)
	s.Error(err)
}

func (s *hitlAdaptersSuite) TestLastTransactionDeleterExecutor_UsesTargetFromState() {
	var capturedID string
	var capturedVersion int64
	fakeDeleter := &fakeLastTransactionDeleter{
		fn: func(_ context.Context, _ string, id string, version int64) error {
			capturedID = id
			capturedVersion = version
			return nil
		},
	}

	executor := NewLastTransactionDeleterExecutor(fakeDeleter)
	state := confirmation.ConfirmState{
		UserID:                   uuid.New().String(),
		OperationKind:            confirmation.OperationDeleteLast,
		TargetTransactionID:      "tx-1",
		TargetTransactionVersion: 7,
	}

	_, err := executor(s.ctx, state)
	s.NoError(err)
	s.Equal("tx-1", capturedID)
	s.Equal(int64(7), capturedVersion)
}

func (s *hitlAdaptersSuite) TestCardDeleterExecutor_UsesCardNameFromState() {
	var capturedName string
	fakeDeleter := &fakeCardDeleter{
		fn: func(_ context.Context, _ uuid.UUID, name string) (tools.CardDeleterResult, error) {
			capturedName = name
			return tools.CardDeleterResult{}, nil
		},
	}

	executor := NewCardDeleterExecutorFn(fakeDeleter)
	state := confirmation.ConfirmState{
		UserID:        uuid.New().String(),
		OperationKind: confirmation.OperationDeleteCard,
		CardName:      "Nubank",
	}

	_, err := executor(s.ctx, state)
	s.NoError(err)
	s.Equal("Nubank", capturedName)
}

func (s *hitlAdaptersSuite) TestBudgetCommitExecutor_UsesDraftFromState() {
	var captured budgetdraft.Draft
	fakeCommitter := &fakeBudgetConfigCommitter{
		fn: func(_ context.Context, _ uuid.UUID, draft budgetdraft.Draft) (string, error) {
			captured = draft
			return "ativado", nil
		},
	}

	executor := NewBudgetCommitExecutor(fakeCommitter)
	draft, err := budgetdraft.Restore(100000, map[string]int{
		budgetdraft.SlugCustoFixo:           3000,
		budgetdraft.SlugConhecimento:        2000,
		budgetdraft.SlugPrazeres:            2000,
		budgetdraft.SlugMetas:               1500,
		budgetdraft.SlugLiberdadeFinanceira: 1500,
	}, "2026-06")
	s.Require().NoError(err)

	state := confirmation.ConfirmState{
		UserID:        uuid.New().String(),
		OperationKind: confirmation.OperationBudgetCommit,
	}
	s.NoError(state.SetBudgetDraft(draft))

	_, err = executor(s.ctx, state)
	s.NoError(err)
	s.Equal(draft.TotalCents(), captured.TotalCents())
	s.Equal(draft.Competence(), captured.Competence())
	s.Equal(draft.Allocations(), captured.Allocations())
}

func (s *hitlAdaptersSuite) TestCardDeleterResolver_MissingCardShortCircuits() {
	fakeLister := &fakeCardLister{
		result: cardoutput.CardList{Items: []cardoutput.Card{}},
	}

	resolver := NewCardDeleterResolver(fakeLister)
	state := confirmation.ConfirmState{
		UserID:        uuid.New().String(),
		OperationKind: confirmation.OperationDeleteCard,
		CardName:      "Inexistente",
	}

	result, err := resolver(s.ctx, state)
	s.NoError(err)
	s.True(result.ShortCircuit)
	s.NotEmpty(result.Reply)
}

func (s *hitlAdaptersSuite) TestCardDeleterResolver_FoundCardComposesPrompt() {
	fakeLister := &fakeCardLister{
		result: cardoutput.CardList{
			Items: []cardoutput.Card{
				{ID: uuid.New().String(), Name: "Nubank", Nickname: "Nubank Black"},
			},
		},
	}

	resolver := NewCardDeleterResolver(fakeLister)
	state := confirmation.ConfirmState{
		UserID:        uuid.New().String(),
		OperationKind: confirmation.OperationDeleteCard,
		CardName:      "nubank",
	}

	result, err := resolver(s.ctx, state)
	s.NoError(err)
	s.False(result.ShortCircuit)
	s.Contains(result.PromptText, "Nubank Black")
}

type fakeLastTransactionEditor struct {
	fn func(context.Context, tools.EditTransactionInput) (tools.EditTransactionResult, error)
}

func (f *fakeLastTransactionEditor) Execute(ctx context.Context, in tools.EditTransactionInput) (tools.EditTransactionResult, error) {
	return f.fn(ctx, in)
}

type fakeLastTransactionDeleter struct {
	fn func(context.Context, string, string, int64) error
}

func (f *fakeLastTransactionDeleter) Execute(ctx context.Context, userID, txID string, version int64) error {
	return f.fn(ctx, userID, txID, version)
}

type fakeCardDeleter struct {
	fn func(context.Context, uuid.UUID, string) (tools.CardDeleterResult, error)
}

func (f *fakeCardDeleter) Execute(ctx context.Context, userID uuid.UUID, cardName string) (tools.CardDeleterResult, error) {
	return f.fn(ctx, userID, cardName)
}

type fakeBudgetConfigCommitter struct {
	fn func(context.Context, uuid.UUID, budgetdraft.Draft) (string, error)
}

func (f *fakeBudgetConfigCommitter) Commit(ctx context.Context, userID uuid.UUID, draft budgetdraft.Draft) (string, error) {
	return f.fn(ctx, userID, draft)
}

type fakeCardLister struct {
	result cardoutput.CardList
	err    error
}

func (f *fakeCardLister) Execute(_ context.Context, _ cardinput.ListCards) (cardoutput.CardList, error) {
	return f.result, f.err
}

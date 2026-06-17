package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	categoriesinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
)

type fakeResolver struct {
	out *categoriesoutput.DictionarySearchOutput
	err error
}

func (f *fakeResolver) Execute(_ context.Context, _ *categoriesinput.SearchDictionaryInput) (*categoriesoutput.DictionarySearchOutput, error) {
	return f.out, f.err
}

type fakeCreator struct {
	called bool
	in     usecases.CreateTransactionCommand
	result usecases.CreateTransactionResult
	err    error
}

func (f *fakeCreator) Execute(_ context.Context, in usecases.CreateTransactionCommand) (usecases.CreateTransactionResult, error) {
	f.called = true
	f.in = in
	return f.result, f.err
}

type LogTransactionSuite struct {
	suite.Suite
}

func TestLogTransactionSuite(t *testing.T) {
	suite.Run(t, new(LogTransactionSuite))
}

func (s *LogTransactionSuite) expenseIntent(amount int64, hint, merchant string) intent.Intent {
	in, err := intent.NewLogExpense(intent.LogExpenseFields{
		AmountCents:  amount,
		Merchant:     merchant,
		CategoryHint: hint,
	})
	s.Require().NoError(err)
	return in
}

func (s *LogTransactionSuite) incomeIntent(amount int64, source string) intent.Intent {
	in, err := intent.NewLogIncome(intent.LogIncomeFields{
		AmountCents: amount,
		Source:      source,
	})
	s.Require().NoError(err)
	return in
}

func (s *LogTransactionSuite) candidates(path string) *categoriesoutput.DictionarySearchOutput {
	return &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Path: path},
		},
	}
}

func (s *LogTransactionSuite) TestInvalidKindRejected() {
	uc := usecases.NewLogTransactionFromAgent(&fakeResolver{}, &fakeCreator{}, noop.NewProvider())
	unknown, err := intent.NewUnknown("oi")
	s.Require().NoError(err)
	_, err = uc.Execute(context.Background(), usecases.LogTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: unknown,
	})
	s.Require().ErrorIs(err, usecases.ErrLogTransactionInvalidIntent)
}

func (s *LogTransactionSuite) TestExpenseNoHintReturnsError() {
	uc := usecases.NewLogTransactionFromAgent(&fakeResolver{}, &fakeCreator{}, noop.NewProvider())
	_, err := uc.Execute(context.Background(), usecases.LogTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "", ""),
	})
	s.Require().ErrorIs(err, usecases.ErrLogTransactionNoCategoryHint)
}

func (s *LogTransactionSuite) TestNoCandidateReturnsNotFound() {
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{Candidates: nil}}
	uc := usecases.NewLogTransactionFromAgent(resolver, &fakeCreator{}, noop.NewProvider())
	_, err := uc.Execute(context.Background(), usecases.LogTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "ifood", ""),
	})
	s.Require().ErrorIs(err, usecases.ErrLogTransactionCategoryNotFound)
}

func (s *LogTransactionSuite) TestAmbiguousReturnsAmbiguous() {
	rootID := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Prazeres", IsAmbiguous: true},
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Custo Fixo"},
		},
	}}
	uc := usecases.NewLogTransactionFromAgent(resolver, &fakeCreator{}, noop.NewProvider())
	_, err := uc.Execute(context.Background(), usecases.LogTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "ifood", ""),
	})
	s.Require().ErrorIs(err, usecases.ErrLogTransactionCategoryAmbiguous)
}

func (s *LogTransactionSuite) TestCreateFailurePropagates() {
	resolver := &fakeResolver{out: s.candidates("Prazeres")}
	boom := errors.New("create exploded")
	creator := &fakeCreator{err: boom}
	uc := usecases.NewLogTransactionFromAgent(resolver, creator, noop.NewProvider())
	_, err := uc.Execute(context.Background(), usecases.LogTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "ifood", "iFood"),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, boom)
	s.True(creator.called)
}

func (s *LogTransactionSuite) TestExpenseHappyPathCreatesOutcome() {
	resolver := &fakeResolver{out: s.candidates("Prazeres > Delivery")}
	creator := &fakeCreator{result: usecases.CreateTransactionResult{AmountCents: 5800, Direction: "expense"}}
	uc := usecases.NewLogTransactionFromAgent(resolver, creator, noop.NewProvider())
	out, err := uc.Execute(context.Background(), usecases.LogTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "ifood", "iFood"),
	})
	s.Require().NoError(err)
	s.True(out.Persisted)
	s.Equal(int64(5800), out.AmountCents)
	s.Equal("Prazeres > Delivery", out.CategoryPath)
	s.True(creator.called)
	s.Equal("outcome", creator.in.Direction)
	s.Equal("iFood", creator.in.Description)
	s.NotEmpty(creator.in.RootCategoryID)
}

func (s *LogTransactionSuite) TestIncomeHappyPathCreatesIncome() {
	resolver := &fakeResolver{out: s.candidates("Salário")}
	creator := &fakeCreator{result: usecases.CreateTransactionResult{AmountCents: 1640000, Direction: "income"}}
	uc := usecases.NewLogTransactionFromAgent(resolver, creator, noop.NewProvider())
	out, err := uc.Execute(context.Background(), usecases.LogTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.incomeIntent(1640000, "salário"),
	})
	s.Require().NoError(err)
	s.True(out.Persisted)
	s.Equal("income", creator.in.Direction)
	s.Equal("ted", creator.in.PaymentMethod)
}

func (s *LogTransactionSuite) TestIncomeWithoutHintUsesDefault() {
	resolver := &fakeResolver{out: s.candidates("Salário")}
	creator := &fakeCreator{result: usecases.CreateTransactionResult{AmountCents: 300000, Direction: "income"}}
	uc := usecases.NewLogTransactionFromAgent(resolver, creator, noop.NewProvider())
	out, err := uc.Execute(context.Background(), usecases.LogTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.incomeIntent(300000, ""),
	})
	s.Require().NoError(err)
	s.True(out.Persisted)
	s.True(creator.called)
}

func (s *LogTransactionSuite) TestExpenseFallsBackToMerchantWhenHintEmpty() {
	resolver := &fakeResolver{out: s.candidates("Prazeres")}
	creator := &fakeCreator{result: usecases.CreateTransactionResult{AmountCents: 5800, Direction: "expense"}}
	uc := usecases.NewLogTransactionFromAgent(resolver, creator, noop.NewProvider())
	out, err := uc.Execute(context.Background(), usecases.LogTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "", "iFood"),
	})
	s.Require().NoError(err)
	s.True(out.Persisted)
}

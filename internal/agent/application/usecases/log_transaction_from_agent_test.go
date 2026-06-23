package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

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
	in     CreateTransactionCommand
	result CreateTransactionResult
	err    error
}

func (f *fakeCreator) Execute(_ context.Context, in CreateTransactionCommand) (CreateTransactionResult, error) {
	f.called = true
	f.in = in
	return f.result, f.err
}

type LogTransactionSuite struct {
	suite.Suite
	ctx context.Context
}

func TestLogTransactionSuite(t *testing.T) {
	suite.Run(t, new(LogTransactionSuite))
}

func (s *LogTransactionSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *LogTransactionSuite) expenseIntent(amount int64, hint, merchant string) intent.Intent {
	in, err := intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents:  amount,
		Merchant:     merchant,
		CategoryHint: hint,
	})
	s.Require().NoError(err)
	return in
}

func (s *LogTransactionSuite) incomeIntent(amount int64, source string) intent.Intent {
	in, err := intent.NewRecordIncome(intent.RecordIncomeFields{
		AmountCents: amount,
		Source:      source,
	})
	s.Require().NoError(err)
	return in
}

func (s *LogTransactionSuite) candidates(path string) *categoriesoutput.DictionarySearchOutput {
	return &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Path: path, Score: 0.95},
		},
	}
}

func (s *LogTransactionSuite) TestInvalidKindRejected() {
	uc := NewRecordTransactionFromAgent(&fakeResolver{}, &fakeCreator{}, fake.NewProvider())
	unknown, err := intent.NewUnknown("oi")
	s.Require().NoError(err)
	_, err = uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: unknown,
	})
	s.Require().ErrorIs(err, ErrLogTransactionInvalidIntent)
}

func (s *LogTransactionSuite) TestExpenseNoHintReturnsError() {
	uc := NewRecordTransactionFromAgent(&fakeResolver{}, &fakeCreator{}, fake.NewProvider())
	_, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "", ""),
	})
	s.Require().ErrorIs(err, ErrLogTransactionNoCategoryHint)
}

func (s *LogTransactionSuite) TestNoCandidateReturnsNotFound() {
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{Candidates: nil}}
	uc := NewRecordTransactionFromAgent(resolver, &fakeCreator{}, fake.NewProvider())
	_, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "ifood", ""),
	})
	s.Require().ErrorIs(err, ErrLogTransactionCategoryNotFound)
}

func (s *LogTransactionSuite) TestMediumScoreNeedsConfirmation() {
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Path: "Prazeres > Streaming", Score: 0.65},
		},
	}}
	uc := NewRecordTransactionFromAgent(resolver, &fakeCreator{}, fake.NewProvider())
	_, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "netflix", ""),
	})
	s.Require().ErrorIs(err, ErrLogTransactionCategoryNeedsConfirmation)

	var needsConfirmation *CategoryNeedsConfirmationError
	s.Require().ErrorAs(err, &needsConfirmation)
	s.Equal("netflix", needsConfirmation.Hint)
	s.Equal([]string{"Prazeres > Streaming"}, needsConfirmation.Candidates)
}

func (s *LogTransactionSuite) TestLowScoreReturnsNotFound() {
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Path: "Prazeres", Score: 0.40},
		},
	}}
	uc := NewRecordTransactionFromAgent(resolver, &fakeCreator{}, fake.NewProvider())
	_, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "xpto", ""),
	})
	s.Require().ErrorIs(err, ErrLogTransactionCategoryNotFound)
}

func (s *LogTransactionSuite) TestAmbiguousReturnsAmbiguous() {
	rootID := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Prazeres", IsAmbiguous: true},
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Custo Fixo"},
		},
	}}
	uc := NewRecordTransactionFromAgent(resolver, &fakeCreator{}, fake.NewProvider())
	_, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "ifood", ""),
	})
	s.Require().ErrorIs(err, ErrLogTransactionCategoryAmbiguous)
}

func (s *LogTransactionSuite) TestAmbiguousCarriesCandidates() {
	rootID := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Prazeres", IsAmbiguous: true},
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Custo Fixo"},
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Conhecimento"},
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Metas"},
		},
	}}
	uc := NewRecordTransactionFromAgent(resolver, &fakeCreator{}, fake.NewProvider())
	_, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "ifood", ""),
	})
	s.Require().ErrorIs(err, ErrLogTransactionCategoryAmbiguous)

	var ambiguous *CategoryAmbiguousError
	s.Require().ErrorAs(err, &ambiguous)
	s.Equal("ifood", ambiguous.Hint)
	s.Equal([]string{"Prazeres", "Custo Fixo", "Conhecimento"}, ambiguous.Candidates)
}

func (s *LogTransactionSuite) TestCategoryAmbiguousErrorUnwrap() {
	err := newCategoryAmbiguousError("ifood", []categoriesoutput.CandidateOutput{
		{Path: "Prazeres"},
		{Path: " "},
	})
	s.Require().ErrorIs(err, ErrLogTransactionCategoryAmbiguous)
	s.Equal([]string{"Prazeres"}, err.Candidates)
	s.Contains(err.Error(), "ifood")
}

func (s *LogTransactionSuite) TestCreateFailurePropagates() {
	resolver := &fakeResolver{out: s.candidates("Prazeres")}
	boom := errors.New("create exploded")
	creator := &fakeCreator{err: boom}
	uc := NewRecordTransactionFromAgent(resolver, creator, fake.NewProvider())
	_, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "ifood", "iFood"),
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, boom)
	s.True(creator.called)
}

func (s *LogTransactionSuite) TestExpenseHappyPathCreatesOutcome() {
	resolver := &fakeResolver{out: s.candidates("Prazeres > Delivery")}
	creator := &fakeCreator{result: CreateTransactionResult{AmountCents: 5800, Direction: "expense"}}
	uc := NewRecordTransactionFromAgent(resolver, creator, fake.NewProvider())
	out, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
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
	creator := &fakeCreator{result: CreateTransactionResult{AmountCents: 1640000, Direction: "income"}}
	uc := NewRecordTransactionFromAgent(resolver, creator, fake.NewProvider())
	out, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
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
	creator := &fakeCreator{result: CreateTransactionResult{AmountCents: 300000, Direction: "income"}}
	uc := NewRecordTransactionFromAgent(resolver, creator, fake.NewProvider())
	out, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.incomeIntent(300000, ""),
	})
	s.Require().NoError(err)
	s.True(out.Persisted)
	s.True(creator.called)
}

func (s *LogTransactionSuite) TestExpenseFallsBackToMerchantWhenHintEmpty() {
	resolver := &fakeResolver{out: s.candidates("Prazeres")}
	creator := &fakeCreator{result: CreateTransactionResult{AmountCents: 5800, Direction: "expense"}}
	uc := NewRecordTransactionFromAgent(resolver, creator, fake.NewProvider())
	out, err := uc.Execute(s.ctx, RecordTransactionFromAgentInput{
		UserID: uuid.NewString(),
		Intent: s.expenseIntent(5800, "", "iFood"),
	})
	s.Require().NoError(err)
	s.True(out.Persisted)
}

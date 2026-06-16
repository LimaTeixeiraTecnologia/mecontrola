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

type fakeUpserter struct {
	called bool
	in     usecases.ExpenseUpsertInput
	result usecases.ExpenseUpsertResult
	err    error
}

func (f *fakeUpserter) Execute(_ context.Context, in usecases.ExpenseUpsertInput) (usecases.ExpenseUpsertResult, error) {
	f.called = true
	f.in = in
	return f.result, f.err
}

type LogExpenseSuite struct {
	suite.Suite
}

func TestLogExpenseSuite(t *testing.T) {
	suite.Run(t, new(LogExpenseSuite))
}

func (s *LogExpenseSuite) buildIntent(amount int64, hint, merchant string) intent.Intent {
	in, err := intent.NewLogExpense(intent.LogExpenseFields{
		AmountCents:  amount,
		Merchant:     merchant,
		CategoryHint: hint,
	})
	s.Require().NoError(err)
	return in
}

func (s *LogExpenseSuite) TestInvalidKindRejected() {
	uc := usecases.NewLogExpenseFromAgent(&fakeResolver{}, &fakeUpserter{}, nil, noop.NewProvider())
	unknown, err := intent.NewUnknown("oi")
	s.Require().NoError(err)
	_, err = uc.Execute(context.Background(), usecases.LogExpenseFromAgentInput{
		UserID: uuid.NewString(),
		Intent: unknown,
	})
	s.Require().ErrorIs(err, usecases.ErrLogExpenseInvalidIntent)
}

func (s *LogExpenseSuite) TestNoHintReturnsError() {
	uc := usecases.NewLogExpenseFromAgent(&fakeResolver{}, &fakeUpserter{}, nil, noop.NewProvider())
	in := s.buildIntent(5800, "", "")
	_, err := uc.Execute(context.Background(), usecases.LogExpenseFromAgentInput{
		UserID: uuid.NewString(),
		Intent: in,
	})
	s.Require().ErrorIs(err, usecases.ErrLogExpenseNoCategoryHint)
}

func (s *LogExpenseSuite) TestNoCandidateReturnsNotFound() {
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{Candidates: nil}}
	uc := usecases.NewLogExpenseFromAgent(resolver, &fakeUpserter{}, nil, noop.NewProvider())
	in := s.buildIntent(5800, "ifood", "")
	_, err := uc.Execute(context.Background(), usecases.LogExpenseFromAgentInput{
		UserID: uuid.NewString(),
		Intent: in,
	})
	s.Require().ErrorIs(err, usecases.ErrLogExpenseCategoryNotFound)
}

func (s *LogExpenseSuite) TestAmbiguousReturnsAmbiguous() {
	categoryID := uuid.New()
	rootID := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: categoryID, RootCategoryID: rootID, Path: "Prazeres", IsAmbiguous: true},
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Custo Fixo"},
		},
	}}
	uc := usecases.NewLogExpenseFromAgent(resolver, &fakeUpserter{}, nil, noop.NewProvider())
	in := s.buildIntent(5800, "ifood", "")
	_, err := uc.Execute(context.Background(), usecases.LogExpenseFromAgentInput{
		UserID: uuid.NewString(),
		Intent: in,
	})
	s.Require().ErrorIs(err, usecases.ErrLogExpenseCategoryAmbiguous)
}

func (s *LogExpenseSuite) TestUpsertFailurePropagates() {
	categoryID := uuid.New()
	rootID := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: categoryID, RootCategoryID: rootID, Path: "Prazeres"},
		},
	}}
	boom := errors.New("upsert exploded")
	upserter := &fakeUpserter{err: boom}
	uc := usecases.NewLogExpenseFromAgent(resolver, upserter, nil, noop.NewProvider())
	in := s.buildIntent(5800, "ifood", "iFood")
	_, err := uc.Execute(context.Background(), usecases.LogExpenseFromAgentInput{
		UserID: uuid.NewString(),
		Intent: in,
	})
	s.Require().Error(err)
	s.Require().ErrorIs(err, boom)
	s.True(upserter.called)
}

func (s *LogExpenseSuite) TestHappyPathPersists() {
	categoryID := uuid.New()
	rootID := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: categoryID, RootCategoryID: rootID, Path: "Prazeres > Delivery"},
		},
	}}
	upserter := &fakeUpserter{result: usecases.ExpenseUpsertResult{
		ID:             uuid.NewString(),
		SubcategoryID:  categoryID.String(),
		RootCategoryID: "prazeres",
		AmountCents:    5800,
		Competence:     "2026-06",
	}}
	uc := usecases.NewLogExpenseFromAgent(resolver, upserter, nil, noop.NewProvider())
	in := s.buildIntent(5800, "ifood", "iFood")
	out, err := uc.Execute(context.Background(), usecases.LogExpenseFromAgentInput{
		UserID: uuid.NewString(),
		Intent: in,
	})
	s.Require().NoError(err)
	s.True(out.Persisted)
	s.Equal(categoryID.String(), out.SubcategoryID)
	s.Equal("prazeres", out.RootCategoryID)
	s.Equal(int64(5800), out.AmountCents)
	s.Equal("Prazeres > Delivery", out.CategoryPath)
	s.True(upserter.called)
	s.Equal("agent", upserter.in.Source)
	s.NotEmpty(upserter.in.ExternalTransactionID)
}

func (s *LogExpenseSuite) TestFallsBackToMerchantWhenHintEmpty() {
	categoryID := uuid.New()
	rootID := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: categoryID, RootCategoryID: rootID, Path: "Prazeres"},
		},
	}}
	upserter := &fakeUpserter{result: usecases.ExpenseUpsertResult{
		SubcategoryID: categoryID.String(),
		AmountCents:   5800,
		Competence:    "2026-06",
	}}
	uc := usecases.NewLogExpenseFromAgent(resolver, upserter, nil, noop.NewProvider())
	in := s.buildIntent(5800, "", "iFood")
	out, err := uc.Execute(context.Background(), usecases.LogExpenseFromAgentInput{
		UserID: uuid.NewString(),
		Intent: in,
	})
	s.Require().NoError(err)
	s.True(out.Persisted)
}

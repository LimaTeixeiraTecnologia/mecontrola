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
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
)

func candidatesOutput(path string) *categoriesoutput.DictionarySearchOutput {
	id := uuid.New()
	return &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: id, RootCategoryID: id, Path: path},
		},
	}
}

type fakeCardPurchaseCreator struct {
	called bool
	in     usecases.CreateCardPurchaseCommand
	result usecases.CreateCardPurchaseResult
	err    error
}

func (f *fakeCardPurchaseCreator) Execute(_ context.Context, in usecases.CreateCardPurchaseCommand) (usecases.CreateCardPurchaseResult, error) {
	f.called = true
	f.in = in
	return f.result, f.err
}

type LogCardPurchaseSuite struct {
	suite.Suite
}

func TestLogCardPurchaseSuite(t *testing.T) {
	suite.Run(t, new(LogCardPurchaseSuite))
}

func (s *LogCardPurchaseSuite) intent() intent.Intent {
	in, err := intent.NewLogCardPurchase(intent.LogCardPurchaseFields{
		AmountCents: 120000, Merchant: "Magalu", CardHint: "nubank", Installments: 6,
	})
	s.Require().NoError(err)
	return in
}

func (s *LogCardPurchaseSuite) TestPersistedWhenCardFound() {
	creator := &fakeCardPurchaseCreator{result: usecases.CreateCardPurchaseResult{CardFound: true, CardName: "Nubank"}}
	uc := usecases.NewLogCardPurchaseFromAgent(&fakeResolver{out: candidatesOutput("Casa > Eletro")}, creator, noop.NewProvider())

	out, err := uc.Execute(context.Background(), usecases.LogCardPurchaseFromAgentInput{UserID: uuid.NewString(), Intent: s.intent()})
	s.Require().NoError(err)
	s.True(out.Persisted)
	s.True(out.CardFound)
	s.Equal("Nubank", out.CardName)
	s.Equal(6, out.Installments)
	s.True(creator.called)
	s.Equal(6, creator.in.Installments)
}

func (s *LogCardPurchaseSuite) TestNotPersistedWhenCardMissing() {
	creator := &fakeCardPurchaseCreator{result: usecases.CreateCardPurchaseResult{CardFound: false}}
	uc := usecases.NewLogCardPurchaseFromAgent(&fakeResolver{out: candidatesOutput("Casa")}, creator, noop.NewProvider())

	out, err := uc.Execute(context.Background(), usecases.LogCardPurchaseFromAgentInput{UserID: uuid.NewString(), Intent: s.intent()})
	s.Require().NoError(err)
	s.False(out.Persisted)
	s.False(out.CardFound)
}

func (s *LogCardPurchaseSuite) TestCategoryNotFoundFails() {
	creator := &fakeCardPurchaseCreator{}
	uc := usecases.NewLogCardPurchaseFromAgent(&fakeResolver{out: &categoriesoutput.DictionarySearchOutput{}}, creator, noop.NewProvider())

	_, err := uc.Execute(context.Background(), usecases.LogCardPurchaseFromAgentInput{UserID: uuid.NewString(), Intent: s.intent()})
	s.Require().Error(err)
	s.False(creator.called)
}

func (s *LogCardPurchaseSuite) TestCreateErrorPropagates() {
	creator := &fakeCardPurchaseCreator{err: errors.New("boom")}
	uc := usecases.NewLogCardPurchaseFromAgent(&fakeResolver{out: candidatesOutput("Casa")}, creator, noop.NewProvider())

	_, err := uc.Execute(context.Background(), usecases.LogCardPurchaseFromAgentInput{UserID: uuid.NewString(), Intent: s.intent()})
	s.Require().Error(err)
}

type fakeRecurringTemplateCreator struct {
	called bool
	in     usecases.CreateRecurringCommand
	result usecases.CreateRecurringResult
	err    error
}

func (f *fakeRecurringTemplateCreator) Execute(_ context.Context, in usecases.CreateRecurringCommand) (usecases.CreateRecurringResult, error) {
	f.called = true
	f.in = in
	return f.result, f.err
}

type CreateRecurringSuite struct {
	suite.Suite
}

func TestCreateRecurringSuite(t *testing.T) {
	suite.Run(t, new(CreateRecurringSuite))
}

func (s *CreateRecurringSuite) intent(direction string) intent.Intent {
	in, err := intent.NewCreateRecurring(intent.CreateRecurringFields{
		AmountCents: 500000, Merchant: "salário", Direction: direction, DayOfMonth: 5,
	})
	s.Require().NoError(err)
	return in
}

func (s *CreateRecurringSuite) TestPersistedIncome() {
	creator := &fakeRecurringTemplateCreator{result: usecases.CreateRecurringResult{Persisted: true}}
	uc := usecases.NewCreateRecurringFromAgent(&fakeResolver{out: candidatesOutput("Salário")}, creator, noop.NewProvider())

	out, err := uc.Execute(context.Background(), usecases.CreateRecurringFromAgentInput{UserID: uuid.NewString(), Intent: s.intent("income")})
	s.Require().NoError(err)
	s.True(out.Persisted)
	s.Equal("income", out.Direction)
	s.Equal("monthly", out.Frequency)
	s.Equal(5, out.DayOfMonth)
	s.True(creator.called)
	s.Equal("income", creator.in.Direction)
}

func (s *CreateRecurringSuite) TestCategoryNotFoundFailsForOutcome() {
	creator := &fakeRecurringTemplateCreator{}
	uc := usecases.NewCreateRecurringFromAgent(&fakeResolver{out: &categoriesoutput.DictionarySearchOutput{}}, creator, noop.NewProvider())

	_, err := uc.Execute(context.Background(), usecases.CreateRecurringFromAgentInput{UserID: uuid.NewString(), Intent: s.intent("outcome")})
	s.Require().Error(err)
	s.False(creator.called)
}

func (s *CreateRecurringSuite) TestCreateErrorPropagates() {
	creator := &fakeRecurringTemplateCreator{err: errors.New("boom")}
	uc := usecases.NewCreateRecurringFromAgent(&fakeResolver{out: candidatesOutput("Salário")}, creator, noop.NewProvider())

	_, err := uc.Execute(context.Background(), usecases.CreateRecurringFromAgentInput{UserID: uuid.NewString(), Intent: s.intent("income")})
	s.Require().Error(err)
}

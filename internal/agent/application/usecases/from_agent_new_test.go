package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
)

func candidatesOutput(path string) *categoriesoutput.DictionarySearchOutput {
	id := uuid.New()
	return &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: id, RootCategoryID: id, Path: path, Score: 0.95},
		},
	}
}

type fakeCardPurchaseCreator struct {
	called bool
	in     CreateCardPurchaseCommand
	result CreateCardPurchaseResult
	err    error
}

func (f *fakeCardPurchaseCreator) Execute(_ context.Context, in CreateCardPurchaseCommand) (CreateCardPurchaseResult, error) {
	f.called = true
	f.in = in
	return f.result, f.err
}

type LogCardPurchaseSuite struct {
	suite.Suite
	ctx context.Context
}

func TestLogCardPurchaseSuite(t *testing.T) {
	suite.Run(t, new(LogCardPurchaseSuite))
}

func (s *LogCardPurchaseSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *LogCardPurchaseSuite) intent() intent.Intent {
	in, err := intent.NewLogCardPurchase(intent.LogCardPurchaseFields{
		AmountCents: 120000, Merchant: "Magalu", CardHint: "nubank", Installments: 6,
	})
	s.Require().NoError(err)
	return in
}

func (s *LogCardPurchaseSuite) TestPersistedWhenCardFound() {
	creator := &fakeCardPurchaseCreator{result: CreateCardPurchaseResult{CardFound: true, CardName: "Nubank"}}
	uc := NewLogCardPurchaseFromAgent(&fakeResolver{out: candidatesOutput("Casa > Eletro")}, creator, fake.NewProvider())

	out, err := uc.Execute(s.ctx, LogCardPurchaseFromAgentInput{UserID: uuid.NewString(), Intent: s.intent()})
	s.Require().NoError(err)
	s.True(out.Persisted)
	s.True(out.CardFound)
	s.Equal("Nubank", out.CardName)
	s.Equal(6, out.Installments)
	s.True(creator.called)
	s.Equal(6, creator.in.Installments)
}

func (s *LogCardPurchaseSuite) TestNotPersistedWhenCardMissing() {
	creator := &fakeCardPurchaseCreator{result: CreateCardPurchaseResult{CardFound: false}}
	uc := NewLogCardPurchaseFromAgent(&fakeResolver{out: candidatesOutput("Casa")}, creator, fake.NewProvider())

	out, err := uc.Execute(s.ctx, LogCardPurchaseFromAgentInput{UserID: uuid.NewString(), Intent: s.intent()})
	s.Require().NoError(err)
	s.False(out.Persisted)
	s.False(out.CardFound)
}

func (s *LogCardPurchaseSuite) TestCategoryNotFoundFails() {
	creator := &fakeCardPurchaseCreator{}
	uc := NewLogCardPurchaseFromAgent(&fakeResolver{out: &categoriesoutput.DictionarySearchOutput{}}, creator, fake.NewProvider())

	_, err := uc.Execute(s.ctx, LogCardPurchaseFromAgentInput{UserID: uuid.NewString(), Intent: s.intent()})
	s.Require().Error(err)
	s.False(creator.called)
}

func (s *LogCardPurchaseSuite) TestCreateErrorPropagates() {
	creator := &fakeCardPurchaseCreator{err: errors.New("boom")}
	uc := NewLogCardPurchaseFromAgent(&fakeResolver{out: candidatesOutput("Casa")}, creator, fake.NewProvider())

	_, err := uc.Execute(s.ctx, LogCardPurchaseFromAgentInput{UserID: uuid.NewString(), Intent: s.intent()})
	s.Require().Error(err)
}

type fakeRecurringTemplateCreator struct {
	called bool
	in     CreateRecurringCommand
	result CreateRecurringResult
	err    error
}

func (f *fakeRecurringTemplateCreator) Execute(_ context.Context, in CreateRecurringCommand) (CreateRecurringResult, error) {
	f.called = true
	f.in = in
	return f.result, f.err
}

type CreateRecurringSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCreateRecurringSuite(t *testing.T) {
	suite.Run(t, new(CreateRecurringSuite))
}

func (s *CreateRecurringSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CreateRecurringSuite) intent(direction string) intent.Intent {
	in, err := intent.NewCreateRecurring(intent.CreateRecurringFields{
		AmountCents: 500000, Merchant: "salário", Direction: direction, DayOfMonth: 5,
	})
	s.Require().NoError(err)
	return in
}

func (s *CreateRecurringSuite) TestPersistedIncome() {
	creator := &fakeRecurringTemplateCreator{result: CreateRecurringResult{Persisted: true}}
	uc := NewCreateRecurringFromAgent(&fakeResolver{out: candidatesOutput("Salário")}, creator, fake.NewProvider())

	out, err := uc.Execute(s.ctx, CreateRecurringFromAgentInput{UserID: uuid.NewString(), Intent: s.intent("income")})
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
	uc := NewCreateRecurringFromAgent(&fakeResolver{out: &categoriesoutput.DictionarySearchOutput{}}, creator, fake.NewProvider())

	_, err := uc.Execute(s.ctx, CreateRecurringFromAgentInput{UserID: uuid.NewString(), Intent: s.intent("outcome")})
	s.Require().Error(err)
	s.False(creator.called)
}

func (s *CreateRecurringSuite) TestCreateErrorPropagates() {
	creator := &fakeRecurringTemplateCreator{err: errors.New("boom")}
	uc := NewCreateRecurringFromAgent(&fakeResolver{out: candidatesOutput("Salário")}, creator, fake.NewProvider())

	_, err := uc.Execute(s.ctx, CreateRecurringFromAgentInput{UserID: uuid.NewString(), Intent: s.intent("income")})
	s.Require().Error(err)
}

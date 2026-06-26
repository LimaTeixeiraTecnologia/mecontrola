package onboarding

import (
	"context"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"

	"github.com/stretchr/testify/suite"
)

type OnboardingInterpreterSuite struct {
	suite.Suite
	ctx context.Context
}

func TestOnboardingInterpreterSuite(t *testing.T) {
	suite.Run(t, new(OnboardingInterpreterSuite))
}

func (s *OnboardingInterpreterSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *OnboardingInterpreterSuite) TestParseBudget_Value() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseBudget(s.ctx, "4000")
	s.NoError(err)
	s.Equal(int64(400000), parsed.IncomeCents)
	s.False(parsed.Ambiguous)
}

func (s *OnboardingInterpreterSuite) TestParseBudget_WithCurrency() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseBudget(s.ctx, "R$ 5.250,50")
	s.NoError(err)
	s.Equal(int64(525050), parsed.IncomeCents)
}

func (s *OnboardingInterpreterSuite) TestParseBudget_DailyCommand() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseBudget(s.ctx, "gastei 50 mercado")
	s.NoError(err)
	s.True(parsed.DailyCommand)
}

func (s *OnboardingInterpreterSuite) TestParseCards_Valid() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseCards(s.ctx, "Nubank 15", 0)
	s.NoError(err)
	s.Equal("Nubank", parsed.Nickname)
	s.Equal(15, parsed.DueDay)
}

func (s *OnboardingInterpreterSuite) TestParseCards_Skip() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseCards(s.ctx, "não uso", 0)
	s.NoError(err)
	s.True(parsed.Skip)
}

func (s *OnboardingInterpreterSuite) TestParseCards_Ambiguous() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseCards(s.ctx, "nubank", 0)
	s.NoError(err)
	s.True(parsed.Ambiguous)
}

func (s *OnboardingInterpreterSuite) TestParseValue_Number() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseValue(s.ctx, "1000")
	s.NoError(err)
	s.False(parsed.Ambiguous)
	s.False(parsed.DailyCommand)
	s.Equal(int64(100000), parsed.ValueCents)
}

func (s *OnboardingInterpreterSuite) TestParseValue_Ambiguous() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseValue(s.ctx, "não sei")
	s.NoError(err)
	s.True(parsed.Ambiguous)
	s.False(parsed.DailyCommand)
}

func (s *OnboardingInterpreterSuite) TestParseValue_DailyCommand() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseValue(s.ctx, "gastei 50 mercado")
	s.NoError(err)
	s.True(parsed.DailyCommand)
}

func (s *OnboardingInterpreterSuite) TestParseObjective() {
	interp := &onboardingInterpreter{}
	parsed, err := interp.ParseObjective(s.ctx, "quitar dívidas")
	s.NoError(err)
	s.Equal("quitar dívidas", parsed.Objective)
	s.False(parsed.Ambiguous)
}

func (s *OnboardingInterpreterSuite) TestParseCategoriesConfirm_AcceptsConfirmation() {
	interp := &onboardingInterpreter{}
	confirmed, err := interp.ParseCategoriesConfirm(s.ctx, "sim")
	s.NoError(err)
	s.True(confirmed)
}

func (s *OnboardingInterpreterSuite) TestParseCategoriesConfirm_RejectsClarification() {
	interp := &onboardingInterpreter{}
	cases := []string{"não entendi", "o que é isso?", "qualquer texto", "não"}
	for _, text := range cases {
		confirmed, err := interp.ParseCategoriesConfirm(s.ctx, text)
		s.NoError(err)
		s.False(confirmed, "expected %q to be rejected", text)
	}
}

func (s *OnboardingInterpreterSuite) TestNewOnboardingInterpreter_Nil() {
	s.Nil(NewOnboardingInterpreter(nil, 100))
}

func (s *OnboardingInterpreterSuite) TestNewOnboardingInterpreter_DefaultTokens() {
	interp := NewOnboardingInterpreter(&fakeIntentInterpreter{}, 0)
	s.NotNil(interp)
}

type fakeIntentInterpreter struct{}

func (f *fakeIntentInterpreter) Interpret(_ context.Context, _ interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	return interfaces.LLMResponse{}, nil
}

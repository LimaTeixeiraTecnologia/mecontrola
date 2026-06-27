package onboarding

import (
	"context"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"

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

type scriptedIntentInterpreter struct {
	rawJSON string
	err     error
}

func (f *scriptedIntentInterpreter) Interpret(_ context.Context, _ interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	if f.err != nil {
		return interfaces.LLMResponse{}, f.err
	}
	return interfaces.LLMResponse{RawJSON: []byte(f.rawJSON)}, nil
}

func (s *OnboardingInterpreterSuite) TestParseObjective_LLMFirst_Save() {
	interp := &onboardingInterpreter{interpreter: &scriptedIntentInterpreter{rawJSON: `{"action":"save","objective":"quitar dívidas"}`}, maxTokens: 256}
	parsed, err := interp.ParseObjective(s.ctx, "tenho umas dívidas e quero me livrar delas")
	s.NoError(err)
	s.Equal("quitar dívidas", parsed.Objective)
	s.False(parsed.Ambiguous)
	s.False(parsed.DailyCommand)
}

func (s *OnboardingInterpreterSuite) TestParseObjective_LLMFirst_Clarify() {
	interp := &onboardingInterpreter{interpreter: &scriptedIntentInterpreter{rawJSON: `{"action":"clarify","objective":""}`}, maxTokens: 256}
	parsed, err := interp.ParseObjective(s.ctx, "asdkjhasd")
	s.NoError(err)
	s.True(parsed.Ambiguous)
}

func (s *OnboardingInterpreterSuite) TestParseObjective_FallbackOnLLMError() {
	interp := &onboardingInterpreter{interpreter: &scriptedIntentInterpreter{err: context.DeadlineExceeded}, maxTokens: 256}
	parsed, err := interp.ParseObjective(s.ctx, "quitar dívidas")
	s.NoError(err)
	s.Equal("quitar dívidas", parsed.Objective)
}

func (s *OnboardingInterpreterSuite) TestParseBudget_LLMFirst_AmountText() {
	interp := &onboardingInterpreter{interpreter: &scriptedIntentInterpreter{rawJSON: `{"action":"save","amount_text":"R$ 5.250,50"}`}, maxTokens: 256}
	parsed, err := interp.ParseBudget(s.ctx, "ganho uns cinco mil duzentos e cinquenta e meio")
	s.NoError(err)
	s.Equal(int64(525050), parsed.IncomeCents)
}

func (s *OnboardingInterpreterSuite) TestParseCategoriesConfirm_LLMFirst() {
	confirm := &onboardingInterpreter{interpreter: &scriptedIntentInterpreter{rawJSON: `{"action":"confirm"}`}, maxTokens: 256}
	ok, err := confirm.ParseCategoriesConfirm(s.ctx, "claro, faz total sentido")
	s.NoError(err)
	s.True(ok)

	clarify := &onboardingInterpreter{interpreter: &scriptedIntentInterpreter{rawJSON: `{"action":"clarify"}`}, maxTokens: 256}
	ok, err = clarify.ParseCategoriesConfirm(s.ctx, "o que é liberdade financeira?")
	s.NoError(err)
	s.False(ok)
}

func (s *OnboardingInterpreterSuite) TestParseCards_LLMFirst_AddAnotherOnLoopZero() {
	interp := &onboardingInterpreter{interpreter: &scriptedIntentInterpreter{rawJSON: `{"action":"add_another","nickname":"","due_day":0}`}, maxTokens: 256}
	parsed, err := interp.ParseCards(s.ctx, "sim, eu uso", 0)
	s.NoError(err)
	s.True(parsed.AddAnother)
}

func (s *OnboardingInterpreterSuite) TestFormatPercent_OneDecimal() {
	s.Equal("50", formatPercent(200000, 400000))
	s.Equal("7,5", formatPercent(30000, 400000))
	s.Equal("12,5", formatPercent(50000, 400000))
	s.Equal("0", formatPercent(0, 400000))
	s.Equal("0", formatPercent(100, 0))
}

func (s *OnboardingInterpreterSuite) TestSummaryCue_RendersDecimalPercent() {
	cue := summaryCue(agentwf.SummaryState{
		Objective:   "quitar dívidas",
		IncomeCents: 400000,
		Values: map[string]int64{
			"fixed_cost":        200000,
			"knowledge":         30000,
			"pleasures":         50000,
			"goals":             70000,
			"financial_freedom": 50000,
		},
	})
	s.Contains(cue, "(50%)")
	s.Contains(cue, "(7,5%)")
}

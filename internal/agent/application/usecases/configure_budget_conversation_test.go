package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
)

type ConfigureBudgetConversationSuite struct {
	suite.Suite
}

func TestConfigureBudgetConversationSuite(t *testing.T) {
	suite.Run(t, new(ConfigureBudgetConversationSuite))
}

func (s *ConfigureBudgetConversationSuite) newSUT(resp string, err error) *usecases.ConfigureBudgetConversation {
	uc, ucErr := usecases.NewConfigureBudgetConversation(&fakeInterpreter{
		resp: interfaces.LLMResponse{RawJSON: []byte(resp)},
		err:  err,
	}, noop.NewProvider())
	s.Require().NoError(ucErr)
	return uc
}

func (s *ConfigureBudgetConversationSuite) TestNewNilDeps() {
	_, err := usecases.NewConfigureBudgetConversation(nil, noop.NewProvider())
	s.Require().Error(err)
	_, err = usecases.NewConfigureBudgetConversation(&fakeInterpreter{}, nil)
	s.Require().Error(err)
}

func (s *ConfigureBudgetConversationSuite) TestEmptyText() {
	uc := s.newSUT(`{}`, nil)
	_, err := uc.Execute(context.Background(), usecases.ConfigureBudgetInput{Text: "   "})
	s.Require().ErrorIs(err, usecases.ErrConfigureBudgetEmptyText)
}

func (s *ConfigureBudgetConversationSuite) TestPartialTurnAsksWhatIsMissing() {
	uc := s.newSUT(`{"total_cents":500000,"allocations":[{"root_slug":"expense.custo_fixo","basis_points":3500}]}`, nil)
	out, err := uc.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "ganho 5 mil e custos fixos 35%",
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.False(out.Complete)
	s.Equal(int64(500000), out.Draft.TotalCents())
	s.Equal(3500, out.Draft.SumBasisPoints())
	s.NotEmpty(out.Reply)
}

func (s *ConfigureBudgetConversationSuite) TestAsksForIncomeWhenTotalMissing() {
	uc := s.newSUT(`{"allocations":[{"root_slug":"expense.metas","basis_points":2000}]}`, nil)
	out, err := uc.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "metas 20%",
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.False(out.Complete)
	s.Contains(out.Reply, "renda")
}

func (s *ConfigureBudgetConversationSuite) TestCompleteWhenSumIs10000() {
	resp := `{"total_cents":800000,"allocations":[
		{"root_slug":"expense.custo_fixo","basis_points":3500},
		{"root_slug":"expense.conhecimento","basis_points":1000},
		{"root_slug":"expense.prazeres","basis_points":2000},
		{"root_slug":"expense.metas","basis_points":2000},
		{"root_slug":"expense.liberdade_financeira","basis_points":1500}
	]}`
	uc := s.newSUT(resp, nil)
	out, err := uc.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "renda 8 mil, fixo 35, conhecimento 10, prazeres 20, metas 20, liberdade 15",
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.True(out.Complete)
	s.Empty(out.Reply)
	s.Equal(int64(800000), out.Draft.TotalCents())
	s.Equal(10000, out.Draft.SumBasisPoints())
}

func (s *ConfigureBudgetConversationSuite) TestMergeOverMultipleTurns() {
	first := s.newSUT(`{"total_cents":500000,"allocations":[{"root_slug":"expense.custo_fixo","basis_points":5000}]}`, nil)
	out1, err := first.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "renda 5 mil, fixo 50%",
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.False(out1.Complete)

	second := s.newSUT(`{"allocations":[{"root_slug":"expense.metas","basis_points":5000}]}`, nil)
	out2, err := second.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "metas 50%",
		Draft: out1.Draft,
	})
	s.Require().NoError(err)
	s.True(out2.Complete)
	s.Equal(int64(500000), out2.Draft.TotalCents())
}

func (s *ConfigureBudgetConversationSuite) TestProviderErrorReturnsError() {
	uc := s.newSUT(`{}`, errors.New("boom"))
	_, err := uc.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "renda 5 mil",
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().Error(err)
}

func (s *ConfigureBudgetConversationSuite) TestInvalidJSONKeepsDraftAndAsks() {
	uc := s.newSUT(`not json`, nil)
	out, err := uc.Execute(context.Background(), usecases.ConfigureBudgetInput{
		Text:  "blá blá",
		Draft: budgetdraft.New("2026-06"),
	})
	s.Require().NoError(err)
	s.False(out.Complete)
	s.NotEmpty(out.Reply)
	s.Equal(int64(0), out.Draft.TotalCents())
}

func (s *ConfigureBudgetConversationSuite) TestForwardsJSONSchema() {
	fake := &fakeInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"allocations":[]}`)}}
	uc, err := usecases.NewConfigureBudgetConversation(fake, noop.NewProvider())
	s.Require().NoError(err)
	_, err = uc.Execute(context.Background(), usecases.ConfigureBudgetInput{Text: "oi", Draft: budgetdraft.New("2026-06")})
	s.Require().NoError(err)
	s.Require().NotNil(fake.lastRequest.JSONSchema)
	s.Equal("mecontrola_budget_config", fake.lastRequest.JSONSchema.Name)
}

package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type ToolCatalogSuite struct {
	suite.Suite
}

func TestToolCatalog(t *testing.T) {
	suite.Run(t, new(ToolCatalogSuite))
}

func (s *ToolCatalogSuite) TestCatalogExposesMVPTools() {
	catalog := usecases.AgentToolCatalog()

	names := make(map[string]bool, len(catalog))
	for _, tool := range catalog {
		s.NotEmpty(tool.Name)
		s.NotEmpty(tool.Description)
		s.NotNil(tool.Parameters)
		names[tool.Name] = true
	}

	s.True(names["record_transaction"])
	s.True(names["monthly_summary"])
	s.True(names["list_cards"])
	s.True(names["configure_budget"])
}

func (s *ToolCatalogSuite) TestRecordTransactionOutcomeMapsToLogExpense() {
	call := interfaces.ToolCall{
		FunctionName: "record_transaction",
		ArgumentsJSON: map[string]any{
			"direction":     "outcome",
			"amount_cents":  float64(5800),
			"merchant":      "iFood",
			"category_hint": "delivery",
		},
	}

	got, err := usecases.ToolCallToIntent(call, "gastei 58 no ifood")
	s.Require().NoError(err)
	s.Equal(intent.KindLogExpense, got.Kind())
	s.Equal(int64(5800), got.AmountCents())
	s.Equal("iFood", got.Merchant())
}

func (s *ToolCatalogSuite) TestRecordTransactionIncomeMapsToLogIncome() {
	call := interfaces.ToolCall{
		FunctionName: "record_transaction",
		ArgumentsJSON: map[string]any{
			"direction":    "income",
			"amount_cents": float64(900000),
			"merchant":     "salario",
		},
	}

	got, err := usecases.ToolCallToIntent(call, "recebi meu salario")
	s.Require().NoError(err)
	s.Equal(intent.KindLogIncome, got.Kind())
	s.Equal(int64(900000), got.AmountCents())
}

func (s *ToolCatalogSuite) TestMonthlySummaryMapsToKind() {
	call := interfaces.ToolCall{FunctionName: "monthly_summary", ArgumentsJSON: map[string]any{"ref_month": "2026-06"}}

	got, err := usecases.ToolCallToIntent(call, "resumo do mes")
	s.Require().NoError(err)
	s.Equal(intent.KindMonthlySummary, got.Kind())
}

func (s *ToolCatalogSuite) TestListCardsAndConfigureBudgetMapToKinds() {
	listCards, err := usecases.ToolCallToIntent(interfaces.ToolCall{FunctionName: "list_cards"}, "meus cartoes")
	s.Require().NoError(err)
	s.Equal(intent.KindListCards, listCards.Kind())

	configBudget, err := usecases.ToolCallToIntent(interfaces.ToolCall{FunctionName: "configure_budget"}, "configurar orcamento")
	s.Require().NoError(err)
	s.Equal(intent.KindConfigureBudget, configBudget.Kind())
}

func (s *ToolCatalogSuite) TestUnsupportedToolReturnsError() {
	_, err := usecases.ToolCallToIntent(interfaces.ToolCall{FunctionName: "launch_rocket"}, "?")
	s.Require().Error(err)
	s.ErrorIs(err, usecases.ErrToolUnsupported)
}

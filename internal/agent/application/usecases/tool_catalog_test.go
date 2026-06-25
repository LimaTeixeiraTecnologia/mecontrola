package usecases

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type ToolCatalogSuite struct {
	suite.Suite
}

func TestToolCatalog(t *testing.T) {
	suite.Run(t, new(ToolCatalogSuite))
}

func (s *ToolCatalogSuite) TestCreateCardMapsToKind() {
	call := interfaces.ToolCall{
		FunctionName: "create_card",
		ArgumentsJSON: map[string]any{
			"nickname":    "nubank",
			"name":        "Nubank Roxinho",
			"closing_day": float64(10),
			"due_day":     float64(17),
			"limit_cents": float64(500000),
		},
	}

	got, err := ToolCallToIntent(call, "cadastra meu nubank")
	s.Require().NoError(err)
	s.Equal(intent.KindCreateCard, got.Kind())
	s.Equal("nubank", got.CardNickname())
	s.Equal("Nubank Roxinho", got.CardName())
	s.Equal(10, got.ClosingDay())
	s.Equal(17, got.DueDay())
	s.Equal(int64(500000), got.LimitCents())
}

func (s *ToolCatalogSuite) TestCreateCardWithoutNicknameFails() {
	call := interfaces.ToolCall{FunctionName: "create_card", ArgumentsJSON: map[string]any{}}
	_, err := ToolCallToIntent(call, "cadastra um cartao")
	s.Require().Error(err)
	s.ErrorIs(err, intent.ErrCardNicknameEmpty)
}

func (s *ToolCatalogSuite) TestCountCardsMapsToKind() {
	got, err := ToolCallToIntent(interfaces.ToolCall{FunctionName: "count_cards"}, "quantos cartoes eu tenho")
	s.Require().NoError(err)
	s.Equal(intent.KindCountCards, got.Kind())
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

	got, err := ToolCallToIntent(call, "gastei 58 no ifood")
	s.Require().NoError(err)
	s.Equal(intent.KindRecordExpense, got.Kind())
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

	got, err := ToolCallToIntent(call, "recebi meu salario")
	s.Require().NoError(err)
	s.Equal(intent.KindRecordIncome, got.Kind())
	s.Equal(int64(900000), got.AmountCents())
}

func (s *ToolCatalogSuite) TestMonthlySummaryMapsToKind() {
	call := interfaces.ToolCall{FunctionName: "monthly_summary", ArgumentsJSON: map[string]any{"ref_month": "2026-06"}}

	got, err := ToolCallToIntent(call, "resumo do mes")
	s.Require().NoError(err)
	s.Equal(intent.KindMonthlySummary, got.Kind())
}

func (s *ToolCatalogSuite) TestListCardsAndConfigureBudgetMapToKinds() {
	listCards, err := ToolCallToIntent(interfaces.ToolCall{FunctionName: "list_cards"}, "meus cartoes")
	s.Require().NoError(err)
	s.Equal(intent.KindListCards, listCards.Kind())

	configBudget, err := ToolCallToIntent(interfaces.ToolCall{FunctionName: "configure_budget"}, "configurar orcamento")
	s.Require().NoError(err)
	s.Equal(intent.KindConfigureBudget, configBudget.Kind())
}

func (s *ToolCatalogSuite) TestUnsupportedToolReturnsError() {
	_, err := ToolCallToIntent(interfaces.ToolCall{FunctionName: "launch_rocket"}, "?")
	s.Require().Error(err)
	s.ErrorIs(err, ErrToolUnsupported)
}

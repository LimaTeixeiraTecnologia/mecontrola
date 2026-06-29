package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func TestDailyLedgerAgentBudgetChangeParsesTranscriptPercentages(t *testing.T) {
	agent := &DailyLedgerAgent{}
	in, err := intent.NewUnknown("percentuais")
	require.NoError(t, err)

	change := agent.budgetChange(`Custos fixos         | 40%
Metas                | 15%
Prazeres             | 20%
 Conhecimento         | 5%
Liberdade Financeira | 20%`, ParsedIntent{Intent: in})

	require.Equal(t, map[string]int{
		budgetdraft.SlugCustoFixo:           4000,
		budgetdraft.SlugMetas:               1500,
		budgetdraft.SlugPrazeres:            2000,
		budgetdraft.SlugConhecimento:        500,
		budgetdraft.SlugLiberdadeFinanceira: 2000,
	}, change.Allocations)
}

func TestDailyLedgerAgentBudgetChangeParsesSingleCategoryPercentage(t *testing.T) {
	agent := &DailyLedgerAgent{}
	in, err := intent.NewUnknown("percentual")
	require.NoError(t, err)

	change := agent.budgetChange("Liberdade Financeira | 20%", ParsedIntent{Intent: in})

	require.Equal(t, map[string]int{
		budgetdraft.SlugLiberdadeFinanceira: 2000,
	}, change.Allocations)
}

func TestDailyLedgerAgentBudgetChangeIgnoresUnknownText(t *testing.T) {
	agent := &DailyLedgerAgent{}
	in, err := intent.NewUnknown("texto livre")
	require.NoError(t, err)

	change := agent.budgetChange("quero começar", ParsedIntent{Intent: in})

	require.Zero(t, change.TotalCents)
	require.Nil(t, change.Allocations)
}

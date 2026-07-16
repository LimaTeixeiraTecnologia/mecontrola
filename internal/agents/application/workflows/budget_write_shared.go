package workflows

import (
	"fmt"
	"math"
	"strings"

	budgetsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
)

func budgetTotalPrompt() string {
	return "Vamos criar seu orçamento. Qual é o valor total (em R$) que você quer planejar para esse mês? (por exemplo: R$ 3.500,00)"
}

func budgetTotalReprompt() string {
	return "Não consegui identificar o valor. Qual é o valor total (em R$) do orçamento? Por exemplo: R$ 3.500,00."
}

func mustCentsFromBRL(amountBRL float64) int64 {
	cents, err := DecideMonthlyBudgetCents(amountBRL)
	if err != nil {
		return 0
	}
	return cents
}

func budgetDistributionPrompt(totalCents int64) string {
	var b strings.Builder
	b.WriteString("Agora vamos distribuir os ")
	b.WriteString(money.FromCents(totalCents).BRL())
	b.WriteString(" entre as 5 categorias. Esta é a sugestão padrão:\n\n")
	for _, slug := range canonicalSlugs {
		bp := defaultDistributionBP[slug]
		cents := totalCents * int64(bp) / 10000
		fmt.Fprintf(&b, "%s: %s (%d%%)\n", categoryLabels[slug], money.FromCents(cents).BRL(), bp/100)
	}
	b.WriteString("\nAceita esta sugestão? Responda \"sim\" para confirmar, ou envie novos valores para cada categoria — pode ser em reais (R$) ou em porcentagem (%).")
	return b.String()
}

func budgetDistributionReprompt(reason string, totalCents int64) string {
	return "Ops, não consegui aplicar essa distribuição: " + reason + "\n\n" + budgetDistributionPrompt(totalCents)
}

func budgetBalanceReason(balance DistributionBalance) string {
	var unitLabel string
	var deltaLabel string
	if balance.Unit == allocationInputReais {
		unitLabel = "o orçamento mensal"
		deltaLabel = money.FromCents(int64(math.Round(balance.DeltaAbs * 100))).BRL()
	} else {
		unitLabel = "100%"
		deltaLabel = fmt.Sprintf("%.1f%%", balance.DeltaAbs)
	}
	switch balance.Status {
	case distributionOver:
		return fmt.Sprintf("passou %s de %s. Envie novamente a distribuição para fechar exatamente %s.", deltaLabel, unitLabel, unitLabel)
	case distributionUnder:
		return fmt.Sprintf("faltou %s para completar %s. Envie novamente a distribuição para fechar exatamente %s.", deltaLabel, unitLabel, unitLabel)
	default:
		return fmt.Sprintf("a distribuição não fecha exatamente %s.", unitLabel)
	}
}

func budgetAlreadyExistsMessage(rawCompetence string) string {
	monthLabel := rawCompetence
	if competence, err := budgetsvo.NewCompetence(rawCompetence); err == nil {
		monthLabel = budgetsvo.FormatCompetencePtBR(competence)
	}
	return fmt.Sprintf("Já existe um orçamento para %s. Não é possível criar outro.", monthLabel)
}

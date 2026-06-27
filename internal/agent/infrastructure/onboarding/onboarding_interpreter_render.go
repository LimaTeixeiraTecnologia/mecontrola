package onboarding

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func (i *onboardingInterpreter) RenderObjectiveSaved(_ context.Context) string {
	return "🎯 Perfeito!\n\nVamos montar tudo pensando nesse objetivo."
}

func (i *onboardingInterpreter) RenderBudgetSaved(_ context.Context, incomeCents int64) string {
	return "✅ Orçamento registrado\n\n💰 " + formatBRL(incomeCents)
}

func (i *onboardingInterpreter) RenderCardSaved(_ context.Context, nickname string, dueDay int) string {
	return fmt.Sprintf("✅ Cartão salvo\n\n💳 %s\n📅 Vencimento: dia %d", strings.TrimSpace(nickname), dueDay)
}

func (i *onboardingInterpreter) RenderValueSaved(_ context.Context, slug string, valueCents int64) string {
	return fmt.Sprintf("✅ %s definido — %s", categoryLabel(slug), formatBRL(valueCents))
}

func (i *onboardingInterpreter) RenderCategoriesConfirmed(_ context.Context) string {
	return "Perfeito!\n\nAgora vamos montar seu planejamento."
}

func (i *onboardingInterpreter) RenderCategoriesClarify(_ context.Context) string {
	return "Sem problema! As 5 categorias são só uma forma simples de organizar todo o seu dinheiro: 💰 Custo Fixo, 🎓 Conhecimento, 🎉 Prazeres, 🎯 Metas e 🏦 Liberdade Financeira.\n\nVamos montar seu planejamento."
}

func (i *onboardingInterpreter) RenderValuesMismatch(_ context.Context, sumCents, incomeCents int64) string {
	return fmt.Sprintf("A soma das categorias está em %s, mas seu orçamento é %s. Vamos ajustar para que os valores somem o orçamento. Quanto você quer destinar para esta categoria?", formatBRL(sumCents), formatBRL(incomeCents))
}

func formatBRL(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	reais := cents / 100
	centavos := cents % 100
	grouped := groupThousands(reais)
	result := "R$ " + grouped
	if centavos != 0 {
		result += "," + fmt.Sprintf("%02d", centavos)
	}
	if negative {
		return "-" + result
	}
	return result
}

func formatPercent(amount, incomeCents int64) string {
	if incomeCents <= 0 {
		return "0"
	}
	tenths := (amount*1000 + incomeCents/2) / incomeCents
	whole := tenths / 10
	frac := tenths % 10
	if frac == 0 {
		return strconv.FormatInt(whole, 10)
	}
	return strconv.FormatInt(whole, 10) + "," + strconv.FormatInt(frac, 10)
}

func groupThousands(value int64) string {
	digits := strconv.FormatInt(value, 10)
	n := len(digits)
	if n <= 3 {
		return digits
	}
	var b strings.Builder
	lead := n % 3
	if lead > 0 {
		b.WriteString(digits[:lead])
		if n > lead {
			b.WriteString(".")
		}
	}
	for idx := lead; idx < n; idx += 3 {
		b.WriteString(digits[idx : idx+3])
		if idx+3 < n {
			b.WriteString(".")
		}
	}
	return b.String()
}

package formatting

import "strings"

func NormalizeOutboundText(text string) string {
	normalized := strings.ReplaceAll(text, "**", "*")
	normalized = addOnboardingSummaryEmoji(normalized)
	normalized = addBudgetConfirmationEmoji(normalized)
	return normalized
}

func addOnboardingSummaryEmoji(text string) string {
	for _, marker := range []string{"Resumo de Onboarding", "Resumo do Onboarding"} {
		if !strings.Contains(text, marker) {
			continue
		}
		if strings.Contains(text, "📊 "+marker) {
			return text
		}
		return strings.Replace(text, marker, "📊 "+marker, 1)
	}
	return text
}

func addBudgetConfirmationEmoji(text string) string {
	if !strings.Contains(strings.ToLower(text), "ativar") || !strings.Contains(strings.ToLower(text), "orçamento") {
		return text
	}
	if strings.Contains(text, "✅ Você confirma") || strings.Contains(text, "🎯 Você confirma") {
		return text
	}
	if strings.Contains(text, "Você confirma") {
		return strings.Replace(text, "Você confirma", "✅ Você confirma", 1)
	}
	if strings.Contains(text, "Por favor, confirme") {
		return strings.Replace(text, "Por favor, confirme", "✅ Por favor, confirme", 1)
	}
	return text
}

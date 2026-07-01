package formatting

import "strings"

func NormalizeOutboundText(text string) string {
	normalized := strings.ReplaceAll(text, "**", "*")
	normalized = addOnboardingSummaryEmoji(normalized)
	normalized = addBudgetConfirmationEmoji(normalized)
	return normalized
}

func addOnboardingSummaryEmoji(text string) string {
	if !strings.Contains(text, "Resumo de Onboarding") {
		return text
	}
	if strings.Contains(text, "📊 Resumo de Onboarding") {
		return text
	}
	return strings.Replace(text, "Resumo de Onboarding", "📊 Resumo de Onboarding", 1)
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

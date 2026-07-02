package formatting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOutboundText_ReplacesDoubleAsterisks(t *testing.T) {
	got := NormalizeOutboundText("**Custo Fixo** e **Metas**")

	require.Equal(t, "*Custo Fixo* e *Metas*", got)
}

func TestNormalizeOutboundText_AddsOnboardingSummaryEmoji(t *testing.T) {
	got := NormalizeOutboundText("### Resumo de Onboarding")

	require.Contains(t, got, "📊 Resumo de Onboarding")
}

func TestNormalizeOutboundText_AddsConfirmationEmoji(t *testing.T) {
	got := NormalizeOutboundText("Você confirma que deseja ativar este orçamento?")

	require.Contains(t, got, "✅ Você confirma")
}

func TestNormalizeOutboundText_DoesNotDuplicateEmoji(t *testing.T) {
	got := NormalizeOutboundText("📊 Resumo de Onboarding\n\n✅ Você confirma que deseja ativar este orçamento?")

	require.Equal(t, "📊 Resumo de Onboarding\n\n✅ Você confirma que deseja ativar este orçamento?", got)
}

func TestNormalizeOutboundText_AddsOnboardingSummaryEmojiVariantDo(t *testing.T) {
	got := NormalizeOutboundText("*Resumo do Onboarding:*")

	require.Contains(t, got, "📊 Resumo do Onboarding")
}

func TestNormalizeOutboundText_DoesNotDuplicateEmojiVariantDo(t *testing.T) {
	got := NormalizeOutboundText("*📊 Resumo do Onboarding:*")

	require.Equal(t, "*📊 Resumo do Onboarding:*", got)
}

func TestNormalizeOutboundText_PreservesAllowedEmojis(t *testing.T) {
	input := "✅ Registrado! 💰 R$ 50,00 em Alimentação. 📊 Seu plano segue no ritmo. 🎯 Meta ativa. ⚠️ Atenção ao limite. 💡 Dica: revise semanalmente."

	got := NormalizeOutboundText(input)

	require.Equal(t, input, got)
}

func TestNormalizeOutboundText_ProductionOnboardingResponse(t *testing.T) {
	input := "1. *Custo Fixo*: Esta categoria abrange todas as despesas que você tem todo mês e que não podem ser evitadas, como aluguel, contas de luz, água, internet, e outras despesas essenciais.\n\n" +
		"*Resumo do Onboarding:*\n" +
		"- *Renda Mensal:* R$8.000,00\n" +
		"- *Distribuição de Despesas:*\n" +
		"  - Conhecimento: 20%\n" +
		"  - Custo Fixo: 30%\n" +
		"  - Liberdade Financeira: 20%\n" +
		"  - Metas: 10%\n" +
		"  - Prazeres: 20%\n\n" +
		"Por favor, confirme se deseja ativar o orçamento com as informações acima."

	got := NormalizeOutboundText(input)

	require.Contains(t, got, "📊 Resumo do Onboarding")
	require.Contains(t, got, "✅ Por favor, confirme")
	require.Contains(t, got, "*Custo Fixo*")
	require.NotContains(t, got, "**")
}

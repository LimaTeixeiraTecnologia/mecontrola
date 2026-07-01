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

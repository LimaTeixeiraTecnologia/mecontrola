package services_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
)

func TestBuildSystemPrompt_RendersContextWithoutFormatErrors(t *testing.T) {
	t.Parallel()

	builder := services.NewPromptBuilder()
	out := builder.BuildSystemPrompt(services.PromptContext{
		UserID:      "11111111-1111-1111-1111-111111111111",
		Channel:     "whatsapp",
		Permissions: []string{"read", "write"},
		Categories:  []services.CategorySeed{{ID: "cat-1", Name: "Mercado"}},
		Cards:       []services.CardSeed{{ID: "card-1", Name: "Nubank", Nickname: "Roxinho", ClosingDay: 5, DueDay: 12, LimitCents: 500000}},
		CurrentDate: time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC),
	})

	require.NotContains(t, out, "%!s(MISSING)")
	require.NotContains(t, out, "%!(EXTRA")
	require.NotContains(t, out, "%!s")

	require.Contains(t, out, "11111111-1111-1111-1111-111111111111")
	require.Contains(t, out, "whatsapp")
	require.Contains(t, out, "read, write")
	require.Contains(t, out, "Mercado")
	require.Contains(t, out, "Nubank")
	require.Contains(t, out, "2026-06-17")
}

func TestBuildSystemPrompt_DescribesModuleBoundaries(t *testing.T) {
	t.Parallel()

	builder := services.NewPromptBuilder()
	out := builder.BuildSystemPrompt(services.PromptContext{
		UserID:      "22222222-2222-2222-2222-222222222222",
		Channel:     "telegram",
		CurrentDate: time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC),
	})

	require.Contains(t, out, "RESPONSABILIDADE E FRONTEIRA DE CADA MODULO")
	require.Contains(t, out, "SOMENTE leitura")
	require.Contains(t, out, "Kiwify")
	require.Contains(t, out, "out_of_scope")
	require.Contains(t, out, "transactions.create")
	require.True(t, strings.Contains(out, "NAO existe consulta de fatura fechada por aqui"))
}

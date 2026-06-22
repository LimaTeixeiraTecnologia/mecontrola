//go:build integration

package e2e_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

func TestParseInbound_RealLLM_ProductionChain(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	models := []string{
		"google/gemini-2.5-flash-lite",
		"mistralai/mistral-small-3.2-24b-instruct",
	}

	core := []struct {
		text       string
		wantKind   intent.Kind
		wantAmount int64
	}{
		{"ifood 58 reais", intent.KindLogExpense, 5800},
		{"gastei 200 no mercado com nubank", intent.KindLogExpense, 20000},
		{"recebi meu salário de 16.400", intent.KindLogIncome, 1640000},
		{"qual a fatura do nubank?", intent.KindQueryCard, 0},
		{"resumo do mês", intent.KindMonthlySummary, 0},
		{"quais meus cartões?", intent.KindListCards, 0},
		{"comprei 1200 em 6x no nubank", intent.KindLogCardPurchase, 120000},
		{"parcelei 600 em 6 vezes no nubank", intent.KindLogCardPurchase, 60000},
		{"todo mês recebo 5000 de salário", intent.KindCreateRecurring, 500000},
		{"mostra minhas transações", intent.KindListTransactions, 0},
	}

	const maxAttempts = 3

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			slug, err := valueobjects.NewModelSlug(model)
			require.NoError(t, err)
			parser := realParserForModel(t, slug)

			for _, tc := range core {
				var (
					gotKind   intent.Kind
					gotAmount int64
					ok        bool
				)
				for attempt := 0; attempt < maxAttempts; attempt++ {
					out, execErr := parser.Execute(context.Background(), usecases.ParseInboundInput{UserID: uuid.New(), Text: tc.text})
					require.NoError(t, execErr)
					gotKind = out.Intent.Kind()
					gotAmount = out.Intent.AmountCents()
					if gotKind == tc.wantKind && (tc.wantAmount == 0 || gotAmount == tc.wantAmount) {
						ok = true
						break
					}
				}
				require.Truef(t, ok, "modelo %s não classificou %q de forma confiável em %d tentativas (último: kind=%s amount=%d) — NAO usar como primário/fallback do parser", model, tc.text, maxAttempts, gotKind.String(), gotAmount)
			}
		})
	}
}

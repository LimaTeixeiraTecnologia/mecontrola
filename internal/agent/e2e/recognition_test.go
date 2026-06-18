//go:build integration

package e2e_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func TestParseInbound_RealLLM_RecognitionMatrix(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	parser := realParser(t)

	cases := []struct {
		text       string
		wantKind   intent.Kind
		wantAmount int64
	}{
		{"ifood 58 reais", intent.KindLogExpense, 5800},
		{"gastei 200 no mercado com nubank", intent.KindLogExpense, 20000},
		{"paguei 1.250,90 parcelado no crédito", intent.KindLogExpense, 125090},
		{"comprei um tênis por 350", intent.KindLogExpense, 35000},
		{"uber 23,90 agora", intent.KindLogExpense, 2390},
		{"recebi meu salário de 16.400", intent.KindLogIncome, 1640000},
		{"caiu 3500 de freela na conta", intent.KindLogIncome, 350000},
		{"ganhei 200 de presente", intent.KindLogIncome, 20000},
		{"quanto gastei em Prazeres?", intent.KindQueryCategory, 0},
		{"como tá meu gasto com alimentação?", intent.KindQueryCategory, 0},
		{"como está minha meta de viagem?", intent.KindQueryGoal, 0},
		{"quanto falta pra meta da casa?", intent.KindQueryGoal, 0},
		{"qual a fatura do nubank?", intent.KindQueryCard, 0},
		{"quanto tá no cartão itaú?", intent.KindQueryCard, 0},
		{"resumo do mês", intent.KindMonthlySummary, 0},
		{"como ficou meu orçamento?", intent.KindMonthlySummary, 0},
		{"fechamento de 2026-05", intent.KindMonthlySummary, 0},
		{"como estou me saindo?", intent.KindHowAmIDoing, 0},
		{"minha saúde financeira tá ok?", intent.KindHowAmIDoing, 0},
		{"quero configurar meu orçamento", intent.KindConfigureBudget, 0},
		{"vamos montar meu planejamento financeiro", intent.KindConfigureBudget, 0},
		{"oi, tudo bem?", intent.KindUnknown, 0},
		{"qual a capital da França?", intent.KindUnknown, 0},
	}

	kindHits, amountHits, amountTotal := 0, 0, 0
	coreHits, coreTotal := 0, 0
	var misses []string

	for _, tc := range cases {
		out, err := parser.Execute(context.Background(), usecases.ParseInboundInput{UserID: uuid.New(), Text: tc.text})
		require.NoError(t, err)
		got := out.Intent.Kind()
		kindOK := got == tc.wantKind
		if kindOK {
			kindHits++
		} else {
			misses = append(misses, tc.text+" => esperado "+tc.wantKind.String()+", obtido "+got.String())
		}
		if tc.wantKind == intent.KindLogExpense || tc.wantKind == intent.KindLogIncome {
			coreTotal++
			if kindOK {
				coreHits++
			}
		}
		amountNote := ""
		if tc.wantAmount > 0 {
			amountTotal++
			if out.Intent.AmountCents() == tc.wantAmount {
				amountHits++
			} else {
				amountNote = " [AMOUNT esperado " + strconv.FormatInt(tc.wantAmount, 10) + " obtido " + strconv.FormatInt(out.Intent.AmountCents(), 10) + "]"
			}
		}
		status := "OK"
		if !kindOK {
			status = "MISS"
		}
		t.Logf("[%s] %-45q => %s (amount=%d)%s", status, tc.text, got.String(), out.Intent.AmountCents(), amountNote)
	}

	overall := 100 * float64(kindHits) / float64(len(cases))
	t.Logf("RECONHECIMENTO GERAL DE INTENT: %d/%d (%.1f%%)", kindHits, len(cases), overall)
	t.Logf("RECONHECIMENTO CORE (log_expense/log_income): %d/%d", coreHits, coreTotal)
	t.Logf("EXTRAÇÃO DE VALOR: %d/%d (%.1f%%)", amountHits, amountTotal, 100*float64(amountHits)/float64(amountTotal))
	for _, m := range misses {
		t.Logf("  MISS: %s", m)
	}
	require.Equal(t, coreTotal, coreHits, "core (gasto/ganho) deve ser 100%% — é o caminho que persiste")
	require.Equal(t, amountTotal, amountHits, "extração de valor deve ser 100%%")
	require.GreaterOrEqual(t, overall, 90.0, "reconhecimento geral abaixo do piso de 90%% — ver MISS acima")
}

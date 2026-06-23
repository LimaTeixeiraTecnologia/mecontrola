//go:build integration

package e2e_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

func derefString(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}

func derefInt(value *int) string {
	if value == nil {
		return "<nil>"
	}
	return strconv.Itoa(*value)
}

func productionParser(t *testing.T) *usecases.ParseInbound {
	t.Helper()
	model := os.Getenv("AGENT_LLM_PRIMARY_MODEL")
	if model == "" {
		cfg, err := configs.LoadConfig("../../..")
		require.NoError(t, err)
		model = cfg.AgentConfig.PrimaryModel
	}
	require.NotEmpty(t, model, "AGENT_LLM_PRIMARY_MODEL ausente")
	slug, err := valueobjects.NewModelSlug(model)
	require.NoError(t, err)
	t.Logf("modelo de producao: %s", slug.String())
	return realParserForModel(t, slug)
}

func TestParseInbound_RealLLM_NewKinds_ProductionModel(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	parser := productionParser(t)

	t.Run("update_card", func(t *testing.T) {
		cases := []struct {
			text         string
			wantNickname string
			wantDueDay   int
		}{
			{"muda o apelido do meu cartão Nubank pra Roxinho", "roxinho", 0},
			{"renomeia o cartão Nubank para Roxinho", "roxinho", 0},
			{"troca o vencimento do cartão Itaú pro dia 10", "", 10},
			{"o cartão Itaú agora vence no dia 10", "", 10},
		}
		for _, tc := range cases {
			out := parseUntilNewKind(t, parser, tc.text, intent.KindUpdateCard, func(i intent.Intent) bool {
				if tc.wantNickname != "" {
					return i.NicknamePtr() != nil && normalizeName(*i.NicknamePtr()) == tc.wantNickname
				}
				return i.DueDayPtr() != nil && *i.DueDayPtr() == tc.wantDueDay
			})
			require.Equalf(t, intent.KindUpdateCard, out.Intent.Kind(), "frase %q: kind divergente (got=%s)", tc.text, out.Intent.Kind().String())
			if tc.wantNickname != "" {
				require.NotNilf(t, out.Intent.NicknamePtr(), "frase %q: new_nickname ausente", tc.text)
				require.Equalf(t, tc.wantNickname, normalizeName(*out.Intent.NicknamePtr()), "frase %q: novo apelido divergente", tc.text)
			} else {
				require.NotNilf(t, out.Intent.DueDayPtr(), "frase %q: new_due_day ausente", tc.text)
				require.Equalf(t, tc.wantDueDay, *out.Intent.DueDayPtr(), "frase %q: novo vencimento divergente", tc.text)
			}
			t.Logf("[update_card OK] %q -> card=%q nickname=%s dueDay=%s", tc.text, out.Intent.CardName(), derefString(out.Intent.NicknamePtr()), derefInt(out.Intent.DueDayPtr()))
		}
	})

	t.Run("delete_card", func(t *testing.T) {
		cases := []string{
			"apaga o cartão C6",
			"remove o cartão C6",
		}
		for _, text := range cases {
			out := parseUntilNewKind(t, parser, text, intent.KindDeleteCard, func(i intent.Intent) bool {
				return i.CardName() != ""
			})
			require.Equalf(t, intent.KindDeleteCard, out.Intent.Kind(), "frase %q: kind divergente (got=%s)", text, out.Intent.Kind().String())
			require.NotEmptyf(t, out.Intent.CardName(), "frase %q: card_name ausente", text)
			t.Logf("[delete_card OK] %q -> card=%q", text, out.Intent.CardName())
		}
	})

	t.Run("edit_category_percentage", func(t *testing.T) {
		cases := []struct {
			text           string
			wantPercentage int
		}{
			{"coloca 30% em prazeres", 30},
			{"define 30 por cento para prazeres", 30},
		}
		for _, tc := range cases {
			out := parseUntilNewKind(t, parser, tc.text, intent.KindEditCategoryPercentage, func(i intent.Intent) bool {
				return i.Percentage() == tc.wantPercentage
			})
			require.Equalf(t, intent.KindEditCategoryPercentage, out.Intent.Kind(), "frase %q: kind divergente (got=%s)", tc.text, out.Intent.Kind().String())
			require.Equalf(t, tc.wantPercentage, out.Intent.Percentage(), "frase %q: percentual divergente", tc.text)
			require.NotEmptyf(t, out.Intent.CategoryName(), "frase %q: category_name ausente", tc.text)
			t.Logf("[edit_category_percentage OK] %q -> category=%q percentage=%d", tc.text, out.Intent.CategoryName(), out.Intent.Percentage())
		}
	})
}

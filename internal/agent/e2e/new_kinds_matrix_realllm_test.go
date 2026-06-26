//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

const matrixAttemptsPerPhrase = 3

type matrixPhrase struct {
	text      string
	kind      intent.Kind
	accept    func(intent.Intent) bool
	expect    string
	isNewName bool
}

type matrixModel struct {
	slug       valueobjects.ModelSlug
	role       string
	hard       bool
	knownFlak  bool
	skip       bool
	skipReason string
}

type phraseResult struct {
	hits     int
	attempts int
	lastKind intent.Kind
	lastNote string
	kindHits int
}

func newKindsMatrixPhrases() []matrixPhrase {
	intPtrEq := func(get func(intent.Intent) *int, want int) func(intent.Intent) bool {
		return func(i intent.Intent) bool {
			p := get(i)
			return p != nil && *p == want
		}
	}
	namePtrEq := func(get func(intent.Intent) *string, want string) func(intent.Intent) bool {
		return func(i intent.Intent) bool {
			p := get(i)
			return p != nil && normalizeName(*p) == want
		}
	}
	pctEq := func(want int) func(intent.Intent) bool {
		return func(i intent.Intent) bool {
			return i.Percentage() == want && i.CategoryName() != ""
		}
	}

	return []matrixPhrase{
		{"muda o apelido do meu cartão Nubank pra Roxinho", intent.KindUpdateCard, namePtrEq(intent.Intent.NicknamePtr, "roxinho"), "update_card nickname=roxinho", false},
		{"renomeia o apelido do cartão Inter pra Laranjinha", intent.KindUpdateCard, namePtrEq(intent.Intent.NicknamePtr, "laranjinha"), "update_card nickname=laranjinha", false},
		{"troca o vencimento do cartão Itaú pro dia 10", intent.KindUpdateCard, intPtrEq(intent.Intent.DueDayPtr, 10), "update_card due_day=10", false},
		{"o cartão Bradesco agora vence no dia 25", intent.KindUpdateCard, intPtrEq(intent.Intent.DueDayPtr, 25), "update_card due_day=25", false},
		{"ajusta o fechamento do cartão Santander pro dia 5", intent.KindUpdateCard, intPtrEq(intent.Intent.ClosingDayPtr, 5), "update_card closing_day=5", false},
		{"muda o dia de fechar a fatura do cartão C6 pra 28", intent.KindUpdateCard, intPtrEq(intent.Intent.ClosingDayPtr, 28), "update_card closing_day=28", false},
		{"renomeia o cartão Nubank para Cartão Principal", intent.KindUpdateCard, namePtrEq(intent.Intent.NamePtr, "cartão principal"), "update_card new_name=cartão principal", true},
		{"altera o nome do cartão Itaú para Itau Black", intent.KindUpdateCard, namePtrEq(intent.Intent.NamePtr, "itau black"), "update_card new_name=itau black", true},

		{"apaga o cartão C6", intent.KindDeleteCard, func(i intent.Intent) bool { return normalizeName(i.CardName()) != "" }, "delete_card card=c6", false},
		{"remove o cartão Nubank", intent.KindDeleteCard, func(i intent.Intent) bool { return normalizeName(i.CardName()) != "" }, "delete_card card=nubank", false},
		{"pode excluir o cartão Itaú", intent.KindDeleteCard, func(i intent.Intent) bool { return normalizeName(i.CardName()) != "" }, "delete_card card=itaú", false},
		{"tira o cartão Bradesco da minha lista", intent.KindDeleteCard, func(i intent.Intent) bool { return normalizeName(i.CardName()) != "" }, "delete_card card=bradesco", false},
		{"não quero mais o cartão Inter, deleta ele", intent.KindDeleteCard, func(i intent.Intent) bool { return normalizeName(i.CardName()) != "" }, "delete_card card=inter", false},

		{"coloca 30% em prazeres", intent.KindEditCategoryPercentage, pctEq(30), "edit_pct prazeres=30", false},
		{"define 30 por cento para prazeres", intent.KindEditCategoryPercentage, pctEq(30), "edit_pct prazeres=30", false},
		{"quero deixar lazer com 15%", intent.KindEditCategoryPercentage, pctEq(15), "edit_pct lazer=15", false},
		{"ajusta moradia para 50 por cento", intent.KindEditCategoryPercentage, pctEq(50), "edit_pct moradia=50", false},
		{"zera o percentual da categoria viagem, coloca 0%", intent.KindEditCategoryPercentage, pctEq(0), "edit_pct viagem=0", false},
		{"bota investimentos em 100 por cento", intent.KindEditCategoryPercentage, pctEq(100), "edit_pct investimentos=100", false},
	}
}

func matrixModels(t *testing.T) []matrixModel {
	t.Helper()
	primary := os.Getenv("AGENT_LLM_PRIMARY_MODEL")
	fallbackRaw := os.Getenv("AGENT_LLM_FALLBACK_MODELS")
	if primary == "" {
		cfg, err := configs.LoadConfig("../../..")
		require.NoError(t, err)
		primary = cfg.AgentConfig.PrimaryModel
		if fallbackRaw == "" {
			fallbackRaw = cfg.AgentConfig.FallbackModels
		}
	}
	require.NotEmpty(t, primary, "AGENT_LLM_PRIMARY_MODEL ausente")

	const mistralSlug = "mistralai/mistral-small-3.2-24b-instruct"
	const haikuSlug = "anthropic/claude-haiku-4.5"
	const incompatibleReason = "incompativel com response_format json_schema: retorna kind=unknown para intents estruturados (fato arquitetural, nao conveniencia)"

	hardModels := map[string]bool{
		valueobjects.ModelSlugGeminiFlashLite().String(): true,
		mistralSlug: true,
	}
	skipReason := map[string]string{
		valueobjects.ModelSlugGPT5Nano().String(): incompatibleReason,
		haikuSlug: incompatibleReason,
	}
	flaky := map[string]bool{
		valueobjects.ModelSlugGPT5Nano().String(): true,
		haikuSlug:   true,
		mistralSlug: false,
	}

	build := func(slug valueobjects.ModelSlug, role string) matrixModel {
		s := slug.String()
		return matrixModel{
			slug:       slug,
			role:       role,
			hard:       hardModels[s] && skipReason[s] == "",
			knownFlak:  flaky[s],
			skip:       skipReason[s] != "",
			skipReason: skipReason[s],
		}
	}

	seen := map[string]bool{}
	models := make([]matrixModel, 0, 4)

	primarySlug, err := valueobjects.NewModelSlug(primary)
	require.NoError(t, err)
	models = append(models, build(primarySlug, "primary"))
	seen[primarySlug.String()] = true

	for _, raw := range strings.Split(fallbackRaw, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		slug, ferr := valueobjects.NewModelSlug(raw)
		require.NoError(t, ferr)
		if seen[slug.String()] {
			continue
		}
		seen[slug.String()] = true
		models = append(models, build(slug, "fallback"))
	}

	for _, raw := range []string{
		valueobjects.ModelSlugGeminiFlashLite().String(),
		mistralSlug,
		valueobjects.ModelSlugGPT5Nano().String(),
		haikuSlug,
	} {
		slug, serr := valueobjects.NewModelSlug(raw)
		require.NoError(t, serr)
		if seen[slug.String()] {
			continue
		}
		seen[slug.String()] = true
		models = append(models, build(slug, "allowlist"))
	}

	return models
}

func runMatrixPhrase(parser *usecases.ParseInbound, p matrixPhrase) phraseResult {
	res := phraseResult{attempts: matrixAttemptsPerPhrase}
	for attempt := 0; attempt < matrixAttemptsPerPhrase; attempt++ {
		out, err := parser.Execute(context.Background(), usecases.ParseInboundInput{UserID: uuid.New(), Text: p.text})
		if err != nil {
			res.lastNote = "err: " + err.Error()
			continue
		}
		res.lastKind = out.Intent.Kind()
		if out.Intent.Kind() == p.kind {
			res.kindHits++
		}
		if out.Intent.Kind() == p.kind && p.accept(out.Intent) {
			res.hits++
		} else {
			res.lastNote = fmt.Sprintf("got kind=%s pct=%d card=%q cat=%q", out.Intent.Kind().String(), out.Intent.Percentage(), out.Intent.CardName(), out.Intent.CategoryName())
		}
	}
	return res
}

func TestParseInbound_RealLLM_NewKinds_MatrixAllModels(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	phrases := newKindsMatrixPhrases()
	models := matrixModels(t)

	type modelSummary struct {
		role     string
		hard     bool
		flaky    bool
		hits     int
		attempts int
		perKind  map[intent.Kind]struct{ hits, attempts int }
	}
	summaries := make([]struct {
		slug string
		s    modelSummary
	}, 0, len(models))

	const minHits = 2

	for _, m := range models {
		sum := modelSummary{role: m.role, hard: m.hard, flaky: m.knownFlak, perKind: map[intent.Kind]struct{ hits, attempts int }{}}

		t.Run(m.slug.String(), func(t *testing.T) {
			if m.skip {
				t.Skipf("modelo %s pulado: %s", m.slug.String(), m.skipReason)
			}
			parser := realParserForModel(t, m.slug)
			for _, p := range phrases {
				res := runMatrixPhrase(parser, p)
				sum.hits += res.hits
				sum.attempts += res.attempts
				pk := sum.perKind[p.kind]
				pk.hits += res.hits
				pk.attempts += res.attempts
				sum.perKind[p.kind] = pk

				status := "OK"
				if res.hits < res.attempts {
					status = "MISS"
				}
				tag := ""
				if p.isNewName {
					tag = " [new_name]"
				}
				t.Logf("[%s] model=%s kind=%s%s phrase=%q -> %d/%d hits kind=%d/%d (%s) note=%q",
					status, m.slug.String(), p.kind.String(), tag, p.text, res.hits, res.attempts, res.kindHits, res.attempts, p.expect, res.lastNote)

				if !m.hard {
					continue
				}

				if m.role == "primary" {
					require.GreaterOrEqualf(t, res.kindHits, minHits,
						"PRIMARY model=%s abaixo da maioria na classificacao de kind em %q (kind=%d/%d, minimo=%d, esperado=%s, note=%s)",
						m.slug.String(), p.text, res.kindHits, res.attempts, minHits, p.kind.String(), res.lastNote)
					require.GreaterOrEqualf(t, res.hits, minHits,
						"PRIMARY model=%s abaixo de %d/%d para %q (got=%d, esperado=%s, note=%s)",
						m.slug.String(), minHits, res.attempts, p.text, res.hits, p.expect, res.lastNote)
					continue
				}

				require.Greaterf(t, res.hits, 0,
					"FALLBACK model=%s falhou TODAS as %d tentativas em %q (best-effort: exige >=1; mistral e flaky por design, ver docs/runbooks/agent-parser-policy.md) (esperado=%s, note=%s)",
					m.slug.String(), res.attempts, p.text, p.expect, res.lastNote)
			}
		})

		summaries = append(summaries, struct {
			slug string
			s    modelSummary
		}{m.slug.String(), sum})
	}

	sort.Slice(summaries, func(i, j int) bool { return summaries[i].slug < summaries[j].slug })

	t.Log("==================== MATRIZ REAL-LLM: NEW KINDS ====================")
	t.Logf("attempts/frase=%d | total frases=%d", matrixAttemptsPerPhrase, len(phrases))
	for _, item := range summaries {
		rate := 0.0
		if item.s.attempts > 0 {
			rate = 100 * float64(item.s.hits) / float64(item.s.attempts)
		}
		tag := "fallback"
		if item.s.hard {
			tag = "PRIMARY(hard)"
		} else if item.s.flaky {
			tag = "flaky(informativo)"
		}
		t.Logf("MODEL %-45s [%s] total=%.1f%% (%d/%d)", item.slug, tag, rate, item.s.hits, item.s.attempts)
		kinds := []intent.Kind{intent.KindUpdateCard, intent.KindDeleteCard, intent.KindEditCategoryPercentage}
		for _, k := range kinds {
			pk := item.s.perKind[k]
			if pk.attempts == 0 {
				continue
			}
			kr := 100 * float64(pk.hits) / float64(pk.attempts)
			t.Logf("    %-28s %.1f%% (%d/%d)", k.String(), kr, pk.hits, pk.attempts)
		}
	}
	t.Log("===================================================================")
}

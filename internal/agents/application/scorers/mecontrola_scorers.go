package scorers

import (
	"context"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

var mecontrolaFinancialTools = []string{
	"register_expense",
	"register_income",
	"create_recurrence",
	"query_month",
	"get_transaction",
	"search_transactions",
	"list_cards",
	"get_card",
	"count_cards",
	"best_purchase_day",
	"query_card_invoice",
	"list_recurrences",
	"update_recurrence",
	"delete_recurrence",
	"list_categories",
	"classify_category",
	"query_plan",
	"adjust_allocation",
	"suggest_allocation",
	"edit_entry",
	"delete_entry",
	"update_card",
}

var mecontrolaFinancialKeywords = []string{
	"R$", "registrei", "lançamento", "resumo", "categoria", "despesa",
	"receita", "orçamento", "mês", "cartão", "parcela", "saldo",
}

const categorizationScorerInstructions = `Você é um avaliador especializado em categorização financeira.
Avalie se a resposta do assistente categorizou ou mencionou uma categoria financeira plausível para o contexto do usuário.
Retorne um objeto JSON com os campos: score (número 0-1) e reason (string).
score=1.0: a categorização é totalmente plausível (ex: "mercado" → Custo Fixo/Alimentação)
score=0.5: a categorização é aceitável mas não é a mais precisa
score=0.0: a categorização está errada ou ausente quando deveria estar presente
Se a resposta não envolve categorização, retorne score=1.0.`

const toneAdherenceScorerInstructions = `Você é um avaliador do Tom de Voz da marca MeControla, um parceiro financeiro pessoal que fala com o usuário via WhatsApp.
O tom oficial é caloroso, direto e motivacional, sem jargão técnico, sem tom robótico ou genérico de assistente virtual.
Avalie se a resposta do assistente soa como o parceiro financeiro MeControla e não como um bot genérico.
Retorne um objeto JSON com os campos: score (número 0-1) e reason (string).
score=1.0: tom caloroso, direto, alinhado à marca, sem jargão técnico
score=0.5: tom aceitável mas burocrático ou impessoal em trechos
score=0.0: tom robótico, técnico, ou incoerente com a marca MeControla
Se a resposta é puramente informacional e curta (ex: pergunta de clarificação simples), avalie apenas se não é hostil ou robótica; nesse caso score mínimo aceitável é 0.7.`

type anyFinancialToolScorer struct {
	id    string
	tools []string
}

func (s *anyFinancialToolScorer) ID() string              { return s.id }
func (s *anyFinancialToolScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *anyFinancialToolScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	if len(s.tools) == 0 {
		return scorer.ScoreResult{Score: 1.0, Reason: "nenhuma ferramenta esperada; trivialmente correto"}, nil
	}
	toolSet := make(map[string]bool, len(s.tools))
	for _, t := range s.tools {
		toolSet[t] = true
	}
	called := 0
	for _, tc := range sample.ToolCalls {
		if toolSet[tc.Name] {
			called++
		}
	}
	if called > 0 {
		return scorer.ScoreResult{
			Score:    1.0,
			Reason:   fmt.Sprintf("invocou %d ferramenta(s) financeira(s)", called),
			Metadata: map[string]any{"called": called},
		}, nil
	}
	return scorer.ScoreResult{
		Score:  0.0,
		Reason: "nenhuma ferramenta financeira invocada",
		Metadata: map[string]any{
			"expected_any_of": s.tools,
		},
	}, nil
}

type expectedToolScorer struct {
	id           string
	expectedTool string
}

func (s *expectedToolScorer) ID() string              { return s.id }
func (s *expectedToolScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *expectedToolScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	for _, tc := range sample.ToolCalls {
		if tc.Name == s.expectedTool {
			return scorer.ScoreResult{
				Score:  1.0,
				Reason: fmt.Sprintf("chamou a tool esperada: %s", s.expectedTool),
				Metadata: map[string]any{
					"expected": s.expectedTool,
					"called":   tc.Name,
				},
			}, nil
		}
	}
	called := make([]string, len(sample.ToolCalls))
	for i, tc := range sample.ToolCalls {
		called[i] = tc.Name
	}
	return scorer.ScoreResult{
		Score:  0.0,
		Reason: fmt.Sprintf("tool esperada %q não foi chamada; chamadas: %v", s.expectedTool, called),
		Metadata: map[string]any{
			"expected": s.expectedTool,
			"called":   called,
		},
	}, nil
}

type textKeywordsScorer struct {
	id       string
	keywords []string
}

func (s *textKeywordsScorer) ID() string              { return s.id }
func (s *textKeywordsScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *textKeywordsScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	if len(s.keywords) == 0 || sample.Output == "" {
		return scorer.ScoreResult{Score: 0.0, Reason: "output vazio ou sem palavras-chave esperadas"}, nil
	}
	lower := strings.ToLower(sample.Output)
	matched := 0
	for _, kw := range s.keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			matched++
		}
	}
	score := float64(matched) / float64(len(s.keywords))
	return scorer.ScoreResult{
		Score:  score,
		Reason: fmt.Sprintf("encontrou %d/%d palavras-chave financeiras", matched, len(s.keywords)),
		Metadata: map[string]any{
			"matched": matched,
			"total":   len(s.keywords),
		},
	}, nil
}

func NewFinancialToolCallAccuracyScorer() scorer.Scorer {
	return &anyFinancialToolScorer{id: "tool-call-accuracy", tools: mecontrolaFinancialTools}
}

func NewExpectedToolScorer(toolName string) scorer.Scorer {
	return &expectedToolScorer{
		id:           "expected-tool:" + toolName,
		expectedTool: toolName,
	}
}

func NewFinancialCompletenessScorer() scorer.Scorer {
	return &textKeywordsScorer{id: "completeness", keywords: mecontrolaFinancialKeywords}
}

func NewCategorizationScorer(provider llm.Provider) scorer.Scorer {
	return scorer.NewLLMJudgedScorer("categorization", provider, categorizationScorerInstructions)
}

func NewToneAdherenceScorer(provider llm.Provider) scorer.Scorer {
	return scorer.NewLLMJudgedScorer("tone_adherence", provider, toneAdherenceScorerInstructions)
}

func BuildMeControlaScorers(provider llm.Provider) []scorer.ScorerEntry {
	return []scorer.ScorerEntry{
		scorer.NewScorerEntry(NewFinancialToolCallAccuracyScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewFinancialCompletenessScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewCategorizationScorer(provider), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewNoEmptyAnswerScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewWhatsAppFormatScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewNoInternalTermsScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewVerbatimRequiredScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewVerbatimToneAdherenceScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewToneAdherenceScorer(provider), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewNoDuplicateWriteScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewNoHallucinationScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewWritePersistenceAccuracyScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewRequiredArgsScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewPaymentMethodProvenanceScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewMonthReferenceCorrectnessScorer(), scorer.AlwaysSample()),
	}
}

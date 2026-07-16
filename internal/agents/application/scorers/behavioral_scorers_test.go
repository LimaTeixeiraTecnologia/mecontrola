package scorers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

type BehavioralScorersSuite struct {
	suite.Suite
	ctx context.Context
}

func TestBehavioralScorersSuite(t *testing.T) {
	suite.Run(t, new(BehavioralScorersSuite))
}

func (s *BehavioralScorersSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *BehavioralScorersSuite) TestNoEmptyAnswerScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 0 quando output esta vazio",
			args: args{sample: scorer.RunSample{Output: ""}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando output e apenas espacos",
			args: args{sample: scorer.RunSample{Output: "   "}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 quando output tem conteudo",
			args: args{sample: scorer.RunSample{Output: "Registrei sua despesa."}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewNoEmptyAnswerScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
			s.Equal(scorer.ScorerKindCodeBased, sc.Kind())
			s.Equal("no_empty_answer", sc.ID())
		})
	}
}

func (s *BehavioralScorersSuite) TestWhatsAppFormatScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1 para resposta em texto simples",
			args: args{sample: scorer.RunSample{Output: "Registrei sua despesa de R$ 50,00."}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando contem header markdown",
			args: args{sample: scorer.RunSample{Output: "## Resumo do mês\nGasto total: R$ 500,00"}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando contem tabela markdown",
			args: args{sample: scorer.RunSample{Output: "Categoria | Valor\n|---|---|\nMercado | R$ 50,00"}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewWhatsAppFormatScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *BehavioralScorersSuite) TestNoInternalTermsScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1 para resposta sem termos internos",
			args: args{sample: scorer.RunSample{Output: "Registrei sua despesa de R$ 50,00 na categoria Alimentação."}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando vaza termo tecnico",
			args: args{sample: scorer.RunSample{Output: "Erro: internal server error ao processar."}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando vaza nome de tool",
			args: args{sample: scorer.RunSample{Output: "Chamando register_expense para você."}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewNoInternalTermsScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *BehavioralScorersSuite) TestVerbatimRequiredScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1 quando nenhum verbatim e esperado",
			args: args{sample: scorer.RunSample{Output: "resposta qualquer"}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 quando output contem o verbatim esperado",
			args: args{sample: scorer.RunSample{
				Output:   "Prefixo. Confirme o valor de R$ 100,00. Sufixo.",
				Metadata: map[string]any{"verbatim_text": "Confirme o valor de R$ 100,00."},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando output nao contem o verbatim esperado",
			args: args{sample: scorer.RunSample{
				Output:   "resposta diferente",
				Metadata: map[string]any{"verbatim_text": "texto obrigatório"},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewVerbatimRequiredScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *BehavioralScorersSuite) TestVerbatimToneAdherenceScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1 quando output esta vazio",
			args: args{sample: scorer.RunSample{Output: ""}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 quando usa negrito simples e emoji oficial",
			args: args{sample: scorer.RunSample{
				Output:   "✅ Encontrei este lançamento:\n\n💰 Valor: *R$ 100,00*\n\nPosso registrar?",
				Metadata: map[string]any{"requires_brand_emoji": true},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando usa negrito duplo",
			args: args{sample: scorer.RunSample{
				Output: "Prontinho! **R$ 100,00** registrado.",
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando marcacao de negrito esta mal formada",
			args: args{sample: scorer.RunSample{
				Output: "Valor *R$ 100,00 sem fechamento",
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando exige emoji da marca e nenhum esta presente",
			args: args{sample: scorer.RunSample{
				Output:   "Lançamento registrado com sucesso.",
				Metadata: map[string]any{"requires_brand_emoji": true},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewVerbatimToneAdherenceScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *BehavioralScorersSuite) TestNoDuplicateWriteScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1 sem write-tools chamadas",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{{Name: "list_cards"}}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 com uma unica chamada de write-tool",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{
				{Name: "register_expense", Args: map[string]any{"amountCents": float64(5000)}},
			}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando a mesma write-tool e chamada duas vezes com os mesmos args",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{
				{Name: "register_expense", Args: map[string]any{"amountCents": float64(5000), "description": "mercado"}},
				{Name: "register_expense", Args: map[string]any{"amountCents": float64(5000), "description": "mercado"}},
			}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 quando a mesma write-tool e chamada com args diferentes",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{
				{Name: "register_expense", Args: map[string]any{"amountCents": float64(5000), "description": "mercado"}},
				{Name: "register_expense", Args: map[string]any{"amountCents": float64(3000), "description": "farmácia"}},
			}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewNoDuplicateWriteScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *BehavioralScorersSuite) TestNoHallucinationScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1 sem marcador de sucesso",
			args: args{sample: scorer.RunSample{Output: "Como posso ajudar?"}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 com marcador de sucesso respaldado por write-tool efetivada (routed)",
			args: args{sample: scorer.RunSample{
				Output:    "Registrei sua despesa de R$ 50,00.",
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: "routed"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 com marcador de sucesso respaldado por write-tool reconciled",
			args: args{sample: scorer.RunSample{
				Output:    "Registrei sua despesa de R$ 50,00.",
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: "reconciled"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 com marcador de sucesso e write-tool com usecaseError",
			args: args{sample: scorer.RunSample{
				Output:    "Registrei sua despesa de R$ 50,00.",
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: "usecaseError"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 com marcador de sucesso e write-tool com replay",
			args: args{sample: scorer.RunSample{
				Output:    "Registrei sua despesa de R$ 50,00.",
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: "replay"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 com marcador de sucesso e write-tool com clarify",
			args: args{sample: scorer.RunSample{
				Output:    "Registrei sua despesa de R$ 50,00.",
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: "clarify"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 com marcador de sucesso sem write-tool chamada",
			args: args{sample: scorer.RunSample{
				Output:    "Registrei sua despesa de R$ 50,00.",
				ToolCalls: []scorer.ToolCallRecord{{Name: "list_cards"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewNoHallucinationScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *BehavioralScorersSuite) TestRequiredArgsScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1 sem write-tools chamadas",
			args: args{sample: scorer.RunSample{ToolCalls: nil}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 quando register_expense tem todos os args obrigatorios",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{
				{Name: "register_expense", Args: map[string]any{
					"amountCents": float64(5000), "description": "mercado", "paymentMethod": "pix",
				}},
			}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando register_expense esta sem paymentMethod",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{
				{Name: "register_expense", Args: map[string]any{
					"amountCents": float64(5000), "description": "mercado",
				}},
			}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando args nao foram capturados",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{
				{Name: "delete_entry", Args: nil},
			}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewRequiredArgsScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *BehavioralScorersSuite) TestMonthReferenceCorrectnessScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1 sem tools de mes chamadas",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense"}}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 quando query_plan tem monthRefKind valido named_without_year",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{
				{Name: "query_plan", Args: map[string]any{"monthRefKind": "named_without_year"}},
			}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando query_month nao tem monthRefKind",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{
				{Name: "query_month", Args: map[string]any{}},
			}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando monthRefKind e invalido",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{
				{Name: "create_budget", Args: map[string]any{"monthRefKind": "yesterday"}},
			}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewMonthReferenceCorrectnessScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *BehavioralScorersSuite) TestExpectedToolOracleScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1 trivial quando metadata nao tem expected_tool",
			args: args{sample: scorer.RunSample{ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense"}}}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1 quando a tool esperada foi chamada",
			args: args{sample: scorer.RunSample{
				Metadata:  map[string]any{"expected_tool": "register_expense"},
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0 quando a tool esperada nao foi chamada",
			args: args{sample: scorer.RunSample{
				Metadata:  map[string]any{"expected_tool": "register_expense"},
				ToolCalls: []scorer.ToolCallRecord{{Name: "query_month"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewExpectedToolOracleScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
			s.Equal(scorer.ScorerKindCodeBased, sc.Kind())
			s.Equal("expected_tool", sc.ID())
		})
	}
}

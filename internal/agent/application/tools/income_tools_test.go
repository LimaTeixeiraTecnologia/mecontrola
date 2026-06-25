package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type stubIncomeSummaryReader struct {
	result IncomeSummaryResult
	err    error
}

func (s *stubIncomeSummaryReader) Execute(_ context.Context, _ IncomeSummaryInput) (IncomeSummaryResult, error) {
	return s.result, s.err
}

type QueryIncomeSummarySuite struct {
	suite.Suite
	ctx  context.Context
	loc  *time.Location
	tool *QueryIncomeSummary
}

func TestQueryIncomeSummarySuite(t *testing.T) {
	suite.Run(t, new(QueryIncomeSummarySuite))
}

func (s *QueryIncomeSummarySuite) SetupTest() {
	s.ctx = context.Background()
	s.loc = time.UTC
}

func (s *QueryIncomeSummarySuite) newTool(reader IncomeSummaryReader) *QueryIncomeSummary {
	obs := fake.NewProvider()
	counter := obs.Metrics().Counter("test_routed_total", "", "1")
	recorder := NewRecorder(counter)
	return NewQueryIncomeSummary(recorder, reader, s.loc, obs)
}

func (s *QueryIncomeSummarySuite) toolInput(refMonth string) ToolInput {
	i, _ := intent.NewQueryIncomeSummary(refMonth)
	return ToolInput{
		UserID:  uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Channel: "whatsapp",
		Intent:  i,
	}
}

func (s *QueryIncomeSummarySuite) TestExecute() {
	type args struct {
		refMonth string
	}
	type dependencies struct {
		reader IncomeSummaryReader
	}
	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result ToolResult, err error)
	}{
		{
			name: "deve retornar resumo formatado quando ha entradas",
			args: args{refMonth: "2026-06"},
			dependencies: dependencies{
				reader: &stubIncomeSummaryReader{
					result: IncomeSummaryResult{
						RefMonth:   "2026-06",
						TotalCents: 500000,
						Sources: []IncomeSourceView{
							{Description: "Salário", AmountCents: 500000},
						},
					},
				},
			},
			expect: func(result ToolResult, err error) {
				s.NoError(err)
				s.Equal(OutcomeRouted, result.Outcome)
				s.Equal(intent.KindQueryIncomeSummary, result.Kind)
				s.Contains(result.Reply, "R$")
			},
		},
		{
			name: "deve retornar mensagem vazia quando nao ha entradas",
			args: args{refMonth: "2026-06"},
			dependencies: dependencies{
				reader: &stubIncomeSummaryReader{
					result: IncomeSummaryResult{RefMonth: "2026-06", TotalCents: 0},
				},
			},
			expect: func(result ToolResult, err error) {
				s.NoError(err)
				s.Equal(OutcomeRouted, result.Outcome)
				s.Equal(intent.KindQueryIncomeSummary, result.Kind)
				s.Contains(result.Reply, "Nenhuma entrada")
			},
		},
		{
			name: "deve retornar OutcomeUsecaseError quando reader falha",
			args: args{refMonth: "2026-06"},
			dependencies: dependencies{
				reader: &stubIncomeSummaryReader{err: errors.New("falha no banco")},
			},
			expect: func(result ToolResult, err error) {
				s.NoError(err)
				s.Equal(OutcomeUsecaseError, result.Outcome)
				s.Equal(intent.KindQueryIncomeSummary, result.Kind)
			},
		},
		{
			name:         "deve retornar OutcomeMissingResolver quando reader e nil",
			args:         args{refMonth: "2026-06"},
			dependencies: dependencies{reader: nil},
			expect: func(result ToolResult, err error) {
				s.NoError(err)
				s.Equal(OutcomeMissingResolver, result.Outcome)
				s.Equal(intent.KindQueryIncomeSummary, result.Kind)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			t := s.newTool(scenario.dependencies.reader)
			result, err := t.Execute(s.ctx, s.toolInput(scenario.args.refMonth))
			scenario.expect(result, err)
		})
	}
}

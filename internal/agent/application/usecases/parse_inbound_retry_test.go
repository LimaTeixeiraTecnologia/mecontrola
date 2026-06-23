package usecases

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type countingInterpreter struct {
	resp  interfaces.LLMResponse
	err   error
	calls int
}

func (c *countingInterpreter) Interpret(_ context.Context, _ interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	c.calls++
	return c.resp, c.err
}

type ParseInboundRetrySuite struct {
	suite.Suite
	ctx context.Context
	obs *fake.Provider
}

func TestParseInboundRetrySuite(t *testing.T) {
	suite.Run(t, new(ParseInboundRetrySuite))
}

func (s *ParseInboundRetrySuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
}

func (s *ParseInboundRetrySuite) retryOutcomes() []string {
	metrics, ok := s.obs.Metrics().(*fake.FakeMetrics)
	s.Require().True(ok)
	counter := metrics.GetCounter("agent_parse_retry_total")
	if counter == nil {
		return nil
	}
	outcomes := make([]string, 0)
	for _, v := range counter.GetValues() {
		for _, f := range v.Fields {
			if f.Key == "outcome" {
				outcomes = append(outcomes, f.StringValue())
			}
		}
	}
	return outcomes
}

func (s *ParseInboundRetrySuite) TestExecute() {
	type args struct {
		input ParseInboundInput
	}
	type dependencies struct {
		primary *countingInterpreter
		retry   *countingInterpreter
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out ParseInboundOutput, err error, d dependencies, outcomes []string)
	}{
		{
			name: "primario unknown + texto de comando + retry valido => resultado do retry (recovered)",
			args: args{input: ParseInboundInput{UserID: uuid.New(), Text: "apaga o cartão C6"}},
			dependencies: dependencies{
				primary: func() *countingInterpreter {
					return &countingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"unknown","raw_text":"apaga o cartão C6"}`)}}
				}(),
				retry: func() *countingInterpreter {
					return &countingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"delete_card","card_name":"C6"}`)}}
				}(),
			},
			expect: func(out ParseInboundOutput, err error, d dependencies, outcomes []string) {
				s.Require().NoError(err)
				s.Equal(intent.KindDeleteCard, out.Intent.Kind())
				s.Equal(1, d.retry.calls)
				s.Contains(outcomes, retryOutcomeRecovered)
			},
		},
		{
			name: "primario unknown + chitchat sem cues => retry nao chamado (skipped)",
			args: args{input: ParseInboundInput{UserID: uuid.New(), Text: "bom dia, tudo bem com você?"}},
			dependencies: dependencies{
				primary: func() *countingInterpreter {
					return &countingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"unknown","raw_text":"bom dia, tudo bem com você?"}`)}}
				}(),
				retry: func() *countingInterpreter {
					return &countingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"delete_card","card_name":"C6"}`)}}
				}(),
			},
			expect: func(out ParseInboundOutput, err error, d dependencies, outcomes []string) {
				s.Require().NoError(err)
				s.Equal(intent.KindUnknown, out.Intent.Kind())
				s.Equal(0, d.retry.calls)
				s.Contains(outcomes, retryOutcomeSkippedNotCmd)
			},
		},
		{
			name: "primario kind valido => retry nao chamado",
			args: args{input: ParseInboundInput{UserID: uuid.New(), Text: "gastei 50 no ifood"}},
			dependencies: dependencies{
				primary: func() *countingInterpreter {
					return &countingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"record_expense","amount_cents":5000,"merchant":"ifood"}`)}}
				}(),
				retry: func() *countingInterpreter {
					return &countingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"unknown"}`)}}
				}(),
			},
			expect: func(out ParseInboundOutput, err error, d dependencies, outcomes []string) {
				s.Require().NoError(err)
				s.Equal(intent.KindRecordExpense, out.Intent.Kind())
				s.Equal(0, d.retry.calls)
				s.Empty(outcomes)
			},
		},
		{
			name: "primario unknown + retry tambem unknown => retorna unknown (still_unknown)",
			args: args{input: ParseInboundInput{UserID: uuid.New(), Text: "troca o vencimento do cartão Itaú pro dia 10"}},
			dependencies: dependencies{
				primary: func() *countingInterpreter {
					return &countingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"unknown","raw_text":"troca o vencimento do cartão Itaú pro dia 10"}`)}}
				}(),
				retry: func() *countingInterpreter {
					return &countingInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte(`{"kind":"unknown","raw_text":"x"}`)}}
				}(),
			},
			expect: func(out ParseInboundOutput, err error, d dependencies, outcomes []string) {
				s.Require().NoError(err)
				s.Equal(intent.KindUnknown, out.Intent.Kind())
				s.Equal(1, d.retry.calls)
				s.Contains(outcomes, retryOutcomeStillUnknown)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.obs = fake.NewProvider()
			var obs observability.Observability = s.obs
			uc, err := NewParseInbound(scenario.dependencies.primary, scenario.dependencies.retry, 2000, obs)
			s.Require().NoError(err)
			out, execErr := uc.Execute(s.ctx, scenario.args.input)
			scenario.expect(out, execErr, scenario.dependencies, s.retryOutcomes())
		})
	}
}

package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	mocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases/mocks"
)

type ParseInboundConfidenceSuite struct {
	suite.Suite

	ctx         context.Context
	obs         observability.Observability
	interpreter *mocks.IntentInterpreter
}

func TestParseInboundConfidenceSuite(t *testing.T) {
	suite.Run(t, new(ParseInboundConfidenceSuite))
}

func (s *ParseInboundConfidenceSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.interpreter = mocks.NewIntentInterpreter(s.T())
}

func (s *ParseInboundConfidenceSuite) TestExecute() {
	type args struct {
		input ParseInboundInput
	}

	type dependencies struct {
		interpreter *mocks.IntentInterpreter
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(output ParseInboundOutput, err error)
	}{
		{
			name: "confidence presente é propagada",
			args: args{input: ParseInboundInput{UserID: uuid.New(), Text: "gastei 58 no ifood"}},
			dependencies: dependencies{
				interpreter: func() *mocks.IntentInterpreter {
					s.interpreter.EXPECT().
						Interpret(mock.Anything, mock.AnythingOfType("interfaces.LLMRequest")).
						Return(interfaces.LLMResponse{RawJSON: []byte(`{"kind":"log_expense","amount_cents":5800,"merchant":"iFood","confidence":0.42}`)}, nil).
						Once()
					return s.interpreter
				}(),
			},
			expect: func(output ParseInboundOutput, err error) {
				s.NoError(err)
				s.InDelta(0.42, output.Confidence.Value(), 1e-9)
			},
		},
		{
			name: "confidence ausente usa default neutro",
			args: args{input: ParseInboundInput{UserID: uuid.New(), Text: "gastei 58 no ifood"}},
			dependencies: dependencies{
				interpreter: func() *mocks.IntentInterpreter {
					s.interpreter.EXPECT().
						Interpret(mock.Anything, mock.AnythingOfType("interfaces.LLMRequest")).
						Return(interfaces.LLMResponse{RawJSON: []byte(`{"kind":"log_expense","amount_cents":5800,"merchant":"iFood"}`)}, nil).
						Once()
					return s.interpreter
				}(),
			},
			expect: func(output ParseInboundOutput, err error) {
				s.NoError(err)
				s.InDelta(defaultConfidence, output.Confidence.Value(), 1e-9)
			},
		},
		{
			name: "confidence acima de 1 é limitada",
			args: args{input: ParseInboundInput{UserID: uuid.New(), Text: "gastei 58 no ifood"}},
			dependencies: dependencies{
				interpreter: func() *mocks.IntentInterpreter {
					s.interpreter.EXPECT().
						Interpret(mock.Anything, mock.AnythingOfType("interfaces.LLMRequest")).
						Return(interfaces.LLMResponse{RawJSON: []byte(`{"kind":"log_expense","amount_cents":5800,"merchant":"iFood","confidence":1.5}`)}, nil).
						Once()
					return s.interpreter
				}(),
			},
			expect: func(output ParseInboundOutput, err error) {
				s.NoError(err)
				s.InDelta(1, output.Confidence.Value(), 1e-9)
			},
		},
		{
			name: "confidence negativa é limitada",
			args: args{input: ParseInboundInput{UserID: uuid.New(), Text: "gastei 58 no ifood"}},
			dependencies: dependencies{
				interpreter: func() *mocks.IntentInterpreter {
					s.interpreter.EXPECT().
						Interpret(mock.Anything, mock.AnythingOfType("interfaces.LLMRequest")).
						Return(interfaces.LLMResponse{RawJSON: []byte(`{"kind":"log_expense","amount_cents":5800,"merchant":"iFood","confidence":-0.3}`)}, nil).
						Once()
					return s.interpreter
				}(),
			},
			expect: func(output ParseInboundOutput, err error) {
				s.NoError(err)
				s.InDelta(0, output.Confidence.Value(), 1e-9)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc, err := NewParseInbound(scenario.dependencies.interpreter, 2000, s.obs)
			s.Require().NoError(err)
			output, execErr := uc.Execute(s.ctx, scenario.args.input)
			scenario.expect(output, execErr)
		})
	}
}

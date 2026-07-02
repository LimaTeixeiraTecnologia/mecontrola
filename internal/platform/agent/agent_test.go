package agent

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type AgentTestSuite struct {
	suite.Suite
	ctx          context.Context
	obs          observability.Observability
	providerMock *llmmocks.Provider
}

func TestAgentTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTestSuite))
}

func (s *AgentTestSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.providerMock = llmmocks.NewProvider(s.T())
}

func (s *AgentTestSuite) TestStream_ResultWithoutDrainingDeltasDoesNotBlockOrLeak() {
	a := NewAgent("agent-1", "instr", s.providerMock, s.obs)

	ts := llmmocks.NewTokenStream(s.T())
	ch := make(chan string, 200)
	for range 200 {
		ch <- "x"
	}
	close(ch)
	ts.EXPECT().Deltas().Return((<-chan string)(ch)).Once()
	ts.EXPECT().Err().Return(nil).Once()
	s.providerMock.EXPECT().
		Stream(mock.Anything, mock.AnythingOfType("llm.Request")).
		Return(ts, nil).
		Once()

	rs, err := a.Stream(s.ctx, Request{
		AgentID:  "agent-1",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	s.NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, resErr := rs.Result(ctx)

	s.NoError(resErr)
	s.Len(result.Content, 200)
	s.Equal(ExecutionModeStream, result.Mode)

	s.Eventually(func() bool {
		_, ok := <-rs.Deltas()
		return !ok
	}, time.Second, 5*time.Millisecond)
}

func (s *AgentTestSuite) TestExecute_Success() {
	type args struct {
		in Request
	}
	type dependencies struct {
		provider *llmmocks.Provider
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result Result, err error)
	}{
		{
			name: "deve executar com sucesso",
			args: args{in: Request{
				AgentID:  "agent-1",
				Messages: []llm.Message{{Role: "user", Content: "hello"}},
			}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{Content: "world"}, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result Result, err error) {
				s.NoError(err)
				s.Equal("world", result.Content)
				s.Equal(ExecutionModeSync, result.Mode)
			},
		},
		{
			name: "deve retornar erro quando provider falha",
			args: args{in: Request{
				AgentID:  "agent-1",
				Messages: []llm.Message{{Role: "user", Content: "hello"}},
			}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{}, errors.New("upstream error")).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result Result, err error) {
				s.Error(err)
				s.Empty(result.Content)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := NewAgent("agent-1", "You are a helpful assistant.", scenario.dependencies.provider, s.obs)
			result, err := a.Execute(s.ctx, scenario.args.in)
			scenario.expect(result, err)
		})
	}
}

func (s *AgentTestSuite) TestExecute_ToolLoop_FeedsResultsBackToModel() {
	var invocations atomic.Int32
	weatherTool := tool.NewTool(
		"get_weather",
		"Get weather",
		llm.Schema{Schema: map[string]any{"type": "object"}},
		llm.Schema{},
		func(_ context.Context, _ map[string]any) (map[string]any, error) {
			invocations.Add(1)
			return map[string]any{"temp": "20C"}, nil
		},
	)

	var secondReq llm.Request
	s.providerMock.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Return(llm.Response{
			ToolCalls: []llm.ToolCall{{
				ID:            "tc1",
				FunctionName:  "get_weather",
				ArgumentsJSON: map[string]any{"city": "New York"},
			}},
		}, nil).
		Once()
	s.providerMock.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Run(func(_ context.Context, req llm.Request) {
			secondReq = req
		}).
		Return(llm.Response{Content: "It is 20C in New York."}, nil).
		Once()

	a := NewAgent("agent-1", "instr", s.providerMock, s.obs, WithTools(weatherTool))
	result, err := a.Execute(s.ctx, Request{
		AgentID:  "agent-1",
		Messages: []llm.Message{{Role: "user", Content: "What is the weather in New York?"}},
	})

	s.NoError(err)
	s.Equal("It is 20C in New York.", result.Content)
	s.Equal(ExecutionModeSync, result.Mode)
	s.Equal(int32(1), invocations.Load())

	var hasAssistantToolCalls, hasToolResult bool
	for _, m := range secondReq.Messages {
		if m.Role == "assistant" && len(m.ToolCalls) == 1 && m.ToolCalls[0].ID == "tc1" {
			hasAssistantToolCalls = true
		}
		if m.Role == "tool" && m.ToolCallID == "tc1" && strings.Contains(m.Content, "20C") {
			hasToolResult = true
		}
	}
	s.True(hasAssistantToolCalls)
	s.True(hasToolResult)
}

func (s *AgentTestSuite) TestExecute_ToolLoop_MaxRoundsEmptyContentErrors() {
	loopTool := tool.NewTool(
		"loop_tool",
		"Always loops",
		llm.Schema{Schema: map[string]any{"type": "object"}},
		llm.Schema{},
		func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	)

	s.providerMock.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Return(llm.Response{
			ToolCalls: []llm.ToolCall{{ID: "tc1", FunctionName: "loop_tool", ArgumentsJSON: map[string]any{}}},
		}, nil).
		Times(5)

	a := NewAgent("agent-1", "instr", s.providerMock, s.obs, WithTools(loopTool))
	result, err := a.Execute(s.ctx, Request{
		AgentID:  "agent-1",
		Messages: []llm.Message{{Role: "user", Content: "loop forever"}},
	})

	s.Error(err)
	s.True(errors.Is(err, ErrMaxToolRounds))
	s.Empty(result.Content)
}

func (s *AgentTestSuite) TestWithMaxToolRounds_DefaultIsPreserved() {
	s.providerMock.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Return(llm.Response{Content: "ok"}, nil).
		Once()

	a := NewAgent("agent-1", "instr", s.providerMock, s.obs)
	impl := a.(*agentImpl)
	s.Equal(defaultMaxToolRounds, impl.maxToolRounds)

	result, err := a.Execute(s.ctx, Request{
		AgentID:  "agent-1",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	s.NoError(err)
	s.Equal("ok", result.Content)
}

func (s *AgentTestSuite) TestWithMaxToolRounds_CustomRoundsRespected() {
	loopTool := tool.NewTool(
		"loop_tool",
		"Always loops",
		llm.Schema{Schema: map[string]any{"type": "object"}},
		llm.Schema{},
		func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	)

	s.providerMock.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Return(llm.Response{
			ToolCalls: []llm.ToolCall{{ID: "tc1", FunctionName: "loop_tool", ArgumentsJSON: map[string]any{}}},
		}, nil).
		Times(12)

	a := NewAgent("agent-1", "instr", s.providerMock, s.obs, WithTools(loopTool), WithMaxToolRounds(12))
	impl := a.(*agentImpl)
	s.Equal(12, impl.maxToolRounds)

	result, err := a.Execute(s.ctx, Request{
		AgentID:  "agent-1",
		Messages: []llm.Message{{Role: "user", Content: "loop"}},
	})
	s.Error(err)
	s.True(errors.Is(err, ErrMaxToolRounds))
	s.Empty(result.Content)
}

func (s *AgentTestSuite) TestExecute_DefaultMaxTokensPrecedence() {
	scenarios := []struct {
		name              string
		agentDefault      int
		requestMaxTokens  int
		expectedMaxTokens int
	}{
		{
			name:              "deve usar max tokens da request quando informado",
			agentDefault:      768,
			requestMaxTokens:  1024,
			expectedMaxTokens: 1024,
		},
		{
			name:              "deve usar default do agente quando request nao informar",
			agentDefault:      768,
			requestMaxTokens:  0,
			expectedMaxTokens: 768,
		},
		{
			name:              "deve preservar zero quando request e default nao informarem",
			agentDefault:      0,
			requestMaxTokens:  0,
			expectedMaxTokens: 0,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			provider := llmmocks.NewProvider(s.T())
			provider.EXPECT().
				Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
				Run(func(_ context.Context, req llm.Request) {
					s.Equal(scenario.expectedMaxTokens, req.MaxTokens)
				}).
				Return(llm.Response{Content: "ok"}, nil).
				Once()

			a := NewAgent(
				"agent-1",
				"instr",
				provider,
				s.obs,
				WithDefaultMaxTokens(scenario.agentDefault),
			)

			result, err := a.Execute(s.ctx, Request{
				AgentID:     "agent-1",
				Messages:    []llm.Message{{Role: "user", Content: "hi"}},
				MaxTokens:   scenario.requestMaxTokens,
				Temperature: 0,
			})

			s.NoError(err)
			s.Equal("ok", result.Content)
		})
	}
}

func (s *AgentTestSuite) TestExecute_TruncatedByLengthPropagation() {
	scenarios := []struct {
		name              string
		response          llm.Response
		expectedTruncated bool
	}{
		{
			name:              "deve propagar truncated by length true sem erro",
			response:          llm.Response{Content: "resposta cortada no mei", TruncatedByLength: true},
			expectedTruncated: true,
		},
		{
			name:              "deve manter truncated by length false quando resposta completa",
			response:          llm.Response{Content: "resposta completa"},
			expectedTruncated: false,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			provider := llmmocks.NewProvider(s.T())
			provider.EXPECT().
				Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
				Return(scenario.response, nil).
				Once()

			a := NewAgent("agent-1", "instr", provider, s.obs)

			result, err := a.Execute(s.ctx, Request{
				AgentID:  "agent-1",
				Messages: []llm.Message{{Role: "user", Content: "hi"}},
			})

			s.NoError(err)
			s.Equal(scenario.response.Content, result.Content)
			s.Equal(scenario.expectedTruncated, result.TruncatedByLength)
		})
	}
}

func (s *AgentTestSuite) TestStream_DefaultMaxTokensPrecedence() {
	scenarios := []struct {
		name              string
		agentDefault      int
		requestMaxTokens  int
		expectedMaxTokens int
	}{
		{
			name:              "deve usar max tokens da request no stream quando informado",
			agentDefault:      768,
			requestMaxTokens:  1024,
			expectedMaxTokens: 1024,
		},
		{
			name:              "deve usar default do agente no stream quando request nao informar",
			agentDefault:      768,
			requestMaxTokens:  0,
			expectedMaxTokens: 768,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			provider := llmmocks.NewProvider(s.T())
			ts := llmmocks.NewTokenStream(s.T())
			ch := make(chan string, 1)
			ch <- "ok"
			close(ch)
			ts.EXPECT().Deltas().Return((<-chan string)(ch)).Once()
			ts.EXPECT().Err().Return(nil).Once()

			provider.EXPECT().
				Stream(mock.Anything, mock.AnythingOfType("llm.Request")).
				RunAndReturn(func(_ context.Context, req llm.Request) (llm.TokenStream, error) {
					s.Equal(scenario.expectedMaxTokens, req.MaxTokens)
					return ts, nil
				}).
				Once()

			a := NewAgent(
				"agent-1",
				"instr",
				provider,
				s.obs,
				WithDefaultMaxTokens(scenario.agentDefault),
			)

			rs, err := a.Stream(s.ctx, Request{
				AgentID:   "agent-1",
				Messages:  []llm.Message{{Role: "user", Content: "hi"}},
				MaxTokens: scenario.requestMaxTokens,
			})
			s.NoError(err)

			result, resErr := rs.Result(s.ctx)
			s.NoError(resErr)
			s.Equal("ok", result.Content)
		})
	}
}

func (s *AgentTestSuite) TestExecute_StructuredOutput() {
	type args struct {
		in Request
	}
	type dependencies struct {
		provider *llmmocks.Provider
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result Result, err error)
	}{
		{
			name: "deve validar contrato com sucesso",
			args: args{in: Request{
				AgentID:  "agent-1",
				Messages: []llm.Message{{Role: "user", Content: "hello"}},
				Decoder:  &alwaysValidDecoder{},
			}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{Content: "ok", RawJSON: []byte(`{"ok":true}`)}, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result Result, err error) {
				s.NoError(err)
				s.Equal("ok", result.Content)
			},
		},
		{
			name: "deve falhar quando contrato nao conforme",
			args: args{in: Request{
				AgentID:  "agent-1",
				Messages: []llm.Message{{Role: "user", Content: "hello"}},
				Decoder:  &alwaysInvalidDecoder{},
			}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{Content: "bad", RawJSON: []byte(`{}`)}, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result Result, err error) {
				s.Error(err)
				s.True(errors.Is(err, ErrContractNotMet))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := NewAgent("agent-1", "You are a helpful assistant.", scenario.dependencies.provider, s.obs)
			result, err := a.Execute(s.ctx, scenario.args.in)
			scenario.expect(result, err)
		})
	}
}

func (s *AgentTestSuite) TestStream_Success() {
	type args struct {
		in Request
	}
	type dependencies struct {
		provider *llmmocks.Provider
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(rs ResultStream, err error)
	}{
		{
			name: "deve fazer stream com sucesso sem contrato",
			args: args{in: Request{
				AgentID:  "agent-1",
				Messages: []llm.Message{{Role: "user", Content: "hello"}},
			}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					ts := llmmocks.NewTokenStream(s.T())
					ch := make(chan string, 2)
					ch <- "hel"
					ch <- "lo"
					close(ch)
					ts.EXPECT().Deltas().Return((<-chan string)(ch)).Once()
					ts.EXPECT().Err().Return(nil).Once()
					s.providerMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(ts, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(rs ResultStream, err error) {
				s.NoError(err)
				s.NotNil(rs)
				result, resErr := rs.Result(s.ctx)
				s.NoError(resErr)
				s.Equal("hello", result.Content)
				s.Equal(ExecutionModeStream, result.Mode)
			},
		},
		{
			name: "deve falhar no stream quando contrato nao conforme",
			args: args{in: Request{
				AgentID:  "agent-1",
				Messages: []llm.Message{{Role: "user", Content: "hello"}},
				Decoder:  &alwaysInvalidDecoder{},
			}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					ts := llmmocks.NewTokenStream(s.T())
					ch := make(chan string, 1)
					ch <- "bad"
					close(ch)
					ts.EXPECT().Deltas().Return((<-chan string)(ch)).Once()
					ts.EXPECT().Err().Return(nil).Once()
					s.providerMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(ts, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(rs ResultStream, err error) {
				s.NoError(err)
				s.NotNil(rs)
				_, resErr := rs.Result(s.ctx)
				s.Error(resErr)
				s.True(errors.Is(resErr, ErrContractNotMet))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := NewAgent("agent-1", "You are a helpful assistant.", scenario.dependencies.provider, s.obs)
			rs, err := a.Stream(s.ctx, scenario.args.in)
			scenario.expect(rs, err)
		})
	}
}

func (s *AgentTestSuite) TestExecute_ToolError_StructuredMessageDeliveredToLLM() {
	type args struct {
		in Request
	}
	type dependencies struct {
		provider *llmmocks.Provider
	}

	var capturedSecondReq llm.Request
	errTool := tool.NewTool(
		"failing_tool",
		"always fails",
		llm.Schema{Schema: map[string]any{"type": "object"}},
		llm.Schema{},
		func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return nil, errors.New("persistência falhou")
		},
	)

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result Result, err error)
	}{
		{
			name: "deve entregar tool message de erro ao LLM e nao content vazio",
			args: args{in: Request{
				AgentID:  "agent-1",
				Messages: []llm.Message{{Role: "user", Content: "registra despesa"}},
			}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{
							ToolCalls: []llm.ToolCall{{
								ID:            "tc-err",
								FunctionName:  "failing_tool",
								ArgumentsJSON: map[string]any{},
							}},
						}, nil).Once()
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Run(func(_ context.Context, req llm.Request) {
							capturedSecondReq = req
						}).
						Return(llm.Response{Content: "não consegui registrar, tente novamente."}, nil).Once()
					return s.providerMock
				}(),
			},
			expect: func(result Result, err error) {
				s.NoError(err)
				s.NotEmpty(result.Content)
				s.Equal(ToolOutcomeUsecaseError, result.ToolOutcome,
					"falha de tool deve marcar usecaseError deterministicamente, independente do texto do LLM")
				var toolMsg llm.Message
				for _, m := range capturedSecondReq.Messages {
					if m.Role == roleTool && m.ToolCallID == "tc-err" {
						toolMsg = m
					}
				}
				s.NotEmpty(toolMsg.Content, "tool message de erro nao pode ser vazio")
				s.Contains(toolMsg.Content, "persistência falhou")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := NewAgent("agent-1", "instr", scenario.dependencies.provider, s.obs, WithTools(errTool))
			result, err := a.Execute(s.ctx, scenario.args.in)
			scenario.expect(result, err)
		})
	}
}

type alwaysValidDecoder struct{}

func (d *alwaysValidDecoder) Schema() llm.Schema      { return llm.Schema{Name: "test"} }
func (d *alwaysValidDecoder) Validate(_ []byte) error { return nil }

type alwaysInvalidDecoder struct{}

func (d *alwaysInvalidDecoder) Schema() llm.Schema      { return llm.Schema{Name: "test"} }
func (d *alwaysInvalidDecoder) Validate(_ []byte) error { return errors.New("contract not satisfied") }

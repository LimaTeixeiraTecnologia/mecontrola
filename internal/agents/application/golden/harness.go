package golden

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type AgentExecutor func(ctx context.Context, messages []llm.Message) (agent.Result, error)

type CapturedToolCall struct {
	Tool string
	Args map[string]any
}

type ToolCaptureSink func(name string, args map[string]any)

func EvaluateCase(ctx context.Context, executor AgentExecutor, c Case) CaseOutcome {
	return EvaluateCaseWithCapture(ctx, executor, c, func() []CapturedToolCall { return nil })
}

func EvaluateCaseWithCapture(ctx context.Context, executor AgentExecutor, c Case, capturedFn func() []CapturedToolCall) CaseOutcome {
	var messages []llm.Message
	for _, turn := range c.PriorTurns {
		messages = append(messages, llm.Message{Role: "user", Content: turn.UserMessage})
	}
	messages = append(messages, llm.Message{Role: "user", Content: c.Input})

	result, err := executor(ctx, messages)
	if err != nil {
		return CaseOutcome{Case: c, Passed: false, Detail: "erro de execução: " + err.Error()}
	}

	if detail, ok := checkExpectedTools(c, result); !ok {
		return CaseOutcome{Case: c, Passed: false, Detail: detail}
	}

	if detail, ok := checkExpectedOutcome(c, result); !ok {
		return CaseOutcome{Case: c, Passed: false, Detail: detail}
	}

	if detail, ok := checkExpectedArgs(c, capturedFn()); !ok {
		return CaseOutcome{Case: c, Passed: false, Detail: detail}
	}

	if c.ResponseProperty != nil && !c.ResponseProperty(result.Content) {
		return CaseOutcome{
			Case:   c,
			Passed: false,
			Detail: "propriedade de resposta não satisfeita (" + c.ResponseDescribe + "); resposta=" + result.Content,
		}
	}

	return CaseOutcome{Case: c, Passed: true}
}

func checkExpectedArgs(c Case, captured []CapturedToolCall) (string, bool) {
	if len(c.ExpectedArgs) == 0 {
		return "", true
	}
	toolName := c.ExpectedTool
	for _, call := range captured {
		if call.Tool != toolName {
			continue
		}
		for key, expected := range c.ExpectedArgs {
			got, present := call.Args[key]
			if !present {
				return "arg obrigatório ausente na tool " + toolName + ": " + key, false
			}
			if !argsEqual(got, expected) {
				return fmt.Sprintf("arg %s esperado=%v (%T) obtido=%v (%T) na tool %s", key, expected, expected, got, got, toolName), false
			}
		}
		return "", true
	}
	return "tool " + toolName + " não foi capturada para checagem de args", false
}

func argsEqual(got, expected any) bool {
	switch expectedVal := expected.(type) {
	case string:
		gotStr, ok := got.(string)
		return ok && gotStr == expectedVal
	case float64:
		gotFloat, ok := asFloat64(got)
		return ok && gotFloat == expectedVal
	default:
		return got == expected
	}
}

func asFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func checkExpectedOutcome(c Case, result agent.Result) (string, bool) {
	if c.ExpectedOutcome == 0 {
		return "", true
	}
	if result.ToolOutcome != c.ExpectedOutcome {
		return "outcome esperado " + c.ExpectedOutcome.String() + " mas obtido " + result.ToolOutcome.String(), false
	}
	return "", true
}

func checkExpectedTools(c Case, result agent.Result) (string, bool) {
	called := make(map[string]bool, len(result.ToolCalls))
	for _, tc := range result.ToolCalls {
		called[tc.Tool] = true
	}

	if c.NoToolExpected {
		for name := range called {
			return "tool " + name + " foi chamada, mas nenhuma era esperada", false
		}
		return "", true
	}

	if len(c.ExpectedTools) > 0 {
		for _, expected := range c.ExpectedTools {
			if !called[expected] {
				return "tool esperada " + expected + " não foi chamada", false
			}
		}
		return "", true
	}

	if len(c.ExpectedAnyOfTools) > 0 {
		for _, expected := range c.ExpectedAnyOfTools {
			if called[expected] {
				return "", true
			}
		}
		return "nenhuma das tools esperadas foi chamada: " + strings.Join(c.ExpectedAnyOfTools, ", "), false
	}

	if c.ExpectedTool != "" && !called[c.ExpectedTool] {
		return "tool esperada " + c.ExpectedTool + " não foi chamada", false
	}

	return "", true
}

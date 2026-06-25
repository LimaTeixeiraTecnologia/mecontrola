package services

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
)

type LLMClass int

const (
	LLMClassParse LLMClass = iota + 1
	LLMClassOnboarding
	LLMClassConversational
)

func (c LLMClass) String() string {
	switch c {
	case LLMClassParse:
		return "parse"
	case LLMClassOnboarding:
		return "onboarding"
	case LLMClassConversational:
		return "conversational"
	default:
		return "unknown"
	}
}

type ClassInterpreter interface {
	Interpret(ctx context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error)
}

type ClassRouter struct {
	interpreters map[LLMClass]ClassInterpreter
}

func NewClassRouter(m map[LLMClass]ClassInterpreter) *ClassRouter {
	return &ClassRouter{interpreters: m}
}

func (r *ClassRouter) Interpreter(class LLMClass) (ClassInterpreter, bool) {
	interp, ok := r.interpreters[class]
	return interp, ok
}

type classMetricInterpreter struct {
	inner   ClassInterpreter
	class   LLMClass
	counter observability.Counter
}

func NewClassMetricInterpreter(inner ClassInterpreter, class LLMClass, o11y observability.Observability) ClassInterpreter {
	counter := o11y.Metrics().Counter(
		"agent_llm_class_total",
		fmt.Sprintf("Total de chamadas LLM por classe %s", class.String()),
		"1",
	)
	return &classMetricInterpreter{inner: inner, class: class, counter: counter}
}

func (c *classMetricInterpreter) Interpret(ctx context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	resp, err := c.inner.Interpret(ctx, req)
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	model := resp.Provider.String()
	if model == "" {
		model = "unknown"
	}
	c.counter.Add(ctx, 1,
		observability.String("class", c.class.String()),
		observability.String("model", model),
		observability.String("outcome", outcome),
	)
	return resp, err
}

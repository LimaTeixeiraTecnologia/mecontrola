package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
)

var ErrFallbackChainExhausted = errors.New("agent.llm.fallback: all providers exhausted")

var ErrFallbackChainEmpty = errors.New("agent.llm.fallback: provider list is empty")

type FallbackChain struct {
	providers      []interfaces.LLMProvider
	o11y           observability.Observability
	breaker        *CircuitBreaker
	attemptsTotal  observability.Counter
	exhaustedTotal observability.Counter
	skippedTotal   observability.Counter
}

func NewFallbackChain(providers []interfaces.LLMProvider, breaker *CircuitBreaker, o11y observability.Observability) (*FallbackChain, error) {
	if len(providers) == 0 {
		return nil, ErrFallbackChainEmpty
	}
	attempts := o11y.Metrics().Counter(
		"agent_llm_fallback_attempts_total",
		"Total de tentativas de provider LLM por modelo e outcome",
		"1",
	)
	exhausted := o11y.Metrics().Counter(
		"agent_llm_fallback_exhausted_total",
		"Total de execucoes em que todos os providers da fallback chain falharam",
		"1",
	)
	skipped := o11y.Metrics().Counter(
		"agent_llm_fallback_skipped_total",
		"Total de providers pulados pela fallback chain por estar com circuito aberto",
		"1",
	)
	return &FallbackChain{
		providers:      providers,
		o11y:           o11y,
		breaker:        breaker,
		attemptsTotal:  attempts,
		exhaustedTotal: exhausted,
		skippedTotal:   skipped,
	}, nil
}

func (c *FallbackChain) Interpret(ctx context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	collected := make([]error, 0, len(c.providers))

	for _, provider := range c.providers {
		slug := provider.Slug().String()
		modelLabel := observability.String("model", slug)

		if c.breaker != nil {
			state, allowed := c.breaker.Allow(slug)
			if !allowed {
				c.skippedTotal.Add(ctx, 1, modelLabel, observability.String("state", state.String()))
				c.o11y.Logger().Info(ctx, "agent.llm.fallback.provider_skipped_circuit_open",
					modelLabel,
					observability.String("state", state.String()),
				)
				collected = append(collected, fmt.Errorf("provider %s: circuit %s", slug, state.String()))
				continue
			}
		}

		resp, err := provider.Interpret(ctx, req)
		if err == nil {
			if c.breaker != nil {
				c.breaker.RecordSuccess(slug)
			}
			c.attemptsTotal.Add(ctx, 1, modelLabel, observability.String("outcome", "ok"))
			return resp, nil
		}

		if c.breaker != nil {
			state := c.breaker.RecordFailure(slug)
			c.o11y.Logger().Warn(ctx, "agent.llm.fallback.provider_failed",
				modelLabel,
				observability.String("circuit_state", state.String()),
				observability.Error(err),
			)
		} else {
			c.o11y.Logger().Warn(ctx, "agent.llm.fallback.provider_failed",
				modelLabel,
				observability.Error(err),
			)
		}
		c.attemptsTotal.Add(ctx, 1, modelLabel, observability.String("outcome", "error"))
		collected = append(collected, fmt.Errorf("provider %s: %w", slug, err))
	}

	c.exhaustedTotal.Add(ctx, 1)
	return interfaces.LLMResponse{}, fmt.Errorf("%w: %w", ErrFallbackChainExhausted, errors.Join(collected...))
}

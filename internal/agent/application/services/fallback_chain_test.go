package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type stubProvider struct {
	slug    valueobjects.ModelSlug
	calls   int
	err     error
	respRaw []byte
}

func (s *stubProvider) Slug() valueobjects.ModelSlug { return s.slug }

func (s *stubProvider) Interpret(_ context.Context, _ interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	s.calls++
	if s.err != nil {
		return interfaces.LLMResponse{}, s.err
	}
	return interfaces.LLMResponse{Provider: s.slug, RawJSON: s.respRaw}, nil
}

func TestFallbackChain_PrimarySucceeds(t *testing.T) {
	primary := &stubProvider{slug: valueobjects.ModelSlugGeminiFlashLite(), respRaw: []byte(`{}`)}
	chain, err := services.NewFallbackChain(
		[]interfaces.LLMProvider{primary},
		services.NewCircuitBreaker(services.CircuitBreakerConfig{}),
		noop.NewProvider(),
	)
	require.NoError(t, err)

	resp, err := chain.Interpret(context.Background(), interfaces.LLMRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Provider.Equal(valueobjects.ModelSlugGeminiFlashLite()))
	assert.Equal(t, 1, primary.calls)
}

func TestFallbackChain_FallsThroughOnError(t *testing.T) {
	primary := &stubProvider{slug: valueobjects.ModelSlugGeminiFlashLite(), err: errors.New("upstream 5xx")}
	fallback := &stubProvider{slug: valueobjects.ModelSlugGPT5Nano(), respRaw: []byte(`{}`)}
	chain, err := services.NewFallbackChain(
		[]interfaces.LLMProvider{primary, fallback},
		services.NewCircuitBreaker(services.CircuitBreakerConfig{}),
		noop.NewProvider(),
	)
	require.NoError(t, err)

	resp, err := chain.Interpret(context.Background(), interfaces.LLMRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Provider.Equal(valueobjects.ModelSlugGPT5Nano()))
	assert.Equal(t, 1, primary.calls)
	assert.Equal(t, 1, fallback.calls)
}

func TestFallbackChain_AllProvidersFail_ReturnsExhausted(t *testing.T) {
	primary := &stubProvider{slug: valueobjects.ModelSlugGeminiFlashLite(), err: errors.New("p1")}
	fallback := &stubProvider{slug: valueobjects.ModelSlugGPT5Nano(), err: errors.New("p2")}
	chain, _ := services.NewFallbackChain(
		[]interfaces.LLMProvider{primary, fallback},
		services.NewCircuitBreaker(services.CircuitBreakerConfig{}),
		noop.NewProvider(),
	)

	_, err := chain.Interpret(context.Background(), interfaces.LLMRequest{})
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrFallbackChainExhausted)
}

func TestFallbackChain_SkipsProvidersWithOpenCircuit(t *testing.T) {
	primary := &stubProvider{slug: valueobjects.ModelSlugGeminiFlashLite(), err: errors.New("upstream")}
	fallback := &stubProvider{slug: valueobjects.ModelSlugGPT5Nano(), respRaw: []byte(`{}`)}
	breaker := services.NewCircuitBreaker(services.CircuitBreakerConfig{
		MaxFailures:   1,
		FailureWindow: 30 * time.Second,
		OpenDuration:  60 * time.Second,
	})

	chain, _ := services.NewFallbackChain(
		[]interfaces.LLMProvider{primary, fallback},
		breaker,
		noop.NewProvider(),
	)

	_, err := chain.Interpret(context.Background(), interfaces.LLMRequest{})
	require.NoError(t, err)
	assert.Equal(t, 1, primary.calls)
	assert.Equal(t, 1, fallback.calls)
	assert.Equal(t, services.CircuitOpen, breaker.State(valueobjects.ModelSlugGeminiFlashLite().String()))

	primary.err = nil
	primary.respRaw = []byte(`{}`)
	_, err = chain.Interpret(context.Background(), interfaces.LLMRequest{})
	require.NoError(t, err)
	assert.Equal(t, 1, primary.calls, "primary deve permanecer pulado enquanto circuito aberto")
	assert.Equal(t, 2, fallback.calls)
}

func TestFallbackChain_RejectsEmptyProviderList(t *testing.T) {
	_, err := services.NewFallbackChain(nil, services.NewCircuitBreaker(services.CircuitBreakerConfig{}), noop.NewProvider())
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrFallbackChainEmpty)
}

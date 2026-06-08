package checkout_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/checkout"
)

func TestKiwifyURLBuilder_Build_AppendsSckParam(t *testing.T) {
	b := checkout.NewKiwifyURLBuilder(
		map[string]string{
			"plan-monthly": "https://pay.kiwify.com.br/abc123",
		},
		[]string{"pay.kiwify.com.br"},
	)

	got, err := b.Build(context.Background(), "plan-monthly", "mytoken")
	require.NoError(t, err)
	assert.Equal(t, "https://pay.kiwify.com.br/abc123?sck=mytoken", got)
}

func TestKiwifyURLBuilder_Build_PreservesExistingQuery(t *testing.T) {
	b := checkout.NewKiwifyURLBuilder(
		map[string]string{
			"plan-a": "https://pay.kiwify.com.br/abc?utm_source=landing",
		},
		[]string{"pay.kiwify.com.br"},
	)

	got, err := b.Build(context.Background(), "plan-a", "tok42")
	require.NoError(t, err)
	assert.Contains(t, got, "sck=tok42")
	assert.Contains(t, got, "utm_source=landing")
}

func TestKiwifyURLBuilder_Build_UnknownPlanReturnsErrUnknownPlan(t *testing.T) {
	b := checkout.NewKiwifyURLBuilder(
		map[string]string{
			"plan-x": "https://pay.kiwify.com.br/xyz",
		},
		[]string{"pay.kiwify.com.br"},
	)

	_, err := b.Build(context.Background(), "no-such-plan", "tok")
	assert.True(t, errors.Is(err, application.ErrUnknownPlan))
}

func TestKiwifyURLBuilder_Build_DisallowedHostReturnsErrCheckoutUnavailable(t *testing.T) {
	b := checkout.NewKiwifyURLBuilder(
		map[string]string{
			"plan-bad": "https://evil.example.com/pay",
		},
		[]string{"pay.kiwify.com.br"},
	)

	_, err := b.Build(context.Background(), "plan-bad", "tok")
	assert.True(t, errors.Is(err, application.ErrCheckoutUnavailable))
}

func TestKiwifyURLBuilder_Build_InvalidURLReturnsErrCheckoutUnavailable(t *testing.T) {
	b := checkout.NewKiwifyURLBuilder(
		map[string]string{
			"plan-inv": "://bad-url",
		},
		[]string{"pay.kiwify.com.br"},
	)

	_, err := b.Build(context.Background(), "plan-inv", "tok")
	assert.True(t, errors.Is(err, application.ErrCheckoutUnavailable))
}

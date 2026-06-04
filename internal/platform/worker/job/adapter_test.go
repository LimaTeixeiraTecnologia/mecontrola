package job_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/job"
)

func TestNewAdapter_PolíticaPadrãoÉSkip(t *testing.T) {
	a := job.NewAdapter("meu-job", "@hourly", func(_ context.Context) error { return nil })
	require.Equal(t, "meu-job", a.Name())
	require.Equal(t, "@hourly", a.Schedule())
	require.Equal(t, job.OverlapSkip, a.OverlapPolicy())
}

func TestNewAdapterWithPolicy_PolíticaAllow(t *testing.T) {
	a := job.NewAdapterWithPolicy("meu-job", "@daily", func(_ context.Context) error { return nil }, job.OverlapAllow)
	require.Equal(t, job.OverlapAllow, a.OverlapPolicy())
}

func TestAdapter_Run_DelegaFunção(t *testing.T) {
	sentinel := errors.New("erro sentinel")
	a := job.NewAdapter("test", "@hourly", func(_ context.Context) error { return sentinel })
	err := a.Run(context.Background())
	require.ErrorIs(t, err, sentinel)
}

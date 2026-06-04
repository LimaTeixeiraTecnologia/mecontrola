package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/consumer"
)

func TestAdapter_NomeETecnologia(t *testing.T) {
	a := consumer.NewAdapter("billing", "kafka", &fakeRunner{})
	require.Equal(t, "billing", a.Name())
	require.Equal(t, "kafka", a.Technology())
}

func TestAdapter_DelegaStart(t *testing.T) {
	started := false
	r := &fakeRunner{startFn: func(_ context.Context) error {
		started = true
		return nil
	}}
	a := consumer.NewAdapter("test", "fake", r)
	err := a.Start(context.Background())
	require.NoError(t, err)
	require.True(t, started)
}

func TestAdapter_DelegaStop(t *testing.T) {
	sentinel := errors.New("stop err")
	r := &fakeRunner{stopFn: func(_ context.Context) error { return sentinel }}
	a := consumer.NewAdapter("test", "fake", r)
	err := a.Stop(context.Background())
	require.ErrorIs(t, err, sentinel)
}

type fakeRunner struct {
	startFn func(context.Context) error
	stopFn  func(context.Context) error
}

func (r *fakeRunner) Start(ctx context.Context) error {
	if r.startFn != nil {
		return r.startFn(ctx)
	}
	return nil
}

func (r *fakeRunner) Stop(ctx context.Context) error {
	if r.stopFn != nil {
		return r.stopFn(ctx)
	}
	return nil
}

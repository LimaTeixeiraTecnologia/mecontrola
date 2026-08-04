package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

func TestBuildO11yConfigRegistersGlobal(t *testing.T) {
	cfg := &configs.Config{}
	o11yConfig := buildO11yConfig(cfg, "test-host")
	assert.True(t, o11yConfig.RegisterGlobal)
}

func TestStartWorkerHeartbeatTicksAndCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	hb, err := startWorkerHeartbeat(ctx, fake.NewProvider(), 5*time.Millisecond)
	require.NoError(t, err)
	require.EqualValues(t, 1, hb.beats.Load())

	assert.Eventually(t, func() bool {
		return hb.beats.Load() >= 3
	}, 500*time.Millisecond, 5*time.Millisecond, "expected the ticker to increment the heartbeat counter")

	cancel()

	select {
	case <-hb.done:
	case <-time.After(time.Second):
		t.Fatal("heartbeat goroutine did not stop after context cancellation")
	}

	stopped := hb.beats.Load()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, stopped, hb.beats.Load(), "heartbeat counter must stop advancing after cancellation")
}

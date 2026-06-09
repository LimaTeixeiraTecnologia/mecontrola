package server_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
)

func TestLifecycle_LimiterStartShutdown(t *testing.T) {
	baseline := runtime.NumGoroutine()

	o11y := noop.NewProvider()
	limiter := ratelimit.New(o11y)

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()

	if err := limiter.Start(startCtx); err != nil {
		t.Fatalf("limiter start failed: %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	done := make(chan error, 1)
	go func() {
		done <- limiter.Shutdown(shutdownCtx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("limiter shutdown failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("limiter shutdown did not complete within 15s")
	}

	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	tolerance := 5
	if delta := after - baseline; delta > tolerance {
		t.Errorf("goroutine leak: baseline=%d after=%d delta=%d tolerance=%d", baseline, after, delta, tolerance)
	}
}

func TestLifecycle_LimiterShutdown_CompletesUnder15s(t *testing.T) {
	o11y := noop.NewProvider()
	limiter := ratelimit.New(o11y)

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := limiter.Start(startCtx); err != nil {
		t.Fatalf("limiter start failed: %v", err)
	}

	start := time.Now()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := limiter.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("limiter shutdown error: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed >= 15*time.Second {
		t.Errorf("shutdown took too long: %v", elapsed)
	}
}

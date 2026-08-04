//go:build integration

package runtimemetrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/runtimemetrics"
)

func TestStartExportsGoroutineCount(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(noop.NewMeterProvider())

	require.NoError(t, runtimemetrics.Start(10*time.Millisecond))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "go.goroutine.count" {
				found = true
			}
		}
	}
	require.True(t, found, "expected go.goroutine.count to be exported via global MeterProvider wiring")
}

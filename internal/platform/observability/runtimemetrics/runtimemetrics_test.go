package runtimemetrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestStart(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(noop.NewMeterProvider())

	err := Start(10 * time.Millisecond)
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	assert.True(t, hasMetric(rm, "go.goroutine.count"), "expected go.goroutine.count to be exported")
}

func TestStartFailsWithoutGlobalMeterProvider(t *testing.T) {
	otel.SetMeterProvider(noop.NewMeterProvider())

	err := Start(10 * time.Millisecond)

	require.ErrorIs(t, err, ErrGlobalMeterProviderNotRegistered)
}

func TestStartDoesNotRegressInstanceMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(noop.NewMeterProvider())

	instanceReader := sdkmetric.NewManualReader()
	instanceProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(instanceReader))
	counter, err := instanceProvider.Meter("instance").Int64Counter("agents_write_total")
	require.NoError(t, err)
	counter.Add(context.Background(), 1)

	require.NoError(t, Start(10*time.Millisecond))

	var instanceRM metricdata.ResourceMetrics
	require.NoError(t, instanceReader.Collect(context.Background(), &instanceRM))
	assert.True(t, hasMetric(instanceRM, "agents_write_total"),
		"metricas de instancia devem continuar sendo exportadas pelo provider proprio apos ativar o MeterProvider global")
	assert.False(t, hasMetric(instanceRM, "go.goroutine.count"),
		"metricas de runtime nao podem vazar para o provider de instancia")

	var globalRM metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &globalRM))
	assert.True(t, hasMetric(globalRM, "go.goroutine.count"),
		"metricas de runtime devem ser exportadas pelo MeterProvider global")
	assert.False(t, hasMetric(globalRM, "agents_write_total"),
		"metricas de instancia nao podem ser duplicadas no provider global")
}

func hasMetric(rm metricdata.ResourceMetrics, name string) bool {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return true
			}
		}
	}
	return false
}

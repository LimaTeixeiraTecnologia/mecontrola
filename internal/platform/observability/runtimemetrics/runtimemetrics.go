package runtimemetrics

import (
	"errors"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
)

var ErrGlobalMeterProviderNotRegistered = errors.New(
	"runtimemetrics: MeterProvider global nao registrado; instrumentacao de runtime seria um no-op silencioso",
)

func Start(minInterval time.Duration) error {
	if _, isNoop := otel.GetMeterProvider().(noop.MeterProvider); isNoop {
		return ErrGlobalMeterProviderNotRegistered
	}
	return runtime.Start(runtime.WithMinimumReadMemStatsInterval(minInterval))
}

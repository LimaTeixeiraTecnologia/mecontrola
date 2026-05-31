package observability

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// FoundationMetrics contém os instrumentos de métricas custom da foundation.
// Todos os campos são interfaces do devkit-go, substituíveis por noops em testes.
type FoundationMetrics struct {
	// BootstrapDuration registra o tempo total do Bootstrap em segundos.
	BootstrapDuration observability.Histogram
	// EventsPublished conta os eventos publicados pelo eventbus, segmentados por event_name e outcome.
	EventsPublished observability.Counter
	// HealthProbeStatus registra o status do health probe por check (db_ping, db_select).
	// Valor 1 = ok, 0 = down.
	HealthProbeStatus observability.UpDownCounter
}

// RegisterFoundationMetrics registra os instrumentos de métricas custom da foundation
// no Metrics provider do devkit-go.
//
// Métricas registradas:
//   - bootstrap_duration_seconds (histogram, s)
//   - events_published_total (counter, {event_name, outcome})
//   - health_probe_status (up-down counter, {check})
//
// Retorna erro se qualquer instrumento não puder ser criado.
func RegisterFoundationMetrics(m observability.Metrics) (*FoundationMetrics, error) {
	if m == nil {
		return nil, fmt.Errorf("observability: Metrics provider não pode ser nil")
	}

	bootstrapDuration := m.Histogram(
		"bootstrap_duration_seconds",
		"Tempo total de inicialização do Bootstrap da aplicação",
		"s",
	)
	if bootstrapDuration == nil {
		return nil, fmt.Errorf("observability: falha ao criar histograma bootstrap_duration_seconds")
	}

	eventsPublished := m.Counter(
		"events_published_total",
		"Total de eventos publicados pelo eventbus, segmentado por event_name e outcome",
		"{event}",
	)
	if eventsPublished == nil {
		return nil, fmt.Errorf("observability: falha ao criar counter events_published_total")
	}

	healthProbeStatus := m.UpDownCounter(
		"health_probe_status",
		"Status do health probe por check (1=ok, 0=down)",
		"1",
	)
	if healthProbeStatus == nil {
		return nil, fmt.Errorf("observability: falha ao criar up-down counter health_probe_status")
	}

	return &FoundationMetrics{
		BootstrapDuration: bootstrapDuration,
		EventsPublished:   eventsPublished,
		HealthProbeStatus: healthProbeStatus,
	}, nil
}

// RecordBootstrapDuration registra a duração do Bootstrap em segundos.
func (fm *FoundationMetrics) RecordBootstrapDuration(ctx context.Context, seconds float64) {
	fm.BootstrapDuration.Record(ctx, seconds)
}

// IncrementEventsPublished incrementa o contador de eventos publicados.
func (fm *FoundationMetrics) IncrementEventsPublished(ctx context.Context, eventName, outcome string) {
	fm.EventsPublished.Add(ctx, 1,
		observability.String("event_name", eventName),
		observability.String("outcome", outcome),
	)
}

// SetHealthProbeStatus define o status de um health probe (1=ok, 0=down).
// check pode ser "db_ping" ou "db_select".
func (fm *FoundationMetrics) SetHealthProbeStatus(ctx context.Context, check string, up bool) {
	value := int64(1)
	if !up {
		value = -1
	}
	fm.HealthProbeStatus.Add(ctx, value,
		observability.String("check", check),
	)
}

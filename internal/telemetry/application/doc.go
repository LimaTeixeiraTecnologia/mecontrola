// Package application orquestra os casos de uso do módulo telemetry.
//
// Responsabilidades: casos de uso de coleta e projeção de eventos de telemetria,
// declaração de ports (TelemetryRepository, MetricsSink, EventPublisher) e
// coordenação via UnitOfWork[T]. Este pacote NÃO pode importar adapters nem
// bibliotecas de IO concretas.
package application

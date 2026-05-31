// Package application orquestra os casos de uso do módulo notifications.
//
// Responsabilidades: casos de uso de envio e agendamento de notificações,
// declaração de ports (NotificationRepository, DeliveryPort, EventPublisher)
// e coordenação via UnitOfWork[T]. Este pacote NÃO pode importar adapters
// nem bibliotecas de IO concretas.
package application

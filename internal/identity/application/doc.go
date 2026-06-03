// Package application orquestra os casos de uso do módulo identity.
//
// Responsabilidades: casos de uso (use cases) que coordenam entidades e
// agregados do domain, declaração de ports (interfaces Repository, EventPublisher)
// implementados pela infrastructure, e unidades de trabalho via UnitOfWork[T].
// Este pacote NÃO pode importar infrastructure nem bibliotecas de IO concretas.
package application

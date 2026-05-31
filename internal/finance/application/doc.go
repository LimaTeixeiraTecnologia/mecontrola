// Package application orquestra os casos de uso do módulo finance.
//
// Responsabilidades: casos de uso de registro de movimentações, consulta de
// saldos, gerenciamento de metas, declaração de ports (TransactionRepository,
// CategoryRepository, EventPublisher) e coordenação via UnitOfWork[T].
// Este pacote NÃO pode importar adapters nem bibliotecas de IO concretas.
package application

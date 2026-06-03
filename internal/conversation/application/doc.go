// Package application orquestra os casos de uso do módulo conversation.
//
// Responsabilidades: casos de uso de recebimento e envio de mensagens, roteamento
// de intent, declaração de ports (MessageRepository, LLMPort, EventPublisher) e
// coordenação via UnitOfWork[T]. Este pacote NÃO pode importar infrastructure nem
// bibliotecas de IO concretas.
package application

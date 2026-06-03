// Package application orquestra os casos de uso do módulo agent.
//
// Responsabilidades: casos de uso de execução de agente, despacho de ferramentas,
// declaração de ports (ToolRegistry, LLMProvider, EventPublisher) e controle
// de budget via UnitOfWork[T]. Este pacote NÃO pode importar infrastructure nem
// bibliotecas de IO concretas.
package application

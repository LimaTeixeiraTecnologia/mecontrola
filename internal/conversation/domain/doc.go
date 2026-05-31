// Package domain contém as regras de negócio puras do módulo conversation.
//
// Responsabilidades: mensagem, thread conversacional, intent, contexto de sessão
// e ciclo de vida da conversa via WhatsApp. Este pacote é o coração hexagonal
// do módulo conversation e NÃO pode importar application, adapters, infrastructure,
// configs ou qualquer biblioteca de IO. Todo código aqui é portável e testável
// sem banco, sem HTTP e sem LLM.
package domain

// Package adapters implementa os ports declarados em application para o módulo conversation.
//
// Responsabilidades: implementações concretas de MessageRepository (Postgres),
// LLMPort (OpenAI/cliente HTTP), handlers de webhook WhatsApp (Chi) e
// adaptadores de eventbus. Este pacote PODE importar domain, application e infrastructure.
package adapters

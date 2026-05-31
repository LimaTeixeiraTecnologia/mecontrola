// Package adapters implementa os ports declarados em application para o módulo agent.
//
// Responsabilidades: implementações concretas de ToolRegistry, LLMProvider
// (OpenAI gpt-4o-mini), handlers HTTP de execução de agente e adaptadores de
// eventbus. Este pacote PODE importar domain, application e infrastructure.
package adapters

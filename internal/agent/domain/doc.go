// Package domain contém as regras de negócio puras do módulo agent.
//
// Responsabilidades: agente conversacional, ferramentas (tools) registradas,
// prompt registry, working memory e budget de custo de inferência. Este pacote
// é o coração hexagonal do módulo agent e NÃO pode importar application,
// adapters, infrastructure, configs ou qualquer biblioteca de IO. Todo código
// aqui é portável e testável sem banco, sem HTTP e sem provider LLM.
package domain

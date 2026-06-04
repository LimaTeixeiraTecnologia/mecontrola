// Package id fornece geração de identificadores UUID v4 para uso transversal
// entre módulos do monolito. A implementação delega a github.com/google/uuid
// e não mantém estado, singleton ou init().
package id

import "github.com/google/uuid"

// Generator abstrai a produção de identificadores únicos.
// Deve ser injetado em qualquer componente que precise gerar IDs para
// garantir determinismo em testes via fake.
type Generator interface {
	// NewID retorna um identificador único como string.
	NewID() string
}

// UUIDGenerator é a implementação de Generator baseada em UUID v4.
// Delega diretamente para github.com/google/uuid, que é thread-safe.
type UUIDGenerator struct{}

// NewUUIDGenerator cria uma nova instância de UUIDGenerator.
func NewUUIDGenerator() UUIDGenerator {
	return UUIDGenerator{}
}

// NewID retorna um UUID v4 canônico (com hifens) como string.
func (UUIDGenerator) NewID() string {
	return uuid.NewString()
}

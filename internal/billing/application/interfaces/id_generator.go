package interfaces

// IDGenerator é o port para geração de identificadores únicos (UUID v4).
// Implementação concreta em infrastructure/id/uuid_generator.go.
// Em testes, substituível por fake determinístico.
type IDGenerator interface {
	NewID() string
}

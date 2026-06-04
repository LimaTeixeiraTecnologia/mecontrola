package interfaces

// IDGenerator é o port para geração de identificadores únicos.
// A implementação concreta vive em internal/platform/id e delega a
// google/uuid.NewString, produzindo UUID v4. Em testes, substitui-se por
// fake determinístico sem dependência de entropia do sistema.
type IDGenerator interface {
	// NewID retorna um UUID v4 canônico (com hifens) como string.
	// O caller deve passar o valor para entities.NewUserID quando aplicável.
	NewID() string
}

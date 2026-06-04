package interfaces

import "encoding/json"

// PIIRedactor é o port para remoção de PII de payloads JSONB (ADR-013).
// A implementação concreta (task 7.0) aplica substituição in-process via parse-modify-marshal.
type PIIRedactor interface {
	Strip(payload json.RawMessage) (json.RawMessage, error)
}

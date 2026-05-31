// Package runtime fornece o Value Object AppMode e o bootstrap de subsistemas
// para os subcomandos do binário mecontrola.
// Referência: ADR-010 — spf13/cobra v1.10.2 + binário único com subcomandos server/worker/migrate.
package runtime

import "fmt"

// AppMode representa o modo de operação do binário mecontrola.
// Aceita somente os valores "server" e "worker".
type AppMode string

const (
	// ModeServer sobe o servidor HTTP e o scheduler placeholder.
	ModeServer AppMode = "server"
	// ModeWorker sobe somente o runtime worker (placeholder até PRDs futuros registrarem jobs).
	ModeWorker AppMode = "worker"
)

// ParseAppMode valida e converte uma string em AppMode.
// Retorna erro descritivo se o valor não pertencer ao conjunto {server, worker}.
func ParseAppMode(s string) (AppMode, error) {
	switch AppMode(s) {
	case ModeServer, ModeWorker:
		return AppMode(s), nil
	default:
		return "", fmt.Errorf("app mode inválido %q: deve ser um de {server, worker}", s)
	}
}

// String implementa fmt.Stringer para AppMode.
func (m AppMode) String() string {
	return string(m)
}

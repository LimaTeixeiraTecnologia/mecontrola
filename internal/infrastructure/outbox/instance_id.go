package outbox

import (
	"fmt"
	"os"
)

// NewInstanceID gera o identificador único da réplica atual do worker no formato
// "hostname-pid" (D-11). Garante unicidade mesmo se dois pods compartilharem hostname,
// e é legível em dashboards e queries operacionais.
//
// Se os.Hostname() falhar, usa "unknown" como hostname para não bloquear o boot.
func NewInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}

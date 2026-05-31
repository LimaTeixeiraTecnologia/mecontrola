package runtime

import "fmt"

type AppMode string

const (
	ModeServer AppMode = "server"
	ModeWorker AppMode = "worker"
)

func ParseAppMode(s string) (AppMode, error) {
	switch AppMode(s) {
	case ModeServer, ModeWorker:
		return AppMode(s), nil
	default:
		return "", fmt.Errorf("app mode inválido %q: deve ser um de {server, worker}", s)
	}
}

func (m AppMode) String() string {
	return string(m)
}

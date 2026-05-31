package runtime

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/clock"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
)

type Foundation struct {
	Bus   *events.Bus
	Clock clock.Clock
}

func Bootstrap(cfg *configs.Config, mode AppMode) (App, error) {
	foundation := buildFoundation()
	subsystems := buildSubsystems(cfg, mode, foundation)
	return &app{
		mode:       mode,
		subsystems: subsystems,
	}, nil
}

func buildFoundation() Foundation {
	return Foundation{
		Bus:   events.NewBus(),
		Clock: clock.NewSystemClock(),
	}
}

func buildSubsystems(cfg *configs.Config, mode AppMode, _ Foundation) []Subsystem {
	_ = cfg // será consumido pelos subsistemas reais nas tarefas 4.0–6.0

	switch mode {
	case ModeServer:
		// Tarefas 4.0–6.0 registrarão: observability, database, http server.
		return []Subsystem{}
	case ModeWorker:
		// Tarefas 4.0 e 7.0 registrarão: observability, eventbus worker.
		return []Subsystem{}
	default:
		return []Subsystem{}
	}
}

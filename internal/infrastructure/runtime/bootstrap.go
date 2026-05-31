package runtime

import (
	"fmt"

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

	subsystems, err := buildSubsystems(cfg, mode, foundation)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}

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

func buildSubsystems(cfg *configs.Config, mode AppMode, _ Foundation) ([]Subsystem, error) {
	switch mode {
	case ModeServer:
		return []Subsystem{newLazyServerSubsystem(cfg)}, nil
	case ModeWorker:
		return []Subsystem{}, nil
	default:
		return []Subsystem{}, nil
	}
}

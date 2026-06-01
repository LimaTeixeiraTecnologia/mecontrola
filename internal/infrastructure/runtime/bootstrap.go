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

// bootstrapper encapsula a lógica de construção de Foundation e Subsystems.
// Separado de App para que Bootstrap não precise de funções standalone.
type bootstrapper struct{}

func (b *bootstrapper) buildFoundation() Foundation {
	return Foundation{
		Bus:   events.NewBus(),
		Clock: clock.NewSystemClock(),
	}
}

func (b *bootstrapper) buildSubsystems(cfg *configs.Config, mode AppMode, _ Foundation) ([]Subsystem, error) {
	switch mode {
	case ModeServer:
		return []Subsystem{b.newServerSubsystem(cfg)}, nil
	case ModeWorker:
		return []Subsystem{}, nil
	default:
		return []Subsystem{}, nil
	}
}

func Bootstrap(cfg *configs.Config, mode AppMode) (App, error) {
	b := &bootstrapper{}
	foundation := b.buildFoundation()

	subsystems, err := b.buildSubsystems(cfg, mode, foundation)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}

	return &app{
		mode:       mode,
		subsystems: subsystems,
	}, nil
}

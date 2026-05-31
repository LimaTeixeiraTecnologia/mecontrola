package runtime

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/clock"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
)

// Foundation agrupa os recursos de infraestrutura singleton injetados em todos
// os subsistemas da aplicação.
type Foundation struct {
	// Bus é o eventbus in-process singleton; compartilhado entre todos os módulos.
	Bus *events.Bus
	// Clock é o relógio da aplicação; substituível por FakeClock em testes.
	Clock clock.Clock
}

// Bootstrap inicializa o runtime para o modo especificado, injetando cfg em
// todos os subsistemas. Os subsistemas concretos serão registrados nas tarefas
// 4.0–7.0; aqui apenas o esqueleto é montado com base no AppMode.
//
// Retorna uma implementação de App pronta para Run/Shutdown.
func Bootstrap(cfg *configs.Config, mode AppMode) (App, error) {
	foundation := buildFoundation()
	subsystems := buildSubsystems(cfg, mode, foundation)
	return &app{
		mode:       mode,
		subsystems: subsystems,
	}, nil
}

// buildFoundation instancia os recursos singleton da foundation:
// eventbus tipado e clock do sistema.
func buildFoundation() Foundation {
	return Foundation{
		Bus:   events.NewBus(),
		Clock: clock.NewSystemClock(),
	}
}

// buildSubsystems seleciona e instancia os subsistemas de acordo com o mode.
// Substituir cada placeholder pelo subsistema real nas tarefas subsequentes.
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

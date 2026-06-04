package identity

import (
	chiserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"

	identityinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	identityrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	platformid "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/runtime"
)

type Ports struct {
	UserRepository identityinterfaces.UserRepository
	IDGenerator    identityinterfaces.IDGenerator
}

type Module struct {
	Ports   Ports
	routers []chiserver.Router
	runners []runtime.Runner
}

type Option func(*options)

type options struct {
	db *database.Manager
}

func WithDatabase(db *database.Manager) Option {
	return func(opts *options) {
		opts.db = db
	}
}

func NewModule(opts ...Option) (*Module, error) {
	settings := options{}
	for _, opt := range opts {
		opt(&settings)
	}

	idGenerator := platformid.NewUUIDGenerator()

	module := &Module{
		Ports: Ports{
			IDGenerator: idGenerator,
		},
	}

	if settings.db != nil {
		module.Ports.UserRepository = identityrepo.NewPgxUserRepository(settings.db, idGenerator)
	}

	return module, nil
}

func (m *Module) Routers() []chiserver.Router {
	return m.routers
}

func (m *Module) Runners() []runtime.Runner {
	return m.runners
}

package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	chiserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
	infrahttp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/http"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/observability"
)

// lazyServerSubsystem constrói as dependências (DB + OTel) no momento do Start,
// não no momento do Bootstrap. Isso permite que Bootstrap seja chamado em testes
// sem um banco real disponível.
type lazyServerSubsystem struct {
	cfg    *configs.Config
	server *chiserver.Server
}

func newLazyServerSubsystem(cfg *configs.Config) *lazyServerSubsystem {
	return &lazyServerSubsystem{cfg: cfg}
}

func (h *lazyServerSubsystem) Name() string { return "http" }

func (h *lazyServerSubsystem) Start(ctx context.Context) error {
	mgr, err := database.NewManager(h.cfg)
	if err != nil {
		return fmt.Errorf("http subsystem: database: %w", err)
	}

	prov, _, err := observability.NewProvider(h.cfg)
	if err != nil {
		return fmt.Errorf("http subsystem: observability: %w", err)
	}

	srv, err := infrahttp.NewServer(h.cfg, infrahttp.Deps{
		DB:       mgr,
		Provider: prov,
	})
	if err != nil {
		return fmt.Errorf("http subsystem: criando servidor: %w", err)
	}

	h.server = srv

	go func() {
		if err := srv.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_ = err
		}
	}()

	return nil
}

func (h *lazyServerSubsystem) Stop(ctx context.Context) error {
	if h.server == nil {
		return nil
	}

	if err := h.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("http subsystem: shutdown: %w", err)
	}

	return nil
}

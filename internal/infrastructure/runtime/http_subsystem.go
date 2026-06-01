package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	cfg              *configs.Config
	server           *chiserver.Server
	shutdownDatabase func(context.Context) error
	shutdownProvider func(context.Context) error
}

func (b *bootstrapper) newServerSubsystem(cfg *configs.Config) *lazyServerSubsystem {
	return &lazyServerSubsystem{cfg: cfg}
}

func (h *lazyServerSubsystem) Name() string { return "http" }

func (h *lazyServerSubsystem) Start(ctx context.Context) error {
	mgr, err := database.NewManager(h.cfg)
	if err != nil {
		return fmt.Errorf("http subsystem: database: %w", err)
	}
	h.shutdownDatabase = mgr.Shutdown

	prov, shutdownProvider, err := observability.NewProvider(h.cfg)
	if err != nil {
		_ = mgr.Shutdown(context.Background())
		return fmt.Errorf("http subsystem: observability: %w", err)
	}
	h.shutdownProvider = shutdownProvider

	srv, err := infrahttp.NewServer(h.cfg, infrahttp.Deps{
		DB:       mgr,
		Provider: prov,
	})
	if err != nil {
		_ = mgr.Shutdown(context.Background())
		_ = shutdownProvider(context.Background())
		return fmt.Errorf("http subsystem: criando servidor: %w", err)
	}

	h.server = srv

	go func() {
		if err := srv.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "http server stopped unexpectedly", "error", err)
		}
	}()

	return nil
}

func (h *lazyServerSubsystem) Stop(ctx context.Context) error {
	var errs []error

	if h.server != nil {
		if err := h.server.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("http subsystem: shutdown: %w", err))
		}
	}

	if h.shutdownDatabase != nil {
		if err := h.shutdownDatabase(ctx); err != nil {
			errs = append(errs, fmt.Errorf("http subsystem: database shutdown: %w", err))
		}
	}

	if h.shutdownProvider != nil {
		if err := h.shutdownProvider(ctx); err != nil {
			errs = append(errs, fmt.Errorf("http subsystem: observability shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

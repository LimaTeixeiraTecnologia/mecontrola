package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	chiserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	infrahttp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/http"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
)

// lazyServerSubsystem constrói as dependências (DB + OTel) no momento do Start,
// não no momento do Bootstrap. Isso permite que Bootstrap seja chamado em testes
// sem um banco real disponível.
type lazyServerSubsystem struct {
	cfg  *configs.Config
	stop func(context.Context) error
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

	prov, shutdownProvider, err := observability.NewProvider(h.cfg)
	if err != nil {
		startErr := fmt.Errorf("http subsystem: observability: %w", err)
		if shutdownErr := mgr.Shutdown(context.Background()); shutdownErr != nil {
			return errors.Join(startErr, fmt.Errorf("http subsystem: database shutdown: %w", shutdownErr))
		}
		return startErr
	}

	srv, err := infrahttp.NewServer(h.cfg, infrahttp.Deps{
		DB:       mgr,
		Provider: prov,
	})
	if err != nil {
		startErr := fmt.Errorf("http subsystem: criando servidor: %w", err)
		var shutdownErrs []error
		if databaseErr := mgr.Shutdown(context.Background()); databaseErr != nil {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("http subsystem: database shutdown: %w", databaseErr))
		}
		if providerErr := shutdownProvider(context.Background()); providerErr != nil {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("http subsystem: observability shutdown: %w", providerErr))
		}
		if len(shutdownErrs) > 0 {
			return errors.Join(append([]error{startErr}, shutdownErrs...)...)
		}
		return startErr
	}

	h.stop = buildServerStopFn(srv, mgr, shutdownProvider)

	go func() {
		if err := srv.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "http server stopped unexpectedly", "error", err)
		}
	}()

	return nil
}

func (h *lazyServerSubsystem) Stop(ctx context.Context) error {
	if h.stop == nil {
		return nil
	}
	return h.stop(ctx)
}

func buildServerStopFn(
	srv *chiserver.Server,
	mgr *database.Manager,
	shutdownProvider func(context.Context) error,
) func(context.Context) error {
	return func(ctx context.Context) error {
		var errs []error
		if err := srv.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("http subsystem: shutdown: %w", err))
		}
		if err := mgr.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("http subsystem: database shutdown: %w", err))
		}
		if err := shutdownProvider(ctx); err != nil {
			errs = append(errs, fmt.Errorf("http subsystem: observability shutdown: %w", err))
		}
		return errors.Join(errs...)
	}
}

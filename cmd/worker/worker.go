package worker

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/spf13/cobra"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/runtime"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Sobe o runtime worker MeControla",
		Long:  "Inicializa o runtime worker do MeControla com módulos de processamento em background.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.Context())
		},
	}
}

func Run(ctx context.Context) error {
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	foundation := runtime.NewFoundation()
	logger := slog.Default()

	// Observability primeiro: o database manager consome o provider via WithObservability
	// para emitir métricas de pool e logs estruturados desde o boot.
	provider, _, err := observability.NewProvider(cfg)
	if err != nil {
		return err
	}

	mgr, err := database.NewManager(ctx, cfg, provider.Observability())
	if err != nil {
		shutdownErr := provider.Shutdown(context.Background())
		if shutdownErr != nil {
			return errors.Join(err, shutdownErr)
		}
		return err
	}

	identityModule, err := identity.NewModule(identity.WithDatabase(mgr))
	if err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	billingModule, err := billing.NewModule(
		billing.WithConfig(cfg),
		billing.WithFoundation(foundation),
		billing.WithLogger(logger),
		billing.WithDatabase(mgr),
		billing.WithProvider(provider),
		billing.WithUserRepository(identityModule.Ports.UserRepository),
	)
	if err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	application, err := runtime.NewApp(
		cfg,
		runtime.ModeWorker,
		slices.Concat(
			identityModule.Runners(),
			billingModule.Runners(),
		)...,
	)
	if err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	if err := application.Run(ctx); err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	slog.InfoContext(ctx, "worker running background modules")

	<-ctx.Done()

	return errors.Join(
		application.Shutdown(context.Background()),
		provider.Shutdown(context.Background()),
		mgr.Shutdown(context.Background()),
	)
}

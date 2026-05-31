// Package worker define o subcomando `mecontrola worker`.
// Sobe o runtime worker idle — placeholder até PRDs futuros registrarem jobs.
package worker

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/runtime"
	"github.com/spf13/cobra"
)

// New retorna o comando cobra para `mecontrola worker`.
func New() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Sobe o runtime worker MeControla",
		Long:  "Inicializa o runtime worker idle do MeControla. Aguarda registro de jobs nos PRDs futuros.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := configs.LoadConfig(".")
			if err != nil {
				return err
			}

			application, err := runtime.Bootstrap(cfg, runtime.ModeWorker)
			if err != nil {
				return err
			}

			if err := application.Run(cmd.Context()); err != nil {
				return err
			}

			return application.Shutdown(cmd.Context())
		},
	}
}

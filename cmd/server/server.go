package server

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/runtime"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Sobe o servidor HTTP MeControla",
		Long:  "Inicializa o servidor HTTP e o scheduler placeholder do MeControla.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := configs.LoadConfig(".")
			if err != nil {
				return err
			}

			application, err := runtime.Bootstrap(cfg, runtime.ModeServer)
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

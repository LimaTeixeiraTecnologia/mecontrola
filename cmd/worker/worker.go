package worker

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Sobe o worker MeControla",
		Long:  "Inicializa o worker do MeControla com módulos de processamento em background.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.Context())
		},
	}
}

func Run(ctx context.Context) error {
	_, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	return nil
}

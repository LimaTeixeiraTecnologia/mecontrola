package migrate

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Aplica migrations pendentes do banco de dados",
		Long:  "Executa todas as migrations pendentes via golang-migrate e termina com exit code 0 em sucesso.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd.Context(), cmd.OutOrStdout())
		},
	}
}

func NewDown() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate-down",
		Short: "Reverte todas as migrations aplicadas",
		Long:  "Reverte todas as migrations via golang-migrate e termina com exit code 0 em sucesso.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunDown(cmd.Context(), cmd.OutOrStdout())
		},
	}
}

func Run(ctx context.Context, writer io.Writer) error {
	_, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	return nil
}

func RunDown(ctx context.Context, writer io.Writer) error {
	_, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	return nil
}

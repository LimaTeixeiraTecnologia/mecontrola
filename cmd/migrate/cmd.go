// Package migrate define o subcomando `mecontrola migrate`.
// Aplica as migrations pendentes via golang-migrate e termina.
// Não sobe servidor HTTP nem worker.
package migrate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
)

// New retorna o comando cobra para `mecontrola migrate`.
func New() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Aplica migrations pendentes do banco de dados",
		Long:  "Executa todas as migrations pendentes via golang-migrate e termina com exit code 0 em sucesso.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := configs.LoadConfig(".")
			if err != nil {
				return err
			}

			mgr, err := database.NewManager(cfg)
			if err != nil {
				return fmt.Errorf("conectando ao banco: %w", err)
			}
			defer func() { _ = mgr.Shutdown(context.Background()) }()

			if err := database.RunMigrations(cmd.Context(), mgr); err != nil {
				return fmt.Errorf("executando migrations: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "migrations aplicadas com sucesso")
			return nil
		},
	}
}

// NewDown retorna o comando cobra para `mecontrola migrate-down`.
func NewDown() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate-down",
		Short: "Reverte todas as migrations aplicadas",
		Long:  "Reverte todas as migrations via golang-migrate e termina com exit code 0 em sucesso.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := configs.LoadConfig(".")
			if err != nil {
				return err
			}

			mgr, err := database.NewManager(cfg)
			if err != nil {
				return fmt.Errorf("conectando ao banco: %w", err)
			}
			defer func() { _ = mgr.Shutdown(context.Background()) }()

			if err := database.RunMigrationsDown(cmd.Context(), mgr); err != nil {
				return fmt.Errorf("revertendo migrations: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "migrations revertidas com sucesso")
			return nil
		},
	}
}

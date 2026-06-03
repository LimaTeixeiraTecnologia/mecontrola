package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
)

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

			if err := database.RunMigrations(cmd.Context(), mgr); err != nil {
				runErr := fmt.Errorf("executando migrations: %w", err)
				if shutdownErr := mgr.Shutdown(context.Background()); shutdownErr != nil {
					return errors.Join(runErr, fmt.Errorf("encerrando conexao com banco: %w", shutdownErr))
				}
				return runErr
			}

			if err := mgr.Shutdown(context.Background()); err != nil {
				return fmt.Errorf("encerrando conexao com banco: %w", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "migrations aplicadas com sucesso"); err != nil {
				return fmt.Errorf("escrevendo saida do comando migrate: %w", err)
			}
			return nil
		},
	}
}

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

			if err := database.RunMigrationsDown(cmd.Context(), mgr); err != nil {
				runErr := fmt.Errorf("revertendo migrations: %w", err)
				if shutdownErr := mgr.Shutdown(context.Background()); shutdownErr != nil {
					return errors.Join(runErr, fmt.Errorf("encerrando conexao com banco: %w", shutdownErr))
				}
				return runErr
			}

			if err := mgr.Shutdown(context.Background()); err != nil {
				return fmt.Errorf("encerrando conexao com banco: %w", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "migrations revertidas com sucesso"); err != nil {
				return fmt.Errorf("escrevendo saida do comando migrate-down: %w", err)
			}
			return nil
		},
	}
}

package migrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
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
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	mgr, err := database.NewManager(ctx, cfg, nil)
	if err != nil {
		return fmt.Errorf("conectando ao banco: %w", err)
	}

	if csv := os.Getenv("ADMIN_WHATSAPP_NUMBERS"); csv != "" {
		if err := database.SetAdminWhatsAppNumbers(ctx, mgr, csv); err != nil {
			shutdownErr := mgr.Shutdown(context.Background())
			if shutdownErr != nil {
				return errors.Join(err, fmt.Errorf("encerrando conexao com banco: %w", shutdownErr))
			}
			return err
		}
	}

	if err := database.RunMigrations(ctx, mgr); err != nil {
		runErr := fmt.Errorf("executando migrations: %w", err)
		if shutdownErr := mgr.Shutdown(context.Background()); shutdownErr != nil {
			return errors.Join(runErr, fmt.Errorf("encerrando conexao com banco: %w", shutdownErr))
		}
		return runErr
	}

	if err := mgr.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("encerrando conexao com banco: %w", err)
	}

	if _, err := fmt.Fprintln(writer, "migrations aplicadas com sucesso"); err != nil {
		return fmt.Errorf("escrevendo saida do comando migrate: %w", err)
	}

	return nil
}

func RunDown(ctx context.Context, writer io.Writer) error {
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	mgr, err := database.NewManager(ctx, cfg, nil)
	if err != nil {
		return fmt.Errorf("conectando ao banco: %w", err)
	}

	if err := database.RunMigrationsDown(ctx, mgr); err != nil {
		runErr := fmt.Errorf("revertendo migrations: %w", err)
		if shutdownErr := mgr.Shutdown(context.Background()); shutdownErr != nil {
			return errors.Join(runErr, fmt.Errorf("encerrando conexao com banco: %w", shutdownErr))
		}
		return runErr
	}

	if err := mgr.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("encerrando conexao com banco: %w", err)
	}

	if _, err := fmt.Fprintln(writer, "migrations revertidas com sucesso"); err != nil {
		return fmt.Errorf("escrevendo saida do comando migrate-down: %w", err)
	}

	return nil
}

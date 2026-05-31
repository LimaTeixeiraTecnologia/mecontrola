package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/migrate"
	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	root := &cobra.Command{
		Use:   "mecontrola",
		Short: "MeControla — agente financeiro conversacional",
		Long: `MeControla é um agente financeiro conversacional via WhatsApp.

Utilize um dos subcomandos para iniciar a aplicação:
  server   — sobe o servidor HTTP e o scheduler placeholder
  worker   — sobe o runtime worker idle
  migrate  — aplica as migrations pendentes e termina`,
		SilenceUsage: true,
	}

	root.SetContext(ctx)
	root.AddCommand(
		server.New(),
		worker.New(),
		migrate.New(),
		migrate.NewDown(),
	)

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err) //nolint:errcheck
		os.Exit(1)
	}
}

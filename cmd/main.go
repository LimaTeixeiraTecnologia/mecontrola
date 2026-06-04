package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/migrate"
	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/worker"
)

func main() {
	root := &cobra.Command{
		Use:   "mecontrola",
		Short: "MeControla — agente financeiro conversacional",
		Long: `MeControla é um agente financeiro conversacional via WhatsApp.

Utilize um dos subcomandos para iniciar a aplicação:
  server   — sobe o servidor HTTP e o scheduler placeholder
  worker   — sobe o worker de módulos em background
  migrate  — aplica as migrations pendentes e termina`,
		SilenceUsage: true,
	}

	root.AddCommand(
		server.New(),
		worker.New(),
		migrate.New(),
		migrate.NewDown(),
	)

	if err := root.Execute(); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}

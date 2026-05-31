// Package main é o entrypoint do binário mecontrola via spf13/cobra.
// Registra os subcomandos server, worker e migrate e executa o root command.
// Sem lógica de negócio — cobertura de testes excluída (D-22).
package main

import (
	"fmt"
	"os"

	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/migrate"
	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/worker"
	"github.com/spf13/cobra"
)

func main() {
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

	root.AddCommand(
		server.New(),
		worker.New(),
		migrate.New(),
		migrate.NewDown(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

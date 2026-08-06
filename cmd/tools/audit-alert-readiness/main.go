package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
)

const defaultStatusDoc = ".specs/prd-alertas-proativos/meta-templates-status.md"

func main() {
	statusDoc := flag.String("status-doc", defaultStatusDoc, "caminho do quadro de status de templates Meta")
	allowlistRaw := flag.String("approved-kinds", "", "kinds liberados para envio real")
	consentSource := flag.Bool("consent-source", false, "existe fonte de consentimento MARKETING implantada")
	asJSON := flag.Bool("json", false, "emite o relatorio em JSON")
	flag.Parse()

	raw, err := os.ReadFile(*statusDoc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-alert-readiness: ler %s: %v\n", *statusDoc, err)
		os.Exit(1)
	}

	rows := ParseTemplateRows(string(raw))
	if len(rows) == 0 {
		fmt.Fprintf(os.Stderr, "audit-alert-readiness: nenhum template encontrado em %s\n", *statusDoc)
		os.Exit(1)
	}

	report := DecideAll(rows, ParseAllowlist(*allowlistRaw), *consentSource)
	problems := Inconsistencies(report)

	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(map[string]any{"readiness": report, "inconsistencies": problems}); err != nil {
			fmt.Fprintf(os.Stderr, "audit-alert-readiness: encode: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := printTable(report, problems); err != nil {
			fmt.Fprintf(os.Stderr, "audit-alert-readiness: print table: %v\n", err)
			os.Exit(1)
		}
	}

	if len(problems) > 0 {
		os.Exit(1)
	}
}

func printTable(report []Readiness, problems []string) error {
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "KIND\tCATEGORIA\tMETA\tRELEASE1\tALLOWLIST\tENTREGAVEL\tMOTIVO"); err != nil {
		return err
	}
	for _, entry := range report {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%t\t%t\t%t\t%s\n",
			entry.Kind, entry.Category, entry.MetaStatus,
			entry.ReleaseOne, entry.Allowlisted, entry.Deliverable, entry.Reason,
		); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	if len(problems) == 0 {
		_, err := fmt.Fprintln(os.Stdout, "\nOK: nenhuma inconsistencia de rollout detectada.")
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, "\nFAIL: inconsistencias de rollout"); err != nil {
		return err
	}
	for _, problem := range problems {
		if _, err := fmt.Fprintf(os.Stdout, "  - %s\n", problem); err != nil {
			return err
		}
	}
	return nil
}

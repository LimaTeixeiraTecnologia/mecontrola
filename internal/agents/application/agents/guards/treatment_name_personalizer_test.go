package guards

import (
	"context"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

func TestTreatmentNamePersonalizerGuard(t *testing.T) {
	scenarios := []struct {
		name    string
		system  string
		content string
		want    string
		handled bool
	}{
		{
			name:    "metadata json",
			system:  `## Working Memory Metadata` + "\n" + `{"nome_tratamento":"Stef","objetivo_financeiro":"Comprar uma casa"}`,
			content: "Não consegui registrar essa movimentação agora.",
			want:    "Stef, não consegui registrar essa movimentação agora.",
			handled: true,
		},
		{
			name:    "working memory fallback",
			system:  "## Working Memory\n## Nome de Tratamento\n\nJJ\n\n## Objetivo Financeiro\n\nReserva",
			content: "Registro concluído.",
			want:    "JJ, registro concluído.",
			handled: true,
		},
		{
			name:    "ja contem nome",
			system:  `## Working Memory Metadata` + "\n" + `{"nome_tratamento":"Stef"}`,
			content: "Stef, registrei aqui.",
			handled: false,
		},
		{
			name:    "corrige casing do nome vigente",
			system:  "## Working Memory\n## Nome de Tratamento\n\nJJ\n\n## Objetivo Financeiro\n\nReserva",
			content: "(RF-05), jJ, neste mês você já gastou *R$ 250,00*.",
			want:    "(RF-05), JJ, neste mês você já gastou *R$ 250,00*.",
			handled: true,
		},
		{
			name:    "sem nome",
			system:  "## Working Memory\n## Objetivo Financeiro\n\nReserva",
			content: "Registro concluído.",
			handled: false,
		},
		{
			name:    "emoji inicial preservado",
			system:  `## Working Memory Metadata` + "\n" + `{"nome_tratamento":"Stef"}`,
			content: "💰 Valor confirmado.",
			want:    "Stef, 💰 Valor confirmado.",
			handled: true,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			guard := NewTreatmentNamePersonalizerGuard()
			decision := guard.Inspect(context.Background(), agent.Request{
				Messages: []llm.Message{{Role: "system", Content: scenario.system}},
			}, agent.Result{Content: scenario.content})
			if decision.Handled != scenario.handled {
				t.Fatalf("Handled = %v; want %v", decision.Handled, scenario.handled)
			}
			if !scenario.handled {
				return
			}
			if decision.Result.Content != scenario.want {
				t.Fatalf("Content = %q; want %q", decision.Result.Content, scenario.want)
			}
		})
	}
}

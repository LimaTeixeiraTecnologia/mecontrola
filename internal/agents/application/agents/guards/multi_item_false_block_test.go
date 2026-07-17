package guards

import (
	"context"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

func TestMultiItemFalseBlockGuard(t *testing.T) {
	scenarios := []struct {
		name        string
		userMessage string
		result      agent.Result
		wantHandled bool
	}{
		{
			name:        "falso bloqueio em item unico e sobreposto",
			userMessage: "Gastei 30 na padaria",
			result:      agent.Result{Content: "Percebi mais de um lançamento na mesma mensagem. Por segurança, registro um de cada vez"},
			wantHandled: true,
		},
		{
			name:        "multiplo genuino nao e tocado",
			userMessage: "gastei 30 no onibus e 15 no cafe",
			result:      agent.Result{Content: "Percebi mais de um lançamento na mesma mensagem"},
			wantHandled: false,
		},
		{
			name:        "sem marcador passa",
			userMessage: "Gastei 30 na padaria",
			result:      agent.Result{Content: "Prontinho! ✅"},
			wantHandled: false,
		},
		{
			name:        "com escrita bem sucedida nao e tocado",
			userMessage: "Gastei 30 na padaria",
			result: agent.Result{
				Content:   "um de cada vez",
				ToolCalls: []agent.ToolCallRecord{{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess}},
			},
			wantHandled: false,
		},
	}

	guard := NewMultiItemFalseBlockGuard()
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			decision := guard.Inspect(context.Background(), agent.Request{
				Messages: []llm.Message{{Role: "user", Content: scenario.userMessage}},
			}, scenario.result)

			if decision.Handled != scenario.wantHandled {
				t.Fatalf("Handled = %v; want %v", decision.Handled, scenario.wantHandled)
			}
			if scenario.wantHandled && decision.Result.Content != multiItemFalseBlockReask {
				t.Fatalf("content = %q; want reask", decision.Result.Content)
			}
		})
	}
}

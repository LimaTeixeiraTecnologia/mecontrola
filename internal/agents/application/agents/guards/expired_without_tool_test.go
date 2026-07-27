package guards

import (
	"context"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

func TestExpiredWithoutToolGuard(t *testing.T) {
	scenarios := []struct {
		name        string
		result      agent.Result
		wantHandled bool
	}{
		{
			name:        "expiracao alucinada sem tool call e reescrita",
			result:      agent.Result{Content: "O registro expirou. Para registrar, envie a informação completa novamente."},
			wantHandled: true,
		},
		{
			name:        "expiracao com maiusculas variadas tambem e reescrita",
			result:      agent.Result{Content: "O REGISTRO EXPIROU. Tente de novo."},
			wantHandled: true,
		},
		{
			name:        "sem marcador passa",
			result:      agent.Result{Content: "Prontinho! ✅"},
			wantHandled: false,
		},
		{
			name: "com tool call nao e tocado",
			result: agent.Result{
				Content:   "O registro expirou. Para continuar, envie a informação completa novamente. 🙂",
				ToolCalls: []agent.ToolCallRecord{{Tool: "edit_entry", Outcome: agent.ToolCallOutcomeSuccess}},
			},
			wantHandled: false,
		},
	}

	guard := NewExpiredWithoutToolGuard()
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			decision := guard.Inspect(context.Background(), agent.Request{}, scenario.result)

			if decision.Handled != scenario.wantHandled {
				t.Fatalf("Handled = %v; want %v", decision.Handled, scenario.wantHandled)
			}
			if scenario.wantHandled && decision.Result.Content != expiredWithoutToolReask {
				t.Fatalf("content = %q; want reask", decision.Result.Content)
			}
		})
	}
}

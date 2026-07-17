package guards

import (
	"context"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type stubExpenseTool struct {
	id      string
	invoked bool
	args    []byte
	message string
}

func (s *stubExpenseTool) ID() string                 { return s.id }
func (s *stubExpenseTool) Description() string        { return "" }
func (s *stubExpenseTool) Parameters() map[string]any { return map[string]any{} }
func (s *stubExpenseTool) Invoke(_ context.Context, argsJSON []byte) ([]byte, string, error) {
	s.invoked = true
	s.args = argsJSON
	return []byte(`{}`), s.message, nil
}

func TestParseRegisterExpenseShortcut(t *testing.T) {
	scenarios := []struct {
		name    string
		input   string
		wantOK  bool
		wantAmt int64
		wantDsc string
	}{
		{name: "producao gastei 500 no mercado", input: "Gastei 500 no mercado", wantOK: true, wantAmt: 50000, wantDsc: "mercado"},
		{name: "producao gastei 20 no cinema", input: "Gastei 20 no cinema", wantOK: true, wantAmt: 2000, wantDsc: "cinema"},
		{name: "producao gastei 30 na padaria", input: "Gastei 30 na padaria", wantOK: true, wantAmt: 3000, wantDsc: "padaria"},
		{name: "paguei conta de luz", input: "paguei 200 na conta de luz", wantOK: true, wantAmt: 20000, wantDsc: "conta de luz"},
		{name: "separador de milhar", input: "gastei 1.234,56 no mercado", wantOK: true, wantAmt: 123456, wantDsc: "mercado"},
		{name: "credito nao dispara", input: "comprei 500 no cartão nubank", wantOK: false},
		{name: "forma de pagamento nao dispara", input: "gastei 500 no mercado no pix", wantOK: false},
		{name: "pergunta nao dispara", input: "quanto gastei no mercado?", wantOK: false},
		{name: "multiplos nao dispara", input: "gastei 30 no onibus e 15 no cafe", wantOK: false},
		{name: "receita nao dispara", input: "Recebi 2 mil de salario", wantOK: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			args, ok := parseRegisterExpenseShortcut(scenario.input, nil)
			if ok != scenario.wantOK {
				t.Fatalf("parseRegisterExpenseShortcut(%q) ok = %v; want %v", scenario.input, ok, scenario.wantOK)
			}
			if !scenario.wantOK {
				return
			}
			amt, amtOK := args["amountCents"].(int64)
			if !amtOK || amt != scenario.wantAmt {
				t.Fatalf("amountCents = %v; want %d", args["amountCents"], scenario.wantAmt)
			}
			if dsc, _ := args["description"].(string); dsc != scenario.wantDsc {
				t.Fatalf("description = %q; want %q", args["description"], scenario.wantDsc)
			}
		})
	}
}

func TestRegisterExpenseShortcutGuardRoutes(t *testing.T) {
	handle := &stubExpenseTool{id: "register_expense", message: "Como você pagou? 💳"}
	guard := NewRegisterExpenseShortcutGuard(handle)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "Gastei 500 no mercado"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit determinístico para despesa única")
	}
	if !handle.invoked {
		t.Fatal("esperava invocação de register_expense")
	}
	if decision.Result.Content != "Como você pagou? 💳" {
		t.Fatalf("content = %q", decision.Result.Content)
	}
	if len(decision.Result.ToolCalls) != 1 || decision.Result.ToolCalls[0].Tool != "register_expense" {
		t.Fatalf("tool calls = %+v", decision.Result.ToolCalls)
	}
}

func TestRegisterExpenseShortcutGuardIgnoresNonExpense(t *testing.T) {
	handle := &stubExpenseTool{id: "register_expense"}
	guard := NewRegisterExpenseShortcutGuard(handle)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "quanto gastei esse mês?"}},
	})

	if decision.Handled {
		t.Fatal("não deveria interceptar consulta")
	}
	if handle.invoked {
		t.Fatal("não deveria invocar register_expense")
	}
}

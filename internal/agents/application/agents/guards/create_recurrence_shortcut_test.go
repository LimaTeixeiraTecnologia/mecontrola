package guards

import (
	"context"
	"strings"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type stubRecurrenceTool struct {
	id      string
	invoked bool
	args    []byte
	message string
}

func (s *stubRecurrenceTool) ID() string                 { return s.id }
func (s *stubRecurrenceTool) Description() string        { return "" }
func (s *stubRecurrenceTool) Parameters() map[string]any { return map[string]any{} }
func (s *stubRecurrenceTool) Invoke(_ context.Context, argsJSON []byte) ([]byte, string, error) {
	s.invoked = true
	s.args = argsJSON
	return []byte(`{}`), s.message, nil
}

func TestParseCreateRecurrenceShortcut(t *testing.T) {
	scenarios := []struct {
		name    string
		input   string
		wantOK  bool
		wantDay int
		wantAmt int64
		wantDsc string
		wantDir string
		wantPay string
	}{
		{name: "producao todo dia 5 pago 1500 de aluguel", input: "todo dia 5 pago 1500 de aluguel", wantOK: true, wantDay: 5, wantAmt: 150000, wantDsc: "aluguel", wantDir: "outcome"},
		{name: "producao todo dia 5 eu recebo salario", input: "todo dia 5 eu recebo R$ 13.874,40 de salário", wantOK: true, wantDay: 5, wantAmt: 1387440, wantDsc: "salário", wantDir: "income"},
		{name: "ganhei recorrente", input: "todo dia 10 ganhei 200 de comissão", wantOK: true, wantDay: 10, wantAmt: 20000, wantDsc: "comissão", wantDir: "income"},
		{name: "com centavos", input: "todo dia 10 pago 99,90 de internet", wantOK: true, wantDay: 10, wantAmt: 9990, wantDsc: "internet", wantDir: "outcome"},
		{name: "receita recorrente", input: "todo dia 1 recebo 5000 de salário", wantOK: true, wantDay: 1, wantAmt: 500000, wantDsc: "salário", wantDir: "income"},
		{name: "com pagamento no fim", input: "todo dia 5 pago 1500 de aluguel no pix", wantOK: true, wantDay: 5, wantAmt: 150000, wantDsc: "aluguel", wantDir: "outcome", wantPay: "pix"},
		{name: "milhar com ponto", input: "todo dia 20 paguei 1.200 de condomínio", wantOK: true, wantDay: 20, wantAmt: 120000, wantDsc: "condomínio", wantDir: "outcome"},
		{name: "producao todo mes eu recebo 800 de pensao sem dia", input: "todo mês eu recebo 800 de pensão", wantOK: true, wantDay: 0, wantAmt: 80000, wantDsc: "pensão", wantDir: "income"},
		{name: "todo mes sem acento despesa sem dia", input: "todo mes pago 100 de luz", wantOK: true, wantDay: 0, wantAmt: 10000, wantDsc: "luz", wantDir: "outcome"},
		{name: "todo mes sem verbo nao dispara", input: "todo mês tem aluguel de 1500", wantOK: false},
		{name: "cartao nao dispara", input: "todo dia 5 pago 300 de streaming no cartão", wantOK: false},
		{name: "dia 29 dispara e clamp materializa no ultimo dia", input: "todo dia 29 pago 100 de luz", wantOK: true, wantDay: 29, wantAmt: 10000, wantDsc: "luz", wantDir: "outcome"},
		{name: "dia invalido nao dispara", input: "todo dia 32 pago 100 de luz", wantOK: false},
		{name: "sem verbo nao dispara", input: "todo dia 5 tem aluguel de 1500", wantOK: false},
		{name: "despesa avulsa nao dispara", input: "gastei 30 no mercado", wantOK: false},
		{name: "pergunta nao dispara", input: "quais são minhas recorrências?", wantOK: false},
		{name: "edicao nao dispara", input: "muda a recorrência do dia 5 para o dia 10", wantOK: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			args, ok := parseCreateRecurrenceShortcut(scenario.input, nil)
			if ok != scenario.wantOK {
				t.Fatalf("parseCreateRecurrenceShortcut(%q) ok = %v; want %v", scenario.input, ok, scenario.wantOK)
			}
			if !scenario.wantOK {
				return
			}
			amt, amtOK := args["amountCents"].(int64)
			if !amtOK || amt != scenario.wantAmt {
				t.Fatalf("amountCents = %v; want %d", args["amountCents"], scenario.wantAmt)
			}
			if day, _ := args["dayOfMonth"].(int); day != scenario.wantDay {
				t.Fatalf("dayOfMonth = %v; want %d", args["dayOfMonth"], scenario.wantDay)
			}
			if dsc, _ := args["description"].(string); dsc != scenario.wantDsc {
				t.Fatalf("description = %q; want %q", args["description"], scenario.wantDsc)
			}
			if dir, _ := args["direction"].(string); dir != scenario.wantDir {
				t.Fatalf("direction = %q; want %q", args["direction"], scenario.wantDir)
			}
			if freq, _ := args["frequency"].(string); freq != "monthly" {
				t.Fatalf("frequency = %q; want monthly", args["frequency"])
			}
			pay, _ := args["paymentMethod"].(string)
			if pay != scenario.wantPay {
				t.Fatalf("paymentMethod = %q; want %q", pay, scenario.wantPay)
			}
		})
	}
}

func TestCreateRecurrenceShortcutGuardRoutes(t *testing.T) {
	handle := &stubRecurrenceTool{id: "create_recurrence", message: "Posso registrar?"}
	guard := NewCreateRecurrenceShortcutGuard(handle)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "todo dia 5 pago 1500 de aluguel"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit determinístico para recorrência mensal")
	}
	if !handle.invoked {
		t.Fatal("esperava invocação de create_recurrence")
	}
	if decision.Result.Content != "Posso registrar?" {
		t.Fatalf("content = %q", decision.Result.Content)
	}
	if len(decision.Result.ToolCalls) != 1 || decision.Result.ToolCalls[0].Tool != "create_recurrence" {
		t.Fatalf("tool calls = %+v", decision.Result.ToolCalls)
	}
}

func TestCreateRecurrenceShortcutGuardRoutesTodoMesSemDia(t *testing.T) {
	handle := &stubRecurrenceTool{id: "create_recurrence", message: "Em qual dia do mês ele se repete?"}
	guard := NewCreateRecurrenceShortcutGuard(handle)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "todo mês eu recebo 800 de pensão"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit determinístico para recorrência mensal sem dia")
	}
	if !handle.invoked {
		t.Fatal("esperava invocação de create_recurrence")
	}
	if strings.Contains(string(handle.args), "dayOfMonth") {
		t.Fatalf("dayOfMonth não deveria ser enviado: %s", handle.args)
	}
}

func TestCreateRecurrenceShortcutGuardIgnoresNonRecurrence(t *testing.T) {
	handle := &stubRecurrenceTool{id: "create_recurrence"}
	guard := NewCreateRecurrenceShortcutGuard(handle)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "gastei 30 no mercado"}},
	})

	if decision.Handled {
		t.Fatal("não deveria interceptar despesa avulsa")
	}
	if handle.invoked {
		t.Fatal("não deveria invocar create_recurrence")
	}
}

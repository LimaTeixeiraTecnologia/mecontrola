package guards

import (
	"context"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type stubEditEntryTool struct {
	id      string
	invoked bool
	args    []byte
	message string
}

func (s *stubEditEntryTool) ID() string                 { return s.id }
func (s *stubEditEntryTool) Description() string        { return "" }
func (s *stubEditEntryTool) Parameters() map[string]any { return map[string]any{} }
func (s *stubEditEntryTool) Invoke(_ context.Context, argsJSON []byte) ([]byte, string, error) {
	s.invoked = true
	s.args = argsJSON
	return []byte(`{}`), s.message, nil
}

func TestParseEditEntryCorrectionShortcut(t *testing.T) {
	scenarios := []struct {
		name       string
		input      string
		wantOK     bool
		wantDesc   string
		wantAmt    int64
		wantSearch int64
	}{
		{
			name:       "producao no cinema eu gastei 30 e nao 29",
			input:      "No cinema eu gastei 30 e não 29, posso editar?",
			wantOK:     true,
			wantDesc:   "cinema",
			wantAmt:    3000,
			wantSearch: 2900,
		},
		{
			name:       "producao corrige lancamento do cinema valor certo",
			input:      "Corrige o lançamento do cinema, o valor certo é 30 e não 29",
			wantOK:     true,
			wantDesc:   "cinema",
			wantAmt:    3000,
			wantSearch: 2900,
		},
		{
			name:       "producao corrige lancamento da tv valor certo",
			input:      "Corrige o lançamento da TV, o valor certo é 6100 e não 6000",
			wantOK:     true,
			wantDesc:   "TV",
			wantAmt:    610000,
			wantSearch: 600000,
		},
		{
			name:     "producao muda valor sem valor antigo",
			input:    "No lançamento da TV, muda o valor para 6100 reais",
			wantOK:   true,
			wantDesc: "TV",
			wantAmt:  610000,
		},
		{
			name:       "producao corrige lancamento do mercado",
			input:      "Corrige o lançamento do mercado, o valor certo é 550 e não 500",
			wantOK:     true,
			wantDesc:   "mercado",
			wantAmt:    55000,
			wantSearch: 50000,
		},
		{name: "cartao nao dispara", input: "Corrige o cartão nubank, o vencimento certo é 10 e não 5", wantOK: false},
		{
			name:       "producao no cartao eu gastei 35 e nao 30",
			input:      "no cartão eu gastei 35 e não 30",
			wantOK:     true,
			wantDesc:   "cartão",
			wantAmt:    3500,
			wantSearch: 3000,
		},
		{name: "vencimento do cartao nao dispara", input: "no cartão nubank o vencimento certo é 10 e não 5", wantOK: false},
		{name: "orcamento nao dispara", input: "Corrige o orçamento, o valor certo é 3000 e não 2500", wantOK: false},
		{name: "nome de tratamento nao dispara", input: "Corrige meu nome, é João e não Pedro", wantOK: false},
		{name: "sem valor numerico nao dispara", input: "Corrige a categoria do lançamento do cinema", wantOK: false},
		{name: "pergunta simples nao dispara", input: "quanto gastei no cinema esse mês?", wantOK: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			args, ok := parseEditEntryCorrectionShortcut(scenario.input, nil)
			if ok != scenario.wantOK {
				t.Fatalf("parseEditEntryCorrectionShortcut(%q) ok = %v; want %v (args=%v)", scenario.input, ok, scenario.wantOK, args)
			}
			if !scenario.wantOK {
				return
			}
			if desc, _ := args["searchTerm"].(string); desc != scenario.wantDesc {
				t.Fatalf("searchTerm = %q; want %q", args["searchTerm"], scenario.wantDesc)
			}
			amt, amtOK := args["amountCents"].(int64)
			if !amtOK || amt != scenario.wantAmt {
				t.Fatalf("amountCents = %v; want %d", args["amountCents"], scenario.wantAmt)
			}
			if scenario.wantSearch != 0 {
				search, searchOK := args["searchAmountCents"].(int64)
				if !searchOK || search != scenario.wantSearch {
					t.Fatalf("searchAmountCents = %v; want %d", args["searchAmountCents"], scenario.wantSearch)
				}
			} else if _, present := args["searchAmountCents"]; present {
				t.Fatalf("searchAmountCents não deveria estar presente, veio %v", args["searchAmountCents"])
			}
		})
	}
}

func TestEditEntryCorrectionShortcutGuardRoutes(t *testing.T) {
	handle := &stubEditEntryTool{id: "edit_entry", message: "💰 Valor: R$ 30,00\n\nPosso atualizar?"}
	guard := NewEditEntryCorrectionShortcutGuard(handle)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "Corrige o lançamento do cinema, o valor certo é 30 e não 29"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit determinístico para correção de lançamento")
	}
	if !handle.invoked {
		t.Fatal("esperava invocação de edit_entry")
	}
	if decision.Result.Content != "💰 Valor: R$ 30,00\n\nPosso atualizar?" {
		t.Fatalf("content = %q", decision.Result.Content)
	}
	if len(decision.Result.ToolCalls) != 1 || decision.Result.ToolCalls[0].Tool != "edit_entry" {
		t.Fatalf("tool calls = %+v", decision.Result.ToolCalls)
	}
}

func TestEditEntryCorrectionShortcutGuardIgnoresNonCorrection(t *testing.T) {
	handle := &stubEditEntryTool{id: "edit_entry"}
	guard := NewEditEntryCorrectionShortcutGuard(handle)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "Gastei 30 no cinema"}},
	})

	if decision.Handled {
		t.Fatal("não deveria interceptar registro normal de despesa")
	}
	if handle.invoked {
		t.Fatal("não deveria invocar edit_entry")
	}
}

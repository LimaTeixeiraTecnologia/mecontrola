package guards

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type stubQueryDayTool struct {
	id      string
	invoked bool
	args    []byte
	raw     []byte
	err     error
}

func (s *stubQueryDayTool) ID() string                 { return s.id }
func (s *stubQueryDayTool) Description() string        { return "" }
func (s *stubQueryDayTool) Parameters() map[string]any { return map[string]any{} }
func (s *stubQueryDayTool) Invoke(_ context.Context, argsJSON []byte) ([]byte, string, error) {
	s.invoked = true
	s.args = argsJSON
	return s.raw, "", s.err
}

func TestParseQueryDayShortcut(t *testing.T) {
	scenarios := []struct {
		name     string
		input    string
		wantOK   bool
		wantKind dayQueryKind
		wantDay  string
	}{
		{name: "producao quanto gastei hoje", input: "quanto gastei hoje?", wantOK: true, wantKind: dayQueryExpense, wantDay: "hoje"},
		{name: "producao quanto gastei ontem", input: "quanto gastei ontem?", wantOK: true, wantKind: dayQueryExpense, wantDay: "ontem"},
		{name: "com eu", input: "quanto eu gastei ontem?", wantOK: true, wantKind: dayQueryExpense, wantDay: "ontem"},
		{name: "receita hoje", input: "quanto recebi hoje?", wantOK: true, wantKind: dayQueryIncome, wantDay: "hoje"},
		{name: "receita ontem", input: "Quanto recebi ontem", wantOK: true, wantKind: dayQueryIncome, wantDay: "ontem"},
		{name: "o que gastei", input: "o que eu gastei hoje?", wantOK: true, wantKind: dayQueryExpense, wantDay: "hoje"},
		{name: "lista gastos", input: "quais foram os gastos de hoje?", wantOK: true, wantKind: dayQueryExpense, wantDay: "hoje"},
		{name: "lista recebimentos", input: "quais recebimentos de ontem?", wantOK: true, wantKind: dayQueryIncome, wantDay: "ontem"},
		{name: "me diz", input: "me diz quanto gastei hoje?", wantOK: true, wantKind: dayQueryExpense, wantDay: "hoje"},
		{name: "mes nao dispara", input: "quanto gastei esse mês?", wantOK: false},
		{name: "registro nao dispara", input: "gastei 45 na farmácia", wantOK: false},
		{name: "pergunta com merchant nao dispara", input: "quanto gastei hoje no mercado?", wantOK: false},
		{name: "pergunta sem dia nao dispara", input: "quanto gastei no mercado?", wantOK: false},
		{name: "fatura nao dispara", input: "quanto está minha fatura do nubank?", wantOK: false},
		{name: "semana nao dispara", input: "quanto gastei essa semana?", wantOK: false},
		{name: "ontem sozinho nao dispara", input: "ontem", wantOK: false},
		{name: "vazio", input: "", wantOK: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			kind, day, ok := parseQueryDayShortcut(scenario.input)
			if ok != scenario.wantOK {
				t.Fatalf("parseQueryDayShortcut(%q) ok = %v; want %v", scenario.input, ok, scenario.wantOK)
			}
			if !scenario.wantOK {
				return
			}
			if kind != scenario.wantKind {
				t.Fatalf("kind = %q; want %q", kind, scenario.wantKind)
			}
			if day != scenario.wantDay {
				t.Fatalf("day = %q; want %q", day, scenario.wantDay)
			}
		})
	}
}

func queryDayRequest(message string) agent.Request {
	return agent.Request{
		Messages: []llm.Message{
			{Role: "user", Content: message},
		},
	}
}

const queryDayStubPayload = `{
	"outcome": "ok",
	"day": "2026-07-28",
	"incomeCents": 190000,
	"incomeBRL": "R$ 1.900,00",
	"outcomeCents": 206700,
	"outcomeBRL": "R$ 2.067,00",
	"totalCents": -16700,
	"totalBRL": "R$ -167,00",
	"entries": [
		{"amountBRL": "R$ 130,00", "direction": "outcome", "description": "mercado"},
		{"amountBRL": "R$ 500,00", "direction": "income", "description": "freelance"},
		{"amountBRL": "R$ 45,00", "direction": "outcome", "description": "farmácia"}
	]
}`

func TestQueryDayShortcutGuard_ExpenseToday(t *testing.T) {
	stub := &stubQueryDayTool{id: "query_day", raw: []byte(queryDayStubPayload)}
	guard := NewQueryDayShortcutGuard(stub)

	decision := guard.Inspect(context.Background(), queryDayRequest("quanto gastei hoje?"))
	if !decision.Handled {
		t.Fatal("guard deveria tratar a consulta de dia")
	}
	if !stub.invoked {
		t.Fatal("tool query_day deveria ter sido invocada")
	}
	if string(stub.args) != `{"dayRefKind":"today"}` {
		t.Fatalf("args = %s; want dayRefKind=today", stub.args)
	}
	content := decision.Result.Content
	wantParts := []string{"Hoje, você gastou um total de *R$ 2.067,00*", "1. *R$ 130,00*: mercado", "2. *R$ 45,00*: farmácia"}
	for _, part := range wantParts {
		if !contains(content, part) {
			t.Fatalf("content não contém %q:\n%s", part, content)
		}
	}
	if contains(content, "freelance") {
		t.Fatalf("consulta de gasto não deve listar receita:\n%s", content)
	}
	if len(decision.Result.ToolCalls) != 1 || decision.Result.ToolCalls[0].Tool != "query_day" {
		t.Fatalf("tool call record ausente: %+v", decision.Result.ToolCalls)
	}
}

func TestQueryDayShortcutGuard_IncomeYesterday(t *testing.T) {
	stub := &stubQueryDayTool{id: "query_day", raw: []byte(queryDayStubPayload)}
	guard := NewQueryDayShortcutGuard(stub)

	decision := guard.Inspect(context.Background(), queryDayRequest("quanto recebi ontem?"))
	if !decision.Handled {
		t.Fatal("guard deveria tratar a consulta de dia")
	}
	if string(stub.args) != `{"dayRefKind":"yesterday"}` {
		t.Fatalf("args = %s; want dayRefKind=yesterday", stub.args)
	}
	content := decision.Result.Content
	if !contains(content, "Ontem, você recebeu um total de *R$ 1.900,00*") {
		t.Fatalf("content inesperado:\n%s", content)
	}
	if !contains(content, "1. *R$ 500,00*: freelance") {
		t.Fatalf("receita não listada:\n%s", content)
	}
	if contains(content, "mercado") {
		t.Fatalf("consulta de receita não deve listar despesa:\n%s", content)
	}
}

func TestQueryDayShortcutGuard_EmptyDay(t *testing.T) {
	stub := &stubQueryDayTool{id: "query_day", raw: []byte(`{"outcome":"ok","day":"2026-07-27","incomeBRL":"R$ 0,00","outcomeBRL":"R$ 0,00","entries":[]}`)}
	guard := NewQueryDayShortcutGuard(stub)

	decision := guard.Inspect(context.Background(), queryDayRequest("quanto gastei ontem?"))
	if !decision.Handled {
		t.Fatal("guard deveria tratar dia vazio")
	}
	if !contains(decision.Result.Content, "Ontem, você não teve lançamentos registrados.") {
		t.Fatalf("content inesperado:\n%s", decision.Result.Content)
	}
}

func TestQueryDayShortcutGuard_ToolErrorFallsThrough(t *testing.T) {
	stub := &stubQueryDayTool{id: "query_day", err: errors.New("db down")}
	guard := NewQueryDayShortcutGuard(stub)

	decision := guard.Inspect(context.Background(), queryDayRequest("quanto gastei hoje?"))
	if decision.Handled {
		t.Fatal("erro da tool deve cair para o fluxo do LLM")
	}
}

func TestQueryDayShortcutGuard_NonQueryPassesThrough(t *testing.T) {
	stub := &stubQueryDayTool{id: "query_day", raw: []byte(queryDayStubPayload)}
	guard := NewQueryDayShortcutGuard(stub)

	decision := guard.Inspect(context.Background(), queryDayRequest("gastei 45 na farmácia no pix"))
	if decision.Handled {
		t.Fatal("registro de despesa não deve ser tratado pelo guard de consulta")
	}
	if stub.invoked {
		t.Fatal("tool não deveria ter sido invocada")
	}
}

func TestQueryDayShortcutGuard_NilHandle(t *testing.T) {
	guard := NewQueryDayShortcutGuard(nil)
	decision := guard.Inspect(context.Background(), queryDayRequest("quanto gastei hoje?"))
	if decision.Handled {
		t.Fatal("handle nil nunca deve tratar")
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

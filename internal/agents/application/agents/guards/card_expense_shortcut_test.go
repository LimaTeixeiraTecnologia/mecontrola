package guards

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type stubCardTool struct {
	id      string
	invoked bool
	args    []byte
	raw     []byte
	message string
}

func (s *stubCardTool) ID() string                 { return s.id }
func (s *stubCardTool) Description() string        { return "" }
func (s *stubCardTool) Parameters() map[string]any { return map[string]any{} }
func (s *stubCardTool) Invoke(_ context.Context, argsJSON []byte) ([]byte, string, error) {
	s.invoked = true
	s.args = argsJSON
	return s.raw, s.message, nil
}

func TestParseCardExpenseShortcut(t *testing.T) {
	scenarios := []struct {
		name    string
		input   string
		wantOK  bool
		wantAmt int64
		wantDsc string
		wantNik string
		wantIns int
	}{
		{name: "producao gastei 45 no cartao nubank", input: "gastei 45 no cartão nubank", wantOK: true, wantAmt: 4500, wantDsc: "crédito", wantNik: "nubank"},
		{name: "capitalizado", input: "Gastei 45 no cartão Nubank", wantOK: true, wantAmt: 4500, wantDsc: "crédito", wantNik: "Nubank"},
		{name: "producao comprei 50 no credito", input: "comprei 50 no crédito", wantOK: true, wantAmt: 5000, wantDsc: "crédito"},
		{name: "producao eletronico em 3x", input: "comprei 600 de eletrônico em 3x no crédito", wantOK: true, wantAmt: 60000, wantDsc: "eletrônico", wantIns: 3},
		{name: "cartao de credito com apelido", input: "gastei 45 no cartão de crédito nubank", wantOK: true, wantAmt: 4500, wantDsc: "crédito", wantNik: "nubank"},
		{name: "comprei no cartao com apelido", input: "comprei 500 no cartão nubank", wantOK: true, wantAmt: 50000, wantDsc: "crédito", wantNik: "nubank"},
		{name: "item antes do cartao sem apelido", input: "gastei 45 na farmácia no cartão", wantOK: true, wantAmt: 4500, wantDsc: "farmácia"},
		{name: "item com conector da", input: "paguei 100 da passagem no cartão nubank", wantOK: true, wantAmt: 10000, wantDsc: "passagem", wantNik: "nubank"},
		{name: "credito com apelido colado", input: "comprei 50 no crédito nubank", wantOK: true, wantAmt: 5000, wantDsc: "crédito", wantNik: "nubank"},
		{name: "parcelei com parcelas e apelido", input: "parcelei 600 em 3x no cartão nubank", wantOK: true, wantAmt: 60000, wantDsc: "crédito", wantNik: "nubank", wantIns: 3},
		{name: "valor com milhar", input: "comprei 1.250,90 de tv em 10x no cartão nubank", wantOK: true, wantAmt: 125090, wantDsc: "tv", wantNik: "nubank", wantIns: 10},
		{name: "fatura nao dispara", input: "paguei 300 da fatura do cartão nubank", wantOK: false},
		{name: "pergunta nao dispara", input: "quanto gastei no cartão?", wantOK: false},
		{name: "pix nao dispara", input: "gastei 45 na farmácia no pix", wantOK: false},
		{name: "receita nao dispara", input: "recebi 500 no crédito", wantOK: false},
		{name: "multiplo nao dispara", input: "gastei 45 no cartão nubank e 15 no café", wantOK: false},
		{name: "parcelas acima do limite nao dispara", input: "comprei 600 de eletrônico em 25x no crédito", wantOK: false},
		{name: "apelido longo demais nao dispara", input: "gastei 45 no cartão que eu uso para tudo", wantOK: false},
		{name: "sem valor nao dispara", input: "gastei no cartão nubank", wantOK: false},
		{name: "consulta fatura nao dispara", input: "qual a fatura do cartão nubank?", wantOK: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parsed, ok := parseCardExpenseShortcut(scenario.input)
			if ok != scenario.wantOK {
				t.Fatalf("parseCardExpenseShortcut(%q) ok = %v; want %v", scenario.input, ok, scenario.wantOK)
			}
			if !scenario.wantOK {
				return
			}
			if parsed.amountCents != scenario.wantAmt {
				t.Fatalf("amountCents = %d; want %d", parsed.amountCents, scenario.wantAmt)
			}
			if parsed.description != scenario.wantDsc {
				t.Fatalf("description = %q; want %q", parsed.description, scenario.wantDsc)
			}
			if parsed.nickname != scenario.wantNik {
				t.Fatalf("nickname = %q; want %q", parsed.nickname, scenario.wantNik)
			}
			if parsed.installments != scenario.wantIns {
				t.Fatalf("installments = %d; want %d", parsed.installments, scenario.wantIns)
			}
		})
	}
}

func TestCardExpenseShortcutGuardRoutesWithNickname(t *testing.T) {
	resolve := &stubCardTool{id: "resolve_card", raw: []byte(`{"found":true,"cardId":"11111111-2222-3333-4444-555555555555","nickname":"nubank","bank":"nubank","dueDay":1}`)}
	register := &stubCardTool{id: "register_expense", raw: []byte(`{"outcome":"clarify"}`), message: "Qual cartão foi utilizado?"}
	guard := NewCardExpenseShortcutGuard(register, resolve)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "gastei 45 no cartão nubank"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit determinístico para despesa no cartão")
	}
	if !resolve.invoked {
		t.Fatal("esperava invocação de resolve_card")
	}
	var resolveArgs map[string]any
	if err := json.Unmarshal(resolve.args, &resolveArgs); err != nil {
		t.Fatalf("resolve args inválidos: %v", err)
	}
	if resolveArgs["nickname"] != "nubank" {
		t.Fatalf("nickname = %v", resolveArgs["nickname"])
	}
	if !register.invoked {
		t.Fatal("esperava invocação de register_expense")
	}
	var registerArgs map[string]any
	if err := json.Unmarshal(register.args, &registerArgs); err != nil {
		t.Fatalf("register args inválidos: %v", err)
	}
	if registerArgs["cardId"] != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("cardId = %v", registerArgs["cardId"])
	}
	if registerArgs["paymentMethod"] != "credit_card" {
		t.Fatalf("paymentMethod = %v", registerArgs["paymentMethod"])
	}
	if registerArgs["description"] != "crédito" {
		t.Fatalf("description = %v", registerArgs["description"])
	}
	if decision.Result.Content != "Qual cartão foi utilizado?" {
		t.Fatalf("content = %q", decision.Result.Content)
	}
	if len(decision.Result.ToolCalls) != 2 || decision.Result.ToolCalls[0].Tool != "resolve_card" || decision.Result.ToolCalls[1].Tool != "register_expense" {
		t.Fatalf("tool calls = %+v", decision.Result.ToolCalls)
	}
}

func TestCardExpenseShortcutGuardRoutesWithoutNickname(t *testing.T) {
	resolve := &stubCardTool{id: "resolve_card"}
	register := &stubCardTool{id: "register_expense", raw: []byte(`{"outcome":"clarify"}`), message: "Qual cartão foi utilizado?"}
	guard := NewCardExpenseShortcutGuard(register, resolve)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "comprei 50 no crédito"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit determinístico para despesa no crédito sem apelido")
	}
	if resolve.invoked {
		t.Fatal("não deveria invocar resolve_card sem apelido")
	}
	if !register.invoked {
		t.Fatal("esperava invocação de register_expense")
	}
	var registerArgs map[string]any
	if err := json.Unmarshal(register.args, &registerArgs); err != nil {
		t.Fatalf("register args inválidos: %v", err)
	}
	if _, hasCard := registerArgs["cardId"]; hasCard {
		t.Fatal("não deveria enviar cardId sem apelido")
	}
	if registerArgs["paymentMethod"] != "credit_card" {
		t.Fatalf("paymentMethod = %v", registerArgs["paymentMethod"])
	}
	if len(decision.Result.ToolCalls) != 1 || decision.Result.ToolCalls[0].Tool != "register_expense" {
		t.Fatalf("tool calls = %+v", decision.Result.ToolCalls)
	}
}

func TestCardExpenseShortcutGuardRoutesWhenCardNotFound(t *testing.T) {
	resolve := &stubCardTool{id: "resolve_card", raw: []byte(`{"found":false,"cardId":"","nickname":"","bank":"","dueDay":0}`)}
	register := &stubCardTool{id: "register_expense", raw: []byte(`{"outcome":"clarify"}`), message: "Qual cartão foi utilizado?"}
	guard := NewCardExpenseShortcutGuard(register, resolve)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "comprei 180 de mochila no cartão roxinho"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit mesmo com cartão não encontrado")
	}
	if !resolve.invoked || !register.invoked {
		t.Fatal("esperava resolve_card e register_expense invocados")
	}
	var registerArgs map[string]any
	if err := json.Unmarshal(register.args, &registerArgs); err != nil {
		t.Fatalf("register args inválidos: %v", err)
	}
	if _, hasCard := registerArgs["cardId"]; hasCard {
		t.Fatal("não deveria enviar cardId quando found=false")
	}
}

func TestCardExpenseShortcutGuardPassesInstallments(t *testing.T) {
	resolve := &stubCardTool{id: "resolve_card", raw: []byte(`{"found":true,"cardId":"11111111-2222-3333-4444-555555555555","nickname":"nubank","bank":"nubank","dueDay":1}`)}
	register := &stubCardTool{id: "register_expense", raw: []byte(`{"outcome":"clarify"}`), message: "resumo"}
	guard := NewCardExpenseShortcutGuard(register, resolve)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "comprei 600 de eletrônico em 3x no crédito nubank"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit para compra parcelada")
	}
	var registerArgs map[string]any
	if err := json.Unmarshal(register.args, &registerArgs); err != nil {
		t.Fatalf("register args inválidos: %v", err)
	}
	if registerArgs["installments"] != float64(3) {
		t.Fatalf("installments = %v", registerArgs["installments"])
	}
	if registerArgs["description"] != "eletrônico" {
		t.Fatalf("description = %v", registerArgs["description"])
	}
}

func TestCardExpenseShortcutGuardIgnoresNonCardExpense(t *testing.T) {
	resolve := &stubCardTool{id: "resolve_card"}
	register := &stubCardTool{id: "register_expense"}
	guard := NewCardExpenseShortcutGuard(register, resolve)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "gastei 45 na farmácia no pix"}},
	})

	if decision.Handled {
		t.Fatal("não deveria interceptar despesa fora do cartão")
	}
	if resolve.invoked || register.invoked {
		t.Fatal("não deveria invocar ferramentas")
	}
}

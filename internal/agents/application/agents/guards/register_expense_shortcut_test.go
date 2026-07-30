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
		wantPay string
	}{
		{name: "producao gastei 500 no mercado", input: "Gastei 500 no mercado", wantOK: true, wantAmt: 50000, wantDsc: "mercado"},
		{name: "producao gastei 20 no cinema", input: "Gastei 20 no cinema", wantOK: true, wantAmt: 2000, wantDsc: "cinema"},
		{name: "producao gastei 30 na padaria", input: "Gastei 30 na padaria", wantOK: true, wantAmt: 3000, wantDsc: "padaria"},
		{name: "paguei conta de luz", input: "paguei 200 na conta de luz", wantOK: true, wantAmt: 20000, wantDsc: "conta de luz"},
		{name: "separador de milhar", input: "gastei 1.234,56 no mercado", wantOK: true, wantAmt: 123456, wantDsc: "mercado"},
		{name: "pix no fim dispara com pagamento", input: "gastei 500 no mercado no pix", wantOK: true, wantAmt: 50000, wantDsc: "mercado", wantPay: "pix"},
		{name: "producao farmacia no pix", input: "gastei 45 na farmácia no pix", wantOK: true, wantAmt: 4500, wantDsc: "farmácia", wantPay: "pix"},
		{name: "producao uber no pix", input: "gastei 30 no uber no pix", wantOK: true, wantAmt: 3000, wantDsc: "uber", wantPay: "pix"},
		{name: "debito no fim", input: "paguei 100 no mercado no débito", wantOK: true, wantAmt: 10000, wantDsc: "mercado", wantPay: "debit_card"},
		{name: "debito em conta antes de debito", input: "paguei 80 na conta de água no débito em conta", wantOK: true, wantAmt: 8000, wantDsc: "conta de água", wantPay: "debit_in_account"},
		{name: "dinheiro no fim", input: "gastei 25 no táxi em dinheiro", wantOK: true, wantAmt: 2500, wantDsc: "táxi", wantPay: "cash"},
		{name: "pix sem conector", input: "gastei 50 na feira pix", wantOK: true, wantAmt: 5000, wantDsc: "feira", wantPay: "pix"},
		{name: "mercado pago nao confunde com descricao mercado", input: "gastei 60 no mercado no mercado pago", wantOK: true, wantAmt: 6000, wantDsc: "mercado", wantPay: "mercado_pago"},
		{name: "apple pay duas palavras", input: "paguei 99 no iphone com apple pay", wantOK: true, wantAmt: 9900, wantDsc: "iphone", wantPay: "apple_pay"},
		{name: "credito nao dispara", input: "comprei 500 no cartão nubank", wantOK: false},
		{name: "comprei simples dispara", input: "Comprei 150 no mercado", wantOK: true, wantAmt: 15000, wantDsc: "mercado"},
		{name: "comprei com pagamento dispara", input: "comprei 45 na farmácia no pix", wantOK: true, wantAmt: 4500, wantDsc: "farmácia", wantPay: "pix"},
		{name: "fiz uma compra dispara", input: "Fiz uma compra de 150 no mercado", wantOK: true, wantAmt: 15000, wantDsc: "mercado"},
		{name: "fiz a compra dispara", input: "Fiz a compra de 150 no mercado", wantOK: true, wantAmt: 15000, wantDsc: "mercado"},
		{name: "realizei uma compra dispara", input: "Realizei uma compra de 150 no mercado", wantOK: true, wantAmt: 15000, wantDsc: "mercado"},
		{name: "realizei a compra com pagamento dispara", input: "realizei a compra de 45 na farmácia no pix", wantOK: true, wantAmt: 4500, wantDsc: "farmácia", wantPay: "pix"},
		{name: "fiz a compra no cartao nao dispara", input: "fiz a compra de 500 no cartão nubank", wantOK: false},
		{name: "fiz a compra do mes com clausula composta dispara", input: "Fiz a compra do mês e foi 150 no mercado", wantOK: true, wantAmt: 15000, wantDsc: "mercado"},
		{name: "fiz a compra do mes com pagamento debito dispara", input: "Fiz a compra do mês e foi 150 no mercado paguei no débito", wantOK: true, wantAmt: 15000, wantDsc: "mercado", wantPay: "debit_card"},
		{name: "fiz compra mercado e paguei no debito dispara", input: "Fiz a compra de 150 reais no mercado e paguei no débito", wantOK: true, wantAmt: 15000, wantDsc: "mercado", wantPay: "debit_card"},
		{name: "pagamento de fatura dispara como despesa", input: "paguei 300 da fatura do cartão nubank", wantOK: true, wantAmt: 30000, wantDsc: "fatura do cartão nubank"},
		{name: "pagamento de fatura com pagamento no fim", input: "paguei 150 da fatura no pix", wantOK: true, wantAmt: 15000, wantDsc: "fatura", wantPay: "pix"},
		{name: "consulta de fatura nao dispara", input: "qual a fatura do cartão nubank?", wantOK: false},
		{name: "consulta de fatura sem valor nao dispara", input: "quanto está a fatura do nubank?", wantOK: false},
		{name: "cartao no fim nao dispara", input: "gastei 45 na farmácia no cartão", wantOK: false},
		{name: "vale pelado dispara como pagamento", input: "gastei 45 no almoço no vale", wantOK: true, wantAmt: 4500, wantDsc: "almoço", wantPay: "vale_refeicao"},
		{name: "vale refeicao extenso dispara", input: "gastei 45 no almoço no vale refeição", wantOK: true, wantAmt: 4500, wantDsc: "almoço", wantPay: "vale_refeicao"},
		{name: "vale alimentacao extenso dispara", input: "gastei 45 no almoço no vale alimentação", wantOK: true, wantAmt: 4500, wantDsc: "almoço", wantPay: "vale_alimentacao"},
		{name: "vr abreviado dispara", input: "gastei 45 no almoço no vr", wantOK: true, wantAmt: 4500, wantDsc: "almoço", wantPay: "vale_refeicao"},
		{name: "va abreviado dispara", input: "gastei 45 no almoço no va", wantOK: true, wantAmt: 4500, wantDsc: "almoço", wantPay: "vale_alimentacao"},
		{name: "vale refeicao com hifen nao dispara", input: "gastei 45 no almoço no vale-refeição", wantOK: false},
		{name: "parcelado nao dispara", input: "gastei 45 na farmácia parcelado", wantOK: false},
		{name: "pix grudado em palavra nao dispara", input: "gastei 50 no pixel art", wantOK: true, wantAmt: 5000, wantDsc: "pixel art"},
		{name: "ted grudado em palavra nao vira pagamento", input: "gastei 50 no united", wantOK: true, wantAmt: 5000, wantDsc: "united"},
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
			pay, _ := args["paymentMethod"].(string)
			if pay != scenario.wantPay {
				t.Fatalf("paymentMethod = %q; want %q", pay, scenario.wantPay)
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

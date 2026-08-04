package guards

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

func TestParseQueryCardInvoiceShortcut(t *testing.T) {
	scenarios := []struct {
		name    string
		input   string
		wantOK  bool
		wantNik string
	}{
		{name: "consulta direta", input: "qual a fatura do cartão nubank?", wantOK: true, wantNik: "nubank"},
		{name: "follow up com artigo", input: "e a fatura do meu cartão nubank?", wantOK: true, wantNik: "nubank"},
		{name: "como esta", input: "como está a fatura do cartão inter?", wantOK: true, wantNik: "inter"},
		{name: "pagamento da fatura nao dispara", input: "paguei 300 da fatura do cartão nubank", wantOK: false},
		{name: "compra no cartao nao dispara", input: "gastei 45 no cartão nubank", wantOK: false},
		{name: "sem apelido nao dispara", input: "e a fatura do meu cartão?", wantOK: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			got, ok := parseQueryCardInvoiceShortcut(scenario.input)
			if ok != scenario.wantOK {
				t.Fatalf("parseQueryCardInvoiceShortcut(%q) ok=%v want=%v", scenario.input, ok, scenario.wantOK)
			}
			if !scenario.wantOK {
				return
			}
			if got != scenario.wantNik {
				t.Fatalf("nickname=%q want=%q", got, scenario.wantNik)
			}
		})
	}
}

func TestQueryCardInvoiceShortcutGuardRoutesResolvedCard(t *testing.T) {
	resolve := &stubCardTool{id: "resolve_card", raw: []byte(`{"found":true,"cardId":"11111111-2222-3333-4444-555555555555","nickname":"nubank","bank":"nubank","dueDay":1}`)}
	query := &stubCardTool{id: "query_card_invoice", raw: []byte(`{"id":"inv-1","cardId":"11111111-2222-3333-4444-555555555555","refMonth":"2026-08","closingAt":"2026-08-25T00:00:00Z","dueAt":"2026-09-01T00:00:00Z","itemsTotalCents":45000,"itemsTotalBRL":"R$ 450,00","items":[{"id":"1"}],"outcome":"","message":""}`)}
	guard := NewQueryCardInvoiceShortcutGuard(resolve, query, nil)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "e a fatura do meu cartão nubank?"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit determinístico para consulta de fatura")
	}
	if !resolve.invoked || !query.invoked {
		t.Fatal("esperava invocação de resolve_card e query_card_invoice")
	}
	var resolveArgs map[string]any
	if err := json.Unmarshal(resolve.args, &resolveArgs); err != nil {
		t.Fatalf("resolve args inválidos: %v", err)
	}
	if resolveArgs["nickname"] != "nubank" {
		t.Fatalf("nickname=%v", resolveArgs["nickname"])
	}
	var queryArgs map[string]any
	if err := json.Unmarshal(query.args, &queryArgs); err != nil {
		t.Fatalf("query args inválidos: %v", err)
	}
	if queryArgs["cardId"] != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("cardId=%v", queryArgs["cardId"])
	}
	if len(decision.Result.ToolCalls) != 2 || decision.Result.ToolCalls[0].Tool != "resolve_card" || decision.Result.ToolCalls[1].Tool != "query_card_invoice" {
		t.Fatalf("tool calls=%+v", decision.Result.ToolCalls)
	}
}

func TestQueryCardInvoiceShortcutGuardFallsBackToListCards(t *testing.T) {
	resolve := &stubCardTool{id: "resolve_card", raw: []byte(`{"found":false,"cardId":"","nickname":"","bank":"","dueDay":0}`)}
	query := &stubCardTool{id: "query_card_invoice"}
	list := &stubCardTool{id: "list_cards", raw: []byte(`{"cards":[{"nickname":"Nubank","bank":"Nubank","dueDay":1}]}`)}
	guard := NewQueryCardInvoiceShortcutGuard(resolve, query, list)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "qual a fatura do meu cartão roxinho?"}},
	})

	if !decision.Handled {
		t.Fatal("esperava tratamento quando o cartão não é encontrado")
	}
	if !resolve.invoked || !list.invoked {
		t.Fatal("esperava invocação de resolve_card e list_cards")
	}
	if query.invoked {
		t.Fatal("não deveria consultar fatura sem cardId resolvido")
	}
	if len(decision.Result.ToolCalls) != 2 || decision.Result.ToolCalls[1].Tool != "list_cards" {
		t.Fatalf("tool calls=%+v", decision.Result.ToolCalls)
	}
}

package guards

import (
	"context"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type stubBudgetTool struct {
	id      string
	invoked bool
	args    []byte
	raw     []byte
}

func (s *stubBudgetTool) ID() string                 { return s.id }
func (s *stubBudgetTool) Description() string        { return "" }
func (s *stubBudgetTool) Parameters() map[string]any { return map[string]any{} }
func (s *stubBudgetTool) Invoke(_ context.Context, argsJSON []byte) ([]byte, string, error) {
	s.invoked = true
	s.args = argsJSON
	return s.raw, "", nil
}

func TestBudgetWriteShortcutGuardRoutesDistribution(t *testing.T) {
	adjust := &stubBudgetTool{
		id:  "adjust_allocation",
		raw: []byte(`{"outcome":"started","competence":"2026-08","confirmationPrompt":"📊 Confirma a nova distribuição?"}`),
	}
	total := &stubBudgetTool{id: "edit_budget_total"}
	guard := NewBudgetWriteShortcutGuard(adjust, total)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "Alterar meus orçamentos para: Custos fixos 50%, Metas 15%, Prazeres 10%, Conhecimento 5%, Liberdade Financeira 20%"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit para alteração de distribuição")
	}
	if !adjust.invoked {
		t.Fatal("esperava invocação de adjust_allocation")
	}
	if total.invoked {
		t.Fatal("não deveria invocar edit_budget_total")
	}
	if got := decision.Result.Content; got != "📊 Confirma a nova distribuição?" {
		t.Fatalf("content = %q", got)
	}
	if got := decision.Result.ToolOutcome; got != agent.ToolOutcomeRouted {
		t.Fatalf("tool outcome = %v", got)
	}
	if len(decision.Result.ToolCalls) != 1 || decision.Result.ToolCalls[0].Tool != "adjust_allocation" {
		t.Fatalf("tool calls = %+v", decision.Result.ToolCalls)
	}
}

func TestBudgetWriteShortcutGuardRoutesTotal(t *testing.T) {
	adjust := &stubBudgetTool{id: "adjust_allocation"}
	total := &stubBudgetTool{
		id:  "edit_budget_total",
		raw: []byte(`{"outcome":"started","competence":"2026-08","confirmationPrompt":"💰 Qual é o novo valor total?"}`),
	}
	guard := NewBudgetWriteShortcutGuard(adjust, total)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "Quero alterar meu orçamento total para R$ 4.000,00"}},
	})

	if !decision.Handled {
		t.Fatal("esperava short-circuit para alteração de total")
	}
	if !total.invoked {
		t.Fatal("esperava invocação de edit_budget_total")
	}
	if adjust.invoked {
		t.Fatal("não deveria invocar adjust_allocation")
	}
	if got := decision.Result.Content; got != "💰 Qual é o novo valor total?" {
		t.Fatalf("content = %q", got)
	}
	if got := decision.Result.ToolOutcome; got != agent.ToolOutcomeRouted {
		t.Fatalf("tool outcome = %v", got)
	}
}

func TestBudgetWriteShortcutGuardPassesNamedMonth(t *testing.T) {
	args, ok := parseBudgetDistributionShortcut("Quero mudar a distribuição do orçamento de junho para: Metas 20%, Prazeres 10%, Custo Fixo 45%, Conhecimento 5%, Liberdade Financeira 20%")
	if !ok {
		t.Fatal("esperava parse de mês nomeado sem ano")
	}
	if got := args["monthRefKind"]; got != "named_without_year" {
		t.Fatalf("monthRefKind = %v", got)
	}
	if got := args["month"]; got != 6 {
		t.Fatalf("month = %v", got)
	}
}

func TestBudgetWriteShortcutGuardIgnoresBudgetQuery(t *testing.T) {
	adjust := &stubBudgetTool{id: "adjust_allocation"}
	total := &stubBudgetTool{id: "edit_budget_total"}
	guard := NewBudgetWriteShortcutGuard(adjust, total)

	decision := guard.Inspect(context.Background(), agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "Como está meu orçamento?"}},
	})

	if decision.Handled {
		t.Fatal("não deveria interceptar consulta de orçamento")
	}
	if adjust.invoked || total.invoked {
		t.Fatal("não deveria invocar tools de escrita")
	}
}

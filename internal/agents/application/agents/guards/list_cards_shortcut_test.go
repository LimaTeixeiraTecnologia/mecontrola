package guards

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type listCardsShortcutTool struct {
	id       string
	schema   map[string]any
	payload  []byte
	verbatim string
	err      error
}

func (t *listCardsShortcutTool) ID() string                 { return t.id }
func (t *listCardsShortcutTool) Description() string        { return "" }
func (t *listCardsShortcutTool) Parameters() map[string]any { return t.schema }
func (t *listCardsShortcutTool) Invoke(_ context.Context, _ []byte) ([]byte, string, error) {
	return t.payload, t.verbatim, t.err
}

func TestListCardsShortcutGuardSuite(t *testing.T) {
	suite.Run(t, new(ListCardsShortcutGuardSuite))
}

type ListCardsShortcutGuardSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *ListCardsShortcutGuardSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ListCardsShortcutGuardSuite) TestName() {
	guard := NewListCardsShortcutGuard(nil)
	s.Equal("list_cards_shortcut", guard.Name())
}

func (s *ListCardsShortcutGuardSuite) TestIsListCardsRequest() {
	scenarios := []struct {
		name    string
		message string
		expect  bool
	}{
		{"listagem explicita com emoji", "quais são meus 💳?", true},
		{"listagem explicita com cartoes", "quais são meus cartões?", true},
		{"listar cartoes", "lista meus cartões", true},
		{"mostrar cartoes", "mostra meus 💳", true},
		{"ver cartoes", "quero ver meus cartões", true},
		{"meus cartoes", "meus cartões", true},
		{"compra no cartao nao dispara", "comprei uma bota no cartão nubank", false},
		{"fatura nao dispara", "quanto está minha fatura do 💳 nubank?", false},
		{"cadastro nao dispara", "cadastra meu 💳 Nubank", false},
		{"recusa cadastro nao dispara", "não quero cadastrar 💳 agora", false},
		{"melhor dia nao dispara", "qual melhor dia para comprar no cartão nubank vencimento 10?", false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, isListCardsRequest(scenario.message))
		})
	}
}

func (s *ListCardsShortcutGuardSuite) TestInspect_ListCardsRequest_CallsToolAndFormatsResponse() {
	handle := &listCardsShortcutTool{
		id:      "list_cards",
		payload: []byte(`{"cards":[{"nickname":"Nubank","bank":"Nubank","dueDay":1},{"nickname":"Inter","bank":"Inter","dueDay":10}]}`),
	}
	guard := NewListCardsShortcutGuard(handle)
	decision := guard.Inspect(s.ctx, agent.Request{Messages: []llm.Message{
		{Role: "user", Content: "quais são meus 💳?"},
	}})

	s.True(decision.Handled)
	s.Contains(decision.Result.Content, "💳")
	s.Contains(decision.Result.Content, "Nubank")
	s.Contains(decision.Result.Content, "Inter")
	s.Len(decision.Result.ToolCalls, 1)
	s.Equal("list_cards", decision.Result.ToolCalls[0].Tool)
}

func (s *ListCardsShortcutGuardSuite) TestInspect_ListCardsRequest_EmptyCards() {
	handle := &listCardsShortcutTool{
		id:      "list_cards",
		payload: []byte(`{"cards":[]}`),
	}
	guard := NewListCardsShortcutGuard(handle)
	decision := guard.Inspect(s.ctx, agent.Request{Messages: []llm.Message{
		{Role: "user", Content: "quais são meus 💳?"},
	}})

	s.True(decision.Handled)
	s.Contains(decision.Result.Content, "💳")
	s.NotContains(decision.Result.Content, "cartões cadastrados")
}

func (s *ListCardsShortcutGuardSuite) TestInspect_NotListCardsRequest_Passes() {
	handle := &listCardsShortcutTool{id: "list_cards"}
	guard := NewListCardsShortcutGuard(handle)
	decision := guard.Inspect(s.ctx, agent.Request{Messages: []llm.Message{
		{Role: "user", Content: "comprei uma bota no 💳 nubank"},
	}})

	s.False(decision.Handled)
}

func (s *ListCardsShortcutGuardSuite) TestInspect_NoHandle_Passes() {
	guard := NewListCardsShortcutGuard(nil)
	decision := guard.Inspect(s.ctx, agent.Request{Messages: []llm.Message{
		{Role: "user", Content: "quais são meus 💳?"},
	}})

	s.False(decision.Handled)
}

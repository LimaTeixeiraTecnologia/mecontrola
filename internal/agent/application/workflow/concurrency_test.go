package workflow

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type SharedRegistryConcurrencySuite struct {
	suite.Suite
	ctx context.Context
}

func TestSharedRegistryConcurrencySuite(t *testing.T) {
	suite.Run(t, new(SharedRegistryConcurrencySuite))
}

func (s *SharedRegistryConcurrencySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SharedRegistryConcurrencySuite) echoTool(name string, kind intent.Kind, calls *atomic.Int64) tools.Tool {
	return tools.NewTool(tools.ToolSpec{Name: name, IntentKind: kind, Description: name}, func(_ context.Context, in tools.ToolInput) (tools.ToolResult, error) {
		calls.Add(1)
		return tools.ToolResult{Reply: in.Channel + "|" + in.Text + "|" + in.MessageID + "|" + in.UserID.String(), Outcome: tools.OutcomeRouted, Kind: kind}, nil
	})
}

func (s *SharedRegistryConcurrencySuite) sharedRegistry(writeCalls, readCalls *atomic.Int64) *IntentRegistry {
	transactions, err := NewIntentWorkflow("transactions", KindTool{Kind: intent.KindRecordExpense, Tool: s.echoTool("expense", intent.KindRecordExpense, writeCalls)})
	s.Require().NoError(err)
	cards, err := NewIntentWorkflow("cards", KindTool{Kind: intent.KindListCards, Tool: s.echoTool("list_cards", intent.KindListCards, readCalls)})
	s.Require().NoError(err)
	registry, err := NewIntentRegistry([]intent.Kind{intent.KindRecordExpense, intent.KindListCards}, transactions, cards)
	s.Require().NoError(err)
	return registry
}

func (s *SharedRegistryConcurrencySuite) execute(registry *IntentRegistry, kind intent.Kind, in intent.Intent, userID uuid.UUID, channel, text, messageID string) {
	wf, ok := registry.Resolve(kind)
	s.Require().True(ok)
	result, execErr := wf.Execute(s.ctx, tools.ToolInput{UserID: userID, Channel: channel, Intent: in, Text: text, MessageID: messageID})
	s.Require().NoError(execErr)
	s.Equal(channel+"|"+text+"|"+messageID+"|"+userID.String(), result.Reply)
	s.Equal(tools.OutcomeRouted, result.Outcome)
}

func (s *SharedRegistryConcurrencySuite) TestConcurrentExecuteIsolatesRequestScopedInput() {
	writeCalls := atomic.Int64{}
	readCalls := atomic.Int64{}
	registry := s.sharedRegistry(&writeCalls, &readCalls)

	const goroutines = 64
	wg := sync.WaitGroup{}
	wg.Add(goroutines)
	for i := range goroutines {
		go func(n int) {
			defer wg.Done()
			userID := uuid.New()
			channel := fmt.Sprintf("chan-%d", n)
			text := fmt.Sprintf("gastei %d no mercado", n)
			messageID := fmt.Sprintf("wamid.%d", n)
			writeIn, buildErr := intent.NewRecordExpense(intent.RecordExpenseFields{AmountCents: int64(100 + n), Merchant: "mercado", CategoryHint: "Prazeres"})
			s.Require().NoError(buildErr)
			s.execute(registry, intent.KindRecordExpense, writeIn, userID, channel, text, messageID)
			s.execute(registry, intent.KindListCards, intent.NewListCards(), userID, channel, text, messageID)
		}(i)
	}
	wg.Wait()

	s.Equal(int64(goroutines), writeCalls.Load())
	s.Equal(int64(goroutines), readCalls.Load())
}

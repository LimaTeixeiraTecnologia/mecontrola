package services

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

type concurrentParser struct {
	calls atomic.Int64
}

func (p *concurrentParser) Parse(_ context.Context, _ uuid.UUID, _ string) (ParsedIntent, error) {
	p.calls.Add(1)
	return ParsedIntent{Intent: intent.NewHowAmIDoing()}, nil
}

type concurrentMonthlySummary struct {
	calls atomic.Int64
}

func (m *concurrentMonthlySummary) Execute(_ context.Context, _ string, _ string) (budgetsoutput.MonthlySummaryOutput, error) {
	m.calls.Add(1)
	return budgetsoutput.MonthlySummaryOutput{}, nil
}

type concurrentFallback struct {
	calls atomic.Int64
}

func (f *concurrentFallback) Reply(_ context.Context, _ uuid.UUID, _, _ string) (string, error) {
	f.calls.Add(1)
	return "fallback", nil
}

type SharedAgentConcurrencySuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestSharedAgentConcurrencySuite(t *testing.T) {
	suite.Run(t, new(SharedAgentConcurrencySuite))
}

func (s *SharedAgentConcurrencySuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
}

func (s *SharedAgentConcurrencySuite) TestSharedRegistryHandleIsRaceFree() {
	counter := func() observability.Counter { return s.obs.Metrics().Counter("agent_test_counter", "", "1") }
	parser := &concurrentParser{}
	summary := &concurrentMonthlySummary{}
	fallback := &concurrentFallback{}
	deps := IntentRouterDeps{
		Parser:         parser,
		Fallback:       fallback,
		MonthlySummary: summary,
	}

	agent, err := newDailyLedgerAgent(s.obs, counter(), counter(), counter(), counter(), time.UTC, deps)
	s.Require().NoError(err)

	const goroutines = 64
	wg := sync.WaitGroup{}
	wg.Add(goroutines)
	for i := range goroutines {
		go func(n int) {
			defer wg.Done()
			principal := Principal{UserID: uuid.New()}
			channel := fmt.Sprintf("chan-%d", n)
			result := agent.Handle(s.ctx, principal, channel, "peer", "como estou indo?", fmt.Sprintf("wamid.%d", n))
			s.Equal(tools.OutcomeRouted, result.Outcome)
			s.Equal(intent.KindHowAmIDoing, result.Kind)
		}(i)
	}
	wg.Wait()

	s.Equal(int64(goroutines), parser.calls.Load())
	s.Equal(int64(goroutines), summary.calls.Load())
}

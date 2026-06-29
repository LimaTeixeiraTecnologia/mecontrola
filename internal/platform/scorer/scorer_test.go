package scorer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
)

type TypesSuite struct {
	suite.Suite
}

func TestTypesSuite(t *testing.T) {
	suite.Run(t, new(TypesSuite))
}

func (s *TypesSuite) TestScorerKind_String() {
	scenarios := []struct {
		kind     ScorerKind
		expected string
	}{
		{ScorerKindCodeBased, "code_based"},
		{ScorerKindLLMJudged, "llm_judged"},
		{ScorerKind(99), "unknown"},
	}
	for _, sc := range scenarios {
		s.Run(sc.expected, func() {
			s.Equal(sc.expected, sc.kind.String())
		})
	}
}

func (s *TypesSuite) TestScorerKind_IsValid() {
	s.True(ScorerKindCodeBased.IsValid())
	s.True(ScorerKindLLMJudged.IsValid())
	s.False(ScorerKind(0).IsValid())
	s.False(ScorerKind(99).IsValid())
}

func (s *TypesSuite) TestParseScorerKind() {
	type args struct{ s string }
	scenarios := []struct {
		name   string
		args   args
		expect func(k ScorerKind, err error)
	}{
		{
			name: "code_based",
			args: args{"code_based"},
			expect: func(k ScorerKind, err error) {
				s.NoError(err)
				s.Equal(ScorerKindCodeBased, k)
			},
		},
		{
			name: "llm_judged",
			args: args{"llm_judged"},
			expect: func(k ScorerKind, err error) {
				s.NoError(err)
				s.Equal(ScorerKindLLMJudged, k)
			},
		},
		{
			name: "invalid",
			args: args{"unknown_kind"},
			expect: func(k ScorerKind, err error) {
				s.Error(err)
				s.Zero(k)
			},
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			k, err := ParseScorerKind(sc.args.s)
			sc.expect(k, err)
		})
	}
}

func (s *TypesSuite) TestSamplingType_String() {
	scenarios := []struct {
		t        SamplingType
		expected string
	}{
		{SamplingTypeRatio, "ratio"},
		{SamplingTypeAlways, "always"},
		{SamplingTypeNever, "never"},
		{SamplingType(99), "unknown"},
	}
	for _, sc := range scenarios {
		s.Run(sc.expected, func() {
			s.Equal(sc.expected, sc.t.String())
		})
	}
}

func (s *TypesSuite) TestSamplingType_IsValid() {
	s.True(SamplingTypeRatio.IsValid())
	s.True(SamplingTypeAlways.IsValid())
	s.True(SamplingTypeNever.IsValid())
	s.False(SamplingType(0).IsValid())
}

func (s *TypesSuite) TestParseSamplingType() {
	scenarios := []struct {
		input  string
		expect func(SamplingType, error)
	}{
		{"ratio", func(t SamplingType, err error) { s.NoError(err); s.Equal(SamplingTypeRatio, t) }},
		{"always", func(t SamplingType, err error) { s.NoError(err); s.Equal(SamplingTypeAlways, t) }},
		{"never", func(t SamplingType, err error) { s.NoError(err); s.Equal(SamplingTypeNever, t) }},
		{"invalid", func(t SamplingType, err error) { s.Error(err); s.Zero(t) }},
	}
	for _, sc := range scenarios {
		s.Run(sc.input, func() {
			t, err := ParseSamplingType(sc.input)
			sc.expect(t, err)
		})
	}
}

type CodeBasedScorerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCodeBasedScorerSuite(t *testing.T) {
	suite.Run(t, new(CodeBasedScorerSuite))
}

func (s *CodeBasedScorerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CodeBasedScorerSuite) TestToolCallAccuracy_AllMatched() {
	sc := NewToolCallAccuracyScorer("tool-accuracy", []string{"weather", "geocode"})
	sample := RunSample{
		Input:  "what is the weather?",
		Output: "sunny",
		ToolCalls: []ToolCallRecord{
			{Name: "weather"},
			{Name: "geocode"},
		},
	}
	result, err := sc.Score(s.ctx, sample)
	s.NoError(err)
	s.Equal(1.0, result.Score)
}

func (s *CodeBasedScorerSuite) TestToolCallAccuracy_PartialMatch() {
	sc := NewToolCallAccuracyScorer("tool-accuracy", []string{"weather", "geocode"})
	sample := RunSample{
		Input:  "what is the weather?",
		Output: "sunny",
		ToolCalls: []ToolCallRecord{
			{Name: "weather"},
		},
	}
	result, err := sc.Score(s.ctx, sample)
	s.NoError(err)
	s.Equal(0.5, result.Score)
}

func (s *CodeBasedScorerSuite) TestToolCallAccuracy_NoExpected() {
	sc := NewToolCallAccuracyScorer("tool-accuracy", []string{})
	result, err := sc.Score(s.ctx, RunSample{Output: "something"})
	s.NoError(err)
	s.Equal(1.0, result.Score)
}

func (s *CodeBasedScorerSuite) TestToolCallAccuracy_Kind() {
	sc := NewToolCallAccuracyScorer("tool-accuracy", []string{})
	s.Equal(ScorerKindCodeBased, sc.Kind())
}

func (s *CodeBasedScorerSuite) TestCompleteness_AllPresent() {
	sc := NewCompletenessScorer("completeness", []string{"city", "temp"})
	sample := RunSample{
		Input:  "weather?",
		Output: `{"city":"Berlin","temp":20}`,
	}
	result, err := sc.Score(s.ctx, sample)
	s.NoError(err)
	s.Equal(1.0, result.Score)
}

func (s *CodeBasedScorerSuite) TestCompleteness_PartiallyPresent() {
	sc := NewCompletenessScorer("completeness", []string{"city", "temp", "humidity"})
	sample := RunSample{
		Input:  "weather?",
		Output: `{"city":"Berlin","temp":20}`,
	}
	result, err := sc.Score(s.ctx, sample)
	s.NoError(err)
	s.InDelta(0.667, result.Score, 0.001)
}

func (s *CodeBasedScorerSuite) TestCompleteness_InvalidJSON() {
	sc := NewCompletenessScorer("completeness", []string{"city"})
	sample := RunSample{Output: "not json"}
	result, err := sc.Score(s.ctx, sample)
	s.NoError(err)
	s.Equal(0.0, result.Score)
	s.Contains(result.Reason, "not valid JSON")
}

func (s *CodeBasedScorerSuite) TestCompleteness_NoRequired() {
	sc := NewCompletenessScorer("completeness", []string{})
	result, err := sc.Score(s.ctx, RunSample{Output: "anything"})
	s.NoError(err)
	s.Equal(1.0, result.Score)
}

type LLMJudgedScorerSuite struct {
	suite.Suite
	ctx          context.Context
	obs          observability.Observability
	providerMock *llmmocks.Provider
}

func TestLLMJudgedScorerSuite(t *testing.T) {
	suite.Run(t, new(LLMJudgedScorerSuite))
}

func (s *LLMJudgedScorerSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.providerMock = llmmocks.NewProvider(s.T())
}

func (s *LLMJudgedScorerSuite) TestScore_Conforming() {
	type args struct {
		sample RunSample
	}
	type dependencies struct {
		provider *llmmocks.Provider
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result ScoreResult, err error)
	}{
		{
			name: "deve retornar score do judge",
			args: args{sample: RunSample{
				Input:          "what is the weather in Berlin?",
				ExpectedOutput: "sunny",
				Output:         "It is sunny in Berlin.",
			}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{
							RawJSON: []byte(`{"score":0.9,"reason":"output matches expected"}`),
						}, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result ScoreResult, err error) {
				s.NoError(err)
				s.Equal(0.9, result.Score)
				s.Equal("output matches expected", result.Reason)
			},
		},
		{
			name: "deve falhar quando provider retorna erro",
			args: args{sample: RunSample{Input: "test", Output: "test"}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{}, errors.New("upstream error")).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result ScoreResult, err error) {
				s.Error(err)
				s.Zero(result.Score)
			},
		},
		{
			name: "deve falhar quando structured output score fora do range",
			args: args{sample: RunSample{Input: "test", Output: "test"}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{
							RawJSON: []byte(`{"score":2.5,"reason":"out of range"}`),
						}, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result ScoreResult, err error) {
				s.Error(err)
				s.True(errors.Is(err, ErrJudgeContractNotMet))
			},
		},
		{
			name: "deve falhar quando json invalido",
			args: args{sample: RunSample{Input: "test", Output: "test"}},
			dependencies: dependencies{
				provider: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{
							RawJSON: []byte(`not json`),
						}, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result ScoreResult, err error) {
				s.Error(err)
				s.True(errors.Is(err, ErrJudgeContractNotMet))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewLLMJudgedScorer("judge", scenario.dependencies.provider, "You are a judge.")
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *LLMJudgedScorerSuite) TestKind() {
	sc := NewLLMJudgedScorer("judge", s.providerMock, "judge instructions")
	s.Equal(ScorerKindLLMJudged, sc.Kind())
}

type fakeScorer struct {
	id     string
	kind   ScorerKind
	result ScoreResult
	err    error
}

func (f *fakeScorer) ID() string       { return f.id }
func (f *fakeScorer) Kind() ScorerKind { return f.kind }
func (f *fakeScorer) Score(_ context.Context, _ RunSample) (ScoreResult, error) {
	return f.result, f.err
}

type fakeResultStore struct {
	mu       sync.Mutex
	inserted []ScorerResult
	err      error
}

func (f *fakeResultStore) Insert(_ context.Context, r ScorerResult) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inserted = append(f.inserted, r)
	return nil
}

type RunnerSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestRunnerSuite(t *testing.T) {
	suite.Run(t, new(RunnerSuite))
}

func (s *RunnerSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
}

func (s *RunnerSuite) TestObserve_AlwaysSampling_InsertsResult() {
	type args struct {
		runID  uuid.UUID
		sample RunSample
	}
	type dependencies struct {
		scorer *fakeScorer
		store  *fakeResultStore
	}

	runID := uuid.New()

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(store *fakeResultStore)
	}{
		{
			name: "deve inserir resultado quando sampling always",
			args: args{
				runID:  runID,
				sample: RunSample{Input: "hello", Output: "world"},
			},
			dependencies: dependencies{
				scorer: &fakeScorer{id: "test", kind: ScorerKindCodeBased, result: ScoreResult{Score: 0.8, Reason: "ok"}},
				store:  &fakeResultStore{},
			},
			expect: func(store *fakeResultStore) {
				s.Len(store.inserted, 1)
				s.Equal(runID, store.inserted[0].RunID)
				s.Equal(0.8, store.inserted[0].Score)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			entries := []ScorerEntry{
				NewScorerEntry(scenario.dependencies.scorer, AlwaysSample()),
			}
			runner := NewScorerRunner(entries, scenario.dependencies.store, s.obs)
			runner.Observe(s.ctx, scenario.args.runID, scenario.args.sample)

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			runner.Shutdown(shutdownCtx)

			scenario.expect(scenario.dependencies.store)
		})
	}
}

func (s *RunnerSuite) TestObserve_PersistError_IsObservedNotSwallowed() {
	store := &fakeResultStore{err: errors.New("db unavailable")}
	sc := &fakeScorer{id: "test", kind: ScorerKindCodeBased, result: ScoreResult{Score: 0.5}}
	entries := []ScorerEntry{NewScorerEntry(sc, AlwaysSample())}

	runner := NewScorerRunner(entries, store, s.obs)
	runner.Observe(s.ctx, uuid.New(), RunSample{Input: "hello", Output: "world"})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runner.Shutdown(shutdownCtx)

	logger := s.obs.Logger().(*fake.FakeLogger)
	found := false
	for _, e := range logger.GetEntries() {
		if e.Message == "scorer.runner.persist.failed" {
			found = true
		}
	}
	s.True(found)

	metrics := s.obs.Metrics().(*fake.FakeMetrics)
	s.NotEmpty(metrics.GetCounter("scorer_runs_total").GetValues())
}

func (s *RunnerSuite) TestObserve_NeverSampling_NoInsert() {
	store := &fakeResultStore{}
	sc := &fakeScorer{id: "test", kind: ScorerKindCodeBased, result: ScoreResult{Score: 1.0}}
	entries := []ScorerEntry{NewScorerEntry(sc, NeverSample())}

	runner := NewScorerRunner(entries, store, s.obs)
	runner.Observe(s.ctx, uuid.New(), RunSample{Input: "hello", Output: "world"})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runner.Shutdown(shutdownCtx)

	s.Empty(store.inserted)
}

func (s *RunnerSuite) TestShutdown_NoLeak() {
	entries := []ScorerEntry{}
	store := &fakeResultStore{}
	runner := NewScorerRunner(entries, store, s.obs)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		runner.Shutdown(shutdownCtx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		s.Fail("Shutdown leaked goroutine — did not return within timeout")
	}
}

func (s *RunnerSuite) TestObserve_ScorerError_DoesNotPropagate() {
	store := &fakeResultStore{}
	sc := &fakeScorer{id: "test", kind: ScorerKindCodeBased, err: errors.New("scorer error")}
	entries := []ScorerEntry{NewScorerEntry(sc, AlwaysSample())}

	runner := NewScorerRunner(entries, store, s.obs)
	runner.Observe(s.ctx, uuid.New(), RunSample{Input: "hello", Output: "world"})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runner.Shutdown(shutdownCtx)

	s.Empty(store.inserted)
}

type blockingScorer struct {
	id       string
	inflight int32
	maxConc  int32
	entered  chan struct{}
	proceed  chan struct{}
}

func (b *blockingScorer) ID() string       { return b.id }
func (b *blockingScorer) Kind() ScorerKind { return ScorerKindCodeBased }
func (b *blockingScorer) Score(_ context.Context, _ RunSample) (ScoreResult, error) {
	cur := atomic.AddInt32(&b.inflight, 1)
	for {
		old := atomic.LoadInt32(&b.maxConc)
		if cur <= old || atomic.CompareAndSwapInt32(&b.maxConc, old, cur) {
			break
		}
	}
	b.entered <- struct{}{}
	<-b.proceed
	atomic.AddInt32(&b.inflight, -1)
	return ScoreResult{Score: 1.0}, nil
}

func (s *RunnerSuite) TestWithWorkers_StartsConfiguredWorkerCount() {
	const workers = 3
	const jobs = 6

	sc := &blockingScorer{
		id:      "blocking",
		entered: make(chan struct{}, jobs),
		proceed: make(chan struct{}),
	}
	entries := []ScorerEntry{NewScorerEntry(sc, AlwaysSample())}
	store := &fakeResultStore{}

	runner := NewScorerRunner(entries, store, s.obs, WithWorkers(workers))

	for i := 0; i < jobs; i++ {
		runner.Observe(s.ctx, uuid.New(), RunSample{Input: "in", Output: "out"})
	}
	for i := 0; i < workers; i++ {
		<-sc.entered
	}
	close(sc.proceed)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runner.Shutdown(shutdownCtx)

	s.Equal(int32(workers), atomic.LoadInt32(&sc.maxConc))
}

func (s *RunnerSuite) TestObserve_BufferFull_IncrementsDroppedCounter() {
	sc := &blockingScorer{
		id:      "blocking",
		entered: make(chan struct{}, 8),
		proceed: make(chan struct{}),
	}
	entries := []ScorerEntry{NewScorerEntry(sc, AlwaysSample())}
	store := &fakeResultStore{}

	runner := NewScorerRunner(entries, store, s.obs, WithWorkers(1))

	runner.Observe(s.ctx, uuid.New(), RunSample{Input: "in", Output: "out"})
	<-sc.entered

	for i := 0; i < 4; i++ {
		runner.Observe(s.ctx, uuid.New(), RunSample{Input: "in", Output: "out"})
	}
	runner.Observe(s.ctx, uuid.New(), RunSample{Input: "in", Output: "out"})

	close(sc.proceed)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runner.Shutdown(shutdownCtx)

	metrics := s.obs.Metrics().(*fake.FakeMetrics)
	dropped := 0
	for _, v := range metrics.GetCounter("scorer_runs_total").GetValues() {
		for _, f := range v.Fields {
			if f.Key == "outcome" && f.StringValue() == "dropped" {
				dropped++
			}
		}
	}
	s.Equal(1, dropped)
}

func (s *RunnerSuite) TestObserve_CancelContext_ShutdownGracefully() {
	store := &fakeResultStore{}
	sc := &fakeScorer{id: "test", kind: ScorerKindCodeBased, result: ScoreResult{Score: 0.5}}
	entries := []ScorerEntry{NewScorerEntry(sc, AlwaysSample())}

	runner := NewScorerRunner(entries, store, s.obs)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	runner.Observe(ctx, uuid.New(), RunSample{Input: "hello", Output: "world"})
	runner.Shutdown(shutdownCtx)
}

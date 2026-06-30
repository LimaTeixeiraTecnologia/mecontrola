//go:build integration

package weather

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/scorers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/weather"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	dbpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	wfpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type WeatherConformanceSuite struct {
	suite.Suite
	ctx          context.Context
	obs          observability.Observability
	db           *sqlx.DB
	store        workflow.Store
	provider     llm.Provider
	providerMock *llmmocks.Provider
	threadMock   *agentmocks.ThreadGateway
	messagesMock *agentmocks.MessageStore
	wmMock       *agentmocks.WorkingMemory
	runStoreMock *agentmocks.RunStore
	hooksMock    *agentmocks.Hooks
}

func TestWeatherConformanceSuite(t *testing.T) {
	suite.Run(t, new(WeatherConformanceSuite))
}

func buildRealProvider(t *testing.T, obs observability.Observability) llm.Provider {
	t.Helper()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" || os.Getenv("RUN_REAL_LLM") != "1" {
		return nil
	}
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}
	client, err := httpclient.NewClient(obs,
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter"),
		httpclient.WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("httpclient: %v", err)
	}
	return llm.NewOpenRouterProvider(client, llm.Config{
		Model:          "google/gemini-2.5-flash-lite",
		EmbedModel:     "openai/text-embedding-3-small",
		BaseURL:        baseURL,
		APIKey:         apiKey,
		HTTPReferer:    "https://mecontrola.app",
		XTitle:         "mecontrola-conformance-test",
		MaxTokens:      256,
		Temperature:    0,
		RequestTimeout: 30 * time.Second,
	}, obs)
}

func (s *WeatherConformanceSuite) requireProvider() {
	if s.provider == nil {
		s.T().Skip("OPENROUTER_API_KEY required for this test")
	}
}

func (s *WeatherConformanceSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.db, _ = dbpostgres.NewTestDatabase(s.T())
	s.store = wfpostgres.NewPostgresStore(s.obs, s.db)
	s.provider = buildRealProvider(s.T(), s.obs)
	s.providerMock = llmmocks.NewProvider(s.T())
	s.threadMock = agentmocks.NewThreadGateway(s.T())
	s.messagesMock = agentmocks.NewMessageStore(s.T())
	s.wmMock = agentmocks.NewWorkingMemory(s.T())
	s.runStoreMock = agentmocks.NewRunStore(s.T())
	s.hooksMock = agentmocks.NewHooks(s.T())
}

func (s *WeatherConformanceSuite) geoServer(lat, lon float64, name string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"results": []map[string]any{
				{"latitude": lat, "longitude": lon, "name": name},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func (s *WeatherConformanceSuite) forecastServer(temp, feelsLike, humidity, windSpeed, windGust float64, code int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"current": map[string]any{
				"temperature_2m":       temp,
				"apparent_temperature": feelsLike,
				"relative_humidity_2m": humidity,
				"wind_speed_10m":       windSpeed,
				"wind_gusts_10m":       windGust,
				"weather_code":         code,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func (s *WeatherConformanceSuite) hourlyForecastServer(weatherCode int, temps, precips []float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"current": map[string]any{
				"time":          time.Now().Format(time.RFC3339),
				"precipitation": 0.0,
				"weathercode":   weatherCode,
			},
			"hourly": map[string]any{
				"precipitation_probability": precips,
				"temperature_2m":            temps,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func (s *WeatherConformanceSuite) newWeatherClient(geocodingBase, forecastBase string) interfaces.WeatherClient {
	return weather.NewClient(
		weather.WithGeocodingBase(geocodingBase),
		weather.WithForecastBase(forecastBase),
	)
}

func (s *WeatherConformanceSuite) defaultTool() tool.ToolHandle {
	return tools.BuildWeatherTool(weather.NewClient())
}

func (s *WeatherConformanceSuite) TestWeatherTool_Schema() {
	wt := s.defaultTool()
	s.Equal("get-weather", wt.ID())
	s.Equal("Get current weather for a location", wt.Description())
	params := wt.Parameters()
	s.NotNil(params)
	s.Equal("object", params["type"])
}

func (s *WeatherConformanceSuite) TestWeatherTool_Interface() {
	wt := s.defaultTool()
	var _ tool.ToolHandle = wt
	s.NotEmpty(wt.ID())
	s.NotEmpty(wt.Description())
	s.NotNil(wt.Parameters())
}

func (s *WeatherConformanceSuite) TestWeatherTool_Invoke_Success() {
	geoSrv := s.geoServer(40.7128, -74.0060, "New York")
	defer geoSrv.Close()

	fcSrv := s.forecastServer(22.5, 20.0, 65.0, 15.0, 25.0, 1)
	defer fcSrv.Close()

	wt := tools.BuildWeatherTool(s.newWeatherClient(geoSrv.URL, fcSrv.URL))

	argsJSON, err := json.Marshal(tools.WeatherInput{Location: "New York"})
	s.Require().NoError(err)

	resultBytes, err := wt.Invoke(s.ctx, argsJSON)
	s.Require().NoError(err)

	var out tools.WeatherOutput
	s.Require().NoError(json.Unmarshal(resultBytes, &out))
	s.Equal("New York", out.Location)
	s.Equal(22.5, out.Temperature)
	s.Equal(domain.WeatherConditionFromCode(1).String(), out.Conditions)
}

func (s *WeatherConformanceSuite) TestWeatherTool_GeocodingFails_LocationNotFound() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer srv.Close()

	wt := tools.BuildWeatherTool(s.newWeatherClient(srv.URL, srv.URL))

	argsJSON, _ := json.Marshal(tools.WeatherInput{Location: "Nowhere"})
	_, err := wt.Invoke(s.ctx, argsJSON)
	s.Error(err)
}

func (s *WeatherConformanceSuite) TestWeatherAgent_Registry() {
	type dependencies struct {
		providerMock *llmmocks.Provider
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(resolved agent.Agent, err error)
	}{
		{
			name:         "deve resolver agente registrado com sucesso",
			dependencies: dependencies{providerMock: s.providerMock},
			expect: func(resolved agent.Agent, err error) {
				s.Require().NoError(err)
				s.Equal("weather-agent", resolved.ID())
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := agents.BuildWeatherAgent(scenario.dependencies.providerMock, s.defaultTool(), nil, s.obs)
			reg := agent.NewAgentRegistry()
			reg.Register(a)
			resolved, err := reg.Resolve("weather-agent")
			scenario.expect(resolved, err)
		})
	}
}

func (s *WeatherConformanceSuite) TestWeatherAgent_NotFound() {
	reg := agent.NewAgentRegistry()
	_, err := reg.Resolve("nonexistent")
	s.Error(err)
}

func (s *WeatherConformanceSuite) TestWeatherAgent_Execute_Sync() {
	s.requireProvider()
	type dependencies struct {
		provider llm.Provider
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(result agent.Result, err error)
	}{
		{
			name:         "deve retornar resposta síncrona com sucesso",
			dependencies: dependencies{provider: s.provider},
			expect: func(result agent.Result, err error) {
				s.Require().NoError(err)
				s.NotEmpty(result.Content)
				s.Equal(agent.ExecutionModeSync, result.Mode)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := agents.BuildWeatherAgent(scenario.dependencies.provider, s.defaultTool(), nil, s.obs)
			result, err := a.Execute(s.ctx, agent.Request{
				AgentID:  a.ID(),
				Messages: []llm.Message{{Role: "user", Content: "What is the weather in New York?"}},
			})
			scenario.expect(result, err)
		})
	}
}

func (s *WeatherConformanceSuite) TestWeatherAgent_Stream() {
	s.requireProvider()
	type dependencies struct {
		provider llm.Provider
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(collected []string, result agent.Result, err error)
	}{
		{
			name:         "deve retornar stream com deltas e resultado final",
			dependencies: dependencies{provider: s.provider},
			expect: func(collected []string, result agent.Result, err error) {
				s.Require().NoError(err)
				s.NotEmpty(collected)
				s.NotEmpty(result.Content)
				s.Equal(agent.ExecutionModeStream, result.Mode)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := agents.BuildWeatherAgent(scenario.dependencies.provider, s.defaultTool(), nil, s.obs)
			rs, err := a.Stream(s.ctx, agent.Request{
				AgentID:  a.ID(),
				Messages: []llm.Message{{Role: "user", Content: "stream weather?"}},
			})
			s.Require().NoError(err)

			var collected []string
			for delta := range rs.Deltas() {
				collected = append(collected, delta)
			}

			result, err := rs.Result(s.ctx)
			scenario.expect(collected, result, err)
		})
	}
}

func (s *WeatherConformanceSuite) TestWeatherWorkflow_Definition() {
	type dependencies struct {
		providerMock *llmmocks.Provider
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(def workflow.Definition[workflows.WeatherState])
	}{
		{
			name:         "deve retornar definição de workflow válida",
			dependencies: dependencies{providerMock: s.providerMock},
			expect: func(def workflow.Definition[workflows.WeatherState]) {
				s.Equal("weather-workflow", def.ID)
				s.True(def.Durable)
				s.Greater(def.MaxAttempts, 0)
				s.NotNil(def.Root)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := agents.BuildWeatherAgent(scenario.dependencies.providerMock, s.defaultTool(), nil, s.obs)
			def := workflows.BuildWeatherWorkflow(a, weather.NewClient(), "https://api.open-meteo.com/v1/forecast")
			scenario.expect(def)
		})
	}
}

func (s *WeatherConformanceSuite) TestWeatherWorkflow_FetchAndPlan() {
	s.requireProvider()

	geoSrv := s.geoServer(48.8566, 2.3522, "Paris")
	defer geoSrv.Close()

	fcSrv := s.hourlyForecastServer(0, []float64{18.0, 22.0, 20.0}, []float64{0, 5, 10})
	defer fcSrv.Close()

	type dependencies struct {
		provider llm.Provider
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(result workflow.RunResult[workflows.WeatherState], err error)
	}{
		{
			name:         "deve buscar clima e gerar atividades com sucesso",
			dependencies: dependencies{provider: s.provider},
			expect: func(result workflow.RunResult[workflows.WeatherState], err error) {
				s.Require().NoError(err)
				s.Equal(workflow.RunStatusSucceeded, result.Status)
				s.Equal("Paris", result.State.Location)
				s.Equal("Clear sky", result.State.Condition)
				s.NotEmpty(result.State.Activities)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := agents.BuildWeatherAgent(scenario.dependencies.provider, s.defaultTool(), nil, s.obs)
			engine := workflow.NewEngine[workflows.WeatherState](s.store, s.obs)
			client := s.newWeatherClient(geoSrv.URL, fcSrv.URL)
			def := workflows.BuildWeatherWorkflow(a, client, fcSrv.URL)
			result, err := engine.Start(s.ctx, def, "test-key-paris", workflows.WeatherState{City: "Paris"})
			scenario.expect(result, err)
		})
	}
}

func (s *WeatherConformanceSuite) TestRuntimeContext_NotPersisted() {
	ctx := workflow.WithRuntime(s.ctx, "ephemeral-value")
	val, ok := workflow.RuntimeFrom(ctx)
	s.True(ok)
	s.Equal("ephemeral-value", val)

	backgroundCtx := context.Background()
	_, ok = workflow.RuntimeFrom(backgroundCtx)
	s.False(ok)
}

func (s *WeatherConformanceSuite) TestTypedStates_RunStatus() {
	validStatuses := []agent.RunStatus{
		agent.RunStatusRunning,
		agent.RunStatusSucceeded,
		agent.RunStatusFailed,
	}
	for _, st := range validStatuses {
		s.True(st.IsValid(), "expected %q to be valid", st.String())
		parsed, err := agent.ParseRunStatus(st.String())
		s.Require().NoError(err)
		s.Equal(st, parsed)
	}

	_, err := agent.ParseRunStatus("invalid-status")
	s.Error(err)
}

func (s *WeatherConformanceSuite) TestTypedStates_ExecutionMode() {
	for _, m := range []agent.ExecutionMode{agent.ExecutionModeSync, agent.ExecutionModeStream} {
		s.True(m.IsValid())
		parsed, err := agent.ParseExecutionMode(m.String())
		s.Require().NoError(err)
		s.Equal(m, parsed)
	}
	_, err := agent.ParseExecutionMode("bad")
	s.Error(err)
}

func (s *WeatherConformanceSuite) TestTypedStates_ToolOutcome() {
	outcomes := []agent.ToolOutcome{
		agent.ToolOutcomeRouted,
		agent.ToolOutcomeClarify,
		agent.ToolOutcomeUsecaseError,
		agent.ToolOutcomeMissingResolver,
		agent.ToolOutcomeReplay,
	}
	for _, o := range outcomes {
		s.True(o.IsValid())
		parsed, err := agent.ParseToolOutcome(o.String())
		s.Require().NoError(err)
		s.Equal(o, parsed)
	}
}

func (s *WeatherConformanceSuite) TestTypedStates_ScorerKind() {
	for _, k := range []scorer.ScorerKind{scorer.ScorerKindCodeBased, scorer.ScorerKindLLMJudged} {
		s.True(k.IsValid())
		parsed, err := scorer.ParseScorerKind(k.String())
		s.Require().NoError(err)
		s.Equal(k, parsed)
	}
}

func (s *WeatherConformanceSuite) TestTypedStates_SamplingType() {
	for _, tp := range []scorer.SamplingType{scorer.SamplingTypeRatio, scorer.SamplingTypeAlways, scorer.SamplingTypeNever} {
		s.True(tp.IsValid())
		parsed, err := scorer.ParseSamplingType(tp.String())
		s.Require().NoError(err)
		s.Equal(tp, parsed)
	}
}

func (s *WeatherConformanceSuite) TestScorerCodeBased_ToolCallAccuracy_Hit() {
	sc := scorers.NewToolCallAccuracyScorer()
	s.Equal("tool-call-accuracy", sc.ID())
	s.Equal(scorer.ScorerKindCodeBased, sc.Kind())

	result, err := sc.Score(s.ctx, scorer.RunSample{
		Input:  "What is the weather?",
		Output: "Sunny.",
		ToolCalls: []scorer.ToolCallRecord{
			{ID: "1", Name: "get-weather"},
		},
	})
	s.Require().NoError(err)
	s.Equal(1.0, result.Score)
}

func (s *WeatherConformanceSuite) TestScorerCodeBased_ToolCallAccuracy_Miss() {
	sc := scorers.NewToolCallAccuracyScorer()

	result, err := sc.Score(s.ctx, scorer.RunSample{
		Input:     "What is the weather?",
		Output:    "Sunny.",
		ToolCalls: []scorer.ToolCallRecord{},
	})
	s.Require().NoError(err)
	s.Equal(0.0, result.Score)
}

func (s *WeatherConformanceSuite) TestScorerCodeBased_Completeness_Full() {
	sc := scorers.NewCompletenessScorer()
	s.Equal("completeness", sc.ID())
	s.Equal(scorer.ScorerKindCodeBased, sc.Kind())

	output := `{"temperature":22.5,"feelsLike":20,"humidity":65,"windSpeed":15,"windGust":25,"conditions":"Sunny","location":"New York"}`
	result, err := sc.Score(s.ctx, scorer.RunSample{Output: output})
	s.Require().NoError(err)
	s.Equal(1.0, result.Score)
}

func (s *WeatherConformanceSuite) TestScorerCodeBased_Completeness_Partial() {
	sc := scorers.NewCompletenessScorer()

	output := `{"temperature":22.5,"location":"New York"}`
	result, err := sc.Score(s.ctx, scorer.RunSample{Output: output})
	s.Require().NoError(err)
	s.Less(result.Score, 1.0)
}

func (s *WeatherConformanceSuite) TestWorkflowState_JSONRoundTrip() {
	state := workflows.WeatherState{
		City:                "Berlin",
		Date:                "2026-06-29T00:00:00Z",
		MaxTemp:             28.5,
		MinTemp:             18.0,
		PrecipitationChance: 10.0,
		Condition:           "Clear sky",
		Location:            "Berlin",
		Activities:          "Visit Brandenburg Gate.",
	}

	data, err := json.Marshal(state)
	s.Require().NoError(err)

	var restored workflows.WeatherState
	s.Require().NoError(json.Unmarshal(data, &restored))
	s.Equal(state, restored)
}

func (s *WeatherConformanceSuite) TestWeatherScorerEntries_Count() {
	type dependencies struct {
		providerMock *llmmocks.Provider
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(entries []scorer.ScorerEntry)
	}{
		{
			name:         "deve retornar 3 entries de scorer",
			dependencies: dependencies{providerMock: s.providerMock},
			expect: func(entries []scorer.ScorerEntry) {
				s.Len(entries, 3)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			entries := scorers.BuildWeatherScorers(scenario.dependencies.providerMock)
			scenario.expect(entries)
		})
	}
}

func (s *WeatherConformanceSuite) TestWorkflowRunStatus_KernelTypes() {
	for _, st := range []workflow.RunStatus{
		workflow.RunStatusRunning,
		workflow.RunStatusSuspended,
		workflow.RunStatusSucceeded,
		workflow.RunStatusFailed,
	} {
		s.True(st.IsValid())
		parsed, err := workflow.ParseRunStatus(st.String())
		s.Require().NoError(err)
		s.Equal(st, parsed)
	}
}

func (s *WeatherConformanceSuite) TestWorkflowStepStatus_KernelTypes() {
	for _, st := range []workflow.StepStatus{
		workflow.StepStatusCompleted,
		workflow.StepStatusSuspended,
		workflow.StepStatusFailed,
		workflow.StepStatusSkipped,
	} {
		s.True(st.IsValid())
	}
}

type weatherStructuredDecoder struct{}

func (d *weatherStructuredDecoder) Schema() llm.Schema {
	return llm.Schema{
		Name:   "weather_report",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location":    map[string]any{"type": "string"},
				"temperature": map[string]any{"type": "number"},
			},
			"required": []any{"location", "temperature"},
		},
	}
}

func (d *weatherStructuredDecoder) Validate(raw []byte) error {
	var payload struct {
		Location    *string  `json:"location"`
		Temperature *float64 `json:"temperature"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if payload.Location == nil || payload.Temperature == nil {
		return errors.New("weather_report: missing required fields")
	}
	return nil
}

func (s *WeatherConformanceSuite) TestWeatherAgent_StructuredOutput() {
	type dependencies struct {
		providerMock *llmmocks.Provider
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		decoder      agent.StructuredDecoder
		expect       func(result agent.Result, err error)
	}{
		{
			name: "deve validar e retornar saída estruturada conforme o contrato",
			dependencies: dependencies{
				providerMock: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{
							Content: "New York is sunny.",
							RawJSON: []byte(`{"location":"New York","temperature":22.5}`),
						}, nil).
						Once()
					return s.providerMock
				}(),
			},
			decoder: &weatherStructuredDecoder{},
			expect: func(result agent.Result, err error) {
				s.Require().NoError(err)
				s.Equal("New York is sunny.", result.Content)
				s.JSONEq(`{"location":"New York","temperature":22.5}`, string(result.RawJSON))
			},
		},
		{
			name: "deve falhar com ErrContractNotMet quando JSON não conforme",
			dependencies: dependencies{
				providerMock: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{
							Content: "incomplete",
							RawJSON: []byte(`{"location":"New York"}`),
						}, nil).
						Once()
					return s.providerMock
				}(),
			},
			decoder: &weatherStructuredDecoder{},
			expect: func(result agent.Result, err error) {
				s.Require().Error(err)
				s.True(errors.Is(err, agent.ErrContractNotMet))
				s.Empty(result.Content)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := agents.BuildWeatherAgent(scenario.dependencies.providerMock, s.defaultTool(), nil, s.obs)
			result, err := a.Execute(s.ctx, agent.Request{
				AgentID:  a.ID(),
				Messages: []llm.Message{{Role: "user", Content: "weather in New York as JSON?"}},
				Decoder:  scenario.decoder,
			})
			scenario.expect(result, err)
		})
	}
}

func (s *WeatherConformanceSuite) TestAgentRuntime_WorkingMemoryInjected() {
	threadID := uuid.New()
	const wmContent = "User prefers temperatures in Celsius and lives in Berlin."
	var capturedReq llm.Request

	type dependencies struct {
		threadMock   *agentmocks.ThreadGateway
		messagesMock *agentmocks.MessageStore
		wmMock       *agentmocks.WorkingMemory
		runStoreMock *agentmocks.RunStore
		providerMock *llmmocks.Provider
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(outcome agent.Outcome, err error)
	}{
		{
			name: "deve injetar working memory no system prompt",
			dependencies: dependencies{
				threadMock: func() *agentmocks.ThreadGateway {
					s.threadMock.EXPECT().
						GetOrCreate(mock.Anything, "res-wm", "thr-wm").
						Return(memory.Thread{
							ID:         threadID,
							ResourceID: "res-wm",
							ThreadID:   "thr-wm",
							CreatedAt:  time.Now(),
							UpdatedAt:  time.Now(),
						}, nil).
						Once()
					return s.threadMock
				}(),
				messagesMock: func() *agentmocks.MessageStore {
					s.messagesMock.EXPECT().
						Recent(mock.Anything, threadID, mock.AnythingOfType("int")).
						Return(nil, nil).
						Once()
					s.messagesMock.EXPECT().
						Append(mock.Anything, threadID, mock.AnythingOfType("memory.Message")).
						Return(nil).
						Maybe()
					return s.messagesMock
				}(),
				wmMock: func() *agentmocks.WorkingMemory {
					s.wmMock.EXPECT().
						Get(mock.Anything, "res-wm").
						Return(wmContent, nil).
						Once()
					return s.wmMock
				}(),
				runStoreMock: func() *agentmocks.RunStore {
					s.runStoreMock.EXPECT().
						Insert(mock.Anything, mock.AnythingOfType("agent.Run")).
						Return(nil).
						Once()
					s.runStoreMock.EXPECT().
						Update(mock.Anything, mock.AnythingOfType("agent.Run")).
						Return(nil).
						Maybe()
					return s.runStoreMock
				}(),
				providerMock: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Run(func(ctx context.Context, req llm.Request) {
							capturedReq = req
						}).
						Return(llm.Response{Content: "Berlin is sunny."}, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(outcome agent.Outcome, err error) {
				s.Require().NoError(err)
				s.Equal(agent.RunStatusSucceeded, outcome.Status)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := agents.BuildWeatherAgent(scenario.dependencies.providerMock, s.defaultTool(), nil, s.obs)
			reg := agent.NewAgentRegistry()
			reg.Register(a)
			runtime := agent.NewAgentRuntime(reg, scenario.dependencies.threadMock, scenario.dependencies.messagesMock, scenario.dependencies.wmMock, scenario.dependencies.runStoreMock, s.obs)
			outcome, err := runtime.Execute(s.ctx, agent.InboundRequest{
				ResourceID: "res-wm",
				ThreadID:   "thr-wm",
				AgentID:    a.ID(),
				Message:    "What's the weather?",
				MessageID:  "msg-wm",
			})
			scenario.expect(outcome, err)

			var systemMsg string
			for _, m := range capturedReq.Messages {
				if m.Role == "system" {
					systemMsg = m.Content
				}
			}
			s.Require().NotEmpty(systemMsg)
			s.Contains(systemMsg, "## Working Memory")
			s.Contains(systemMsg, wmContent)
		})
	}
}

func (s *WeatherConformanceSuite) TestAgentRuntime_ThreadRun() {
	s.requireProvider()

	threadID := uuid.New()

	type dependencies struct {
		threadMock   *agentmocks.ThreadGateway
		messagesMock *agentmocks.MessageStore
		wmMock       *agentmocks.WorkingMemory
		runStoreMock *agentmocks.RunStore
		provider     llm.Provider
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(outcome agent.Outcome, err error)
	}{
		{
			name: "deve executar run auditável com sucesso",
			dependencies: dependencies{
				threadMock: func() *agentmocks.ThreadGateway {
					s.threadMock.EXPECT().
						GetOrCreate(mock.Anything, "res-1", "thr-1").
						Return(memory.Thread{
							ID:         threadID,
							ResourceID: "res-1",
							ThreadID:   "thr-1",
							CreatedAt:  time.Now(),
							UpdatedAt:  time.Now(),
						}, nil).
						Once()
					return s.threadMock
				}(),
				messagesMock: func() *agentmocks.MessageStore {
					s.messagesMock.EXPECT().
						Recent(mock.Anything, threadID, mock.AnythingOfType("int")).
						Return(nil, nil).
						Once()
					s.messagesMock.EXPECT().
						Append(mock.Anything, threadID, mock.AnythingOfType("memory.Message")).
						Return(nil).
						Maybe()
					return s.messagesMock
				}(),
				wmMock: func() *agentmocks.WorkingMemory {
					s.wmMock.EXPECT().
						Get(mock.Anything, "res-1").
						Return("", nil).
						Once()
					return s.wmMock
				}(),
				runStoreMock: func() *agentmocks.RunStore {
					s.runStoreMock.EXPECT().
						Insert(mock.Anything, mock.AnythingOfType("agent.Run")).
						Return(nil).
						Once()
					s.runStoreMock.EXPECT().
						Update(mock.Anything, mock.AnythingOfType("agent.Run")).
						Return(nil).
						Maybe()
					return s.runStoreMock
				}(),
				provider: s.provider,
			},
			expect: func(outcome agent.Outcome, err error) {
				s.Require().NoError(err)
				s.Equal(agent.RunStatusSucceeded, outcome.Status)
				s.Equal(agent.ExecutionModeSync, outcome.Mode)
				s.NotEqual(uuid.Nil, outcome.RunID)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := agents.BuildWeatherAgent(scenario.dependencies.provider, s.defaultTool(), nil, s.obs)
			reg := agent.NewAgentRegistry()
			reg.Register(a)
			runtime := agent.NewAgentRuntime(reg, scenario.dependencies.threadMock, scenario.dependencies.messagesMock, scenario.dependencies.wmMock, scenario.dependencies.runStoreMock, s.obs)
			outcome, err := runtime.Execute(s.ctx, agent.InboundRequest{
				ResourceID: "res-1",
				ThreadID:   "thr-1",
				AgentID:    a.ID(),
				Message:    "What is the weather in London?",
				MessageID:  "msg-1",
			})
			scenario.expect(outcome, err)
		})
	}
}

func (s *WeatherConformanceSuite) TestAgentRuntime_ValidationError_EmptyAgentID() {
	type dependencies struct {
		threadMock   *agentmocks.ThreadGateway
		messagesMock *agentmocks.MessageStore
		wmMock       *agentmocks.WorkingMemory
		runStoreMock *agentmocks.RunStore
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve retornar erro quando agentID estiver vazio",
			dependencies: dependencies{
				threadMock:   s.threadMock,
				messagesMock: s.messagesMock,
				wmMock:       s.wmMock,
				runStoreMock: s.runStoreMock,
			},
			expect: func(err error) {
				s.Error(err)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			reg := agent.NewAgentRegistry()
			runtime := agent.NewAgentRuntime(reg, scenario.dependencies.threadMock, scenario.dependencies.messagesMock, scenario.dependencies.wmMock, scenario.dependencies.runStoreMock, s.obs)
			_, err := runtime.Execute(s.ctx, agent.InboundRequest{
				ResourceID: "res-1",
				ThreadID:   "thr-1",
				AgentID:    "",
				Message:    "hello",
			})
			scenario.expect(err)
		})
	}
}

func (s *WeatherConformanceSuite) TestHooks_LifecycleExecuted() {
	s.requireProvider()
	type dependencies struct {
		provider  llm.Provider
		hooksMock *agentmocks.Hooks
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve executar hooks before e after com sucesso",
			dependencies: dependencies{
				provider: s.provider,
				hooksMock: func() *agentmocks.Hooks {
					s.hooksMock.EXPECT().
						BeforeExecute(mock.Anything, "weather-agent", mock.AnythingOfType("agent.Request")).
						Return(s.ctx).
						Once()
					s.hooksMock.EXPECT().
						AfterExecute(mock.Anything, "weather-agent", mock.AnythingOfType("agent.Result"), mock.Anything).
						Return().
						Once()
					return s.hooksMock
				}(),
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := agents.BuildWeatherAgent(scenario.dependencies.provider, s.defaultTool(), scenario.dependencies.hooksMock, s.obs)
			_, err := a.Execute(s.ctx, agent.Request{
				AgentID:  a.ID(),
				Messages: []llm.Message{{Role: "user", Content: "What's the weather?"}},
			})
			scenario.expect(err)
		})
	}
}

func (s *WeatherConformanceSuite) TestWorkflow_SuspendResume_Idempotent() {
	suspendStep := workflow.NewStepFunc[workflows.WeatherState]("suspend-step",
		func(ctx context.Context, state workflows.WeatherState) (workflow.StepOutput[workflows.WeatherState], error) {
			if state.City != "London-resumed" {
				return workflow.StepOutput[workflows.WeatherState]{
					State:   state,
					Status:  workflow.StepStatusSuspended,
					Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: "Please provide city."},
				}, nil
			}
			return workflow.StepOutput[workflows.WeatherState]{State: state, Status: workflow.StepStatusCompleted}, nil
		})

	resumeStep := workflow.NewStepFunc[workflows.WeatherState]("resume-step",
		func(ctx context.Context, state workflows.WeatherState) (workflow.StepOutput[workflows.WeatherState], error) {
			state.Activities = "resumed-" + state.City
			return workflow.StepOutput[workflows.WeatherState]{State: state, Status: workflow.StepStatusCompleted}, nil
		})

	def := workflow.Definition[workflows.WeatherState]{
		ID:          "weather-suspend-test",
		Root:        workflow.Sequence[workflows.WeatherState]("root", suspendStep, resumeStep),
		Durable:     true,
		MaxAttempts: 1,
	}

	engine := workflow.NewEngine[workflows.WeatherState](s.store, s.obs)

	result, err := engine.Start(s.ctx, def, "key-suspend", workflows.WeatherState{City: "London"})
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, result.Status)

	resumePayload, err := json.Marshal(map[string]any{"city": "London-resumed"})
	s.Require().NoError(err)
	result2, err := engine.Resume(s.ctx, def, "key-suspend", resumePayload)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result2.Status)
	s.Contains(result2.State.Activities, "resumed-")
}

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	weathermocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	agentpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type WorkflowSuite struct {
	suite.Suite
	ctx         context.Context
	agentMock   *agentmocks.Agent
	weatherMock *weathermocks.WeatherClient
}

func TestWorkflowSuite(t *testing.T) {
	suite.Run(t, new(WorkflowSuite))
}

func (s *WorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.agentMock = agentmocks.NewAgent(s.T())
	s.weatherMock = weathermocks.NewWeatherClient(s.T())
}

func (s *WorkflowSuite) TestBuildWeatherWorkflow_IDAndStructure() {
	s.agentMock.EXPECT().ID().Return("weather-agent").Maybe()

	def := BuildWeatherWorkflow(s.agentMock, s.weatherMock, "https://api.open-meteo.com/v1/forecast", http.DefaultClient)

	s.Equal(WeatherWorkflowID, def.ID)
	s.NotNil(def.Root)
	s.True(def.Durable)
	s.Equal(3, def.MaxAttempts)
}

func (s *WorkflowSuite) TestFetchWeatherStep_Success() {
	s.weatherMock.EXPECT().Geocode(mock.Anything, "London").Return(51.5074, -0.1278, "London", nil).Once()

	forecastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"current": map[string]any{
				"weathercode": 0,
			},
			"hourly": map[string]any{
				"temperature_2m":            []float64{20.0, 22.0, 25.0, 23.0},
				"precipitation_probability": []float64{10.0, 5.0, 0.0, 2.0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer forecastServer.Close()

	step := workflow.NewStepFunc(
		StepFetchWeatherID,
		BuildFetchWeatherStep(s.weatherMock, forecastServer.URL, forecastServer.Client()),
	)

	state := WeatherState{City: "London"}
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal("London", out.State.Location)
	s.Equal("Céu limpo", out.State.Condition)
	s.InDelta(25.0, out.State.MaxTemp, 0.001)
	s.InDelta(20.0, out.State.MinTemp, 0.001)
	s.InDelta(10.0, out.State.PrecipitationChance, 0.001)
	s.NotEmpty(out.State.Date)
}

func (s *WorkflowSuite) TestFetchWeatherStep_LocationNotFound() {
	s.weatherMock.EXPECT().Geocode(mock.Anything, "NonExistentCity").
		Return(0, 0, "", fmt.Errorf("geocode: %w", interfaces.ErrLocationNotFound)).Once()

	step := workflow.NewStepFunc(
		StepFetchWeatherID,
		BuildFetchWeatherStep(s.weatherMock, "https://unused.example/forecast", http.DefaultClient),
	)

	state := WeatherState{City: "NonExistentCity"}
	out, err := step.Execute(s.ctx, state)

	s.Error(err)
	s.Equal(workflow.StepStatusFailed, out.Status)
}

func (s *WorkflowSuite) TestPlanActivitiesStep_StreamSuccess() {
	type args struct {
		state WeatherState
	}
	type dependencies struct {
		agentMock *agentmocks.Agent
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out workflow.StepOutput[WeatherState], err error)
	}{
		{
			name: "deve retornar activities quando agent stream tem sucesso",
			args: args{state: WeatherState{
				Location:  "London",
				Condition: "Clear sky",
				MaxTemp:   25.0,
				MinTemp:   18.0,
			}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					stream := &fakeResultStream{deltas: []string{"Go", " outdoors!"}}
					s.agentMock.EXPECT().ID().Return("weather-agent").Once()
					s.agentMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(stream, nil).
						Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[WeatherState], err error) {
				s.NoError(err)
				s.Equal(workflow.StepStatusCompleted, out.Status)
				s.Equal("Go outdoors!", out.State.Activities)
			},
		},
		{
			name: "deve retornar erro quando stream falha",
			args: args{state: WeatherState{Location: "London"}},
			dependencies: dependencies{
				agentMock: func() *agentmocks.Agent {
					s.agentMock.EXPECT().ID().Return("weather-agent").Once()
					s.agentMock.EXPECT().
						Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
						Return(nil, errors.New("stream error")).
						Once()
					return s.agentMock
				}(),
			},
			expect: func(out workflow.StepOutput[WeatherState], err error) {
				s.Error(err)
				s.Equal(workflow.StepStatusFailed, out.Status)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := workflow.NewStepFunc(
				StepPlanActivities,
				BuildPlanActivitiesStep(scenario.dependencies.agentMock),
			)
			out, err := step.Execute(s.ctx, scenario.args.state)
			scenario.expect(out, err)
		})
	}
}

func (s *WorkflowSuite) TestPlanActivitiesStep_StreamOver64DeltasDoesNotBlock() {
	const numDeltas = 100
	deltas := make([]string, numDeltas)
	for i := range deltas {
		deltas[i] = "x"
	}

	stream := &fakeResultStream{deltas: deltas}
	s.agentMock.EXPECT().ID().Return("weather-agent").Once()
	s.agentMock.EXPECT().
		Stream(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(stream, nil).
		Once()

	step := workflow.NewStepFunc(
		StepPlanActivities,
		BuildPlanActivitiesStep(s.agentMock),
	)

	state := WeatherState{Location: "London", Condition: "Clear sky"}
	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Len(out.State.Activities, numDeltas)
}

type fakeResultStream struct {
	deltas []string
}

func (f *fakeResultStream) Deltas() <-chan string {
	ch := make(chan string, len(f.deltas))
	for _, d := range f.deltas {
		ch <- d
	}
	close(ch)
	return ch
}

func (f *fakeResultStream) Result(_ context.Context) (agentpkg.Result, error) {
	content := ""
	for _, d := range f.deltas {
		content += d
	}
	return agentpkg.Result{Content: content}, nil
}

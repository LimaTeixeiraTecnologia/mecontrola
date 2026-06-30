package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	WeatherWorkflowID  = "weather-workflow"
	StepFetchWeatherID = "fetch-weather"
	StepPlanActivities = "plan-activities"
)

type WeatherState struct {
	City                string  `json:"city"`
	Date                string  `json:"date"`
	MaxTemp             float64 `json:"maxTemp"`
	MinTemp             float64 `json:"minTemp"`
	PrecipitationChance float64 `json:"precipitationChance"`
	Condition           string  `json:"condition"`
	Location            string  `json:"location"`
	Activities          string  `json:"activities"`
}

type hourlyForecastResponse struct {
	Current struct {
		WeatherCode int `json:"weathercode"`
	} `json:"current"`
	Hourly struct {
		PrecipProb  []float64 `json:"precipitation_probability"`
		Temperature []float64 `json:"temperature_2m"`
	} `json:"hourly"`
}

var weatherCodeMap = map[int]string{
	0: "Clear sky", 1: "Mainly clear", 2: "Partly cloudy", 3: "Overcast",
	45: "Foggy", 48: "Depositing rime fog",
	51: "Light drizzle", 53: "Moderate drizzle", 55: "Dense drizzle",
	61: "Slight rain", 63: "Moderate rain", 65: "Heavy rain",
	71: "Slight snow fall", 73: "Moderate snow fall", 75: "Heavy snow fall",
	95: "Thunderstorm",
}

func weatherConditionFromCode(code int) string {
	if s, ok := weatherCodeMap[code]; ok {
		return s
	}
	return "Unknown"
}

func BuildWeatherWorkflow(a agent.Agent, client interfaces.WeatherClient, forecastBase string) workflow.Definition[WeatherState] {
	fetchStep := workflow.NewStepFunc[WeatherState](StepFetchWeatherID, BuildFetchWeatherStep(client, forecastBase))
	planStep := workflow.NewStepFunc[WeatherState](StepPlanActivities, BuildPlanActivitiesStep(a))

	return workflow.Definition[WeatherState]{
		ID:          WeatherWorkflowID,
		Root:        workflow.Sequence("root", fetchStep, planStep),
		Durable:     true,
		MaxAttempts: 3,
	}
}

func BuildFetchWeatherStep(client interfaces.WeatherClient, forecastBase string) func(context.Context, WeatherState) (workflow.StepOutput[WeatherState], error) {
	return func(ctx context.Context, state WeatherState) (workflow.StepOutput[WeatherState], error) {
		lat, lon, name, err := client.Geocode(ctx, state.City)
		if err != nil {
			return workflow.StepOutput[WeatherState]{State: state, Status: workflow.StepStatusFailed}, err
		}

		url := fmt.Sprintf(
			"%s?latitude=%f&longitude=%f&current=weathercode&timezone=auto&hourly=precipitation_probability,temperature_2m",
			forecastBase, lat, lon,
		)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return workflow.StepOutput[WeatherState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.workflow.fetch_weather: new_request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return workflow.StepOutput[WeatherState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.workflow.fetch_weather: do: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		var data hourlyForecastResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return workflow.StepOutput[WeatherState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.workflow.fetch_weather: decode: %w", err)
		}

		maxTemp := -math.MaxFloat64
		minTemp := math.MaxFloat64
		for _, t := range data.Hourly.Temperature {
			if t > maxTemp {
				maxTemp = t
			}
			if t < minTemp {
				minTemp = t
			}
		}
		if len(data.Hourly.Temperature) == 0 {
			maxTemp = 0
			minTemp = 0
		}

		maxPrecip := 0.0
		for _, p := range data.Hourly.PrecipProb {
			if p > maxPrecip {
				maxPrecip = p
			}
		}

		state.Date = time.Now().UTC().Format(time.RFC3339)
		state.MaxTemp = maxTemp
		state.MinTemp = minTemp
		state.PrecipitationChance = maxPrecip
		state.Condition = weatherConditionFromCode(data.Current.WeatherCode)
		state.Location = name

		return workflow.StepOutput[WeatherState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func BuildPlanActivitiesStep(a agent.Agent) func(context.Context, WeatherState) (workflow.StepOutput[WeatherState], error) {
	return func(ctx context.Context, state WeatherState) (workflow.StepOutput[WeatherState], error) {
		forecastJSON, err := json.Marshal(map[string]any{
			"location":            state.Location,
			"date":                state.Date,
			"maxTemp":             state.MaxTemp,
			"minTemp":             state.MinTemp,
			"precipitationChance": state.PrecipitationChance,
			"condition":           state.Condition,
		})
		if err != nil {
			return workflow.StepOutput[WeatherState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.workflow.plan_activities: marshal: %w", err)
		}

		prompt := fmt.Sprintf(
			"Based on the following weather forecast for %s, suggest appropriate activities:\n%s\n\nKeep your response concise.",
			state.Location, string(forecastJSON),
		)

		stream, err := a.Stream(ctx, agent.Request{
			AgentID:  a.ID(),
			Messages: []llm.Message{{Role: "user", Content: prompt}},
		})
		if err != nil {
			return workflow.StepOutput[WeatherState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.workflow.plan_activities: stream: %w", err)
		}

		for range stream.Deltas() {
		}

		result, err := stream.Result(ctx)
		if err != nil {
			return workflow.StepOutput[WeatherState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.workflow.plan_activities: stream_result: %w", err)
		}

		state.Activities = result.Content

		return workflow.StepOutput[WeatherState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

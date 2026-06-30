package tools

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type WeatherInput struct {
	Location string `json:"location"`
}

type WeatherOutput struct {
	Temperature float64 `json:"temperature"`
	FeelsLike   float64 `json:"feelsLike"`
	Humidity    float64 `json:"humidity"`
	WindSpeed   float64 `json:"windSpeed"`
	WindGust    float64 `json:"windGust"`
	Conditions  string  `json:"conditions"`
	Location    string  `json:"location"`
}

func BuildWeatherTool(client interfaces.WeatherClient) tool.ToolHandle {
	in := llm.Schema{
		Name:   "weather_tool_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{"type": "string", "description": "Nome da cidade"},
			},
			"required":             []string{"location"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "weather_tool_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"temperature": map[string]any{"type": "number"},
				"feelsLike":   map[string]any{"type": "number"},
				"humidity":    map[string]any{"type": "number"},
				"windSpeed":   map[string]any{"type": "number"},
				"windGust":    map[string]any{"type": "number"},
				"conditions":  map[string]any{"type": "string"},
				"location":    map[string]any{"type": "string"},
			},
			"required":             []string{"temperature", "feelsLike", "humidity", "windSpeed", "windGust", "conditions", "location"},
			"additionalProperties": false,
		},
	}

	exec := buildWeatherExec(client)
	return tool.NewTool[WeatherInput, WeatherOutput]("get-weather", "Obter condições climáticas atuais de uma localidade", in, out, exec)
}

func buildWeatherExec(client interfaces.WeatherClient) func(context.Context, WeatherInput) (WeatherOutput, error) {
	return func(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
		lat, lon, name, err := client.Geocode(ctx, in.Location)
		if err != nil {
			return WeatherOutput{}, fmt.Errorf("agents.tool.get_weather: geocode: %w", err)
		}
		forecast, err := client.Forecast(ctx, lat, lon)
		if err != nil {
			return WeatherOutput{}, fmt.Errorf("agents.tool.get_weather: forecast: %w", err)
		}
		return WeatherOutput{
			Temperature: forecast.Temperature,
			FeelsLike:   forecast.FeelsLike,
			Humidity:    forecast.Humidity,
			WindSpeed:   forecast.WindSpeed,
			WindGust:    forecast.WindGust,
			Conditions:  forecast.Condition.String(),
			Location:    name,
		}, nil
	}
}

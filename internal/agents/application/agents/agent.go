package agents

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

const (
	weatherAgentID = "weather-agent"

	weatherAgentInstructions = `You are a helpful weather assistant. When a user asks about weather:
- If they don't mention a location, ask them for it.
- If the location name is not in English, translate it to English before using the get-weather tool.
- Use the get-weather tool to retrieve current weather data.
- Provide concise weather information including temperature, conditions, and other relevant details.
- When asked about activities or what to do given the weather, suggest appropriate activities based on the current conditions.
- Keep responses brief and helpful.`
)

func BuildWeatherAgent(provider llm.Provider, weatherTool tool.ToolHandle, hooks agent.Hooks, o11y observability.Observability) agent.Agent {
	opts := []agent.AgentOption{agent.WithTools(weatherTool)}
	if hooks != nil {
		opts = append(opts, agent.WithHooks(hooks))
	}
	return agent.NewAgent(
		weatherAgentID,
		weatherAgentInstructions,
		provider,
		o11y,
		opts...,
	)
}

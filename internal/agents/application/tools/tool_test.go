package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/domain"
)

func TestBuildWeatherToolSuccess(t *testing.T) {
	client := mocks.NewWeatherClient(t)
	client.EXPECT().Geocode(mock.Anything, "Tokyo").Return(35.6, 139.7, "Tokyo", nil).Once()
	client.EXPECT().Forecast(mock.Anything, 35.6, 139.7).Return(domain.Forecast{
		Temperature: 22.0,
		FeelsLike:   21.0,
		Humidity:    65.0,
		WindSpeed:   8.0,
		WindGust:    12.0,
		Condition:   domain.WeatherConditionMainlyClear,
		Location:    "Tokyo",
	}, nil).Once()

	handle := BuildWeatherTool(client)

	assert.Equal(t, "get-weather", handle.ID())
	assert.NotEmpty(t, handle.Description())
	assert.NotEmpty(t, handle.Parameters())

	argsJSON, _ := json.Marshal(WeatherInput{Location: "Tokyo"})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result WeatherOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.InDelta(t, 22.0, result.Temperature, 0.01)
	assert.Equal(t, "Mainly clear", result.Conditions)
	assert.Equal(t, "Tokyo", result.Location)
}

func TestBuildWeatherToolLocationNotFound(t *testing.T) {
	client := mocks.NewWeatherClient(t)
	client.EXPECT().Geocode(mock.Anything, "nonexistentcity12345").
		Return(0, 0, "", fmt.Errorf("geocode: %w", interfaces.ErrLocationNotFound)).Once()

	handle := BuildWeatherTool(client)

	argsJSON, _ := json.Marshal(WeatherInput{Location: "nonexistentcity12345"})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
	assert.True(t, errors.Is(err, interfaces.ErrLocationNotFound))
}

func TestBuildWeatherToolUpstreamError(t *testing.T) {
	client := mocks.NewWeatherClient(t)
	client.EXPECT().Geocode(mock.Anything, "Tokyo").Return(35.6, 139.7, "Tokyo", nil).Once()
	client.EXPECT().Forecast(mock.Anything, 35.6, 139.7).
		Return(domain.Forecast{}, errors.New("upstream status 500")).Once()

	handle := BuildWeatherTool(client)

	argsJSON, _ := json.Marshal(WeatherInput{Location: "Tokyo"})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
}

func TestWeatherToolSchemaValidation(t *testing.T) {
	client := mocks.NewWeatherClient(t)

	handle := BuildWeatherTool(client)

	_, err := handle.Invoke(context.Background(), []byte(`{}`))
	require.Error(t, err)
}

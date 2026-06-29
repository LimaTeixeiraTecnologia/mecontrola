package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/weather"
)

func buildTestServers(t *testing.T, geoBody, forecastBody string, geoStatus, forecastStatus int) (*httptest.Server, *httptest.Server) {
	t.Helper()
	geo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(geoStatus)
		_, _ = w.Write([]byte(geoBody))
	}))
	fore := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(forecastStatus)
		_, _ = w.Write([]byte(forecastBody))
	}))
	return geo, fore
}

func TestBuildWeatherToolSuccess(t *testing.T) {
	geo, fore := buildTestServers(t,
		`{"results":[{"latitude":35.6,"longitude":139.7,"name":"Tokyo"}]}`,
		`{"current":{"temperature_2m":22.0,"apparent_temperature":21.0,"relative_humidity_2m":65.0,"wind_speed_10m":8.0,"wind_gusts_10m":12.0,"weather_code":1}}`,
		http.StatusOK, http.StatusOK,
	)
	defer geo.Close()
	defer fore.Close()

	client := weather.NewClient(
		weather.WithHTTPClient(geo.Client()),
		weather.WithGeocodingBase(geo.URL),
		weather.WithForecastBase(fore.URL),
	)
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
	geo, fore := buildTestServers(t,
		`{"results":[]}`,
		`{}`,
		http.StatusOK, http.StatusOK,
	)
	defer geo.Close()
	defer fore.Close()

	client := weather.NewClient(
		weather.WithHTTPClient(geo.Client()),
		weather.WithGeocodingBase(geo.URL),
		weather.WithForecastBase(fore.URL),
	)
	handle := BuildWeatherTool(client)

	argsJSON, _ := json.Marshal(WeatherInput{Location: "nonexistentcity12345"})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
	assert.True(t, errors.Is(err, weather.ErrLocationNotFound))
}

func TestBuildWeatherToolUpstreamError(t *testing.T) {
	geo, fore := buildTestServers(t,
		`{"results":[{"latitude":35.6,"longitude":139.7,"name":"Tokyo"}]}`,
		``,
		http.StatusOK, http.StatusInternalServerError,
	)
	defer geo.Close()
	defer fore.Close()

	client := weather.NewClient(
		weather.WithHTTPClient(geo.Client()),
		weather.WithGeocodingBase(geo.URL),
		weather.WithForecastBase(fore.URL),
	)
	handle := BuildWeatherTool(client)

	argsJSON, _ := json.Marshal(WeatherInput{Location: "Tokyo"})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
}

func TestWeatherToolSchemaValidation(t *testing.T) {
	geo, fore := buildTestServers(t, `{}`, `{}`, http.StatusOK, http.StatusOK)
	defer geo.Close()
	defer fore.Close()

	client := weather.NewClient(
		weather.WithHTTPClient(geo.Client()),
		weather.WithGeocodingBase(geo.URL),
		weather.WithForecastBase(fore.URL),
	)
	handle := BuildWeatherTool(client)

	_, err := handle.Invoke(context.Background(), []byte(`{}`))
	require.Error(t, err)
}

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewForecast(t *testing.T) {
	scenarios := []struct {
		name        string
		temperature float64
		feelsLike   float64
		humidity    float64
		windSpeed   float64
		windGust    float64
		condition   WeatherCondition
		wantErr     bool
	}{
		{
			name:        "valid forecast",
			temperature: 25.0,
			feelsLike:   23.0,
			humidity:    60.0,
			windSpeed:   10.0,
			windGust:    15.0,
			condition:   WeatherConditionClearSky,
			wantErr:     false,
		},
		{
			name:        "humidity out of range high",
			temperature: 25.0,
			feelsLike:   23.0,
			humidity:    101.0,
			windSpeed:   10.0,
			windGust:    15.0,
			condition:   WeatherConditionClearSky,
			wantErr:     true,
		},
		{
			name:        "humidity out of range low",
			temperature: 25.0,
			feelsLike:   23.0,
			humidity:    -1.0,
			windSpeed:   10.0,
			windGust:    15.0,
			condition:   WeatherConditionClearSky,
			wantErr:     true,
		},
		{
			name:        "negative wind speed",
			temperature: 25.0,
			feelsLike:   23.0,
			humidity:    60.0,
			windSpeed:   -5.0,
			windGust:    15.0,
			condition:   WeatherConditionClearSky,
			wantErr:     true,
		},
		{
			name:        "negative wind gust",
			temperature: 25.0,
			feelsLike:   23.0,
			humidity:    60.0,
			windSpeed:   10.0,
			windGust:    -1.0,
			condition:   WeatherConditionClearSky,
			wantErr:     true,
		},
		{
			name:        "invalid condition",
			temperature: 25.0,
			feelsLike:   23.0,
			humidity:    60.0,
			windSpeed:   10.0,
			windGust:    15.0,
			condition:   WeatherCondition(0),
			wantErr:     true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			f, err := NewForecast(s.temperature, s.feelsLike, s.humidity, s.windSpeed, s.windGust, s.condition)
			if s.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.temperature, f.Temperature)
			assert.Equal(t, s.feelsLike, f.FeelsLike)
			assert.Equal(t, s.humidity, f.Humidity)
			assert.Equal(t, s.windSpeed, f.WindSpeed)
			assert.Equal(t, s.windGust, f.WindGust)
			assert.Equal(t, s.condition, f.Condition)
		})
	}
}

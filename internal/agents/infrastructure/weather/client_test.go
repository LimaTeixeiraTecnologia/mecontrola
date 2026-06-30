package weather

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/domain"
)

func TestGeocodeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"latitude":-23.5,"longitude":-46.6,"name":"São Paulo"}]}`))
	}))
	defer srv.Close()

	cl := NewClient(WithHTTPClient(srv.Client()), WithGeocodingBase(srv.URL))
	lat, lon, name, err := cl.Geocode(context.Background(), "São Paulo")
	require.NoError(t, err)
	assert.InDelta(t, -23.5, lat, 0.01)
	assert.InDelta(t, -46.6, lon, 0.01)
	assert.Equal(t, "São Paulo", name)
}

func TestGeocodeNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	cl := NewClient(WithHTTPClient(srv.Client()), WithGeocodingBase(srv.URL))
	_, _, _, err := cl.Geocode(context.Background(), "nonexistentcity12345")
	require.Error(t, err)
	assert.True(t, errors.Is(err, interfaces.ErrLocationNotFound))
}

func TestGeocodeUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cl := NewClient(WithHTTPClient(srv.Client()), WithGeocodingBase(srv.URL))
	_, _, _, err := cl.Geocode(context.Background(), "Tokyo")
	require.Error(t, err)
}

func TestForecastSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"current": {
				"temperature_2m": 28.5,
				"apparent_temperature": 27.0,
				"relative_humidity_2m": 70.0,
				"wind_speed_10m": 12.3,
				"wind_gusts_10m": 20.1,
				"weather_code": 0
			}
		}`))
	}))
	defer srv.Close()

	cl := NewClient(WithHTTPClient(srv.Client()), WithForecastBase(srv.URL))
	f, err := cl.Forecast(context.Background(), -23.5, -46.6)
	require.NoError(t, err)
	assert.InDelta(t, 28.5, f.Temperature, 0.01)
	assert.InDelta(t, 27.0, f.FeelsLike, 0.01)
	assert.InDelta(t, 70.0, f.Humidity, 0.01)
	assert.InDelta(t, 12.3, f.WindSpeed, 0.01)
	assert.InDelta(t, 20.1, f.WindGust, 0.01)
	assert.Equal(t, domain.WeatherConditionClearSky, f.Condition)
}

func TestForecastUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cl := NewClient(WithHTTPClient(srv.Client()), WithForecastBase(srv.URL))
	_, err := cl.Forecast(context.Background(), -23.5, -46.6)
	require.Error(t, err)
}

func TestForecastWeatherCodeMapping(t *testing.T) {
	scenarios := []struct {
		code      int
		condition domain.WeatherCondition
	}{
		{0, domain.WeatherConditionClearSky},
		{3, domain.WeatherConditionOvercast},
		{61, domain.WeatherConditionSlightRain},
		{95, domain.WeatherConditionThunderstorm},
		{999, domain.WeatherConditionUnknown},
	}
	for _, s := range scenarios {
		t.Run(s.condition.String(), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"current":{"temperature_2m":20,"apparent_temperature":19,"relative_humidity_2m":50,"wind_speed_10m":5,"wind_gusts_10m":8,"weather_code":` + itoa(s.code) + `}}`))
			}))
			defer srv.Close()

			cl := NewClient(WithHTTPClient(srv.Client()), WithForecastBase(srv.URL))
			f, err := cl.Forecast(context.Background(), 0, 0)
			require.NoError(t, err)
			assert.Equal(t, s.condition, f.Condition)
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

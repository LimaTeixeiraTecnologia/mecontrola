package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/domain"
)

const (
	defaultGeocodingBase = "https://geocoding-api.open-meteo.com/v1/search"
	defaultForecastBase  = "https://api.open-meteo.com/v1/forecast"
)

type Client interface {
	Geocode(ctx context.Context, name string) (lat, lon float64, resolved string, err error)
	Forecast(ctx context.Context, lat, lon float64) (domain.Forecast, error)
}

type httpClient struct {
	http          *http.Client
	geocodingBase string
	forecastBase  string
}

type Option func(*httpClient)

func WithHTTPClient(c *http.Client) Option {
	return func(cl *httpClient) { cl.http = c }
}

func WithGeocodingBase(base string) Option {
	return func(cl *httpClient) { cl.geocodingBase = base }
}

func WithForecastBase(base string) Option {
	return func(cl *httpClient) { cl.forecastBase = base }
}

func NewClient(opts ...Option) Client {
	cl := &httpClient{
		http:          http.DefaultClient,
		geocodingBase: defaultGeocodingBase,
		forecastBase:  defaultForecastBase,
	}
	for _, opt := range opts {
		opt(cl)
	}
	return cl
}

type geocodingResponse struct {
	Results []struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Name      string  `json:"name"`
	} `json:"results"`
}

func (c *httpClient) Geocode(ctx context.Context, name string) (float64, float64, string, error) {
	geoURL := fmt.Sprintf("%s?name=%s&count=1", c.geocodingBase, url.QueryEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geoURL, nil)
	if err != nil {
		return 0, 0, "", fmt.Errorf("weather.client.geocode: new_request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, 0, "", fmt.Errorf("weather.client.geocode: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 500 {
		return 0, 0, "", fmt.Errorf("weather.client.geocode: upstream status %d", resp.StatusCode)
	}

	var data geocodingResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, "", fmt.Errorf("weather.client.geocode: decode: %w", err)
	}
	if len(data.Results) == 0 {
		return 0, 0, "", fmt.Errorf("weather.client.geocode: %w: %q", ErrLocationNotFound, name)
	}
	r := data.Results[0]
	return r.Latitude, r.Longitude, r.Name, nil
}

type forecastResponse struct {
	Current struct {
		Temperature         float64 `json:"temperature_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		Humidity            float64 `json:"relative_humidity_2m"`
		WindSpeed           float64 `json:"wind_speed_10m"`
		WindGust            float64 `json:"wind_gusts_10m"`
		WeatherCode         int     `json:"weather_code"`
	} `json:"current"`
}

func (c *httpClient) Forecast(ctx context.Context, lat, lon float64) (domain.Forecast, error) {
	forecastURL := fmt.Sprintf(
		"%s?latitude=%f&longitude=%f&current=temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,wind_gusts_10m,weather_code",
		c.forecastBase, lat, lon,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, forecastURL, nil)
	if err != nil {
		return domain.Forecast{}, fmt.Errorf("weather.client.forecast: new_request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return domain.Forecast{}, fmt.Errorf("weather.client.forecast: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 500 {
		return domain.Forecast{}, fmt.Errorf("weather.client.forecast: upstream status %d", resp.StatusCode)
	}

	var data forecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return domain.Forecast{}, fmt.Errorf("weather.client.forecast: decode: %w", err)
	}

	condition := domain.WeatherConditionFromCode(data.Current.WeatherCode)
	return domain.Forecast{
		Temperature: data.Current.Temperature,
		FeelsLike:   data.Current.ApparentTemperature,
		Humidity:    data.Current.Humidity,
		WindSpeed:   data.Current.WindSpeed,
		WindGust:    data.Current.WindGust,
		Condition:   condition,
	}, nil
}

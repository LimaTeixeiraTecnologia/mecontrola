package domain

import (
	"errors"
	"fmt"
)

type Forecast struct {
	Temperature float64
	FeelsLike   float64
	Humidity    float64
	WindSpeed   float64
	WindGust    float64
	Condition   WeatherCondition
}

var errInvalidForecast = errors.New("domain: invalid forecast")

func NewForecast(temperature, feelsLike, humidity, windSpeed, windGust float64, condition WeatherCondition) (Forecast, error) {
	var errs []error
	if humidity < 0 || humidity > 100 {
		errs = append(errs, fmt.Errorf("humidity: %w: must be in range 0..100", errInvalidForecast))
	}
	if windSpeed < 0 {
		errs = append(errs, fmt.Errorf("wind_speed: %w: must be non-negative", errInvalidForecast))
	}
	if windGust < 0 {
		errs = append(errs, fmt.Errorf("wind_gust: %w: must be non-negative", errInvalidForecast))
	}
	if !condition.IsValid() {
		errs = append(errs, fmt.Errorf("condition: %w: invalid weather condition", errInvalidForecast))
	}
	if len(errs) > 0 {
		return Forecast{}, errors.Join(errs...)
	}
	return Forecast{
		Temperature: temperature,
		FeelsLike:   feelsLike,
		Humidity:    humidity,
		WindSpeed:   windSpeed,
		WindGust:    windGust,
		Condition:   condition,
	}, nil
}

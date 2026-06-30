package interfaces

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/domain"
)

type WeatherClient interface {
	Geocode(ctx context.Context, name string) (lat, lon float64, resolved string, err error)
	Forecast(ctx context.Context, lat, lon float64) (domain.Forecast, error)
}

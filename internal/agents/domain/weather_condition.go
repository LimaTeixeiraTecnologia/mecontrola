package domain

import (
	"errors"
	"fmt"
)

type WeatherCondition int

const (
	WeatherConditionClearSky WeatherCondition = iota + 1
	WeatherConditionMainlyClear
	WeatherConditionPartlyCloudy
	WeatherConditionOvercast
	WeatherConditionFoggy
	WeatherConditionDepositingRimeFog
	WeatherConditionLightDrizzle
	WeatherConditionModerateDrizzle
	WeatherConditionDenseDrizzle
	WeatherConditionSlightRain
	WeatherConditionModerateRain
	WeatherConditionHeavyRain
	WeatherConditionSlightSnowFall
	WeatherConditionModerateSnowFall
	WeatherConditionHeavySnowFall
	WeatherConditionThunderstorm
	WeatherConditionUnknown
)

var weatherConditionLabels = map[WeatherCondition]string{
	WeatherConditionClearSky:          "Clear sky",
	WeatherConditionMainlyClear:       "Mainly clear",
	WeatherConditionPartlyCloudy:      "Partly cloudy",
	WeatherConditionOvercast:          "Overcast",
	WeatherConditionFoggy:             "Foggy",
	WeatherConditionDepositingRimeFog: "Depositing rime fog",
	WeatherConditionLightDrizzle:      "Light drizzle",
	WeatherConditionModerateDrizzle:   "Moderate drizzle",
	WeatherConditionDenseDrizzle:      "Dense drizzle",
	WeatherConditionSlightRain:        "Slight rain",
	WeatherConditionModerateRain:      "Moderate rain",
	WeatherConditionHeavyRain:         "Heavy rain",
	WeatherConditionSlightSnowFall:    "Slight snow fall",
	WeatherConditionModerateSnowFall:  "Moderate snow fall",
	WeatherConditionHeavySnowFall:     "Heavy snow fall",
	WeatherConditionThunderstorm:      "Thunderstorm",
}

func (c WeatherCondition) String() string {
	if label, ok := weatherConditionLabels[c]; ok {
		return label
	}
	return "Unknown"
}

func (c WeatherCondition) IsValid() bool {
	return c >= WeatherConditionClearSky && c <= WeatherConditionUnknown
}

var errInvalidWeatherCondition = errors.New("domain: invalid weather condition")

func ParseWeatherCondition(s string) (WeatherCondition, error) {
	for _, c := range allWeatherConditions {
		if c.String() == s {
			return c, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", errInvalidWeatherCondition, s)
}

var allWeatherConditions = []WeatherCondition{
	WeatherConditionClearSky,
	WeatherConditionMainlyClear,
	WeatherConditionPartlyCloudy,
	WeatherConditionOvercast,
	WeatherConditionFoggy,
	WeatherConditionDepositingRimeFog,
	WeatherConditionLightDrizzle,
	WeatherConditionModerateDrizzle,
	WeatherConditionDenseDrizzle,
	WeatherConditionSlightRain,
	WeatherConditionModerateRain,
	WeatherConditionHeavyRain,
	WeatherConditionSlightSnowFall,
	WeatherConditionModerateSnowFall,
	WeatherConditionHeavySnowFall,
	WeatherConditionThunderstorm,
	WeatherConditionUnknown,
}

var weatherCodeToCondition = map[int]WeatherCondition{
	0:  WeatherConditionClearSky,
	1:  WeatherConditionMainlyClear,
	2:  WeatherConditionPartlyCloudy,
	3:  WeatherConditionOvercast,
	45: WeatherConditionFoggy,
	48: WeatherConditionDepositingRimeFog,
	51: WeatherConditionLightDrizzle,
	53: WeatherConditionModerateDrizzle,
	55: WeatherConditionDenseDrizzle,
	61: WeatherConditionSlightRain,
	63: WeatherConditionModerateRain,
	65: WeatherConditionHeavyRain,
	71: WeatherConditionSlightSnowFall,
	73: WeatherConditionModerateSnowFall,
	75: WeatherConditionHeavySnowFall,
	95: WeatherConditionThunderstorm,
}

func WeatherConditionFromCode(code int) WeatherCondition {
	if c, ok := weatherCodeToCondition[code]; ok {
		return c
	}
	return WeatherConditionUnknown
}

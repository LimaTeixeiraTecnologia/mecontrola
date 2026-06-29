package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeatherConditionFromCode(t *testing.T) {
	scenarios := []struct {
		code      int
		condition WeatherCondition
	}{
		{0, WeatherConditionClearSky},
		{1, WeatherConditionMainlyClear},
		{2, WeatherConditionPartlyCloudy},
		{3, WeatherConditionOvercast},
		{45, WeatherConditionFoggy},
		{48, WeatherConditionDepositingRimeFog},
		{51, WeatherConditionLightDrizzle},
		{53, WeatherConditionModerateDrizzle},
		{55, WeatherConditionDenseDrizzle},
		{61, WeatherConditionSlightRain},
		{63, WeatherConditionModerateRain},
		{65, WeatherConditionHeavyRain},
		{71, WeatherConditionSlightSnowFall},
		{73, WeatherConditionModerateSnowFall},
		{75, WeatherConditionHeavySnowFall},
		{95, WeatherConditionThunderstorm},
		{999, WeatherConditionUnknown},
	}
	for _, s := range scenarios {
		t.Run(s.condition.String(), func(t *testing.T) {
			assert.Equal(t, s.condition, WeatherConditionFromCode(s.code))
		})
	}
}

func TestWeatherConditionIsValid(t *testing.T) {
	for _, c := range allWeatherConditions {
		assert.True(t, c.IsValid(), c.String())
	}
	assert.False(t, WeatherCondition(0).IsValid())
	assert.False(t, WeatherCondition(999).IsValid())
}

func TestParseWeatherCondition(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		c, err := ParseWeatherCondition("Clear sky")
		require.NoError(t, err)
		assert.Equal(t, WeatherConditionClearSky, c)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := ParseWeatherCondition("not-a-condition")
		assert.Error(t, err)
	})
}

func TestWeatherConditionString(t *testing.T) {
	assert.Equal(t, "Clear sky", WeatherConditionClearSky.String())
	assert.Equal(t, "Unknown", WeatherConditionUnknown.String())
	assert.Equal(t, "Unknown", WeatherCondition(0).String())
}

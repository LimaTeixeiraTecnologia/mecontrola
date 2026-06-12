package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
)

func TestRefMonthResolver_From(t *testing.T) {
	sut := services.RefMonthResolver{}

	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)

	tests := []struct {
		name     string
		utcTime  time.Time
		expected string
	}{
		{
			name:     "01:00 UTC = 22:00 do dia anterior em Brasília (verão -3h)",
			utcTime:  time.Date(2024, 2, 1, 1, 0, 0, 0, time.UTC),
			expected: "2024-01",
		},
		{
			name:     "04:00 UTC = 01:00 Brasília — mesmo dia",
			utcTime:  time.Date(2024, 2, 1, 4, 0, 0, 0, time.UTC),
			expected: "2024-02",
		},
		{
			name:     "mês de janeiro (início do ano)",
			utcTime:  time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			expected: "2024-01",
		},
		{
			name:     "mês de dezembro (fim do ano)",
			utcTime:  time.Date(2024, 12, 15, 12, 0, 0, 0, time.UTC),
			expected: "2024-12",
		},
		{
			name:     "virada de ano: 31 dez UTC -> ainda 31 dez BR",
			utcTime:  time.Date(2024, 12, 31, 12, 0, 0, 0, time.UTC),
			expected: "2024-12",
		},
		{
			name:     "1 jan 2025 meio-dia UTC -> jan 2025 BR",
			utcTime:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: "2025-01",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sut.From(tc.utcTime, loc)
			assert.Equal(t, tc.expected, got.String())
		})
	}
}

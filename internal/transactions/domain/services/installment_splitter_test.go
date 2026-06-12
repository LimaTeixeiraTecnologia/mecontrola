package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestInstallmentSplitter_Split(t *testing.T) {
	sut := services.InstallmentSplitter{}

	tests := []struct {
		name         string
		totalCents   int64
		installments int
		expected     []int64
	}{
		{
			name:         "1 parcela — total inteiro",
			totalCents:   100,
			installments: 1,
			expected:     []int64{100},
		},
		{
			name:         "2 parcelas — total par",
			totalCents:   200,
			installments: 2,
			expected:     []int64{100, 100},
		},
		{
			name:         "3 parcelas — total ímpar, 100 centavos",
			totalCents:   100,
			installments: 3,
			expected:     []int64{34, 33, 33},
		},
		{
			name:         "3 parcelas — 101 centavos",
			totalCents:   101,
			installments: 3,
			expected:     []int64{34, 34, 33},
		},
		{
			name:         "12 parcelas — total par",
			totalCents:   1200,
			installments: 12,
			expected:     []int64{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100},
		},
		{
			name:         "12 parcelas — ímpar 1201",
			totalCents:   1201,
			installments: 12,
			expected:     []int64{101, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100},
		},
		{
			name:         "24 parcelas — total divisível",
			totalCents:   2400,
			installments: 24,
			expected: func() []int64 {
				result := make([]int64, 24)
				for i := range result {
					result[i] = 100
				}
				return result
			}(),
		},
		{
			name:         "24 parcelas — 1 centavo indivisível",
			totalCents:   25,
			installments: 24,
			expected: func() []int64 {
				result := make([]int64, 24)
				for i := range result {
					result[i] = 1
				}
				result[0] = 2
				return result
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			total, err := valueobjects.NewMoney(tc.totalCents)
			require.NoError(t, err)

			n, err := valueobjects.NewInstallmentCount(tc.installments)
			require.NoError(t, err)

			got := sut.Split(total, n)
			require.Len(t, got, tc.installments)

			gotCents := make([]int64, len(got))
			var sum int64
			for i, m := range got {
				gotCents[i] = m.Cents()
				sum += m.Cents()
			}

			assert.Equal(t, tc.expected, gotCents)
			assert.Equal(t, tc.totalCents, sum, "soma das parcelas deve ser igual ao total")
		})
	}
}

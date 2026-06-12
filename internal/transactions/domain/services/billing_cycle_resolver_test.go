package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func mustSnapshot(closing, due int) valueobjects.CardBillingSnapshot {
	s, err := valueobjects.NewCardBillingSnapshot(closing, due)
	if err != nil {
		panic(err)
	}
	return s
}

func TestBillingCycleResolver_Resolve(t *testing.T) {
	sut := services.BillingCycleResolver{}
	loc := time.UTC

	tests := []struct {
		name            string
		purchasedAt     time.Time
		snapshot        valueobjects.CardBillingSnapshot
		installments    int
		wantRefMonths   []string
		wantDueDay0     int
		wantClosingDay0 int
	}{
		{
			name:            "compra antes do fechamento → mesmo mês",
			purchasedAt:     time.Date(2024, 1, 10, 0, 0, 0, 0, loc),
			snapshot:        mustSnapshot(15, 25),
			installments:    1,
			wantRefMonths:   []string{"2024-01"},
			wantDueDay0:     25,
			wantClosingDay0: 15,
		},
		{
			name:            "compra no dia do fechamento → mesmo mês",
			purchasedAt:     time.Date(2024, 1, 15, 0, 0, 0, 0, loc),
			snapshot:        mustSnapshot(15, 25),
			installments:    1,
			wantRefMonths:   []string{"2024-01"},
			wantDueDay0:     25,
			wantClosingDay0: 15,
		},
		{
			name:            "compra após fechamento → próximo mês",
			purchasedAt:     time.Date(2024, 1, 16, 0, 0, 0, 0, loc),
			snapshot:        mustSnapshot(15, 25),
			installments:    1,
			wantRefMonths:   []string{"2024-02"},
			wantDueDay0:     25,
			wantClosingDay0: 15,
		},
		{
			name:            "clamp: due_day=30, fevereiro tem 29 (2024 bissexto)",
			purchasedAt:     time.Date(2024, 1, 10, 0, 0, 0, 0, loc),
			snapshot:        mustSnapshot(15, 30),
			installments:    1,
			wantRefMonths:   []string{"2024-01"},
			wantDueDay0:     30,
			wantClosingDay0: 15,
		},
		{
			name:            "clamp: due_day=30, fevereiro 2025 (não bissexto) → dia 28",
			purchasedAt:     time.Date(2025, 1, 10, 0, 0, 0, 0, loc),
			snapshot:        mustSnapshot(15, 30),
			installments:    1,
			wantRefMonths:   []string{"2025-01"},
			wantDueDay0:     30,
			wantClosingDay0: 15,
		},
		{
			name:            "3 parcelas — virada de mês",
			purchasedAt:     time.Date(2024, 11, 10, 0, 0, 0, 0, loc),
			snapshot:        mustSnapshot(15, 25),
			installments:    3,
			wantRefMonths:   []string{"2024-11", "2024-12", "2025-01"},
			wantDueDay0:     25,
			wantClosingDay0: 15,
		},
		{
			name:            "clamp: closing_day=31 em abril (30 dias)",
			purchasedAt:     time.Date(2024, 3, 10, 0, 0, 0, 0, loc),
			snapshot:        mustSnapshot(31, 10),
			installments:    2,
			wantRefMonths:   []string{"2024-03", "2024-04"},
			wantDueDay0:     10,
			wantClosingDay0: 31,
		},
		{
			name:            "due_day < closing_day: due cai no mês seguinte ao fechamento",
			purchasedAt:     time.Date(2024, 1, 10, 0, 0, 0, 0, loc),
			snapshot:        mustSnapshot(25, 10),
			installments:    1,
			wantRefMonths:   []string{"2024-01"},
			wantDueDay0:     10,
			wantClosingDay0: 25,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := valueobjects.NewInstallmentCount(tc.installments)
			require.NoError(t, err)

			refMonths, closings, dues := sut.Resolve(tc.purchasedAt, tc.snapshot, n)

			require.Len(t, refMonths, tc.installments)
			require.Len(t, closings, tc.installments)
			require.Len(t, dues, tc.installments)

			for i, ref := range tc.wantRefMonths {
				assert.Equal(t, ref, refMonths[i].String(), "ref_month[%d]", i)
			}

			assert.Equal(t, tc.wantDueDay0, dues[0].Day(), "due_at day[0]")
			assert.Equal(t, tc.wantClosingDay0, closings[0].Day(), "closing_at day[0]")
		})
	}
}

func TestBillingCycleResolver_Clamp_FevBissexto(t *testing.T) {
	sut := services.BillingCycleResolver{}
	loc := time.UTC

	snapshot := mustSnapshot(15, 30)
	purchasedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, loc)

	n, err := valueobjects.NewInstallmentCount(2)
	require.NoError(t, err)

	_, _, dues := sut.Resolve(purchasedAt, snapshot, n)

	assert.Equal(t, 30, dues[0].Day(), "janeiro tem 31 dias — day 30 passa")

	purchasedAtFeb := time.Date(2024, 1, 20, 0, 0, 0, 0, loc)
	_, _, duesFeb := sut.Resolve(purchasedAtFeb, snapshot, n)
	assert.Equal(t, 29, duesFeb[0].Day(), "fevereiro 2024 bissexto — clamp para 29")
	assert.Equal(t, "2024-02", duesFeb[0].Format("2006-01"))
}

func TestBillingCycleResolver_Clamp_FevNaoBissexto(t *testing.T) {
	sut := services.BillingCycleResolver{}
	loc := time.UTC

	snapshot := mustSnapshot(15, 30)
	purchasedAt := time.Date(2025, 1, 20, 0, 0, 0, 0, loc)

	n, err := valueobjects.NewInstallmentCount(1)
	require.NoError(t, err)

	_, _, dues := sut.Resolve(purchasedAt, snapshot, n)
	assert.Equal(t, 28, dues[0].Day(), "fevereiro 2025 não bissexto — clamp para 28")
	assert.Equal(t, "2025-02", dues[0].Format("2006-01"))
}

func TestBillingCycleResolver_DueNextMonth(t *testing.T) {
	sut := services.BillingCycleResolver{}
	loc := time.UTC

	snapshot := mustSnapshot(25, 10)

	t.Run("compra em janeiro — fecha jan/25, vence fev/10", func(t *testing.T) {
		n, err := valueobjects.NewInstallmentCount(1)
		require.NoError(t, err)

		_, closings, dues := sut.Resolve(time.Date(2024, 1, 10, 0, 0, 0, 0, loc), snapshot, n)
		assert.Equal(t, "2024-01", closings[0].Format("2006-01"), "fechamento em janeiro")
		assert.Equal(t, 25, closings[0].Day())
		assert.Equal(t, "2024-02", dues[0].Format("2006-01"), "vencimento no mês seguinte")
		assert.Equal(t, 10, dues[0].Day())
	})

	t.Run("3 parcelas — due sempre um mês após closing", func(t *testing.T) {
		n, err := valueobjects.NewInstallmentCount(3)
		require.NoError(t, err)

		_, closings, dues := sut.Resolve(time.Date(2024, 11, 10, 0, 0, 0, 0, loc), snapshot, n)
		for i := range 3 {
			closeMonth := closings[i].Format("2006-01")
			dueMonth := dues[i].Format("2006-01")
			assert.Equal(t, 25, closings[i].Day(), "closing day[%d]", i)
			assert.Equal(t, 10, dues[i].Day(), "due day[%d]", i)
			assert.NotEqual(t, closeMonth, dueMonth, "due deve estar no mês seguinte ao fechamento [%d]", i)
		}
	})

	t.Run("due_day=10, closing_day=25 — clamp de fevereiro (dia 10 existe)", func(t *testing.T) {
		n, err := valueobjects.NewInstallmentCount(1)
		require.NoError(t, err)

		_, _, dues := sut.Resolve(time.Date(2025, 1, 10, 0, 0, 0, 0, loc), snapshot, n)
		assert.Equal(t, "2025-02", dues[0].Format("2006-01"), "vencimento em fevereiro")
		assert.Equal(t, 10, dues[0].Day(), "dia 10 existe em fevereiro")
	})

	t.Run("closing em dezembro → due em janeiro do ano seguinte", func(t *testing.T) {
		n, err := valueobjects.NewInstallmentCount(1)
		require.NoError(t, err)

		_, closings, dues := sut.Resolve(time.Date(2024, 12, 10, 0, 0, 0, 0, loc), snapshot, n)
		assert.Equal(t, "2024-12", closings[0].Format("2006-01"))
		assert.Equal(t, "2025-01", dues[0].Format("2006-01"), "virada de ano")
		assert.Equal(t, 10, dues[0].Day())
	})
}

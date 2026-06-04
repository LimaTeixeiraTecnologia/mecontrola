package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

func FuzzNewMoneyBRL(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1))
	f.Add(int64(-1))
	f.Add(int64(2990))
	f.Add(int64(29780))
	f.Add(int64(-9999))
	f.Add(int64(1<<62 - 1))
	f.Add(int64(-1 << 62))

	f.Fuzz(func(t *testing.T, cents int64) {
		m, err := valueobjects.NewMoneyBRL(cents)
		if err != nil {
			if !errors.Is(err, valueobjects.ErrNegativeAmount) {
				t.Errorf("erro inesperado para cents=%d: %v", cents, err)
			}
			return
		}
		if m.Cents() != cents {
			t.Errorf("Cents() retornou %d, esperado %d", m.Cents(), cents)
		}
	})
}

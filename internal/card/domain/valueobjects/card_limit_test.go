package valueobjects_test

import (
	"errors"
	"testing"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

func TestNewCardLimit(t *testing.T) {
	scenarios := []struct {
		name    string
		cents   int64
		wantErr error
		isZero  bool
	}{
		{name: "zero is valid and IsZero true", cents: 0, wantErr: nil, isZero: true},
		{name: "positive small", cents: 50000, wantErr: nil, isZero: false},
		{name: "positive boundary max", cents: 100_000_000, wantErr: nil, isZero: false},
		{name: "negative rejected", cents: -1, wantErr: domain.ErrCardLimitNegative, isZero: false},
		{name: "overflow rejected", cents: 100_000_001, wantErr: domain.ErrCardLimitTooLarge, isZero: false},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			vo, err := valueobjects.NewCardLimit(sc.cents)
			if !errors.Is(err, sc.wantErr) {
				t.Fatalf("err: got %v, want %v", err, sc.wantErr)
			}
			if err != nil {
				return
			}
			if vo.Cents() != sc.cents {
				t.Errorf("Cents: got %d, want %d", vo.Cents(), sc.cents)
			}
			if vo.IsZero() != sc.isZero {
				t.Errorf("IsZero: got %t, want %t", vo.IsZero(), sc.isZero)
			}
		})
	}
}

func TestCardLimit_ZeroValue(t *testing.T) {
	var z valueobjects.CardLimit
	if z.Cents() != 0 {
		t.Errorf("zero value Cents: got %d, want 0", z.Cents())
	}
	if !z.IsZero() {
		t.Error("zero value IsZero must be true")
	}
}

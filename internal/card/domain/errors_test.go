package domain_test

import (
	"errors"
	"testing"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

func TestSentinels(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrCardNotFound", domain.ErrCardNotFound},
		{"ErrNicknameConflict", domain.ErrNicknameConflict},
		{"ErrInvalidClosingDay", domain.ErrInvalidClosingDay},
		{"ErrInvalidDueDay", domain.ErrInvalidDueDay},
		{"ErrInvalidCardName", domain.ErrInvalidCardName},
		{"ErrInvalidNickname", domain.ErrInvalidNickname},
		{"ErrInvalidPurchaseDate", domain.ErrInvalidPurchaseDate},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			if s.err == nil {
				t.Fatalf("%s must not be nil", s.name)
			}
			if !errors.Is(s.err, s.err) {
				t.Errorf("errors.Is(%s, %s) = false, want true", s.name, s.name)
			}
		})
	}
}

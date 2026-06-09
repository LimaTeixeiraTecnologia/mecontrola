package services_test

import (
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
)

func TestSaoPauloLocation(t *testing.T) {
	loc := services.SaoPauloLocation()
	if loc == nil {
		t.Fatal("SaoPauloLocation() returned nil")
	}
	if loc.String() != "America/Sao_Paulo" {
		t.Errorf("location name: got %q, want %q", loc.String(), "America/Sao_Paulo")
	}

	loc2 := services.SaoPauloLocation()
	if loc != loc2 {
		t.Error("SaoPauloLocation() must return the same pointer (sync.Once)")
	}
}

func TestMustLoadSaoPauloOrExit(t *testing.T) {
	services.MustLoadSaoPauloOrExit()

	loc := services.SaoPauloLocation()
	if loc == nil {
		t.Fatal("location must not be nil after MustLoadSaoPauloOrExit")
	}
}

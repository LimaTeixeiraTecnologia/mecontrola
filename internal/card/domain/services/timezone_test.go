package services_test

import (
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
)

func TestNewSaoPauloLocation(t *testing.T) {
	loc, err := services.NewSaoPauloLocation()
	if err != nil {
		t.Fatalf("NewSaoPauloLocation() error = %v", err)
	}
	if loc == nil {
		t.Fatal("NewSaoPauloLocation() returned nil location")
	}
	if loc.String() != "America/Sao_Paulo" {
		t.Errorf("location name: got %q, want %q", loc.String(), "America/Sao_Paulo")
	}
}

package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
)

func TestExtractFunnelToken(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		src      string
		expected string
	}{
		{name: "s1 presente retorna s1", s1: "token-s1", src: "token-src", expected: "token-s1"},
		{name: "s1 vazio retorna src", s1: "", src: "token-src", expected: "token-src"},
		{name: "ambos vazios retorna vazio", s1: "", src: "", expected: ""},
		{name: "apenas s1 presente", s1: "only-s1", src: "", expected: "only-s1"},
		{name: "apenas src presente", s1: "", src: "only-src", expected: "only-src"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := usecases.ExtractFunnelTokenForTest(tt.s1, tt.src)
			assert.Equal(t, tt.expected, result)
		})
	}
}

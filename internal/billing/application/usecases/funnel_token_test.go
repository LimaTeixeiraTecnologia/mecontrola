package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
)

func TestExtractFunnelToken(t *testing.T) {
	tests := []struct {
		name     string
		sck      string
		s1       string
		src      string
		expected string
	}{
		{name: "sck presente tem prioridade sobre s1 e src", sck: "token-sck", s1: "token-s1", src: "token-src", expected: "token-sck"},
		{name: "sck presente tem prioridade sobre src sem s1", sck: "token-sck", s1: "", src: "token-src", expected: "token-sck"},
		{name: "sck vazio s1 presente retorna s1", sck: "", s1: "token-s1", src: "token-src", expected: "token-s1"},
		{name: "sck e s1 vazios retorna src", sck: "", s1: "", src: "token-src", expected: "token-src"},
		{name: "todos vazios retorna vazio", sck: "", s1: "", src: "", expected: ""},
		{name: "apenas sck presente", sck: "only-sck", s1: "", src: "", expected: "only-sck"},
		{name: "apenas s1 presente fallback legado", sck: "", s1: "only-s1", src: "", expected: "only-s1"},
		{name: "apenas src presente fallback legado", sck: "", s1: "", src: "only-src", expected: "only-src"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := usecases.ExtractFunnelTokenForTest(tt.sck, tt.s1, tt.src)
			assert.Equal(t, tt.expected, result)
		})
	}
}

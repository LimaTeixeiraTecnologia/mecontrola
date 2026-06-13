package server

import (
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

func TestResolveCORSOrigins(t *testing.T) {
	scenarios := []struct {
		name     string
		origins  string
		expected string
	}{
		{
			name:     "deve retornar origins configurados quando definidos",
			origins:  "https://app.mecontrola.com.br,https://checkout.mecontrola.com.br",
			expected: "https://app.mecontrola.com.br,https://checkout.mecontrola.com.br",
		},
		{
			name:     "deve retornar string vazia quando origins nao configurados",
			origins:  "",
			expected: "",
		},
		{
			name:     "deve retornar wildcard quando wildcard configurado explicitamente",
			origins:  "*",
			expected: "*",
		},
		{
			name:     "deve retornar origem unica sem modificacao",
			origins:  "https://app.mecontrola.com.br",
			expected: "https://app.mecontrola.com.br",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			cfg := &configs.Config{
				HTTPConfig: configs.HTTPConfig{
					CORSAllowedOrigins: sc.origins,
				},
			}
			got := resolveCORSOrigins(cfg)
			if got != sc.expected {
				t.Errorf("resolveCORSOrigins() = %q, want %q", got, sc.expected)
			}
		})
	}
}

func TestResolveCORSOrigins_ProductionNeverWildcardFallback(t *testing.T) {
	cfg := &configs.Config{
		AppConfig: configs.AppConfig{
			Environment: "production",
		},
		HTTPConfig: configs.HTTPConfig{
			CORSAllowedOrigins: "https://app.mecontrola.com.br",
		},
	}

	got := resolveCORSOrigins(cfg)
	if got == "*" {
		t.Errorf("resolveCORSOrigins() must not return wildcard in production, got %q", got)
	}
}

func TestResolveCORSOrigins_EmptyDoesNotFallbackToWildcard(t *testing.T) {
	cfg := &configs.Config{
		HTTPConfig: configs.HTTPConfig{
			CORSAllowedOrigins: "",
		},
	}

	got := resolveCORSOrigins(cfg)
	if got == "*" {
		t.Errorf("resolveCORSOrigins() must not silently fall back to wildcard when origins empty, got %q", got)
	}
}

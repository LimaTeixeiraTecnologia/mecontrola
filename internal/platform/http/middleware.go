package http

import (
	"net/http"
	"os"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
)

// CORSAllowedOriginsFromEnv lê a variável de ambiente CORS_ALLOWED_ORIGINS e retorna
// a lista de origens permitidas, ou vazio se não configurada.
// Default vazio em produção: rejeita qualquer origin (sem "*" jamais).
func CORSAllowedOriginsFromEnv() []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	return ParseCORSOrigins(raw)
}

// ParseCORSOrigins analisa uma string de origens separadas por vírgula.
// Retorna vazio se a string for vazia ou inválida.
// Nunca inclui "*" na lista (proibido por ADR-008).
func ParseCORSOrigins(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}
	}

	parsed, err := common.ParseOrigins(trimmed)
	if err != nil {
		return []string{}
	}

	result := make([]string, 0, len(parsed))
	for _, o := range parsed {
		if o != "*" {
			result = append(result, o)
		}
	}

	return result
}

// CORSAllowlist retorna um middleware que aplica política de CORS com allowlist estrita.
// Se origins estiver vazio, nenhuma origin externa é permitida (rejeita tudo).
// Wildcard "*" não é aceito (ADR-008).
func CORSAllowlist(origins []string) func(http.Handler) http.Handler {
	allowed := make([]string, 0, len(origins))
	for _, o := range origins {
		if o != "*" && strings.TrimSpace(o) != "" {
			allowed = append(allowed, o)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			if !common.IsOriginAllowed(origin, allowed) {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "3600")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

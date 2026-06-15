package ratelimit

import (
	"net"
	"net/http"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type KeyExtractor func(r *http.Request) string

func ByIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if remoteIP == "" {
		return r.RemoteAddr
	}
	return remoteIP
}

func ByUserID(r *http.Request) string {
	p, ok := auth.FromContext(r.Context())
	if !ok {
		return ""
	}
	return p.UserID.String()
}

func ByUserIDFallbackIP(r *http.Request) string {
	if key := ByUserID(r); key != "" {
		return key
	}
	return ByIP(r)
}

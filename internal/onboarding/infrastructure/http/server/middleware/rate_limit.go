package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/JailtonJunior94/devkit-go/pkg/responses"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu             sync.Mutex
	limiters       map[string]*ipLimiter
	rps            rate.Limit
	burst          int
	trustedProxies []*net.IPNet
	allowlist      []*net.IPNet
	cleanupTicker  *time.Ticker
	done           chan struct{}
}

func NewRateLimiter(requestsPerMinute int, burst int, trustedCIDRs []string, allowlistCIDRs []string) *RateLimiter {
	proxies := parseCIDRs(trustedCIDRs)
	allowNets := parseCIDRs(allowlistCIDRs)
	rl := &RateLimiter{
		limiters:       make(map[string]*ipLimiter),
		rps:            rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:          burst,
		trustedProxies: proxies,
		allowlist:      allowNets,
		cleanupTicker:  time.NewTicker(5 * time.Minute),
		done:           make(chan struct{}),
	}
	go rl.gcLoop()
	return rl
}

func (rl *RateLimiter) Stop() {
	rl.cleanupTicker.Stop()
	close(rl.done)
}

func (rl *RateLimiter) gcLoop() {
	for {
		select {
		case <-rl.done:
			return
		case <-rl.cleanupTicker.C:
			rl.gc()
		}
	}
}

func (rl *RateLimiter) gc() {
	cutoff := time.Now().Add(-10 * time.Minute)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, l := range rl.limiters {
		if l.lastSeen.Before(cutoff) {
			delete(rl.limiters, ip)
		}
	}
}

func (rl *RateLimiter) limiterFor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	l, ok := rl.limiters[ip]
	if !ok {
		l = &ipLimiter{
			limiter:  rate.NewLimiter(rl.rps, rl.burst),
			lastSeen: time.Now(),
		}
		rl.limiters[ip] = l
	}
	l.lastSeen = time.Now()
	return l.limiter
}

func (rl *RateLimiter) realIP(r *http.Request) string {
	remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if remoteIP == "" {
		remoteIP = r.RemoteAddr
	}

	if rl.isTrustedProxy(remoteIP) {
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.SplitN(xff, ",", 2)
			return strings.TrimSpace(parts[0])
		}
	}

	return remoteIP
}

func (rl *RateLimiter) isTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range rl.trustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func (rl *RateLimiter) isAllowlisted(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range rl.allowlist {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.realIP(r)
		if rl.isAllowlisted(ip) {
			next.ServeHTTP(w, r)
			return
		}
		if !rl.limiterFor(ip).Allow() {
			responses.Error(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseCIDRs(cidrs []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		if cidr == "" {
			continue
		}
		_, n, err := net.ParseCIDR(cidr)
		if err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}

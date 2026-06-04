package cache

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

// EntitlementLRU é um wrapper em torno de expirable.LRU que implementa EntitlementCache.
// Usa hashicorp/golang-lru/v2/expirable com TTL fixo (ADR-004).
// Capacidade e TTL são fixados na construção e não variam por entrada.
type EntitlementLRU struct {
	inner *expirable.LRU[string, entitlementEntry]
}

type entitlementEntry struct {
	decision  output.EntitlementDecision
	expiresAt time.Time
}

func NewEntitlementLRU(capacity int, defaultTTL time.Duration) *EntitlementLRU {
	return &EntitlementLRU{
		inner: expirable.NewLRU[string, entitlementEntry](capacity, nil, defaultTTL),
	}
}

func (c *EntitlementLRU) Get(userID identityentities.UserID) (output.EntitlementDecision, bool) {
	entry, ok := c.inner.Get(userID.String())
	if !ok {
		return output.EntitlementDecision{}, false
	}
	if time.Now().After(entry.expiresAt) {
		c.inner.Remove(userID.String())
		return output.EntitlementDecision{}, false
	}
	return entry.decision, true
}

func (c *EntitlementLRU) Set(userID identityentities.UserID, decision output.EntitlementDecision, ttl time.Duration) {
	if ttl <= 0 {
		c.inner.Remove(userID.String())
		return
	}
	c.inner.Add(userID.String(), entitlementEntry{
		decision:  decision,
		expiresAt: time.Now().Add(ttl),
	})
}

func (c *EntitlementLRU) Invalidate(userID identityentities.UserID) {
	c.inner.Remove(userID.String())
}

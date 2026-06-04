package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type EntitlementLRUSuite struct {
	suite.Suite
}

func TestEntitlementLRUSuite(t *testing.T) {
	suite.Run(t, new(EntitlementLRUSuite))
}

func (s *EntitlementLRUSuite) newUserID(v string) identityentities.UserID {
	id, err := identityentities.NewUserID(v)
	s.Require().NoError(err)
	return id
}

func (s *EntitlementLRUSuite) TestGet_Miss() {
	c := NewEntitlementLRU(10, 5*time.Minute)
	userID := s.newUserID("550e8400-e29b-41d4-a716-446655440001")

	_, ok := c.Get(userID)

	s.False(ok)
}

func (s *EntitlementLRUSuite) TestGet_Hit() {
	c := NewEntitlementLRU(10, 5*time.Minute)
	userID := s.newUserID("550e8400-e29b-41d4-a716-446655440001")
	decision := output.EntitlementDecision{Status: "granted"}

	c.Set(userID, decision, 5*time.Minute)
	got, ok := c.Get(userID)

	s.True(ok)
	s.Equal(decision, got)
}

func (s *EntitlementLRUSuite) TestSet_Overwrite() {
	c := NewEntitlementLRU(10, 5*time.Minute)
	userID := s.newUserID("550e8400-e29b-41d4-a716-446655440001")

	c.Set(userID, output.EntitlementDecision{Status: "granted"}, 5*time.Minute)
	c.Set(userID, output.EntitlementDecision{Status: "denied"}, 5*time.Minute)
	got, ok := c.Get(userID)

	s.True(ok)
	s.Equal("denied", got.Status)
}

func (s *EntitlementLRUSuite) TestInvalidate_RemovesEntry() {
	c := NewEntitlementLRU(10, 5*time.Minute)
	userID := s.newUserID("550e8400-e29b-41d4-a716-446655440001")
	c.Set(userID, output.EntitlementDecision{Status: "granted"}, 5*time.Minute)

	c.Invalidate(userID)

	_, ok := c.Get(userID)
	s.False(ok)
}

func (s *EntitlementLRUSuite) TestInvalidate_NonExistentKey_NoError() {
	c := NewEntitlementLRU(10, 5*time.Minute)
	userID := s.newUserID("550e8400-e29b-41d4-a716-446655440001")

	s.NotPanics(func() {
		c.Invalidate(userID)
	})
}

func TestEntitlementLRU_EvictsByCapacity(t *testing.T) {
	suite.Run(t, new(evictionSuite))
}

type evictionSuite struct {
	suite.Suite
}

func (s *evictionSuite) TestCapacityEviction() {
	c := NewEntitlementLRU(2, 5*time.Minute)

	uid1, _ := identityentities.NewUserID("550e8400-e29b-41d4-a716-446655440001")
	uid2, _ := identityentities.NewUserID("550e8400-e29b-41d4-a716-446655440002")
	uid3, _ := identityentities.NewUserID("550e8400-e29b-41d4-a716-446655440003")

	c.Set(uid1, output.EntitlementDecision{Status: "granted"}, 5*time.Minute)
	c.Set(uid2, output.EntitlementDecision{Status: "granted"}, 5*time.Minute)
	c.Set(uid3, output.EntitlementDecision{Status: "granted"}, 5*time.Minute)

	_, ok := c.Get(uid1)
	s.False(ok, "uid1 deve ter sido evicted por LRU (capacity=2, inseriu 3)")

	_, ok2 := c.Get(uid2)
	_, ok3 := c.Get(uid3)
	s.True(ok2 || ok3, "uid2 ou uid3 devem estar no cache")
}

func TestEntitlementLRU_ExpiresByTTL(t *testing.T) {
	suite.Run(t, new(ttlSuite))
}

type ttlSuite struct {
	suite.Suite
}

func (s *ttlSuite) TestTTLExpiration() {
	c := NewEntitlementLRU(10, 50*time.Millisecond)
	userID, _ := identityentities.NewUserID("550e8400-e29b-41d4-a716-446655440001")

	c.Set(userID, output.EntitlementDecision{Status: "granted"}, 50*time.Millisecond)

	_, okBefore := c.Get(userID)
	s.True(okBefore, "deve existir antes do TTL expirar")

	time.Sleep(100 * time.Millisecond)

	_, okAfter := c.Get(userID)
	s.False(okAfter, "deve ter expirado após TTL")
}

func (s *ttlSuite) TestPerEntryTTLShorterThanDefault() {
	c := NewEntitlementLRU(10, 5*time.Minute)
	userID, _ := identityentities.NewUserID("550e8400-e29b-41d4-a716-446655440001")

	c.Set(userID, output.EntitlementDecision{Status: "granted"}, 30*time.Millisecond)

	time.Sleep(80 * time.Millisecond)

	_, ok := c.Get(userID)
	s.False(ok, "ttl por entrada deve vencer antes do ttl default")
}

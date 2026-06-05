package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type WhatsAppHistoryEntrySuite struct {
	suite.Suite
}

func TestWhatsAppHistoryEntrySuite(t *testing.T) {
	suite.Run(t, new(WhatsAppHistoryEntrySuite))
}

func (s *WhatsAppHistoryEntrySuite) TestNewWhatsAppHistoryEntry_GeneratesValidID() {
	entry := entities.NewWhatsAppHistoryEntry("user-123", "+5511987654321", "initial_link")

	assert.Regexp(s.T(), `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, entry.ID())
}

func (s *WhatsAppHistoryEntrySuite) TestNewWhatsAppHistoryEntry_LinkedAtIsNonZero() {
	before := time.Now().UTC()
	entry := entities.NewWhatsAppHistoryEntry("user-123", "+5511987654321", "initial_link")
	after := time.Now().UTC()

	s.False(entry.LinkedAt().IsZero())
	s.True(!entry.LinkedAt().Before(before))
	s.True(!entry.LinkedAt().After(after))
}

func (s *WhatsAppHistoryEntrySuite) TestNewWhatsAppHistoryEntry_ActiveIsTrue() {
	entry := entities.NewWhatsAppHistoryEntry("user-123", "+5511987654321", "initial_link")

	s.True(entry.Active())
}

func (s *WhatsAppHistoryEntrySuite) TestNewWhatsAppHistoryEntry_FieldsSetCorrectly() {
	entry := entities.NewWhatsAppHistoryEntry("user-abc", "+5511912345678", "reactivation")

	s.Equal("user-abc", entry.UserID())
	s.Equal("+5511912345678", entry.Number())
	s.Equal("reactivation", entry.Reason())
	s.True(entry.UnlinkedAt().IsZero())
}

func (s *WhatsAppHistoryEntrySuite) TestNewWhatsAppHistoryEntry_UniqueIDs() {
	e1 := entities.NewWhatsAppHistoryEntry("user-1", "+5511987654321", "r1")
	e2 := entities.NewWhatsAppHistoryEntry("user-1", "+5511987654321", "r1")

	s.NotEqual(e1.ID(), e2.ID())
}

package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type WhatsAppHistoryEntrySuite struct {
	suite.Suite
}

func TestWhatsAppHistoryEntrySuite(t *testing.T) {
	suite.Run(t, new(WhatsAppHistoryEntrySuite))
}

func (s *WhatsAppHistoryEntrySuite) SetupTest() {}

func (s *WhatsAppHistoryEntrySuite) TestNewWhatsAppHistoryEntry() {
	type args struct {
		userID string
		number string
		reason string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(entities.WhatsAppHistoryEntry, time.Time, time.Time)
	}{
		{
			name: "deve preencher campos obrigatorios",
			args: args{userID: "user-abc", number: "+5511912345678", reason: "reactivation"},
			expect: func(entry entities.WhatsAppHistoryEntry, before time.Time, after time.Time) {
				s.Equal("user-abc", entry.UserID())
				s.Equal("+5511912345678", entry.Number())
				s.Equal("reactivation", entry.Reason())
				s.True(entry.Active())
				s.True(entry.UnlinkedAt().IsZero())
				s.False(entry.LinkedAt().Before(before))
				s.False(entry.LinkedAt().After(after))
			},
		},
		{
			name: "deve gerar ids unicos",
			args: args{userID: "user-1", number: "+5511987654321", reason: "initial_link"},
			expect: func(entry entities.WhatsAppHistoryEntry, _ time.Time, _ time.Time) {
				second := entities.NewWhatsAppHistoryEntry("user-1", "+5511987654321", "initial_link")
				s.NotEqual(entry.ID(), second.ID())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			before := time.Now().UTC()
			entry := entities.NewWhatsAppHistoryEntry(scenario.args.userID, scenario.args.number, scenario.args.reason)
			after := time.Now().UTC()

			scenario.expect(entry, before, after)
		})
	}
}

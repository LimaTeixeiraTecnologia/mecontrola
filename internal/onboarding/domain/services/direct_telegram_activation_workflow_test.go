package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type DirectTelegramActivationWorkflowSuite struct {
	suite.Suite
}

func TestDirectTelegramActivationWorkflow(t *testing.T) {
	suite.Run(t, new(DirectTelegramActivationWorkflowSuite))
}

func (s *DirectTelegramActivationWorkflowSuite) buildToken(mobile, email string) entities.MagicToken {
	now := time.Now().UTC()
	return entities.HydrateMagicToken(
		"token-id",
		[]byte("hash"),
		valueobjects.TokenStatusPaid,
		"plan-x",
		now.Add(24*time.Hour),
		now,
		now,
		time.Time{},
		time.Time{},
		"",
		"sub-1",
		mobile,
		email,
		"sale-1",
		"",
		"",
		valueobjects.ActivationPathDirect,
		"",
	)
}

func (s *DirectTelegramActivationWorkflowSuite) TestDecide() {
	w := services.NewDirectTelegramActivationWorkflow()

	cases := []struct {
		name        string
		token       entities.MagicToken
		flagEnabled bool
		expected    services.DirectActivationOutcome
		expectData  bool
	}{
		{
			name:        "flag desabilitada preserva regra WhatsApp",
			token:       s.buildToken("+5511987654321", "user@example.com"),
			flagEnabled: false,
			expected:    services.OutcomeRequiresWhatsAppActivation,
		},
		{
			name:        "flag habilitada sem email bloqueia fail-safe",
			token:       s.buildToken("+5511987654321", ""),
			flagEnabled: true,
			expected:    services.OutcomeDirectBlocked,
		},
		{
			name:        "flag habilitada sem mobile bloqueia fail-safe",
			token:       s.buildToken("", "user@example.com"),
			flagEnabled: true,
			expected:    services.OutcomeDirectBlocked,
		},
		{
			name:        "flag habilitada com dados completos permite ativacao direta",
			token:       s.buildToken("+5511987654321", "user@example.com"),
			flagEnabled: true,
			expected:    services.OutcomeDirectAllowed,
			expectData:  true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			decision := w.Decide(tc.token, tc.flagEnabled)
			s.Equal(tc.expected, decision.Outcome)
			if tc.expectData {
				s.Equal("+5511987654321", decision.CustomerMobileE164)
				s.Equal("user@example.com", decision.CustomerEmail)
			} else {
				s.Empty(decision.CustomerMobileE164)
				s.Empty(decision.CustomerEmail)
			}
		})
	}
}

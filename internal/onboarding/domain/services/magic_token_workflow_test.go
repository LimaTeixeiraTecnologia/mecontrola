package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type MagicTokenWorkflowSuite struct {
	suite.Suite
	now      time.Time
	workflow services.MagicTokenWorkflow
}

func TestMagicTokenWorkflowSuite(t *testing.T) {
	suite.Run(t, new(MagicTokenWorkflowSuite))
}

func (s *MagicTokenWorkflowSuite) SetupTest() {
	s.now = time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	s.workflow = services.NewMagicTokenWorkflow()
}

func (s *MagicTokenWorkflowSuite) newPendingToken() entities.MagicToken {
	token, err := entities.NewMagicToken("token-id-1", []byte{0xde, 0xad, 0xbe, 0xef}, "plan-1", s.now.Add(24*time.Hour))
	s.Require().NoError(err)
	return token
}

func (s *MagicTokenWorkflowSuite) newPaidToken() entities.MagicToken {
	t, err := s.newPendingToken().MarkPaid("sub-001", "+5511999990000", "u@test.com", "ext-1", s.now)
	s.Require().NoError(err)
	return t
}

func (s *MagicTokenWorkflowSuite) markPaidCmd() services.MarkPaidCommand {
	return services.MarkPaidCommand{
		SubscriptionID:     "sub-001",
		CustomerMobileE164: "+5511999990000",
		CustomerEmail:      "u@test.com",
		ExternalSaleID:     "ext-1",
		PaidAt:             s.now,
	}
}

func (s *MagicTokenWorkflowSuite) TestDecideMarkPaid() {
	scenarios := []struct {
		name    string
		current func() entities.MagicToken
		expect  func(decision services.MarkPaidDecision, err error)
	}{
		{
			name:    "deve transicionar pendente para pago",
			current: s.newPendingToken,
			expect: func(decision services.MarkPaidDecision, err error) {
				s.NoError(err)
				s.Equal(services.MarkPaidOutcomeTransitioned, decision.Outcome)
				s.Equal(valueobjects.TokenStatusPaid, decision.Token.Status())
				s.Equal("sub-001", decision.Token.SubscriptionID())
				s.WithinDuration(s.now, decision.Token.PaidAt(), time.Second)
			},
		},
		{
			name:    "deve ser noop quando ja pago",
			current: s.newPaidToken,
			expect: func(decision services.MarkPaidDecision, err error) {
				s.NoError(err)
				s.Equal(services.MarkPaidOutcomeNoChange, decision.Outcome)
				s.Equal(valueobjects.TokenStatusPaid, decision.Token.Status())
			},
		},
		{
			name: "deve ser noop quando expirado",
			current: func() entities.MagicToken {
				t, err := s.newPendingToken().MarkExpired()
				s.Require().NoError(err)
				return t
			},
			expect: func(decision services.MarkPaidDecision, err error) {
				s.NoError(err)
				s.Equal(services.MarkPaidOutcomeNoChange, decision.Outcome)
				s.Equal(valueobjects.TokenStatusExpired, decision.Token.Status())
			},
		},
		{
			name: "deve falhar quando subscription id ausente",
			current: func() entities.MagicToken {
				return s.newPendingToken()
			},
			expect: func(decision services.MarkPaidDecision, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			cmd := s.markPaidCmd()
			if scenario.name == "deve falhar quando subscription id ausente" {
				cmd.SubscriptionID = ""
			}
			decision, err := s.workflow.DecideMarkPaid(scenario.current(), cmd)
			scenario.expect(decision, err)
		})
	}
}

func (s *MagicTokenWorkflowSuite) TestDecideConsume() {
	baseCmd := func() services.ConsumeCommand {
		return services.ConsumeCommand{
			UserID:         "user-1",
			FromE164:       "+5511999990000",
			ActivationPath: valueobjects.ActivationPathDirect,
			EventID:        "evt-1",
		}
	}

	scenarios := []struct {
		name    string
		current func() entities.MagicToken
		cmd     services.ConsumeCommand
		expect  func(decision services.ConsumeDecision, err error)
	}{
		{
			name:    "deve consumir token pago e emitir evento",
			current: s.newPaidToken,
			cmd:     baseCmd(),
			expect: func(decision services.ConsumeDecision, err error) {
				s.NoError(err)
				s.Equal(valueobjects.TokenStatusConsumed, decision.Token.Status())
				s.Equal("user-1", decision.Token.ConsumedByUserID())
				s.Equal("evt-1", decision.Event.EventID)
				s.Equal("token-id-1", decision.Event.TokenID)
				s.Equal("user-1", decision.Event.UserID)
				s.Equal("sub-001", decision.Event.SubscriptionID)
				s.Equal(valueobjects.ActivationPathDirect, decision.Event.ActivationPath)
				s.Equal(s.now, decision.Event.BoundAt)
				s.Equal("deadbeef", decision.Event.TokenHashPrefix)
			},
		},
		{
			name:    "deve rejeitar quando token nao esta pago",
			current: s.newPendingToken,
			cmd:     baseCmd(),
			expect: func(decision services.ConsumeDecision, err error) {
				s.ErrorIs(err, domain.ErrTransitionNotAllowed)
			},
		},
		{
			name:    "deve falhar quando user id ausente",
			current: s.newPaidToken,
			cmd: services.ConsumeCommand{
				UserID:         "",
				FromE164:       "+5511999990000",
				ActivationPath: valueobjects.ActivationPathDirect,
				EventID:        "evt-1",
			},
			expect: func(decision services.ConsumeDecision, err error) {
				s.Error(err)
			},
		},
		{
			name:    "deve falhar quando event id ausente",
			current: s.newPaidToken,
			cmd: services.ConsumeCommand{
				UserID:         "user-1",
				FromE164:       "+5511999990000",
				ActivationPath: valueobjects.ActivationPathDirect,
				EventID:        "",
			},
			expect: func(decision services.ConsumeDecision, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			decision, err := s.workflow.DecideConsume(scenario.current(), scenario.cmd, s.now)
			scenario.expect(decision, err)
		})
	}
}

func (s *MagicTokenWorkflowSuite) TestDecideConsume_DeterministicHashPrefix() {
	token := s.newPaidToken()
	cmd := services.ConsumeCommand{
		UserID:         "user-1",
		FromE164:       "+5511999990000",
		ActivationPath: valueobjects.ActivationPathFallbackE164,
		EventID:        "evt-1",
	}
	a, errA := s.workflow.DecideConsume(token, cmd, s.now)
	b, errB := s.workflow.DecideConsume(token, cmd, s.now)
	s.Require().NoError(errA)
	s.Require().NoError(errB)
	s.Equal(a.Event, b.Event)
}

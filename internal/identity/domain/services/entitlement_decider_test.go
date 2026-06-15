package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
	domainmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
)

type EntitlementDeciderSuite struct {
	suite.Suite
	decider services.EntitlementDecider
	now     time.Time
}

func TestEntitlementDeciderSuite(t *testing.T) {
	suite.Run(t, new(EntitlementDeciderSuite))
}

func (s *EntitlementDeciderSuite) SetupTest() {
	s.decider = services.EntitlementDecider{}
	s.now = time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
}

func (s *EntitlementDeciderSuite) TestDecide() {
	type tc struct {
		name         string
		sub          func() domain.Subscription
		wantEntitled bool
		wantReason   domain.Reason
	}

	cases := []tc{
		{
			name:         "nega acesso quando assinatura e nil",
			sub:          func() domain.Subscription { return nil },
			wantEntitled: false,
			wantReason:   domain.ReasonNoSubscription,
		},
		{
			name: "concede acesso para assinatura ativa com periodo vigente",
			sub: func() domain.Subscription {
				m := domainmocks.NewSubscription(s.T())
				m.EXPECT().Status().Return(domain.SubscriptionActive)
				m.EXPECT().PeriodEnd().Return(s.now.Add(24 * time.Hour))
				return m
			},
			wantEntitled: true,
			wantReason:   domain.ReasonActive,
		},
		{
			name: "nega acesso para assinatura ativa com periodo expirado",
			sub: func() domain.Subscription {
				m := domainmocks.NewSubscription(s.T())
				m.EXPECT().Status().Return(domain.SubscriptionActive)
				m.EXPECT().PeriodEnd().Return(s.now.Add(-24 * time.Hour))
				return m
			},
			wantEntitled: false,
			wantReason:   domain.ReasonExpired,
		},
		{
			name: "concede acesso em grace period de past_due",
			sub: func() domain.Subscription {
				m := domainmocks.NewSubscription(s.T())
				m.EXPECT().Status().Return(domain.SubscriptionPastDue)
				m.EXPECT().GracePeriodEnd().Return(s.now.Add(72 * time.Hour))
				return m
			},
			wantEntitled: true,
			wantReason:   domain.ReasonPastDueGrace,
		},
		{
			name: "nega acesso em past_due sem grace period",
			sub: func() domain.Subscription {
				m := domainmocks.NewSubscription(s.T())
				m.EXPECT().Status().Return(domain.SubscriptionPastDue)
				m.EXPECT().GracePeriodEnd().Return(time.Time{})
				return m
			},
			wantEntitled: false,
			wantReason:   domain.ReasonPastDueNoGrace,
		},
		{
			name: "concede acesso para canceled_pending com periodo vigente",
			sub: func() domain.Subscription {
				m := domainmocks.NewSubscription(s.T())
				m.EXPECT().Status().Return(domain.SubscriptionCanceledPending)
				m.EXPECT().PeriodEnd().Return(s.now.Add(24 * time.Hour))
				return m
			},
			wantEntitled: true,
			wantReason:   domain.ReasonCanceledPending,
		},
		{
			name: "nega acesso para assinatura expirada",
			sub: func() domain.Subscription {
				m := domainmocks.NewSubscription(s.T())
				m.EXPECT().Status().Return(domain.SubscriptionExpired)
				return m
			},
			wantEntitled: false,
			wantReason:   domain.ReasonExpired,
		},
		{
			name: "nega acesso para assinatura reembolsada",
			sub: func() domain.Subscription {
				m := domainmocks.NewSubscription(s.T())
				m.EXPECT().Status().Return(domain.SubscriptionRefunded)
				return m
			},
			wantEntitled: false,
			wantReason:   domain.ReasonRefunded,
		},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			got := s.decider.Decide(c.sub(), s.now)
			s.Equal(c.wantEntitled, got.Entitled)
			s.Equal(c.wantReason, got.Reason)
		})
	}
}

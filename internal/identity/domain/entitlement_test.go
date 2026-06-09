package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
	domainmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/mocks"
)

type EntitlementSuite struct {
	suite.Suite
	now time.Time
}

func TestEntitlementSuite(t *testing.T) {
	suite.Run(t, new(EntitlementSuite))
}

func (s *EntitlementSuite) SetupTest() {
	s.now = time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
}

func (s *EntitlementSuite) TestIsEntitled() {
	type args struct {
		useNil    bool
		status    domain.SubscriptionStatus
		periodEnd time.Time
		graceEnd  time.Time
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(bool, domain.Reason)
	}{
		{
			name: "deve negar acesso quando nao houver assinatura",
			args: args{useNil: true},
			expect: func(entitled bool, reason domain.Reason) {
				s.False(entitled)
				s.Equal(domain.ReasonNoSubscription, reason)
			},
		},
		{
			name: "deve conceder acesso para assinatura ativa com periodo vigente",
			args: args{status: domain.SubscriptionActive, periodEnd: s.now.Add(24 * time.Hour)},
			expect: func(entitled bool, reason domain.Reason) {
				s.True(entitled)
				s.Equal(domain.ReasonActive, reason)
			},
		},
		{
			name: "deve negar acesso para assinatura ativa com periodo expirado",
			args: args{status: domain.SubscriptionActive, periodEnd: s.now.Add(-24 * time.Hour)},
			expect: func(entitled bool, reason domain.Reason) {
				s.False(entitled)
				s.Equal(domain.ReasonExpired, reason)
			},
		},
		{
			name: "deve conceder acesso para trialing com periodo vigente",
			args: args{status: domain.SubscriptionTrialing, periodEnd: s.now.Add(24 * time.Hour)},
			expect: func(entitled bool, reason domain.Reason) {
				s.True(entitled)
				s.Equal(domain.ReasonTrialing, reason)
			},
		},
		{
			name: "deve negar acesso para trialing com periodo expirado",
			args: args{status: domain.SubscriptionTrialing, periodEnd: s.now.Add(-24 * time.Hour)},
			expect: func(entitled bool, reason domain.Reason) {
				s.False(entitled)
				s.Equal(domain.ReasonExpired, reason)
			},
		},
		{
			name: "deve conceder acesso para past due dentro da carencia",
			args: args{status: domain.SubscriptionPastDue, graceEnd: s.now.Add(24 * time.Hour)},
			expect: func(entitled bool, reason domain.Reason) {
				s.True(entitled)
				s.Equal(domain.ReasonPastDueGrace, reason)
			},
		},
		{
			name: "deve negar acesso para past due sem carencia",
			args: args{status: domain.SubscriptionPastDue, graceEnd: s.now.Add(-24 * time.Hour)},
			expect: func(entitled bool, reason domain.Reason) {
				s.False(entitled)
				s.Equal(domain.ReasonPastDueNoGrace, reason)
			},
		},
		{
			name: "deve negar acesso para past due sem grace period definido",
			args: args{status: domain.SubscriptionPastDue, graceEnd: time.Time{}},
			expect: func(entitled bool, reason domain.Reason) {
				s.False(entitled)
				s.Equal(domain.ReasonPastDueNoGrace, reason)
			},
		},
		{
			name: "deve conceder acesso para cancelado com periodo vigente",
			args: args{status: domain.SubscriptionCanceledPending, periodEnd: s.now.Add(24 * time.Hour)},
			expect: func(entitled bool, reason domain.Reason) {
				s.True(entitled)
				s.Equal(domain.ReasonCanceledPending, reason)
			},
		},
		{
			name: "deve negar acesso para cancelado com periodo expirado",
			args: args{status: domain.SubscriptionCanceledPending, periodEnd: s.now.Add(-24 * time.Hour)},
			expect: func(entitled bool, reason domain.Reason) {
				s.False(entitled)
				s.Equal(domain.ReasonExpired, reason)
			},
		},
		{
			name: "deve negar acesso para assinatura expirada",
			args: args{status: domain.SubscriptionExpired},
			expect: func(entitled bool, reason domain.Reason) {
				s.False(entitled)
				s.Equal(domain.ReasonExpired, reason)
			},
		},
		{
			name: "deve negar acesso para assinatura reembolsada",
			args: args{status: domain.SubscriptionRefunded},
			expect: func(entitled bool, reason domain.Reason) {
				s.False(entitled)
				s.Equal(domain.ReasonRefunded, reason)
			},
		},
		{
			name: "deve negar acesso para status desconhecido",
			args: args{status: domain.SubscriptionStatus("UNKNOWN_STATUS")},
			expect: func(entitled bool, reason domain.Reason) {
				s.False(entitled)
				s.Equal(domain.ReasonExpired, reason)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var subscription domain.Subscription
			if !scenario.args.useNil {
				mockSubscription := domainmocks.NewSubscription(s.T())
				mockSubscription.EXPECT().Status().Return(scenario.args.status).Once()
				switch scenario.args.status {
				case domain.SubscriptionActive, domain.SubscriptionTrialing, domain.SubscriptionCanceledPending:
					mockSubscription.EXPECT().PeriodEnd().Return(scenario.args.periodEnd).Once()
				case domain.SubscriptionPastDue:
					mockSubscription.EXPECT().GracePeriodEnd().Return(scenario.args.graceEnd).Once()
				}
				subscription = mockSubscription
			}

			entitled, reason := domain.IsEntitled(subscription, s.now)
			scenario.expect(entitled, reason)
		})
	}
}

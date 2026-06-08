package valueobjects_test

import (
	"github.com/stretchr/testify/suite"
	"testing"

	"github.com/stretchr/testify/assert"

	billingvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identitydomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
)

type StatusSuite struct {
	suite.Suite
}

func TestStatusSuite(t *testing.T) {
	suite.Run(t, new(StatusSuite))
}

func (s *StatusSuite) SetupTest() {}

func (s *StatusSuite) TestStatus() {
	type args struct {
		status billingvo.Status
	}

	scenarios := []struct {
		name   string
		args   args
		expect func()
	}{
		{
			name: "deve refletir string e helpers do status trialing",
			args: args{status: billingvo.StatusTrialing},
			expect: func() {
				assert.Equal(s.T(), "TRIALING", billingvo.StatusTrialing.String())
				assert.False(s.T(), billingvo.StatusTrialing.IsTerminal())
				assert.False(s.T(), billingvo.StatusTrialing.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status active",
			args: args{status: billingvo.StatusActive},
			expect: func() {
				assert.Equal(s.T(), "ACTIVE", billingvo.StatusActive.String())
				assert.False(s.T(), billingvo.StatusActive.IsTerminal())
				assert.True(s.T(), billingvo.StatusActive.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status past due",
			args: args{status: billingvo.StatusPastDue},
			expect: func() {
				assert.Equal(s.T(), "PAST_DUE", billingvo.StatusPastDue.String())
				assert.False(s.T(), billingvo.StatusPastDue.IsTerminal())
				assert.True(s.T(), billingvo.StatusPastDue.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status canceled pending",
			args: args{status: billingvo.StatusCanceledPending},
			expect: func() {
				assert.Equal(s.T(), "CANCELED_PENDING", billingvo.StatusCanceledPending.String())
				assert.False(s.T(), billingvo.StatusCanceledPending.IsTerminal())
				assert.True(s.T(), billingvo.StatusCanceledPending.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status expired",
			args: args{status: billingvo.StatusExpired},
			expect: func() {
				assert.Equal(s.T(), "EXPIRED", billingvo.StatusExpired.String())
				assert.True(s.T(), billingvo.StatusExpired.IsTerminal())
				assert.False(s.T(), billingvo.StatusExpired.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status refunded",
			args: args{status: billingvo.StatusRefunded},
			expect: func() {
				assert.Equal(s.T(), "REFUNDED", billingvo.StatusRefunded.String())
				assert.True(s.T(), billingvo.StatusRefunded.IsTerminal())
				assert.False(s.T(), billingvo.StatusRefunded.IsActiveForBilling())
			},
		},
		{
			name: "deve manter zero value reservado",
			args: args{status: 0},
			expect: func() {
				assert.Equal(s.T(), "", billingvo.Status(0).String())
				assert.False(s.T(), billingvo.Status(0).IsTerminal())
				assert.False(s.T(), billingvo.Status(0).IsActiveForBilling())
			},
		},
		{
			name: "deve manter alinhamento com status de assinatura da identidade",
			expect: func() {
				billingStatuses := []billingvo.Status{
					billingvo.StatusTrialing,
					billingvo.StatusActive,
					billingvo.StatusPastDue,
					billingvo.StatusCanceledPending,
					billingvo.StatusExpired,
					billingvo.StatusRefunded,
				}
				identityStatuses := []identitydomain.SubscriptionStatus{
					identitydomain.SubscriptionTrialing,
					identitydomain.SubscriptionActive,
					identitydomain.SubscriptionPastDue,
					identitydomain.SubscriptionCanceledPending,
					identitydomain.SubscriptionExpired,
					identitydomain.SubscriptionRefunded,
				}

				if assert.Len(s.T(), billingStatuses, len(identityStatuses)) {
					for idx := range billingStatuses {
						assert.Equal(s.T(), string(identityStatuses[idx]), billingStatuses[idx].String())
					}
				}
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			_ = scenario.args
			scenario.expect()
		})
	}
}

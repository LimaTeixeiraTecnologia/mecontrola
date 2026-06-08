package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

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
		expect func(args)
	}{
		{
			name: "deve refletir string e helpers do status trialing",
			args: args{status: billingvo.StatusTrialing},
			expect: func(args args) {
				assert.Equal(s.T(), "TRIALING", args.status.String())
				assert.False(s.T(), args.status.IsTerminal())
				assert.False(s.T(), args.status.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status active",
			args: args{status: billingvo.StatusActive},
			expect: func(args args) {
				assert.Equal(s.T(), "ACTIVE", args.status.String())
				assert.False(s.T(), args.status.IsTerminal())
				assert.True(s.T(), args.status.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status past due",
			args: args{status: billingvo.StatusPastDue},
			expect: func(args args) {
				assert.Equal(s.T(), "PAST_DUE", args.status.String())
				assert.False(s.T(), args.status.IsTerminal())
				assert.True(s.T(), args.status.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status canceled pending",
			args: args{status: billingvo.StatusCanceledPending},
			expect: func(args args) {
				assert.Equal(s.T(), "CANCELED_PENDING", args.status.String())
				assert.False(s.T(), args.status.IsTerminal())
				assert.True(s.T(), args.status.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status expired",
			args: args{status: billingvo.StatusExpired},
			expect: func(args args) {
				assert.Equal(s.T(), "EXPIRED", args.status.String())
				assert.True(s.T(), args.status.IsTerminal())
				assert.False(s.T(), args.status.IsActiveForBilling())
			},
		},
		{
			name: "deve refletir string e helpers do status refunded",
			args: args{status: billingvo.StatusRefunded},
			expect: func(args args) {
				assert.Equal(s.T(), "REFUNDED", args.status.String())
				assert.True(s.T(), args.status.IsTerminal())
				assert.False(s.T(), args.status.IsActiveForBilling())
			},
		},
		{
			name: "deve manter zero value reservado",
			args: args{status: 0},
			expect: func(args args) {
				assert.Equal(s.T(), "", args.status.String())
				assert.False(s.T(), args.status.IsTerminal())
				assert.False(s.T(), args.status.IsActiveForBilling())
			},
		},
		{
			name: "deve manter alinhamento com status de assinatura da identidade",
			expect: func(args args) {
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
			scenario.expect(scenario.args)
		})
	}
}

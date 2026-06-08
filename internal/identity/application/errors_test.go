package application_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
)

type ErrorsSuite struct {
	suite.Suite
}

func TestErrorsSuite(t *testing.T) {
	suite.Run(t, new(ErrorsSuite))
}

func (s *ErrorsSuite) SetupTest() {}

func (s *ErrorsSuite) TestSentinelsRemainDistinctAndWrappable() {
	type args struct {
		left    error
		right   error
		wrapped error
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(args)
	}{
		{
			name: "deve manter err user not found distinto de err whatsapp number in use",
			args: args{
				left:    application.ErrUserNotFound,
				right:   application.ErrWhatsAppNumberInUse,
				wrapped: fmt.Errorf("ctx: %w", application.ErrUserNotFound),
			},
			expect: func(current args) {
				s.False(errors.Is(current.left, current.right))
				s.True(errors.Is(current.wrapped, current.left))
			},
		},
		{
			name: "deve manter err user not found distinto de err email in use",
			args: args{
				left:    application.ErrUserNotFound,
				right:   application.ErrEmailInUse,
				wrapped: fmt.Errorf("ctx: %w", application.ErrUserNotFound),
			},
			expect: func(current args) {
				s.False(errors.Is(current.left, current.right))
				s.True(errors.Is(current.wrapped, current.left))
			},
		},
		{
			name: "deve manter err entitlement not found distinto de err email in use",
			args: args{
				left:    application.ErrEntitlementNotFound,
				right:   application.ErrEmailInUse,
				wrapped: fmt.Errorf("identity.repository.entitlement.find_by_user_id: %w", application.ErrEntitlementNotFound),
			},
			expect: func(current args) {
				s.False(errors.Is(current.left, current.right))
				s.True(errors.Is(current.wrapped, current.left))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(scenario.args)
		})
	}
}

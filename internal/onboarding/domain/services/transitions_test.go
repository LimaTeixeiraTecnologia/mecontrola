package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type TransitionSuite struct {
	suite.Suite
}

func TestTransitionSuite(t *testing.T) {
	suite.Run(t, new(TransitionSuite))
}

func (s *TransitionSuite) SetupTest() {}

func (s *TransitionSuite) TestPermissions() {
	type args struct {
		status valueobjects.TokenStatus
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(services.TransitionService)
	}{
		{
			name: "deve permitir marcar pago apenas quando pendente",
			args: args{status: valueobjects.TokenStatusPending},
			expect: func(service services.TransitionService) {
				s.True(service.CanMarkPaid(valueobjects.TokenStatusPending))
				s.False(service.CanMarkPaid(valueobjects.TokenStatusPaid))
				s.False(service.CanMarkPaid(valueobjects.TokenStatusConsumed))
				s.False(service.CanMarkPaid(valueobjects.TokenStatusExpired))
			},
		},
		{
			name: "deve permitir consumir apenas quando pago",
			args: args{status: valueobjects.TokenStatusPaid},
			expect: func(service services.TransitionService) {
				s.True(service.CanConsume(valueobjects.TokenStatusPaid))
				s.False(service.CanConsume(valueobjects.TokenStatusPending))
				s.False(service.CanConsume(valueobjects.TokenStatusConsumed))
				s.False(service.CanConsume(valueobjects.TokenStatusExpired))
			},
		},
		{
			name: "deve permitir outreach apenas quando pago",
			args: args{status: valueobjects.TokenStatusPaid},
			expect: func(service services.TransitionService) {
				s.True(service.CanMarkOutreach(valueobjects.TokenStatusPaid))
				s.False(service.CanMarkOutreach(valueobjects.TokenStatusPending))
				s.False(service.CanMarkOutreach(valueobjects.TokenStatusConsumed))
				s.False(service.CanMarkOutreach(valueobjects.TokenStatusExpired))
			},
		},
		{
			name: "deve permitir expirar somente pendente e pago",
			args: args{status: valueobjects.TokenStatusPending},
			expect: func(service services.TransitionService) {
				s.True(service.CanExpire(valueobjects.TokenStatusPending))
				s.True(service.CanExpire(valueobjects.TokenStatusPaid))
				s.False(service.CanExpire(valueobjects.TokenStatusConsumed))
				s.False(service.CanExpire(valueobjects.TokenStatusExpired))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			service := services.NewTransitionService()
			scenario.expect(service)
		})
	}
}

func (s *TransitionSuite) TestValidateConsume() {
	type args struct {
		status valueobjects.TokenStatus
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(error)
	}{
		{
			name: "deve aceitar token pago",
			args: args{status: valueobjects.TokenStatusPaid},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando token estiver pendente",
			args: args{status: valueobjects.TokenStatusPending},
			expect: func(err error) {
				s.ErrorIs(err, domain.ErrTokenNotYetPaid)
			},
		},
		{
			name: "deve retornar erro quando token estiver expirado",
			args: args{status: valueobjects.TokenStatusExpired},
			expect: func(err error) {
				s.ErrorIs(err, domain.ErrTokenExpired)
			},
		},
		{
			name: "deve retornar erro quando token ja estiver consumido",
			args: args{status: valueobjects.TokenStatusConsumed},
			expect: func(err error) {
				s.ErrorIs(err, domain.ErrTokenAlreadyConsumedSame)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			service := services.NewTransitionService()
			scenario.expect(service.ValidateConsume(scenario.args.status))
		})
	}
}

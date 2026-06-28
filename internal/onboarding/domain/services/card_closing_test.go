package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
)

type CardClosingSuite struct {
	suite.Suite
}

func TestCardClosingSuite(t *testing.T) {
	suite.Run(t, new(CardClosingSuite))
}

func (s *CardClosingSuite) TestDeriveClosingDay() {
	type args struct {
		dueDay     int
		offsetDays int
	}

	scenarios := []struct {
		name   string
		args   args
		expect int
	}{
		{name: "offset padrao 10", args: args{dueDay: 15, offsetDays: 10}, expect: 5},
		{name: "sem offset", args: args{dueDay: 15, offsetDays: 0}, expect: 15},
		{name: "wrap para dia anterior no mes anterior", args: args{dueDay: 5, offsetDays: 10}, expect: 26},
		{name: "wrap no limite 31", args: args{dueDay: 1, offsetDays: 1}, expect: 31},
		{name: "offset grande", args: args{dueDay: 10, offsetDays: 40}, expect: 1},
		{name: "offset negativo", args: args{dueDay: 10, offsetDays: -5}, expect: 15},
		{name: "dia 31 com offset", args: args{dueDay: 31, offsetDays: 10}, expect: 21},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			got := services.DeriveClosingDay(scenario.args.dueDay, scenario.args.offsetDays)
			s.Equal(scenario.expect, got)
		})
	}
}

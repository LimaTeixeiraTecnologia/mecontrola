package checkout_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/checkout"
)

type KiwifyURLBuilderSuite struct {
	suite.Suite
}

func TestKiwifyURLBuilderSuite(t *testing.T) {
	suite.Run(t, new(KiwifyURLBuilderSuite))
}

func (s *KiwifyURLBuilderSuite) SetupTest() {}

func (s *KiwifyURLBuilderSuite) TestBuild() {
	type args struct {
		plans        map[string]string
		allowedHosts []string
		planID       string
		token        string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(string, error)
	}{
		{
			name: "deve adicionar sck na url",
			args: args{
				plans:        map[string]string{"plan-monthly": "https://pay.kiwify.com.br/abc123"},
				allowedHosts: []string{"pay.kiwify.com.br"},
				planID:       "plan-monthly",
				token:        "mytoken",
			},
			expect: func(url string, err error) {
				s.Require().NoError(err)
				s.Equal("https://pay.kiwify.com.br/abc123?sck=mytoken", url)
			},
		},
		{
			name: "deve preservar query existente",
			args: args{
				plans:        map[string]string{"plan-a": "https://pay.kiwify.com.br/abc?utm_source=landing"},
				allowedHosts: []string{"pay.kiwify.com.br"},
				planID:       "plan-a",
				token:        "tok42",
			},
			expect: func(url string, err error) {
				s.Require().NoError(err)
				s.Contains(url, "sck=tok42")
				s.Contains(url, "utm_source=landing")
			},
		},
		{
			name: "deve retornar erro para plano desconhecido",
			args: args{
				plans:        map[string]string{"plan-x": "https://pay.kiwify.com.br/xyz"},
				allowedHosts: []string{"pay.kiwify.com.br"},
				planID:       "no-such-plan",
				token:        "tok",
			},
			expect: func(url string, err error) {
				s.Empty(url)
				s.ErrorIs(err, application.ErrUnknownPlan)
			},
		},
		{
			name: "deve retornar erro para host nao permitido",
			args: args{
				plans:        map[string]string{"plan-bad": "https://evil.example.com/pay"},
				allowedHosts: []string{"pay.kiwify.com.br"},
				planID:       "plan-bad",
				token:        "tok",
			},
			expect: func(url string, err error) {
				s.Empty(url)
				s.ErrorIs(err, application.ErrCheckoutUnavailable)
			},
		},
		{
			name: "deve retornar erro para url invalida",
			args: args{
				plans:        map[string]string{"plan-inv": "://bad-url"},
				allowedHosts: []string{"pay.kiwify.com.br"},
				planID:       "plan-inv",
				token:        "tok",
			},
			expect: func(url string, err error) {
				s.Empty(url)
				s.True(errors.Is(err, application.ErrCheckoutUnavailable))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			builder := checkout.NewKiwifyURLBuilder(scenario.args.plans, scenario.args.allowedHosts)
			url, err := builder.Build(context.Background(), scenario.args.planID, scenario.args.token)
			scenario.expect(url, err)
		})
	}
}

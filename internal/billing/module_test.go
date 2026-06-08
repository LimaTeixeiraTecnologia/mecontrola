package billing_test

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
	onboardingmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases/mocks"
)

type ModuleSuite struct {
	suite.Suite
}

func TestModule(t *testing.T) {
	suite.Run(t, new(ModuleSuite))
}

func (s *ModuleSuite) SetupTest() {}

func (s *ModuleSuite) TestNewBillingModule() {
	type args struct {
		cfg *configs.Config
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(billing.BillingModule, error)
	}{
		{
			name: "deve expor handlers de notificacao para registro no worker",
			args: args{
				cfg: &configs.Config{},
			},
			expect: func(module billing.BillingModule, err error) {
				s.Require().NoError(err)
				s.Require().Len(module.EventHandlers, 3)
				s.Equal("billing.subscription.past_due", module.EventHandlers[0].EventType)
				s.Equal("billing.subscription.refunded", module.EventHandlers[1].EventType)
				s.Equal("billing.subscription.expired_after_grace", module.EventHandlers[2].EventType)
				for _, registration := range module.EventHandlers {
					s.NotNil(registration.Handler)
				}
			},
		},
		{
			name: "deve falhar no bootstrap com configuracao invalida do cliente da kiwify",
			args: args{
				cfg: &configs.Config{
					KiwifyConfig: configs.KiwifyConfig{APIBaseURL: "://invalid"},
				},
			},
			expect: func(module billing.BillingModule, err error) {
				s.Error(err)
				s.Empty(module.EventHandlers)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			module, err := billing.NewBillingModule(scenario.args.cfg, noop.NewProvider(), onboardingmocks.NewFakeManager())
			scenario.expect(module, err)
		})
	}
}

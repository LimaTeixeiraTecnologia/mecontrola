package billing_test

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
)

type fakeManager struct{}

func (m *fakeManager) Driver() database.Driver            { return "" }
func (m *fakeManager) DBTX(context.Context) database.DBTX { return nil }
func (m *fakeManager) BeginTx(context.Context, database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (m *fakeManager) Ping(context.Context) error     { return nil }
func (m *fakeManager) Shutdown(context.Context) error { return nil }

type ModuleSuite struct {
	suite.Suite
}

func TestModule(t *testing.T) {
	suite.Run(t, new(ModuleSuite))
}

func (s *ModuleSuite) TestNotificationHandlersAreExposedForWorkerRegistration() {
	module, err := billing.NewBillingModule(&configs.Config{}, noop.NewProvider(), &fakeManager{})

	s.Require().NoError(err)
	s.Require().Len(module.EventHandlers, 3)
	s.Equal("billing.subscription.past_due", module.EventHandlers[0].EventType)
	s.Equal("billing.subscription.refunded", module.EventHandlers[1].EventType)
	s.Equal("billing.subscription.expired_after_grace", module.EventHandlers[2].EventType)
	for _, registration := range module.EventHandlers {
		s.NotNil(registration.Handler)
	}
}

func (s *ModuleSuite) TestInvalidKiwifyClientConfigFailsBootstrap() {
	module, err := billing.NewBillingModule(&configs.Config{
		KiwifyConfig: configs.KiwifyConfig{APIBaseURL: "://invalid"},
	}, noop.NewProvider(), &fakeManager{})

	s.Error(err)
	s.Empty(module.EventHandlers)
}

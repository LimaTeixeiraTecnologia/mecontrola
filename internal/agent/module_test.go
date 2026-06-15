package agent_test

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

type stubWhatsAppGateway struct{}

func (s *stubWhatsAppGateway) SendTextMessage(_ context.Context, _, _ string) error {
	return nil
}

func TestNewAgentModule_StubModeBuildsRoutes(t *testing.T) {
	module, err := agent.NewAgentModule(
		&configs.Config{},
		noop.NewProvider(),
		identity.IdentityModule{},
		nil,
		card.CardModule{},
		transactions.TransactionsModule{},
		nil,
		&stubWhatsAppGateway{},
	)

	assert.NoError(t, err)
	assert.Equal(t, agent.ModeStub, module.Mode)
	assert.NotNil(t, module.WhatsAppAgentRoute)
	assert.Nil(t, module.TelegramAgentRoute)
}

func TestNewAgentModule_RequiresWhatsAppGateway(t *testing.T) {
	module, err := agent.NewAgentModule(
		&configs.Config{},
		noop.NewProvider(),
		identity.IdentityModule{},
		nil,
		card.CardModule{},
		transactions.TransactionsModule{},
		nil,
		nil,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "whatsapp gateway is nil")
	assert.Equal(t, agent.AgentModule{}, module)
}

package e2e_test

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	budgetuc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	carduc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	catuc "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

type stubWhatsAppGateway struct{}

func (s *stubWhatsAppGateway) SendTextMessage(_ context.Context, _, _ string) error {
	return nil
}

func TestNewAgentModule_RequiresOpenRouterConfig(t *testing.T) {
	module, err := agent.NewAgentModule(
		&configs.Config{},
		noop.NewProvider(),
		identity.IdentityModule{},
		nil,
		card.CardModule{},
		transactions.TransactionsModule{},
		nil,
		&stubWhatsAppGateway{},
		nil,
	)

	assert.Error(t, err)
	assert.Equal(t, agent.AgentModule{}, module)
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
		nil,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "whatsapp gateway is nil")
	assert.Equal(t, agent.AgentModule{}, module)
}

func TestNewAgentModule_FailsWhenTransactionsDisabled(t *testing.T) {
	_, err := agent.NewAgentModule(
		&configs.Config{},
		noop.NewProvider(),
		identity.IdentityModule{},
		&categories.CategoriesModule{
			ListCategoriesUC:   new(catuc.ListCategories),
			GetCategoryUC:      new(catuc.GetCategory),
			ListDictionaryUC:   new(catuc.ListDictionary),
			SearchDictionaryUC: new(catuc.SearchDictionary),
		},
		card.CardModule{ListCardsUC: new(carduc.ListCards), CreateCardUC: new(carduc.CreateCard)},
		transactions.TransactionsModule{},
		&budgets.BudgetsModule{ListAlertsUC: new(budgetuc.ListAlerts)},
		&stubWhatsAppGateway{},
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transactions")
}

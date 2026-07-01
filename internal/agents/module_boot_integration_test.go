//go:build integration

package agents_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

type bootStubPublisher struct{}

func (p *bootStubPublisher) Publish(_ context.Context, _ outbox.Event) error { return nil }

type bootStubWhatsAppGateway struct{}

func (g *bootStubWhatsAppGateway) SendTextMessage(_ context.Context, _, _ string) error { return nil }

type ModuleBootSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
	cfg *configs.Config
}

func TestModuleBootSuite(t *testing.T) {
	suite.Run(t, new(ModuleBootSuite))
}

func (s *ModuleBootSuite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	s.db = db

	cfg, err := configs.LoadConfig("../..")
	s.Require().NoError(err, "carregar config")
	cfg.TransactionsConfig.Enabled = true
	s.cfg = cfg
}

func (s *ModuleBootSuite) buildDeps() agents.Deps {
	o11y := fake.NewProvider()
	passthrough := func(next http.Handler) http.Handler { return next }

	categoriesModule := categories.NewCategoriesModule(s.db, o11y, passthrough)
	s.Require().NotNil(categoriesModule)

	cardModule, err := card.NewCardModule(s.ctx, s.cfg, o11y, s.db, passthrough, nil, nil)
	s.Require().NoError(err, "card module")

	budgetsModule, err := budgets.NewBudgetsModule(s.cfg, o11y, s.db, categoriesModule, passthrough, nil, nil)
	s.Require().NoError(err, "budgets module")

	txModule, err := transactions.NewTransactionsModule(s.cfg, o11y, s.db, cardModule, categoriesModule, passthrough)
	s.Require().NoError(err, "transactions module")

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		apiKey = "boot-test-placeholder-key"
	}
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}

	return agents.Deps{
		DB:              s.db,
		O11y:            o11y,
		OutboxPublisher: &bootStubPublisher{},
		LLM: agents.LLMConfig{
			Model:       "openai/gpt-4o-mini",
			EmbedModel:  "openai/text-embedding-3-small",
			APIKey:      apiKey,
			BaseURL:     baseURL,
			MaxTokens:   512,
			Temperature: 0,
		},
		CategoriesModule:   categoriesModule,
		CardModule:         cardModule,
		BudgetsModule:      budgetsModule,
		TransactionsModule: txModule,
		WhatsAppGateway:    &bootStubWhatsAppGateway{},
	}
}

func (s *ModuleBootSuite) TestBootCompositionRoot() {
	module, err := agents.NewModule(s.buildDeps())
	s.Require().NoError(err, "agents.NewModule deve subir com deps completos e testcontainer real")

	s.Require().NotNil(module.HandleInbound, "HandleInbound deve estar wired")
	s.Require().NotNil(module.WhatsAppAgentRoute, "rota de inbound do WhatsApp deve estar wired")
	s.Require().NotEmpty(module.Jobs, "jobs do módulo (retenção do ledger) devem estar registrados")

	var hasInboundConsumer bool
	for _, h := range module.EventHandlers {
		if h.EventType == agents.EventTypeWhatsAppInbound {
			hasInboundConsumer = true
		}
	}
	s.Require().True(hasInboundConsumer,
		"consumer de inbound do WhatsApp (%s) deve estar registrado", agents.EventTypeWhatsAppInbound)

	module.Shutdown(s.ctx)
}

func (s *ModuleBootSuite) TestBootFailsWithoutLLMKey() {
	deps := s.buildDeps()
	deps.LLM.APIKey = ""
	_, err := agents.NewModule(deps)
	s.Require().Error(err, "NewModule deve falhar sem api key de LLM")
}

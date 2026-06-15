package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/commands"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/dispatcher"
	agentevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/loader"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	tgdispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/outbound"
	tgpayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/payload"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

const (
	ModeStub       = "stub"
	ModeOpenRouter = "openrouter"
)

var ErrAPIKeyRequired = errors.New("agent.llm: OPENROUTER_API_KEY is required when AGENT_MODE=openrouter")

type AgentModule struct {
	Mode               string
	WhatsAppAgentRoute func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	TelegramAgentRoute tgdispatcher.AgentRoute
}

type llmRuntime struct {
	Handler *usecases.HandleInboundMessage
	Mode    string
}

type llmReply struct {
	Text    string
	Outcome string
	Module  string
	Action  string
}

type llmRuntimeDeps struct {
	Ports          interfaces.ModulePorts
	Loader         interfaces.PromptContextLoader
	Dispatcher     interfaces.IntentDispatcher
	EventPublisher interfaces.IntentEventPublisher
}

type agentModuleBuilder struct {
	cfg                *configs.Config
	o11y               observability.Observability
	identityModule     identity.IdentityModule
	categoriesModule   *categories.CategoriesModule
	cardModule         card.CardModule
	transactionsModule transactions.TransactionsModule
	budgetsModule      *budgets.BudgetsModule
	whatsAppGateway    whatsAppGateway
}

func NewAgentModule(
	cfg *configs.Config,
	o11y observability.Observability,
	identityModule identity.IdentityModule,
	categoriesModule *categories.CategoriesModule,
	cardModule card.CardModule,
	transactionsModule transactions.TransactionsModule,
	budgetsModule *budgets.BudgetsModule,
	whatsAppGateway whatsAppGateway,
) (AgentModule, error) {
	builder := &agentModuleBuilder{
		cfg:                cfg,
		o11y:               o11y,
		identityModule:     identityModule,
		categoriesModule:   categoriesModule,
		cardModule:         cardModule,
		transactionsModule: transactionsModule,
		budgetsModule:      budgetsModule,
		whatsAppGateway:    whatsAppGateway,
	}
	return builder.build()
}

func (b *agentModuleBuilder) build() (AgentModule, error) {
	if b.cfg == nil {
		return AgentModule{}, fmt.Errorf("agent.module: config is nil")
	}
	if b.whatsAppGateway == nil {
		return AgentModule{}, fmt.Errorf("agent.module: whatsapp gateway is nil")
	}

	llmModule, err := b.buildLLMModule()
	if err != nil {
		return AgentModule{}, err
	}

	return AgentModule{
		Mode:               llmModule.Mode,
		WhatsAppAgentRoute: b.buildWhatsAppAgentRoute(llmModule),
		TelegramAgentRoute: b.buildTelegramAgentRoute(llmModule),
	}, nil
}

func (b *agentModuleBuilder) buildLLMModule() (*llmRuntime, error) {
	if b.cfg.AgentConfig.Mode == "" || b.cfg.AgentConfig.Mode == ModeStub {
		return newLLMRuntime(b.cfg.AgentConfig, b.o11y, llmRuntimeDeps{})
	}
	if b.categoriesModule == nil || b.categoriesModule.ListCategoriesUC == nil {
		return nil, fmt.Errorf("agent.module: categories module is incomplete")
	}
	if b.cardModule.ListCardsUC == nil {
		return nil, fmt.Errorf("agent.module: card list use case is nil")
	}
	if b.cardModule.CreateCardUC == nil {
		return nil, fmt.Errorf("agent.module: card create use case is nil")
	}

	categoriesAdapter := dispatcher.NewCategoriesAdapter(b.categoriesModule.ListCategoriesUC)
	cardsAdapter := dispatcher.NewCardsAdapterFull(b.cardModule.ListCardsUC, b.cardModule.CreateCardUC)
	ports := interfaces.ModulePorts{
		Categories:  categoriesAdapter,
		Cards:       cardsAdapter,
		CardsCreate: cardsAdapter,
	}
	if b.budgetsModule != nil && b.budgetsModule.ListAlertsUC != nil {
		ports.Budgets = dispatcher.NewBudgetsAdapter(b.budgetsModule.ListAlertsUC)
	}
	if b.transactionsModule.ListTransactionsUC != nil {
		txAdapter := dispatcher.NewTransactionsAdapterFull(
			b.transactionsModule.ListTransactionsUC,
			b.transactionsModule.CreateTransactionUC,
			b.transactionsModule.DeleteTransactionUC,
			b.transactionsModule.GetTransactionUC,
		)
		ports.Transactions = txAdapter
		ports.TransactionsCreate = txAdapter
		ports.TransactionsDelete = txAdapter
	}

	deps := llmRuntimeDeps{
		Ports: ports,
		Loader: loader.NewPromptContextLoader(
			b.categoriesModule.ListCategoriesUC,
			b.cardModule.ListCardsUC,
			b.o11y,
		),
		EventPublisher: agentevents.NewIntentEventPublisher(b.identityModule.OutboxPublisher, b.o11y),
	}

	llmModule, err := newLLMRuntime(b.cfg.AgentConfig, b.o11y, deps)
	if err != nil {
		return nil, fmt.Errorf("agent.module: %w", err)
	}
	return llmModule, nil
}

func (b *agentModuleBuilder) buildWhatsAppAgentRoute(llmModule *llmRuntime) func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
	templates := map[string]string{
		"agent_stub_received": b.cfg.WhatsAppConfig.AgentStubReceived,
	}
	stubAgent := NewStubAgent(b.whatsAppGateway, templates, b.o11y)
	useLLM := llmModule != nil && llmModule.Mode == ModeOpenRouter && llmModule.Handler != nil

	if !useLLM {
		return func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
			if err := stubAgent.HandleMessage(ctx, msg); err != nil {
				b.o11y.Logger().Warn(ctx, "whatsapp.dispatcher.agent_route_failed",
					observability.Error(err),
				)
			}
			return wadispatcher.OutcomeAgent
		}
	}

	return func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
		principal, ok := auth.FromContext(ctx)
		if !ok {
			b.o11y.Logger().Warn(ctx, "whatsapp.dispatcher.agent_route_missing_principal")
			return wadispatcher.OutcomeAgent
		}
		reply, err := llmModule.HandleText(ctx, principal.UserID, string(auth.SourceWhatsApp), msg.Text)
		if err != nil {
			b.o11y.Logger().Warn(ctx, "whatsapp.dispatcher.llm_agent_failed",
				observability.Error(err),
			)
			return wadispatcher.OutcomeAgent
		}
		if reply.Text == "" {
			return wadispatcher.OutcomeAgent
		}
		if sendErr := b.whatsAppGateway.SendTextMessage(ctx, msg.From, reply.Text); sendErr != nil {
			b.o11y.Logger().Warn(ctx, "whatsapp.dispatcher.llm_agent_reply_failed",
				observability.Error(sendErr),
			)
		}
		return wadispatcher.OutcomeAgent
	}
}

func (b *agentModuleBuilder) buildTelegramAgentRoute(llmModule *llmRuntime) tgdispatcher.AgentRoute {
	if llmModule == nil || llmModule.Mode != ModeOpenRouter || llmModule.Handler == nil {
		return nil
	}

	gateway, err := outbound.NewSharedGateway(b.o11y, outbound.FactoryConfig{
		APIBaseURL: b.cfg.TelegramConfig.APIBaseURL,
		BotToken:   b.cfg.TelegramConfig.BotToken,
		Timeout:    b.cfg.TelegramConfig.OutboundTimeout,
	})
	if err != nil {
		b.o11y.Logger().Warn(context.Background(), "agent.module.telegram_agent_gateway_failed",
			observability.Error(err),
		)
		return nil
	}

	return func(ctx context.Context, msg tgpayload.Message) tgdispatcher.RouteOutcome {
		principal, ok := auth.FromContext(ctx)
		if !ok {
			b.o11y.Logger().Warn(ctx, "telegram.dispatcher.agent_route_missing_principal")
			return tgdispatcher.OutcomeAgent
		}
		reply, err := llmModule.HandleText(ctx, principal.UserID, string(auth.SourceTelegram), msg.Text)
		if err != nil {
			b.o11y.Logger().Warn(ctx, "telegram.dispatcher.llm_agent_failed",
				observability.Error(err),
			)
			return tgdispatcher.OutcomeAgent
		}
		if reply.Text == "" {
			return tgdispatcher.OutcomeAgent
		}
		if sendErr := gateway.SendTextMessage(ctx, msg.ChatID, reply.Text); sendErr != nil {
			b.o11y.Logger().Warn(ctx, "telegram.dispatcher.llm_agent_reply_failed",
				observability.Error(sendErr),
			)
		}
		return tgdispatcher.OutcomeAgent
	}
}

type emptyContextLoader struct{}

func (emptyContextLoader) Load(_ context.Context, _ uuid.UUID, _ string) (interfaces.PromptSeed, error) {
	return interfaces.PromptSeed{Permissions: []string{"read", "write"}}, nil
}

func newLLMRuntime(cfg configs.AgentConfig, o11y observability.Observability, deps llmRuntimeDeps) (*llmRuntime, error) {
	if cfg.Mode == "" || cfg.Mode == ModeStub {
		return &llmRuntime{Mode: ModeStub}, nil
	}
	dispatcherImpl := deps.Dispatcher
	loaderImpl := deps.Loader
	if cfg.Mode != ModeOpenRouter {
		return nil, fmt.Errorf("agent.llm: unknown mode %q (allowed: stub|openrouter)", cfg.Mode)
	}
	if strings.TrimSpace(cfg.OpenRouterAPIKey) == "" {
		return nil, ErrAPIKeyRequired
	}

	client, err := httpclient.NewClient(o11y,
		httpclient.WithBaseURL(cfg.OpenRouterBaseURL),
		httpclient.WithTarget("openrouter"),
		httpclient.WithTimeout(cfg.RequestTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: http client: %w", err)
	}

	primary, err := valueobjects.NewModelSlug(cfg.PrimaryModel)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: primary model: %w", err)
	}
	providers := []interfaces.LLMProvider{
		openrouter.NewProvider(client, openrouter.ProviderConfig{
			Slug:           primary,
			APIKey:         cfg.OpenRouterAPIKey,
			HTTPReferer:    cfg.HTTPReferer,
			XTitle:         cfg.XTitle,
			MaxTokens:      cfg.MaxTokens,
			Temperature:    cfg.Temperature,
			RequestTimeout: cfg.RequestTimeout,
		}, o11y),
	}
	for _, raw := range parseFallbackList(cfg.FallbackModels) {
		slug, slugErr := valueobjects.NewModelSlug(raw)
		if slugErr != nil {
			return nil, fmt.Errorf("agent.llm: fallback model %q: %w", raw, slugErr)
		}
		providers = append(providers, openrouter.NewProvider(client, openrouter.ProviderConfig{
			Slug:           slug,
			APIKey:         cfg.OpenRouterAPIKey,
			HTTPReferer:    cfg.HTTPReferer,
			XTitle:         cfg.XTitle,
			MaxTokens:      cfg.MaxTokens,
			Temperature:    cfg.Temperature,
			RequestTimeout: cfg.RequestTimeout,
		}, o11y))
	}

	breaker := appservices.NewCircuitBreaker(appservices.CircuitBreakerConfig{
		MaxFailures:   cfg.CircuitFailures,
		FailureWindow: cfg.CircuitWindow,
		OpenDuration:  cfg.CircuitCooldown,
	})
	chain, err := appservices.NewFallbackChain(providers, breaker, o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: fallback chain: %w", err)
	}

	if loaderImpl == nil {
		loaderImpl = emptyContextLoader{}
	}
	if dispatcherImpl == nil {
		dispatcherImpl = dispatcher.NewIntentDispatcher(deps.Ports, o11y)
	}

	handler := usecases.NewHandleInboundMessage(
		loaderImpl,
		chain,
		dispatcherImpl,
		deps.EventPublisher,
		appservices.NewPromptBuilder(),
		appservices.NewIntentValidator(),
		domainservices.NewIntentWorkflow(),
		o11y,
	)

	return &llmRuntime{Handler: handler, Mode: ModeOpenRouter}, nil
}

func (m *llmRuntime) HandleText(ctx context.Context, userID uuid.UUID, channel, text string) (llmReply, error) {
	if m == nil || m.Handler == nil {
		return llmReply{Text: "", Outcome: "stub"}, nil
	}
	result, err := m.Handler.Execute(ctx, commands.RawInterpretMessage{UserID: userID, Channel: channel, Text: text})
	if err != nil {
		return llmReply{}, err
	}
	reply := llmReply{Text: result.ReplyText, Outcome: result.Outcome.Kind.String()}
	if !result.Outcome.Intent.IsError() && result.Outcome.Kind == domainservices.IntentOutcomeRouted {
		reply.Module = result.Outcome.Intent.Module().String()
		reply.Action = result.Outcome.Intent.Action().String()
	}
	return reply, nil
}

func parseFallbackList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/commands"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/dispatcher"
	agentevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/loader"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	agentrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	tgdispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/outbound"
	tgpayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/payload"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

var ErrAPIKeyRequired = errors.New("agent.llm: OPENROUTER_API_KEY is required")

type whatsAppGateway interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type AgentModule struct {
	WhatsAppAgentRoute    func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	TelegramAgentRoute    tgdispatcher.AgentRoute
	ParseInbound          *usecases.ParseInbound
	IntentRouter          *appservices.IntentRouter
	SessionUnitOfWork     uow.UnitOfWork
	SessionRepository     interfaces.AgentSessionRepository
	SessionRepositoryFact interfaces.AgentSessionRepositoryFactory
}

type AgentModuleOption func(*agentModuleBuilder)

func WithSessionStore(db *sqlx.DB) AgentModuleOption {
	return func(b *agentModuleBuilder) {
		b.sessionDB = db
	}
}

type llmRuntime struct {
	Handler        *usecases.HandleInboundMessage
	ParseInbound   *usecases.ParseInbound
	Conversational *usecases.ComposeConversationalReply
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
	budgetConfigurator appservices.BudgetConfigurator
	onboarding         appservices.OnboardingContinuation
	sessionDB          *sqlx.DB
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
	budgetConfigurator appservices.BudgetConfigurator,
	onboarding appservices.OnboardingContinuation,
	opts ...AgentModuleOption,
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
		budgetConfigurator: budgetConfigurator,
		onboarding:         onboarding,
	}
	for _, opt := range opts {
		opt(builder)
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

	intentRouter, err := b.buildIntentRouter(llmModule)
	if err != nil {
		return AgentModule{}, err
	}

	module := AgentModule{
		WhatsAppAgentRoute: b.buildWhatsAppAgentRoute(intentRouter),
		TelegramAgentRoute: b.buildTelegramAgentRoute(intentRouter),
		ParseInbound:       llmModule.ParseInbound,
		IntentRouter:       intentRouter,
	}
	b.attachSessionStore(&module)
	return module, nil
}

func (b *agentModuleBuilder) attachSessionStore(module *AgentModule) {
	if b.sessionDB == nil {
		return
	}
	factory := agentrepo.NewRepositoryFactory(b.o11y)
	module.SessionRepositoryFact = factory
	module.SessionRepository = factory.AgentSessionRepository(b.sessionDB)
	module.SessionUnitOfWork = uow.NewUnitOfWork(b.sessionDB)
}

func (b *agentModuleBuilder) buildLLMModule() (*llmRuntime, error) {
	if b.categoriesModule == nil || b.categoriesModule.ListCategoriesUC == nil {
		return nil, fmt.Errorf("agent.module: categories module is incomplete")
	}
	if b.cardModule.ListCardsUC == nil {
		return nil, fmt.Errorf("agent.module: card list use case is nil")
	}
	if b.cardModule.CreateCardUC == nil {
		return nil, fmt.Errorf("agent.module: card create use case is nil")
	}

	categoriesAdapter := dispatcher.NewCategoriesAdapterFull(
		b.categoriesModule.ListCategoriesUC,
		b.categoriesModule.GetCategoryUC,
		b.categoriesModule.ListDictionaryUC,
		b.categoriesModule.SearchDictionaryUC,
	)
	cardsAdapter := dispatcher.NewCardsAdapterFull(
		b.cardModule.ListCardsUC,
		b.cardModule.GetCardUC,
		b.cardModule.CreateCardUC,
		b.cardModule.UpdateCardUC,
		b.cardModule.UpdateCardLimitUC,
		b.cardModule.SoftDeleteCardUC,
	)
	ports := interfaces.ModulePorts{
		Categories:               categoriesAdapter,
		CategoriesGet:            categoriesAdapter,
		CategoriesListDictionary: categoriesAdapter,
		CategoriesSearch:         categoriesAdapter,
		Cards:                    cardsAdapter,
		CardsGet:                 cardsAdapter,
		CardsCreate:              cardsAdapter,
		CardsUpdate:              cardsAdapter,
		CardsDelete:              cardsAdapter,
	}
	if b.budgetsModule == nil || b.budgetsModule.ListAlertsUC == nil {
		return nil, fmt.Errorf("agent.module: budgets module is incomplete")
	}
	budgetsAdapter := dispatcher.NewBudgetsAdapterFull(
		b.budgetsModule.ListAlertsUC,
		b.budgetsModule.GetMonthlySummaryUC,
		b.budgetsModule.CreateBudgetUC,
		b.budgetsModule.ActivateBudgetUC,
		b.budgetsModule.CreateRecurrenceUC,
		b.budgetsModule.UpsertExpenseUC,
		b.budgetsModule.DeleteDraftBudgetUC,
		b.budgetsModule.DeleteExpenseUC,
	)
	ports.Budgets = budgetsAdapter
	ports.BudgetsGet = budgetsAdapter
	ports.BudgetsCreate = budgetsAdapter
	ports.BudgetsUpdate = budgetsAdapter
	ports.BudgetsDelete = budgetsAdapter

	if b.transactionsModule.ListTransactionsUC == nil || b.transactionsModule.CreateTransactionUC == nil {
		return nil, fmt.Errorf("agent.module: transactions desabilitado; defina TRANSACTIONS_ENABLED=true")
	}
	txAdapter := dispatcher.NewTransactionsAdapterFull(
		b.transactionsModule.ListTransactionsUC,
		b.transactionsModule.CreateTransactionUC,
		b.transactionsModule.DeleteTransactionUC,
		b.transactionsModule.GetTransactionUC,
		b.transactionsModule.CreateCardPurchaseUC,
		b.transactionsModule.CreateRecurringTemplateUC,
		b.transactionsModule.ListRecurringTemplatesUC,
	)
	ports.Transactions = txAdapter
	ports.TransactionsGet = txAdapter
	ports.TransactionsCreate = txAdapter
	ports.TransactionsDelete = txAdapter
	ports.CardPurchasesCreate = txAdapter
	ports.RecurringCreate = txAdapter
	ports.RecurringList = txAdapter

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

func (b *agentModuleBuilder) buildIntentRouter(llmModule *llmRuntime) (*appservices.IntentRouter, error) {
	useLLM := llmModule != nil && llmModule.Handler != nil && llmModule.ParseInbound != nil
	if !useLLM {
		return nil, nil
	}

	deps := appservices.IntentRouterDeps{
		Parser:          &intentParserAdapter{uc: llmModule.ParseInbound},
		Fallback:        &fallbackAdapter{runtime: llmModule},
		WhatsAppGateway: b.whatsAppGateway,
	}
	if b.identityModule.OutboxPublisher != nil {
		deps.EventPublisher = agentevents.NewIntentEventPublisher(b.identityModule.OutboxPublisher, b.o11y)
	}
	b.fillIntentRouterDeps(&deps)
	b.attachExpenseLogger(&deps)
	b.attachCardPurchaseLogger(&deps)
	b.attachTransactionQueries(&deps)
	b.attachRecurring(&deps)
	deps.TelegramGateway = b.buildTelegramGateway()

	router, err := appservices.NewIntentRouter(b.o11y, deps)
	if err != nil {
		return nil, fmt.Errorf("agent.module: intent router: %w", err)
	}
	return router, nil
}

func (b *agentModuleBuilder) fillIntentRouterDeps(deps *appservices.IntentRouterDeps) {
	if b.budgetsModule != nil && b.budgetsModule.GetMonthlySummaryUC != nil {
		deps.MonthlySummary = b.budgetsModule.GetMonthlySummaryUC
	}
	if b.cardModule.ListCardsUC != nil {
		deps.CardLister = b.cardModule.ListCardsUC
	}
	if b.cardModule.InvoiceForUC != nil {
		deps.CardInvoice = b.cardModule.InvoiceForUC
	}
	if b.cardModule.CreateCardUC != nil {
		deps.CardCreator = agentbinding.NewCardCreatorAdapter(b.cardModule.CreateCardUC)
	}
	if b.cardModule.CountCardsUC != nil {
		deps.CardCounter = agentbinding.NewCardCounterAdapter(b.cardModule.CountCardsUC)
	}
	if b.budgetConfigurator != nil {
		deps.BudgetConfig = b.budgetConfigurator
	}
	if b.onboarding != nil {
		deps.Onboarding = b.onboarding
	}
}

func (b *agentModuleBuilder) attachExpenseLogger(deps *appservices.IntentRouterDeps) {
	if b.transactionsModule.CreateTransactionUC == nil {
		return
	}
	if b.categoriesModule == nil || b.categoriesModule.SearchDictionaryUC == nil {
		return
	}
	logTransaction := usecases.NewLogTransactionFromAgent(
		b.categoriesModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(b.transactionsModule.CreateTransactionUC),
		b.o11y,
	)
	deps.ExpenseLogger = agentbinding.NewTransactionLoggerAdapter(logTransaction)
}

func (b *agentModuleBuilder) attachCardPurchaseLogger(deps *appservices.IntentRouterDeps) {
	if b.transactionsModule.CreateCardPurchaseUC == nil {
		return
	}
	if b.cardModule.ListCardsUC == nil {
		return
	}
	if b.categoriesModule == nil || b.categoriesModule.SearchDictionaryUC == nil {
		return
	}
	logCardPurchase := usecases.NewLogCardPurchaseFromAgent(
		b.categoriesModule.SearchDictionaryUC,
		agentbinding.NewCardPurchaseCreatorAdapter(b.cardModule.ListCardsUC, b.transactionsModule.CreateCardPurchaseUC),
		b.o11y,
	)
	deps.CardPurchaseLog = agentbinding.NewCardPurchaseLoggerAdapter(logCardPurchase)
}

func (b *agentModuleBuilder) attachTransactionQueries(deps *appservices.IntentRouterDeps) {
	if b.transactionsModule.ListTransactionsUC == nil {
		return
	}
	deps.TransactionLister = agentbinding.NewTransactionListerAdapter(b.transactionsModule.ListTransactionsUC)
	if b.transactionsModule.DeleteTransactionUC != nil {
		deps.LastDeleter = agentbinding.NewLastTransactionDeleterAdapter(b.transactionsModule.DeleteTransactionUC)
	}
	if b.transactionsModule.GetTransactionUC != nil && b.transactionsModule.UpdateTransactionUC != nil {
		deps.LastEditor = agentbinding.NewLastTransactionEditorAdapter(
			b.transactionsModule.GetTransactionUC,
			b.transactionsModule.UpdateTransactionUC,
		)
	}
}

func (b *agentModuleBuilder) attachRecurring(deps *appservices.IntentRouterDeps) {
	if b.transactionsModule.CreateRecurringTemplateUC != nil &&
		b.categoriesModule != nil && b.categoriesModule.SearchDictionaryUC != nil {
		createRecurring := usecases.NewCreateRecurringFromAgent(
			b.categoriesModule.SearchDictionaryUC,
			agentbinding.NewRecurringTemplateCreatorAdapter(b.transactionsModule.CreateRecurringTemplateUC),
			b.o11y,
		)
		deps.RecurringCreator = agentbinding.NewRecurringCreatorAdapter(createRecurring)
	}
	if b.transactionsModule.ListRecurringTemplatesUC != nil {
		deps.RecurringLister = agentbinding.NewRecurringListerAdapter(b.transactionsModule.ListRecurringTemplatesUC)
	}
}

func (b *agentModuleBuilder) buildTelegramGateway() appservices.TelegramOutbound {
	if !b.cfg.TelegramConfig.Enabled {
		return nil
	}
	tgGateway, err := outbound.NewSharedGateway(b.o11y, outbound.FactoryConfig{
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
	return tgGateway
}

func (b *agentModuleBuilder) buildWhatsAppAgentRoute(router *appservices.IntentRouter) func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
	if router == nil {
		return func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
			return wadispatcher.OutcomeAgent
		}
	}

	return func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
		principal, ok := auth.FromContext(ctx)
		if !ok {
			b.o11y.Logger().Warn(ctx, "whatsapp.dispatcher.agent_route_missing_principal")
			return wadispatcher.OutcomeAgent
		}
		_ = router.RouteWhatsApp(ctx, appservices.Principal{UserID: principal.UserID}, appservices.InboundMessage{
			Text:       msg.Text,
			WhatsAppTo: msg.From,
			MessageID:  msg.WAMID,
		})
		return wadispatcher.OutcomeAgent
	}
}

func (b *agentModuleBuilder) buildTelegramAgentRoute(router *appservices.IntentRouter) tgdispatcher.AgentRoute {
	if router == nil {
		return nil
	}

	return func(ctx context.Context, msg tgpayload.Message) tgdispatcher.RouteOutcome {
		principal, ok := auth.FromContext(ctx)
		if !ok {
			b.o11y.Logger().Warn(ctx, "telegram.dispatcher.agent_route_missing_principal")
			return tgdispatcher.OutcomeAgent
		}
		_ = router.RouteTelegram(ctx, appservices.Principal{UserID: principal.UserID}, appservices.InboundMessage{
			Text:       msg.Text,
			TelegramTo: msg.ChatID,
			MessageID:  fmt.Sprintf("%d", msg.MessageID),
		})
		return tgdispatcher.OutcomeAgent
	}
}

type intentParserAdapter struct {
	uc *usecases.ParseInbound
}

func (a *intentParserAdapter) Parse(ctx context.Context, userID uuid.UUID, text string) (appservices.ParsedIntent, error) {
	out, err := a.uc.Execute(ctx, usecases.ParseInboundInput{UserID: userID, Text: text})
	if err != nil {
		return appservices.ParsedIntent{}, err
	}
	return appservices.ParsedIntent{Intent: out.Intent, Raw: out.Raw}, nil
}

type fallbackAdapter struct {
	runtime *llmRuntime
}

func (a *fallbackAdapter) Reply(ctx context.Context, userID uuid.UUID, channel, text string) (string, error) {
	if a.runtime == nil || a.runtime.Conversational == nil {
		return "", nil
	}
	out, err := a.runtime.Conversational.Execute(ctx, usecases.ComposeConversationalInput{UserID: userID, Channel: channel, Text: text})
	if err != nil {
		return "", err
	}
	return out.Reply, nil
}

type emptyContextLoader struct{}

func (emptyContextLoader) Load(_ context.Context, _ uuid.UUID, _ string) (interfaces.PromptSeed, error) {
	return interfaces.PromptSeed{Permissions: []string{"read", "write"}}, nil
}

func newLLMRuntime(cfg configs.AgentConfig, o11y observability.Observability, deps llmRuntimeDeps) (*llmRuntime, error) {
	dispatcherImpl := deps.Dispatcher
	loaderImpl := deps.Loader
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
		appservices.NewIntentSafetyGuard(),
		domainservices.NewIntentWorkflow(),
		o11y,
	)

	parseInbound, err := usecases.NewParseInbound(chain, o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: parse inbound: %w", err)
	}

	conversational, err := usecases.NewComposeConversationalReply(chain, cfg.ProseMaxTokens, o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: conversational reply: %w", err)
	}

	return &llmRuntime{Handler: handler, ParseInbound: parseInbound, Conversational: conversational}, nil
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

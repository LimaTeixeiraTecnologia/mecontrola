package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/sanitize"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	agentevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/events"
	agentconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/messaging/database/consumers"
	agentonboarding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/providers/openrouter"
	agentrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
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

type inboundPublisher interface {
	PublishWhatsApp(ctx context.Context, userID uuid.UUID, peer, text, messageID string) error
	PublishTelegram(ctx context.Context, userID uuid.UUID, chatID int64, text, messageID string) error
}

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type AgentModule struct {
	WhatsAppAgentRoute    func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	TelegramAgentRoute    tgdispatcher.AgentRoute
	ParseInbound          *usecases.ParseInbound
	IntentRouter          *appservices.IntentRouter
	SessionUnitOfWork     uow.UnitOfWork
	SessionRepository     interfaces.AgentSessionRepository
	SessionRepositoryFact interfaces.AgentSessionRepositoryFactory
	EventHandlers         []EventHandlerRegistration
}

type AgentModuleOption func(*agentModuleBuilder)

func WithSessionStore(db *sqlx.DB) AgentModuleOption {
	return func(b *agentModuleBuilder) {
		b.sessionDB = db
	}
}

func WithOutboxPublisher(pub outbox.Publisher) AgentModuleOption {
	return func(b *agentModuleBuilder) {
		b.outboxPublisher = pub
	}
}

type OnboardingLLMUseCases struct {
	GetContext       *onbusecases.GetOnboardingContext
	SaveObjective    *onbusecases.SaveOnboardingObjective
	SaveIncome       *onbusecases.SaveOnboardingIncome
	SaveCard         *onbusecases.SaveOnboardingCard
	SaveBudgetSplits *onbusecases.SaveOnboardingBudgetSplits
	MarkFirstTx      *onbusecases.MarkFirstTransactionRecorded
	Complete         *onbusecases.CompleteOnboardingSession
	SetPhase         *onbusecases.SetOnboardingPhase
}

func WithOnboardingLLM(uc OnboardingLLMUseCases) AgentModuleOption {
	return func(b *agentModuleBuilder) {
		b.onboardingLLM = &uc
	}
}

type llmRuntime struct {
	ParseInbound          *usecases.ParseInbound
	Conversational        *usecases.ComposeConversationalReply
	Interpreter           usecases.IntentInterpreter
	OnboardingInterpreter usecases.IntentInterpreter
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
	budgetConfigurator tools.BudgetConfigurator
	onboarding         appservices.OnboardingContinuation
	onboardingLLM      *OnboardingLLMUseCases
	sessionDB          *sqlx.DB
	sessionRepo        interfaces.AgentSessionRepository
	sessionRepoFact    interfaces.AgentSessionRepositoryFactory
	sessionUoW         uow.UnitOfWork
	decisionRepoFact   interfaces.AgentDecisionRepositoryFactory
	decisionUoW        uow.UnitOfWork
	threadRepoFact     interfaces.AgentThreadRepositoryFactory
	runRepoFact        interfaces.AgentRunRepositoryFactory
	runtimeUoW         uow.UnitOfWork
	wmRepo             interfaces.WorkingMemoryRepository
	obsRepo            interfaces.ObservationRepository
	outboxPublisher    outbox.Publisher
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
	budgetConfigurator tools.BudgetConfigurator,
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

	b.prepareSessionStore()

	llmModule, err := b.buildLLMModule()
	if err != nil {
		return AgentModule{}, err
	}

	intentRouter, err := b.buildIntentRouter(llmModule)
	if err != nil {
		return AgentModule{}, err
	}

	var pub inboundPublisher
	if b.outboxPublisher != nil {
		pub = agentevents.NewInboundEventPublisher(b.outboxPublisher, b.o11y)
	}

	module := AgentModule{
		WhatsAppAgentRoute:    b.buildWhatsAppAgentRoute(pub),
		TelegramAgentRoute:    b.buildTelegramAgentRoute(pub),
		ParseInbound:          llmModule.ParseInbound,
		IntentRouter:          intentRouter,
		SessionRepository:     b.sessionRepo,
		SessionRepositoryFact: b.sessionRepoFact,
		SessionUnitOfWork:     b.sessionUoW,
		EventHandlers:         b.buildEventHandlers(intentRouter),
	}
	return module, nil
}

func (b *agentModuleBuilder) buildEventHandlers(router *appservices.IntentRouter) []EventHandlerRegistration {
	if router == nil {
		return nil
	}
	return []EventHandlerRegistration{
		{
			EventType: agentevents.EventTypeWhatsAppInbound,
			Handler:   agentconsumers.NewWhatsAppInboundConsumer(router, b.o11y),
		},
		{
			EventType: agentevents.EventTypeTelegramInbound,
			Handler:   agentconsumers.NewTelegramInboundConsumer(router, b.o11y),
		},
	}
}

func (b *agentModuleBuilder) prepareSessionStore() {
	if b.sessionDB == nil {
		return
	}
	factory := agentrepo.NewRepositoryFactory(b.o11y)
	b.sessionRepoFact = factory
	b.sessionRepo = factory.AgentSessionRepository(b.sessionDB)
	b.sessionUoW = uow.NewUnitOfWork(b.sessionDB)
	b.decisionRepoFact = agentrepo.NewDecisionRepositoryFactory(b.o11y)
	b.decisionUoW = uow.NewUnitOfWork(b.sessionDB)
	b.threadRepoFact = agentrepo.NewThreadRepositoryFactory(b.o11y)
	b.runRepoFact = agentrepo.NewRunRepositoryFactory(b.o11y)
	b.runtimeUoW = uow.NewUnitOfWork(b.sessionDB)
	wmFactory := agentrepo.NewWorkingMemoryRepositoryFactory(b.o11y)
	b.wmRepo = wmFactory.WorkingMemoryRepository(b.sessionDB)
	obsFactory := agentrepo.NewObservationRepositoryFactory(b.o11y)
	b.obsRepo = obsFactory.ObservationRepository(b.sessionDB)
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

	if b.budgetsModule == nil || b.budgetsModule.ListAlertsUC == nil {
		return nil, fmt.Errorf("agent.module: budgets module is incomplete")
	}
	if b.transactionsModule.ListTransactionsUC == nil || b.transactionsModule.CreateTransactionUC == nil {
		return nil, fmt.Errorf("agent.module: transactions desabilitado; defina TRANSACTIONS_ENABLED=true")
	}

	llmModule, err := newLLMRuntime(b.cfg.AgentConfig, b.o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.module: %w", err)
	}
	if b.sessionRepo != nil && b.wmRepo != nil && b.obsRepo != nil {
		redactor, redactErr := sanitize.NewSanitizer(sanitize.DefaultMaxRunes)
		if redactErr == nil {
			obsSvc := appservices.NewObservationMemory(llmModule.Interpreter, b.obsRepo, b.o11y, 6, 3)
			conv, convErr := usecases.NewComposeConversationalReply(
				llmModule.Interpreter,
				b.cfg.AgentConfig.ProseMaxTokens,
				b.o11y,
				b.sessionRepo,
				b.wmRepo,
				obsSvc,
				redactor,
			)
			if convErr == nil {
				llmModule.Conversational = conv
			} else {
				b.o11y.Logger().Warn(context.Background(), "agent.module.conversational_rebuild_failed", observability.Error(convErr))
			}
		} else {
			b.o11y.Logger().Warn(context.Background(), "agent.module.sanitizer_create_failed", observability.Error(redactErr))
		}
	}
	return llmModule, nil
}

func (b *agentModuleBuilder) buildIntentRouter(llmModule *llmRuntime) (*appservices.IntentRouter, error) {
	useLLM := llmModule != nil && llmModule.ParseInbound != nil
	if !useLLM {
		return nil, nil
	}

	deps := appservices.IntentRouterDeps{
		Parser:              &intentParserAdapter{uc: llmModule.ParseInbound},
		Fallback:            &fallbackAdapter{runtime: llmModule},
		WhatsAppGateway:     b.whatsAppGateway,
		PolicyMinConfidence: b.cfg.AgentConfig.PolicyMinConfidence,
	}
	if b.outboxPublisher != nil {
		deps.EventPublisher = agentevents.NewIntentEventPublisher(b.outboxPublisher, b.o11y)
	}
	b.fillIntentRouterDeps(&deps)
	b.attachExpenseRecorder(&deps)
	b.attachCardPurchaseLogger(&deps)
	b.attachTransactionQueries(&deps)
	b.attachRecurring(&deps)
	b.attachBudgetConfigSession(&deps, llmModule)
	b.attachOnboardingLLM(&deps, llmModule)
	b.attachDecisionAudit(&deps)
	deps.TelegramGateway = b.buildTelegramGateway()

	router, err := appservices.NewIntentRouter(b.o11y, deps)
	if err != nil {
		return nil, fmt.Errorf("agent.module: intent router: %w", err)
	}
	b.attachRuntime(router)
	return router, nil
}

func (b *agentModuleBuilder) attachRuntime(router *appservices.IntentRouter) {
	if b.threadRepoFact == nil || b.runRepoFact == nil || b.runtimeUoW == nil {
		b.o11y.Logger().Warn(context.Background(), "agent.module.runtime",
			observability.String("mode", "legacy"),
			observability.String("reason", "session_store_missing"),
		)
		return
	}
	threads := agentbinding.NewThreadGatewayAdapter(b.threadRepoFact, b.runtimeUoW)
	runs := agentbinding.NewRunGatewayAdapter(b.runRepoFact, b.runtimeUoW)
	router.EnableRuntime(threads, runs)
	b.o11y.Logger().Info(context.Background(), "agent.module.runtime",
		observability.String("mode", "enabled"),
	)
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
	if b.cardModule.ListCardsUC != nil && b.cardModule.UpdateCardUC != nil {
		deps.CardUpdater = agentbinding.NewCardUpdaterAdapter(b.cardModule.ListCardsUC, b.cardModule.UpdateCardUC)
	}
	if b.cardModule.ListCardsUC != nil && b.cardModule.SoftDeleteCardUC != nil {
		deps.CardDeleter = agentbinding.NewCardDeleterAdapter(b.cardModule.ListCardsUC, b.cardModule.SoftDeleteCardUC)
	}
	if b.budgetsModule != nil && b.budgetsModule.EditCategoryPercentageUC != nil {
		deps.CategoryPercentageEditor = agentbinding.NewCategoryPercentageEditorAdapter(b.budgetsModule.EditCategoryPercentageUC)
	}
	if b.budgetConfigurator != nil {
		deps.BudgetConfig = b.budgetConfigurator
	}
	if b.onboarding != nil {
		deps.Onboarding = b.onboarding
	}
}

func (b *agentModuleBuilder) attachExpenseRecorder(deps *appservices.IntentRouterDeps) {
	if b.transactionsModule.CreateTransactionUC == nil {
		return
	}
	if b.categoriesModule == nil || b.categoriesModule.SearchDictionaryUC == nil {
		return
	}
	logTransaction := usecases.NewRecordTransactionFromAgent(
		b.categoriesModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(b.transactionsModule.CreateTransactionUC),
		b.o11y,
	)
	deps.ExpenseRecorder = agentbinding.NewTransactionLoggerAdapter(logTransaction)
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
	logCardPurchase := usecases.NewRecordCardPurchaseFromAgent(
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

func (b *agentModuleBuilder) attachBudgetConfigSession(deps *appservices.IntentRouterDeps, llmModule *llmRuntime) {
	if b.sessionRepo == nil || b.sessionUoW == nil {
		return
	}
	if llmModule == nil || llmModule.Interpreter == nil {
		return
	}
	if b.budgetsModule == nil || b.budgetsModule.CreateBudgetUC == nil || b.budgetsModule.ActivateBudgetUC == nil {
		return
	}
	conversationUC, err := usecases.NewConfigureBudgetConversation(llmModule.Interpreter, b.o11y)
	if err != nil {
		b.o11y.Logger().Warn(context.Background(), "agent.module.budget_config_session_failed",
			observability.Error(err),
		)
		return
	}
	deps.BudgetConvo = agentbinding.NewBudgetConversationAdapter(conversationUC)
	deps.BudgetCommitter = agentbinding.NewBudgetConfigCommitterAdapter(
		b.budgetsModule.CreateBudgetUC,
		b.budgetsModule.ActivateBudgetUC,
	)
	deps.BudgetSession = agentbinding.NewBudgetSessionGatewayAdapter(b.sessionRepo, b.sessionUoW)
	deps.PendingExpenseConfirmation = agentbinding.NewPendingExpenseConfirmationAdapter(b.sessionRepo, b.sessionUoW)
}

func (b *agentModuleBuilder) attachDecisionAudit(deps *appservices.IntentRouterDeps) {
	if b.decisionRepoFact == nil || b.decisionUoW == nil {
		return
	}
	deps.Decision = appservices.DecisionAuditDeps{
		Factory: b.decisionRepoFact,
		UoW:     b.decisionUoW,
	}
	redactor, err := sanitize.NewSanitizer(sanitize.DefaultMaxRunes)
	if err != nil {
		b.o11y.Logger().Warn(context.Background(), "agent.module.decision_audit_redactor_failed",
			observability.Error(err),
		)
		return
	}
	deps.Redactor = redactor
}

func (b *agentModuleBuilder) attachOnboardingLLM(deps *appservices.IntentRouterDeps, llmModule *llmRuntime) {
	if !b.cfg.AgentConfig.OnboardingLLMEnabled {
		b.o11y.Logger().Info(context.Background(), "agent.module.onboarding_route",
			observability.String("mode", "deterministic"),
			observability.String("reason", "flag_disabled"),
		)
		return
	}
	if reason := b.onboardingLLMUnavailable(deps, llmModule); reason != "" {
		b.o11y.Logger().Warn(context.Background(), "agent.module.onboarding_route",
			observability.String("mode", "deterministic"),
			observability.String("reason", reason),
		)
		return
	}
	uc := b.onboardingLLM
	reader := agentonboarding.NewOnboardingStateReader(uc.GetContext)
	dispatcher := agentonboarding.NewOnboardingToolDispatcher(
		uc.SaveObjective,
		uc.SaveIncome,
		uc.SaveCard,
		uc.SaveBudgetSplits,
		uc.MarkFirstTx,
		uc.Complete,
		uc.GetContext,
		b.wmRepo,
		deps.ExpenseRecorder,
	)
	phaseSetter := agentonboarding.NewOnboardingPhaseSetter(uc.SetPhase)
	v2session := agentbinding.NewOnboardingSessionGateway(b.sessionRepo)
	runTurn, err := usecases.NewRunOnboardingTurn(llmModule.OnboardingInterpreter, reader, dispatcher, phaseSetter, b.cfg.AgentConfig.OnboardingMaxTokens, b.o11y, b.sessionRepo, v2session)
	if err != nil {
		b.o11y.Logger().Warn(context.Background(), "agent.module.onboarding_route",
			observability.String("mode", "deterministic"),
			observability.String("reason", "run_turn_build_failed"),
			observability.Error(err),
		)
		return
	}
	deps.OnboardingRunner = agentonboarding.NewOnboardingTurnRunnerAdapter(runTurn)
	b.o11y.Logger().Info(context.Background(), "agent.module.onboarding_route",
		observability.String("mode", "llm"),
	)
}

func (b *agentModuleBuilder) onboardingLLMUnavailable(deps *appservices.IntentRouterDeps, llmModule *llmRuntime) string {
	if b.onboardingLLM == nil {
		return "usecases_missing"
	}
	if llmModule == nil || llmModule.OnboardingInterpreter == nil {
		return "interpreter_missing"
	}
	if deps.ExpenseRecorder == nil {
		return "expense_logger_missing"
	}
	if b.onboardingLLM.GetContext == nil {
		return "context_reader_missing"
	}
	if b.onboardingLLM.SetPhase == nil {
		return "phase_setter_missing"
	}
	return ""
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

func (b *agentModuleBuilder) buildWhatsAppAgentRoute(pub inboundPublisher) func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
	if pub == nil {
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
		if err := pub.PublishWhatsApp(ctx, principal.UserID, msg.From, msg.Text, msg.WAMID); err != nil {
			b.o11y.Logger().Warn(ctx, "whatsapp.dispatcher.agent_route_publish_failed",
				observability.Error(err),
			)
		}
		return wadispatcher.OutcomeAgent
	}
}

func (b *agentModuleBuilder) buildTelegramAgentRoute(pub inboundPublisher) tgdispatcher.AgentRoute {
	if pub == nil {
		return nil
	}

	return func(ctx context.Context, msg tgpayload.Message) tgdispatcher.RouteOutcome {
		principal, ok := auth.FromContext(ctx)
		if !ok {
			b.o11y.Logger().Warn(ctx, "telegram.dispatcher.agent_route_missing_principal")
			return tgdispatcher.OutcomeAgent
		}
		if err := pub.PublishTelegram(ctx, principal.UserID, msg.ChatID, msg.Text, fmt.Sprintf("%d", msg.MessageID)); err != nil {
			b.o11y.Logger().Warn(ctx, "telegram.dispatcher.agent_route_publish_failed",
				observability.Error(err),
			)
		}
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
	return appservices.ParsedIntent{
		Intent:       out.Intent,
		Confidence:   out.Confidence,
		Raw:          out.Raw,
		DirectReply:  out.DirectReply,
		LLMModel:     out.LLMModel,
		PromptSHA256: out.PromptSHA256,
	}, nil
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

func newLLMRuntime(cfg configs.AgentConfig, o11y observability.Observability) (*llmRuntime, error) {
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

	makeProvider := func(slug valueobjects.ModelSlug) interfaces.LLMProvider {
		return openrouter.NewProvider(client, openrouter.ProviderConfig{
			Slug:           slug,
			APIKey:         cfg.OpenRouterAPIKey,
			HTTPReferer:    cfg.HTTPReferer,
			XTitle:         cfg.XTitle,
			MaxTokens:      cfg.MaxTokens,
			Temperature:    cfg.Temperature,
			RequestTimeout: cfg.RequestTimeout,
		}, o11y)
	}
	newBreaker := func() *appservices.CircuitBreaker {
		return appservices.NewCircuitBreaker(appservices.CircuitBreakerConfig{
			MaxFailures:   cfg.CircuitFailures,
			FailureWindow: cfg.CircuitWindow,
			OpenDuration:  cfg.CircuitCooldown,
		})
	}

	primary, err := valueobjects.NewModelSlug(cfg.PrimaryModel)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: primary model: %w", err)
	}
	fallbackSlugs := make([]valueobjects.ModelSlug, 0)
	for _, raw := range parseFallbackList(cfg.FallbackModels) {
		slug, slugErr := valueobjects.NewModelSlug(raw)
		if slugErr != nil {
			return nil, fmt.Errorf("agent.llm: fallback model %q: %w", raw, slugErr)
		}
		fallbackSlugs = append(fallbackSlugs, slug)
	}

	chain, err := buildLLMChain(makeProvider, newBreaker, primary, fallbackSlugs, o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: fallback chain: %w", err)
	}

	retryChain, err := buildRetryChain(makeProvider, newBreaker, fallbackSlugs, o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: retry chain: %w", err)
	}

	var onboardingInterpreter usecases.IntentInterpreter
	if strings.TrimSpace(cfg.OnboardingModel) != "" {
		onbSlug, onbErr := valueobjects.NewModelSlug(cfg.OnboardingModel)
		if onbErr != nil {
			return nil, fmt.Errorf("agent.llm: onboarding model %q: %w", cfg.OnboardingModel, onbErr)
		}
		onboardingInterpreter, err = buildLLMChain(makeProvider, newBreaker, onbSlug, fallbackSlugs, o11y)
		if err != nil {
			return nil, fmt.Errorf("agent.llm: onboarding chain: %w", err)
		}
	}

	parseInbound, err := usecases.NewParseInbound(chain, retryChain, cfg.MaxInputChars, o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: parse inbound: %w", err)
	}

	conversational, err := usecases.NewComposeConversationalReply(chain, cfg.ProseMaxTokens, o11y, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: conversational reply: %w", err)
	}

	return &llmRuntime{
		ParseInbound:          parseInbound,
		Conversational:        conversational,
		Interpreter:           chain,
		OnboardingInterpreter: onboardingInterpreter,
	}, nil
}

func buildRetryChain(
	makeProvider func(valueobjects.ModelSlug) interfaces.LLMProvider,
	newBreaker func() *appservices.CircuitBreaker,
	fallbacks []valueobjects.ModelSlug,
	o11y observability.Observability,
) (usecases.IntentInterpreter, error) {
	if len(fallbacks) == 0 {
		return nil, nil
	}
	return buildLLMChain(makeProvider, newBreaker, fallbacks[0], fallbacks[1:], o11y)
}

func buildLLMChain(
	makeProvider func(valueobjects.ModelSlug) interfaces.LLMProvider,
	newBreaker func() *appservices.CircuitBreaker,
	primary valueobjects.ModelSlug,
	fallbacks []valueobjects.ModelSlug,
	o11y observability.Observability,
) (usecases.IntentInterpreter, error) {
	providers := make([]interfaces.LLMProvider, 0, len(fallbacks)+1)
	providers = append(providers, makeProvider(primary))
	for _, slug := range fallbacks {
		if slug.Equal(primary) {
			continue
		}
		providers = append(providers, makeProvider(slug))
	}
	chain, err := appservices.NewFallbackChain(providers, newBreaker(), o11y)
	if err != nil {
		return nil, err
	}
	return chain, nil
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

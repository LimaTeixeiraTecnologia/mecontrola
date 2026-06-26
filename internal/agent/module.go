package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/capability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/sanitize"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
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
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	wfpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

var ErrAPIKeyRequired = errors.New("agent.llm: OPENROUTER_API_KEY is required")

type whatsAppGateway interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type inboundPublisher interface {
	PublishWhatsApp(ctx context.Context, userID uuid.UUID, peer, text, messageID string) error
}

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type AgentModule struct {
	WhatsAppAgentRoute            func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	ParseInbound                  *usecases.ParseInbound
	IntentRouter                  *appservices.IntentRouter
	SessionUnitOfWork             uow.UnitOfWork
	SessionRepository             interfaces.AgentSessionRepository
	SessionRepositoryFact         interfaces.AgentSessionRepositoryFactory
	EventHandlers                 []EventHandlerRegistration
	WorkflowKernelHousekeepingJob worker.Job
}

type OnboardingLLMUseCases struct {
	GetContext         *onbusecases.GetOnboardingContext
	SaveObjective      *onbusecases.SaveOnboardingObjective
	SaveIncome         *onbusecases.SaveOnboardingIncome
	SaveCard           *onbusecases.SaveOnboardingCard
	SaveBudgetSplits   *onbusecases.SaveOnboardingBudgetSplits
	MarkFirstTx        *onbusecases.MarkFirstTransactionRecorded
	Complete           *onbusecases.CompleteOnboardingSession
	SetPhase           *onbusecases.SetOnboardingPhase
	AppendTurn         *onbusecases.AppendOnboardingTurn
	LoadTurns          *onbusecases.LoadOnboardingTurns
	MarkWelcomeSent    *onbusecases.MarkWelcomeSent
	SuggestBudgetSplit *onbusecases.SuggestBudgetSplit
}

type AgentModuleDeps struct {
	SessionStore    *sqlx.DB
	OutboxPublisher outbox.Publisher
	OnboardingLLM   *OnboardingLLMUseCases
}

type llmRuntime struct {
	ParseInbound          *usecases.ParseInbound
	Conversational        *usecases.ComposeConversationalReply
	Interpreter           usecases.IntentInterpreter
	OnboardingInterpreter usecases.IntentInterpreter
	ConvInterpreter       usecases.IntentInterpreter
	ConvMaxTokens         int
	Router                *appservices.ClassRouter
}

type agentModuleWiring struct {
	cfg                *configs.Config
	o11y               observability.Observability
	identityModule     identity.IdentityModule
	categoriesModule   *categories.CategoriesModule
	cardModule         card.CardModule
	transactionsModule transactions.TransactionsModule
	budgetsModule      *budgets.BudgetsModule
	whatsAppGateway    whatsAppGateway
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
	wfStoreFactory     platform.StoreFactory
	wfHousekeepingJob  worker.Job
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
	deps AgentModuleDeps,
) (AgentModule, error) {
	if cfg == nil {
		return AgentModule{}, fmt.Errorf("agent.module: config is nil")
	}
	if whatsAppGateway == nil {
		return AgentModule{}, fmt.Errorf("agent.module: whatsapp gateway is nil")
	}

	w := &agentModuleWiring{
		cfg:                cfg,
		o11y:               o11y,
		identityModule:     identityModule,
		categoriesModule:   categoriesModule,
		cardModule:         cardModule,
		transactionsModule: transactionsModule,
		budgetsModule:      budgetsModule,
		whatsAppGateway:    whatsAppGateway,
		sessionDB:          deps.SessionStore,
		outboxPublisher:    deps.OutboxPublisher,
		onboardingLLM:      deps.OnboardingLLM,
	}

	prepareSessionStore(w)

	llmModule, err := buildLLMModule(w)
	if err != nil {
		return AgentModule{}, err
	}

	intentRouter, err := buildIntentRouter(w, llmModule)
	if err != nil {
		return AgentModule{}, err
	}

	var pub inboundPublisher
	if w.outboxPublisher != nil {
		pub = agentevents.NewInboundEventPublisher(w.outboxPublisher, w.o11y)
	}

	module := AgentModule{
		WhatsAppAgentRoute:            buildWhatsAppAgentRoute(w, pub),
		ParseInbound:                  llmModule.ParseInbound,
		IntentRouter:                  intentRouter,
		SessionRepository:             w.sessionRepo,
		SessionRepositoryFact:         w.sessionRepoFact,
		SessionUnitOfWork:             w.sessionUoW,
		EventHandlers:                 buildEventHandlers(w, intentRouter),
		WorkflowKernelHousekeepingJob: w.wfHousekeepingJob,
	}
	return module, nil
}

func buildEventHandlers(w *agentModuleWiring, router *appservices.IntentRouter) []EventHandlerRegistration {
	if router == nil {
		return nil
	}
	handlers := []EventHandlerRegistration{
		{
			EventType: agentevents.EventTypeWhatsAppInbound,
			Handler:   agentconsumers.NewWhatsAppInboundConsumer(router, w.o11y),
		},
		{
			EventType: "onboarding.subscription_bound",
			Handler:   buildOnboardingBoundConsumer(w, router),
		},
	}
	if w.onboardingLLM != nil && w.onboardingLLM.GetContext != nil && w.wmRepo != nil {
		handlers = append(handlers, EventHandlerRegistration{
			EventType: "onboarding.completed",
			Handler:   agentconsumers.NewOnboardingCompletedConsumer(w.onboardingLLM.GetContext, w.wmRepo, w.o11y),
		})
	}
	return handlers
}

func buildOnboardingBoundConsumer(w *agentModuleWiring, router *appservices.IntentRouter) *agentconsumers.OnboardingBoundConsumer {
	opts := make([]agentconsumers.OnboardingBoundConsumerOption, 0)
	if w.decisionRepoFact != nil && w.decisionUoW != nil {
		store := agentconsumers.NewGreetingDecisionStore(w.decisionRepoFact, w.decisionUoW)
		opts = append(opts, agentconsumers.WithGreetingDecisionStore(store))
	}
	if w.onboardingLLM != nil && w.onboardingLLM.GetContext != nil {
		opts = append(opts, agentconsumers.WithOnboardingStateChecker(agentonboarding.NewOnboardingStateReader(w.onboardingLLM.GetContext)))
	}
	if w.onboardingLLM != nil && w.onboardingLLM.MarkWelcomeSent != nil {
		opts = append(opts, agentconsumers.WithGreetingWelcomeMarker(agentonboarding.NewGreetingWelcomeMarker(w.onboardingLLM.MarkWelcomeSent)))
	}
	return agentconsumers.NewOnboardingBoundConsumer(router, w.o11y, opts...)
}

func prepareSessionStore(w *agentModuleWiring) {
	if w.sessionDB == nil {
		return
	}
	factory := agentrepo.NewRepositoryFactory(w.o11y)
	w.sessionRepoFact = factory
	w.sessionRepo = factory.AgentSessionRepository(w.sessionDB)
	w.sessionUoW = uow.NewUnitOfWork(w.sessionDB)
	w.decisionRepoFact = agentrepo.NewDecisionRepositoryFactory(w.o11y)
	w.decisionUoW = uow.NewUnitOfWork(w.sessionDB)
	w.threadRepoFact = agentrepo.NewThreadRepositoryFactory(w.o11y)
	w.runRepoFact = agentrepo.NewRunRepositoryFactory(w.o11y)
	w.runtimeUoW = uow.NewUnitOfWork(w.sessionDB)
	wmFactory := agentrepo.NewWorkingMemoryRepositoryFactory(w.o11y)
	w.wmRepo = wmFactory.WorkingMemoryRepository(w.sessionDB)
	obsFactory := agentrepo.NewObservationRepositoryFactory(w.o11y)
	w.obsRepo = obsFactory.ObservationRepository(w.sessionDB)
	w.wfStoreFactory = wfpostgres.NewStoreFactory(w.o11y)
}

func buildLLMModule(w *agentModuleWiring) (*llmRuntime, error) {
	if w.categoriesModule == nil || w.categoriesModule.ListCategoriesUC == nil {
		return nil, fmt.Errorf("agent.module: categories module is incomplete")
	}
	if w.cardModule.ListCardsUC == nil {
		return nil, fmt.Errorf("agent.module: card list use case is nil")
	}
	if w.cardModule.CreateCardUC == nil {
		return nil, fmt.Errorf("agent.module: card create use case is nil")
	}

	if w.budgetsModule == nil || w.budgetsModule.ListAlertsUC == nil {
		return nil, fmt.Errorf("agent.module: budgets module is incomplete")
	}
	if w.transactionsModule.ListTransactionsUC == nil || w.transactionsModule.CreateTransactionUC == nil {
		return nil, fmt.Errorf("agent.module: transactions desabilitado; defina TRANSACTIONS_ENABLED=true")
	}

	llmModule, err := newLLMRuntime(w.cfg.AgentConfig, w.o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.module: %w", err)
	}
	if w.sessionRepo != nil && w.wmRepo != nil && w.obsRepo != nil {
		redactor, redactErr := sanitize.NewSanitizer(sanitize.DefaultMaxRunes)
		if redactErr == nil {
			conv, convErr := rebuildConversationalReply(w, llmModule, redactor)
			if convErr == nil {
				llmModule.Conversational = conv
			} else {
				w.o11y.Logger().Warn(context.Background(), "agent.module.conversational_rebuild_failed", observability.Error(convErr))
			}
		} else {
			w.o11y.Logger().Warn(context.Background(), "agent.module.sanitizer_create_failed", observability.Error(redactErr))
		}
	}
	return llmModule, nil
}

func rebuildConversationalReply(
	w *agentModuleWiring,
	llmModule *llmRuntime,
	redactor *sanitize.Sanitizer,
) (*usecases.ComposeConversationalReply, error) {
	if llmModule == nil {
		return nil, fmt.Errorf("agent.module: llm runtime is nil")
	}
	convInterpreter := llmModule.ConvInterpreter
	if convInterpreter == nil {
		convInterpreter = llmModule.Interpreter
	}
	convMaxTokens := llmModule.ConvMaxTokens
	if convMaxTokens <= 0 {
		convMaxTokens = w.cfg.AgentConfig.ProseMaxTokens
	}
	obsSvc := appservices.NewObservationMemory(llmModule.Interpreter, w.obsRepo, w.o11y, 6, 3)
	return usecases.NewComposeConversationalReply(
		convInterpreter,
		convMaxTokens,
		w.o11y,
		w.sessionRepo,
		w.wmRepo,
		obsSvc,
		redactor,
	)
}

func buildIntentRouter(w *agentModuleWiring, llmModule *llmRuntime) (*appservices.IntentRouter, error) {
	useLLM := llmModule != nil && llmModule.ParseInbound != nil
	if !useLLM {
		return nil, nil
	}

	deps := appservices.IntentRouterDeps{
		Parser:              &intentParserAdapter{uc: llmModule.ParseInbound},
		Fallback:            &fallbackAdapter{runtime: llmModule},
		WhatsAppGateway:     w.whatsAppGateway,
		PolicyMinConfidence: w.cfg.AgentConfig.PolicyMinConfidence,
	}
	catalog, err := capability.BuildCatalog()
	if err != nil {
		return nil, fmt.Errorf("agent.module: capability catalog: %w", err)
	}
	deps.CapabilityCatalog = catalog
	fillIntentRouterDeps(w, &deps)
	attachExpenseRecorder(w, &deps)
	attachCardPurchaseLogger(w, &deps)
	attachTransactionQueries(w, &deps)
	attachRecurring(w, &deps)
	attachBudgetConfigSession(w, &deps)
	attachOnboardingLLM(w, &deps, llmModule)
	attachDecisionAudit(w, &deps)
	if err := attachKernel(w, &deps); err != nil {
		return nil, fmt.Errorf("agent.module: kernel wiring: %w", err)
	}
	router, err := appservices.NewIntentRouter(w.o11y, deps)
	if err != nil {
		return nil, fmt.Errorf("agent.module: intent router: %w", err)
	}
	if err := attachRuntime(w, router, catalog); err != nil {
		return nil, err
	}
	return router, nil
}

func attachRuntime(w *agentModuleWiring, router *appservices.IntentRouter, catalog *capability.Catalog) error {
	if w.threadRepoFact == nil || w.runRepoFact == nil || w.runtimeUoW == nil {
		w.o11y.Logger().Warn(context.Background(), "agent.module.runtime",
			observability.String("mode", "legacy"),
			observability.String("reason", "session_store_missing"),
		)
		return nil
	}
	threads := agentbinding.NewThreadGatewayAdapter(w.threadRepoFact, w.runtimeUoW)
	runs := agentbinding.NewRunGatewayAdapter(w.runRepoFact, w.runtimeUoW)
	router.EnableRuntime(catalog, threads, runs)
	w.o11y.Logger().Info(context.Background(), "agent.module.runtime",
		observability.String("mode", "enabled"),
	)
	return nil
}

func fillIntentRouterDeps(w *agentModuleWiring, deps *appservices.IntentRouterDeps) {
	if w.budgetsModule != nil && w.budgetsModule.GetMonthlySummaryUC != nil {
		deps.MonthlySummary = w.budgetsModule.GetMonthlySummaryUC
	}
	fillCardDeps(w, deps)
	fillBudgetCategoryDeps(w, deps)
}

func fillCardDeps(w *agentModuleWiring, deps *appservices.IntentRouterDeps) {
	if w.cardModule.ListCardsUC != nil {
		deps.CardLister = w.cardModule.ListCardsUC
	}
	if w.cardModule.InvoiceForUC != nil {
		deps.CardInvoice = w.cardModule.InvoiceForUC
	}
	if w.cardModule.CreateCardUC != nil {
		deps.CardCreator = agentbinding.NewCardCreatorAdapter(w.cardModule.CreateCardUC)
	}
	if w.cardModule.CountCardsUC != nil {
		deps.CardCounter = agentbinding.NewCardCounterAdapter(w.cardModule.CountCardsUC)
	}
	if w.cardModule.ListCardsUC != nil && w.cardModule.UpdateCardUC != nil {
		deps.CardUpdater = agentbinding.NewCardUpdaterAdapter(w.cardModule.ListCardsUC, w.cardModule.UpdateCardUC)
	}
	if w.cardModule.ListCardsUC != nil && w.cardModule.SoftDeleteCardUC != nil {
		deps.CardDeleter = agentbinding.NewCardDeleterAdapter(w.cardModule.ListCardsUC, w.cardModule.SoftDeleteCardUC)
	}
}

func fillBudgetCategoryDeps(w *agentModuleWiring, deps *appservices.IntentRouterDeps) {
	if w.budgetsModule != nil && w.budgetsModule.EditCategoryPercentageUC != nil {
		deps.CategoryPercentageEditor = agentbinding.NewCategoryPercentageEditorAdapter(w.budgetsModule.EditCategoryPercentageUC)
	}
	if w.budgetsModule != nil && w.budgetsModule.CreateRecurrenceUC != nil {
		deps.BudgetRecurrenceCreator = agentbinding.NewBudgetRecurrenceCreatorAdapter(w.budgetsModule.CreateRecurrenceUC)
	}
	if w.categoriesModule != nil && w.categoriesModule.ListCategoriesUC != nil {
		deps.CategoryLister = agentbinding.NewListCategoriesBinding(w.categoriesModule.ListCategoriesUC)
	}
	if w.categoriesModule != nil && w.categoriesModule.SearchDictionaryUC != nil {
		deps.CategoryClassifier = agentbinding.NewClassifyCategoryBinding(w.categoriesModule.SearchDictionaryUC)
	}
}

func attachExpenseRecorder(w *agentModuleWiring, deps *appservices.IntentRouterDeps) {
	if w.transactionsModule.CreateTransactionUC == nil {
		return
	}
	if w.categoriesModule == nil || w.categoriesModule.SearchDictionaryUC == nil {
		return
	}
	logTransaction := usecases.NewRecordTransactionFromAgent(
		w.categoriesModule.SearchDictionaryUC,
		agentbinding.NewTransactionCreatorAdapter(w.transactionsModule.CreateTransactionUC),
		w.o11y,
	)
	deps.ExpenseRecorder = agentbinding.NewTransactionLoggerAdapter(logTransaction)
}

func attachCardPurchaseLogger(w *agentModuleWiring, deps *appservices.IntentRouterDeps) {
	if w.transactionsModule.CreateCardPurchaseUC == nil {
		return
	}
	if w.cardModule.ListCardsUC == nil {
		return
	}
	if w.categoriesModule == nil || w.categoriesModule.SearchDictionaryUC == nil {
		return
	}
	logCardPurchase := usecases.NewRecordCardPurchaseFromAgent(
		w.categoriesModule.SearchDictionaryUC,
		agentbinding.NewCardPurchaseCreatorAdapter(w.cardModule.ListCardsUC, w.transactionsModule.CreateCardPurchaseUC),
		w.o11y,
	)
	deps.CardPurchaseLog = agentbinding.NewCardPurchaseLoggerAdapter(logCardPurchase)
}

func attachTransactionQueries(w *agentModuleWiring, deps *appservices.IntentRouterDeps) {
	if w.transactionsModule.ListTransactionsUC == nil {
		return
	}
	deps.TransactionLister = agentbinding.NewTransactionListerAdapter(w.transactionsModule.ListTransactionsUC)
	deps.IncomeSummaryReader = agentbinding.NewIncomeSummaryReaderAdapter(w.transactionsModule.ListTransactionsUC)
	if w.transactionsModule.SearchTransactionsUC != nil {
		deps.TransactionSearcher = agentbinding.NewTransactionSearcherAdapter(w.transactionsModule.SearchTransactionsUC)
	}
	if w.transactionsModule.DeleteTransactionUC != nil {
		deps.LastDeleter = agentbinding.NewLastTransactionDeleterAdapter(w.transactionsModule.DeleteTransactionUC)
	}
	if w.transactionsModule.GetTransactionUC != nil && w.transactionsModule.UpdateTransactionUC != nil {
		deps.LastEditor = agentbinding.NewLastTransactionEditorAdapter(
			w.transactionsModule.GetTransactionUC,
			w.transactionsModule.UpdateTransactionUC,
		)
	}
}

func attachRecurring(w *agentModuleWiring, deps *appservices.IntentRouterDeps) {
	if w.transactionsModule.CreateRecurringTemplateUC != nil &&
		w.categoriesModule != nil && w.categoriesModule.SearchDictionaryUC != nil {
		createRecurring := usecases.NewCreateRecurringFromAgent(
			w.categoriesModule.SearchDictionaryUC,
			agentbinding.NewRecurringTemplateCreatorAdapter(w.transactionsModule.CreateRecurringTemplateUC),
			w.o11y,
		)
		deps.RecurringCreator = agentbinding.NewRecurringCreatorAdapter(createRecurring)
	}
	if w.transactionsModule.ListRecurringTemplatesUC != nil {
		deps.RecurringLister = agentbinding.NewRecurringListerAdapter(w.transactionsModule.ListRecurringTemplatesUC)
	}
}

func attachBudgetConfigSession(w *agentModuleWiring, deps *appservices.IntentRouterDeps) {
	if w.sessionRepo == nil || w.sessionUoW == nil {
		return
	}
	if w.budgetsModule == nil || w.budgetsModule.CreateBudgetUC == nil || w.budgetsModule.ActivateBudgetUC == nil {
		return
	}
	conversationUC, err := usecases.NewConfigureBudgetConversation(w.o11y)
	if err != nil {
		w.o11y.Logger().Warn(context.Background(), "agent.module.budget_config_session_failed",
			observability.Error(err),
		)
		return
	}
	deps.BudgetConvo = agentbinding.NewBudgetConversationAdapter(conversationUC)
	deps.BudgetCommitter = agentbinding.NewBudgetConfigCommitterAdapter(
		w.budgetsModule.CreateBudgetUC,
		w.budgetsModule.ActivateBudgetUC,
	)
	deps.BudgetSession = agentbinding.NewBudgetSessionGatewayAdapter(w.sessionRepo, w.sessionUoW)
}

func attachKernel(w *agentModuleWiring, deps *appservices.IntentRouterDeps) error {
	if w.sessionDB == nil || w.wfStoreFactory == nil {
		return errors.New("session_store_or_factory_missing: kernel requires sessionDB and wfStoreFactory")
	}
	store := w.wfStoreFactory.Store(w.sessionDB)
	settleReg := appservices.NewSettleRegistry()
	retryPolicy := platform.RetryPolicy{
		MaxAttempts: w.cfg.WorkflowKernelConfig.MaxAttempts,
		BaseBackoff: w.cfg.WorkflowKernelConfig.RetryBaseBackoff,
		MaxBackoff:  w.cfg.WorkflowKernelConfig.RetryMaxBackoff,
	}

	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, w.o11y)
	confirmDef := buildConfirmDefinition(w, deps, settleReg)

	kernelDeps := &appservices.KernelDeps{
		SettleReg:     settleReg,
		ConfirmEngine: confirmEngine,
		ConfirmDef:    confirmDef,
		RetryPolicy:   retryPolicy,
		MaxAttempts:   w.cfg.WorkflowKernelConfig.MaxAttempts,
	}

	if w.categoriesModule == nil || w.categoriesModule.SearchDictionaryUC == nil {
		return errors.New("categories_module_missing: kernel requires categoriesModule.SearchDictionaryUC")
	}
	engine := platform.NewEngine[steps.ExpenseState](store, w.o11y)
	planEngine := platform.NewEngine[agentwf.PlanState](store, w.o11y)
	resolver := agentbinding.NewKernelCategoryResolver(w.categoriesModule.SearchDictionaryUC)
	persistFn := agentbinding.NewKernelPersistFunc(deps.ExpenseRecorder, deps.CardPurchaseLog)
	kernelDeps.Engine = engine
	kernelDeps.PlanEngine = planEngine
	kernelDeps.CategoryResolver = resolver
	kernelDeps.PersistFn = persistFn
	w.o11y.Logger().Info(context.Background(), "agent.module.kernel_enabled",
		observability.String("workflow", agentwf.TransactionsWriteWorkflowID),
	)

	deps.Kernel = kernelDeps
	wfUoW := uow.NewUnitOfWork(w.sessionDB)
	hkJob, hkErr := platform.NewHousekeepingJob(wfUoW, w.wfStoreFactory, w.cfg.WorkflowKernelConfig, w.o11y.Logger())
	if hkErr != nil {
		return fmt.Errorf("invalid housekeeping config: %w", hkErr)
	}
	w.wfHousekeepingJob = hkJob
	w.o11y.Logger().Info(context.Background(), "agent.module.confirm_kernel_enabled",
		observability.String("workflow", agentwf.DestructiveConfirmWorkflowID),
	)
	return nil
}

func buildConfirmDefinition(w *agentModuleWiring, deps *appservices.IntentRouterDeps, settleReg *appservices.SettleRegistry) platform.Definition[confirmation.ConfirmState] {
	lister := deps.TransactionLister
	targets := map[confirmation.OperationKind]steps.TargetResolver{
		confirmation.OperationDeleteLast:   agentbinding.NewLastTransactionDeleterResolver(lister),
		confirmation.OperationEditLast:     agentbinding.NewLastTransactionEditorResolver(lister),
		confirmation.OperationDeleteCard:   agentbinding.NewCardDeleterResolver(deps.CardLister),
		confirmation.OperationBudgetCommit: agentbinding.NewBudgetCommitResolver(),
		confirmation.OperationDeleteByRef:  agentbinding.NewDeleteByRefResolver(),
		confirmation.OperationEditByRef:    agentbinding.NewEditByRefResolver(),
	}
	executors := map[confirmation.OperationKind]steps.DestructiveExecutor{
		confirmation.OperationDeleteLast:   agentbinding.NewLastTransactionDeleterExecutor(deps.LastDeleter),
		confirmation.OperationEditLast:     agentbinding.NewLastTransactionEditorExecutor(deps.LastEditor),
		confirmation.OperationDeleteCard:   agentbinding.NewCardDeleterExecutorFn(deps.CardDeleter),
		confirmation.OperationBudgetCommit: agentbinding.NewBudgetCommitExecutor(deps.BudgetCommitter),
		confirmation.OperationDeleteByRef:  agentbinding.NewDeleteByRefExecutor(deps.LastDeleter),
		confirmation.OperationEditByRef:    agentbinding.NewEditByRefExecutor(deps.LastEditor),
	}
	return agentwf.NewDestructiveConfirmDefinition(agentwf.DestructiveConfirmDeps{
		Authorize: func(ctx context.Context, state confirmation.ConfirmState) bool {
			principal, ok := auth.FromContext(ctx)
			if !ok {
				return false
			}
			uid, err := uuid.Parse(state.UserID)
			if err != nil {
				return false
			}
			return uid != uuid.Nil && principal.UserID == uid
		},
		Replay:     appservices.NewConfirmReplayFunc(w.o11y, deps.Decision, deps.Redactor),
		Policy:     func(_ context.Context, _ confirmation.ConfirmState) (bool, string) { return false, "" },
		AuditBegin: appservices.NewConfirmAuditBeginFunc(w.o11y, deps.Decision, deps.Redactor),
		OnSettle: func(id uuid.UUID, fn steps.ConfirmAuditSettleFunc) {
			settleReg.Register(id, func(ctx context.Context, executed bool) { fn(ctx, executed) })
		},
		Searcher:       deps.TransactionSearcher,
		Targets:        targets,
		Executors:      executors,
		TTL:            10 * time.Minute,
		DenyReply:      "Não consegui concluir essa ação agora. Tente de novo em instantes 🙏",
		ReplayReply:    "Essa mensagem já foi processada ✅",
		AuditFailReply: "Não foi possível processar sua mensagem agora. Pode tentar de novo em instantes? 🙏",
		RetryPolicy: platform.RetryPolicy{
			MaxAttempts: w.cfg.WorkflowKernelConfig.MaxAttempts,
			BaseBackoff: w.cfg.WorkflowKernelConfig.RetryBaseBackoff,
			MaxBackoff:  w.cfg.WorkflowKernelConfig.RetryMaxBackoff,
		},
		MaxAttempts:   w.cfg.WorkflowKernelConfig.MaxAttempts,
		Observability: w.o11y,
	})
}

func attachDecisionAudit(w *agentModuleWiring, deps *appservices.IntentRouterDeps) {
	if w.decisionRepoFact == nil || w.decisionUoW == nil {
		return
	}
	deps.Decision = appservices.DecisionAuditDeps{
		Factory: w.decisionRepoFact,
		UoW:     w.decisionUoW,
	}
	redactor, err := sanitize.NewSanitizer(sanitize.DefaultMaxRunes)
	if err != nil {
		w.o11y.Logger().Warn(context.Background(), "agent.module.decision_audit_redactor_failed",
			observability.Error(err),
		)
		return
	}
	deps.Redactor = redactor
}

func attachOnboardingLLM(w *agentModuleWiring, deps *appservices.IntentRouterDeps, llmModule *llmRuntime) {
	if reason := onboardingLLMUnavailable(w, deps, llmModule); reason != "" {
		w.o11y.Logger().Warn(context.Background(), "agent.module.onboarding_route",
			observability.String("mode", "deterministic"),
			observability.String("reason", reason),
		)
		return
	}
	uc := w.onboardingLLM
	reader := agentonboarding.NewOnboardingStateReader(uc.GetContext)
	dispatcher := agentonboarding.NewOnboardingToolDispatcher(
		uc.SaveObjective,
		uc.SaveIncome,
		uc.SaveCard,
		uc.SaveBudgetSplits,
		uc.MarkFirstTx,
		uc.Complete,
		deps.ExpenseRecorder,
	)
	phaseSetter := agentonboarding.NewOnboardingPhaseSetter(uc.SetPhase)
	v2session := agentbinding.NewOnboardingSessionGateway(w.sessionRepo)

	var historyGateway usecases.OnboardingHistoryGatewayIface
	if uc.AppendTurn != nil && uc.LoadTurns != nil && uc.MarkWelcomeSent != nil {
		historyGateway = agentonboarding.NewOnboardingHistoryGateway(uc.AppendTurn, uc.LoadTurns, uc.MarkWelcomeSent)
	}
	var splitSuggester usecases.BudgetSplitSuggesterIface
	if uc.SuggestBudgetSplit != nil {
		splitSuggester = agentonboarding.NewBudgetSplitSuggester(uc.SuggestBudgetSplit)
	}
	runTurn, err := usecases.NewRunOnboardingTurn(llmModule.OnboardingInterpreter, reader, dispatcher, phaseSetter, w.cfg.AgentConfig.OnboardingMaxTokens, w.o11y, historyGateway, splitSuggester, v2session)
	if err != nil {
		w.o11y.Logger().Warn(context.Background(), "agent.module.onboarding_route",
			observability.String("mode", "deterministic"),
			observability.String("reason", "run_turn_build_failed"),
			observability.Error(err),
		)
		return
	}
	deps.OnboardingRunner = agentonboarding.NewOnboardingTurnRunnerAdapter(runTurn)
	w.o11y.Logger().Info(context.Background(), "agent.module.onboarding_route",
		observability.String("mode", "llm"),
	)
}

func onboardingLLMUnavailable(w *agentModuleWiring, deps *appservices.IntentRouterDeps, llmModule *llmRuntime) string {
	if w.onboardingLLM == nil {
		return "usecases_missing"
	}
	if llmModule == nil || llmModule.OnboardingInterpreter == nil {
		return "interpreter_missing"
	}
	if deps.ExpenseRecorder == nil {
		return "expense_logger_missing"
	}
	if w.onboardingLLM.GetContext == nil {
		return "context_reader_missing"
	}
	if w.onboardingLLM.SetPhase == nil {
		return "phase_setter_missing"
	}
	return ""
}

func buildWhatsAppAgentRoute(w *agentModuleWiring, pub inboundPublisher) func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
	if pub == nil {
		return func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
			return wadispatcher.OutcomeAgent
		}
	}

	return func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
		principal, ok := auth.FromContext(ctx)
		if !ok {
			w.o11y.Logger().Warn(ctx, "whatsapp.dispatcher.agent_route_missing_principal")
			return wadispatcher.OutcomeAgent
		}
		if err := pub.PublishWhatsApp(ctx, principal.UserID, msg.From, msg.Text, msg.WAMID); err != nil {
			w.o11y.Logger().Warn(ctx, "whatsapp.dispatcher.agent_route_publish_failed",
				observability.Error(err),
			)
		}
		return wadispatcher.OutcomeAgent
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
		Plan:         out.Plan,
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

func newLLMRuntime(cfg configs.AgentConfig, o11y observability.Observability) (*llmRuntime, error) { //nolint:revive // wiring de runtime LLM por classe
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

	makeProviderWithTokens := func(maxTokens int) func(valueobjects.ModelSlug) interfaces.LLMProvider {
		return func(slug valueobjects.ModelSlug) interfaces.LLMProvider {
			return openrouter.NewProvider(client, openrouter.ProviderConfig{
				Slug:           slug,
				APIKey:         cfg.OpenRouterAPIKey,
				HTTPReferer:    cfg.HTTPReferer,
				XTitle:         cfg.XTitle,
				MaxTokens:      maxTokens,
				Temperature:    cfg.Temperature,
				RequestTimeout: cfg.RequestTimeout,
			}, o11y)
		}
	}
	makeProvider := makeProviderWithTokens(cfg.MaxTokens)
	makeParseProvider := makeProviderWithTokens(resolveParseMaxTokens(cfg.ParseMaxTokens, cfg.MaxTokens))
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

	parseChain, err := buildClassChain(makeParseProvider, newBreaker, cfg.ParsePrimaryModel, cfg.ParseFallbackModels, fallbackSlugs, primary, o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: parse chain: %w", err)
	}

	retryChain, err := buildRetryChain(makeParseProvider, newBreaker, fallbackSlugs, o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: retry chain: %w", err)
	}

	onbPrimary := cfg.OnboardingPrimaryModel
	if strings.TrimSpace(onbPrimary) == "" {
		onbPrimary = cfg.OnboardingModel
	}
	onbFallbacks := cfg.OnboardingFallbackModels
	if strings.TrimSpace(onbFallbacks) == "" {
		onbFallbacks = cfg.FallbackModels
	}
	onboardingChain, onbErr := buildClassChain(makeProvider, newBreaker, onbPrimary, onbFallbacks, fallbackSlugs, primary, o11y)
	if onbErr != nil {
		return nil, fmt.Errorf("agent.llm: onboarding chain: %w", onbErr)
	}

	convPrimary := cfg.ConvPrimaryModel
	if strings.TrimSpace(convPrimary) == "" {
		convPrimary = cfg.PrimaryModel
	}
	convFallbacks := cfg.ConvFallbackModels
	if strings.TrimSpace(convFallbacks) == "" {
		convFallbacks = cfg.FallbackModels
	}
	convChain, convErr := buildClassChain(makeProvider, newBreaker, convPrimary, convFallbacks, fallbackSlugs, primary, o11y)
	if convErr != nil {
		return nil, fmt.Errorf("agent.llm: conversational chain: %w", convErr)
	}

	parseInbound, err := usecases.NewParseInbound(parseChain, retryChain, cfg.MaxInputChars, o11y)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: parse inbound: %w", err)
	}

	convMaxTokens := cfg.ConvMaxTokens
	if convMaxTokens <= 0 {
		convMaxTokens = cfg.ProseMaxTokens
	}
	conversational, err := usecases.NewComposeConversationalReply(convChain, convMaxTokens, o11y, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: conversational reply: %w", err)
	}

	router := appservices.NewClassRouter(map[appservices.LLMClass]appservices.ClassInterpreter{
		appservices.LLMClassParse:          appservices.NewClassMetricInterpreter(parseChain, appservices.LLMClassParse, o11y),
		appservices.LLMClassOnboarding:     appservices.NewClassMetricInterpreter(onboardingChain, appservices.LLMClassOnboarding, o11y),
		appservices.LLMClassConversational: appservices.NewClassMetricInterpreter(convChain, appservices.LLMClassConversational, o11y),
	})

	return &llmRuntime{
		ParseInbound:          parseInbound,
		Conversational:        conversational,
		Interpreter:           parseChain,
		OnboardingInterpreter: onboardingChain,
		ConvInterpreter:       convChain,
		ConvMaxTokens:         convMaxTokens,
		Router:                router,
	}, nil
}

func buildClassChain(
	makeProvider func(valueobjects.ModelSlug) interfaces.LLMProvider,
	newBreaker func() *appservices.CircuitBreaker,
	classPrimary, classFallbacks string,
	globalFallbackSlugs []valueobjects.ModelSlug,
	globalPrimarySlug valueobjects.ModelSlug,
	o11y observability.Observability,
) (usecases.IntentInterpreter, error) {
	if strings.TrimSpace(classPrimary) == "" {
		return buildLLMChain(makeProvider, newBreaker, globalPrimarySlug, globalFallbackSlugs, o11y)
	}
	slug, err := valueobjects.NewModelSlug(classPrimary)
	if err != nil {
		return nil, fmt.Errorf("agent.llm: class primary model %q: %w", classPrimary, err)
	}
	var fallbacks []valueobjects.ModelSlug
	for _, raw := range parseFallbackList(classFallbacks) {
		fs, fsErr := valueobjects.NewModelSlug(raw)
		if fsErr != nil {
			return nil, fmt.Errorf("agent.llm: class fallback model %q: %w", raw, fsErr)
		}
		fallbacks = append(fallbacks, fs)
	}
	if len(fallbacks) == 0 {
		fallbacks = globalFallbackSlugs
	}
	return buildLLMChain(makeProvider, newBreaker, slug, fallbacks, o11y)
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

func resolveParseMaxTokens(parse, global int) int {
	if parse > 0 {
		return parse
	}
	return global
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
